package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootVersionFlag(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("crex --version: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, Version) {
		t.Errorf("--version output = %q, want it to contain version %q", out, Version)
	}
	if !strings.Contains(out, Commit) {
		t.Errorf("--version output = %q, want it to contain commit %q", out, Commit)
	}
}

func TestRootVersionFlagShorthand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"-v"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("crex -v: %v", err)
	}
	if !strings.Contains(buf.String(), Version) {
		t.Errorf("-v output = %q, want it to contain version %q", buf.String(), Version)
	}
}

func TestNormalizeSingleDashVersion(t *testing.T) {
	got := normalizeSingleDashVersion([]string{"-version"})
	if len(got) != 1 || got[0] != "--version" {
		t.Errorf("normalize(-version) = %v, want [--version]", got)
	}
	// Anything else passes through untouched — including flag values that
	// merely contain the word.
	passthrough := []string{"save", "-v", "--version", "my-version-layout"}
	got = normalizeSingleDashVersion(passthrough)
	for i, a := range passthrough {
		if got[i] != a {
			t.Errorf("normalize(%q) = %q, want unchanged", a, got[i])
		}
	}
}
