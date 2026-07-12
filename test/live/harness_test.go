//go:build live

// Package live is the crex live audit harness: end-to-end tests that drive
// the real backends (cmux and Ghostty) and assert on ground truth — pane
// pixel frames and debug.terminals on cmux, shell PIDs and lsof cwds on
// Ghostty. Run with:
//
//	make audit            # both backends (fails if either is missing)
//	make audit-cmux       # cmux matrix only
//	make audit-ghostty    # Ghostty matrix only
//
// The harness builds crex from the working tree, uses a throwaway layouts
// dir per test, and cleans up every workspace/tab it creates. It adds
// workspaces/tabs to your running session while it works.
//
// Set CREX_AUDIT_SKIP_MISSING=1 to skip (instead of fail) tests whose
// backend is not available.
package live

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	crexBin  string
	repoRoot string
	homeDir  string
	crexConf string // isolated empty config — the audit must not inherit the user's settings (e.g. a pinned default backend)
)

func TestMain(m *testing.M) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot = filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	var err error
	homeDir, err = os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit: cannot resolve home dir:", err)
		os.Exit(1)
	}

	tmp, err := os.MkdirTemp("", "crex-audit-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		os.Exit(1)
	}
	crexBin = filepath.Join(tmp, "crex")
	crexConf = filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(crexConf, []byte("# isolated audit config — defaults only\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "audit:", err)
		os.Exit(1)
	}
	build := exec.Command("go", "build", "-o", crexBin, "./cmd/crex")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "audit: building crex from working tree failed: %v\n%s", err, out)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// backendMissing fails the test (the audit requires both backends) unless
// CREX_AUDIT_SKIP_MISSING=1 downgrades it to a skip.
func backendMissing(t *testing.T, name, hint string) {
	t.Helper()
	msg := fmt.Sprintf("backend %s not available — the audit requires BOTH backends (%s)", name, hint)
	if os.Getenv("CREX_AUDIT_SKIP_MISSING") == "1" {
		t.Skip(msg)
	}
	t.Fatal(msg)
}

// crexEnv builds the child environment: the current env minus every CMUX_*
// and CREX_* var, plus the given overrides ("K=V").
func crexEnv(overrides ...string) []string {
	env := []string{}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CMUX_") || strings.HasPrefix(kv, "CREX_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, overrides...)
}

// runCrex runs the freshly built crex with the given layouts dir and env
// overrides, returning combined output.
func runCrex(t *testing.T, layoutsDir string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, crexBin, append([]string{"--layouts-dir", layoutsDir, "--config", crexConf}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	t.Logf("crex %s → err=%v\n%s", strings.Join(args, " "), err, out)
	return string(out), err
}

// sh runs a shell command and returns trimmed stdout.
func sh(t *testing.T, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "/bin/sh", "-c", script).Output()
	return strings.TrimSpace(string(out))
}

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(timeout, interval time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(interval)
	}
	return cond()
}

// multisetEqual reports whether two string slices are equal ignoring order.
func multisetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	as, bs := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	for i := range as {
		if as[i] != bs[i] {
			return false
		}
	}
	return true
}

// canonPath normalizes /private/tmp vs /tmp so lsof output compares cleanly.
func canonPath(p string) string {
	return strings.Replace(p, "/private/tmp", "/tmp", 1)
}

// installDemo copies the bundled demo layout into the given layouts dir so
// the audit exercises the exact demo.toml shipped from the working tree.
func installDemo(t *testing.T, layoutsDir string) {
	t.Helper()
	src := filepath.Join(repoRoot, "internal", "demo", "demo.toml")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading bundled demo layout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(layoutsDir, "demo.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// demoQuadCWDs is the expected per-pane cwd multiset of the 📁 files
// workspace in the bundled demo layout.
func demoQuadCWDs() []string {
	return []string{
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Downloads"),
		homeDir,
		filepath.Join(homeDir, "Desktop"),
	}
}
