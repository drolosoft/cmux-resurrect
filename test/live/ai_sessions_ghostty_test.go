//go:build live

package live

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// Ghostty reports no terminal device per surface, so save cannot prove which
// pane a process runs in. It still must never hand a conversation running in
// one window to the shells of another. The windows here are the test's own;
// the user's are never touched, and Ghostty saves the front window, so each
// save first brings the right window to the front.

// The helpers below name a window by one of its terminals. Ghostty changes a
// window's own id when it gains or loses tabs ("window-…" becomes
// "tab-group-…"), while a terminal keeps its id for life.

// ghosttyWindowHolding is an AppleScript fragment that runs action on the
// window holding a terminal, as w, and returns what action returns.
func ghosttyWindowHolding(terminalID, action string) string {
	return fmt.Sprintf(`tell application "Ghostty"
  repeat with w in windows
    repeat with term in terminals of w
      if id of term is %q then
        %s
      end if
    end repeat
  end repeat
  return ""
end tell`, terminalID, action)
}

// openGhosttyWindow opens a Ghostty window whose terminal starts in dir,
// registers its teardown at once, and returns that terminal once its shell
// reports a directory.
func openGhosttyWindow(t *testing.T, dir string) string {
	t.Helper()

	terminalID := osa(t, fmt.Sprintf(`tell application "Ghostty"
  set cfg to new surface configuration from {initial working directory:%q}
  set w to new window with configuration cfg
  return id of terminal 1 of w
end tell`, dir))
	if terminalID == "" {
		t.Fatal("Ghostty opened no window")
	}
	t.Cleanup(func() { closeGhosttyTerminal(t, terminalID) })

	waitGhosttyShell(t, terminalID)
	return terminalID
}

// addGhosttyTab opens a tab, its terminal starting in dir, in the window
// holding another terminal, registers its teardown, and returns the new
// terminal once its shell reports a directory.
func addGhosttyTab(t *testing.T, besideTerminal, dir string) string {
	t.Helper()

	terminalID := osa(t, ghosttyWindowHolding(besideTerminal, fmt.Sprintf(`set cfg to new surface configuration from {initial working directory:%q}
        set newTab to new tab in w with configuration cfg
        return id of terminal 1 of newTab`, dir)))
	if terminalID == "" {
		t.Fatalf("no new tab beside Ghostty terminal %s", besideTerminal)
	}
	t.Cleanup(func() { closeGhosttyTerminal(t, terminalID) })

	waitGhosttyShell(t, terminalID)
	return terminalID
}

// waitGhosttyShell waits until a terminal's shell reports its directory,
// which it does from its first prompt on: text typed earlier can be lost.
func waitGhosttyShell(t *testing.T, terminalID string) {
	t.Helper()

	script := fmt.Sprintf(`tell application "Ghostty" to working directory of terminal id %q`, terminalID)
	if !waitFor(60*time.Second, time.Second, func() bool { return osa(t, script) != "" }) {
		t.Fatalf("the shell of Ghostty terminal %s never reported a directory", terminalID)
	}
}

// ghosttyType types one line into a Ghostty terminal and presses Enter.
func ghosttyType(t *testing.T, terminalID, line string) {
	t.Helper()

	osa(t, fmt.Sprintf(`tell application "Ghostty"
  set target to terminal id %q
  input text %q to target
  send key "enter" to target
end tell`, terminalID, line))
}

// ghosttyTerminalExists reports whether Ghostty still has a terminal.
func ghosttyTerminalExists(t *testing.T, terminalID string) bool {
	t.Helper()

	return osa(t, fmt.Sprintf(`tell application "Ghostty" to exists terminal id %q`, terminalID)) == "true"
}

// bringGhosttyWindowToFront activates the window holding a terminal and waits
// until Ghostty lists it first: a Ghostty save captures that window.
func bringGhosttyWindowToFront(t *testing.T, terminalID string) {
	t.Helper()

	inFront := fmt.Sprintf(`tell application "Ghostty"
  repeat with term in terminals of window 1
    if id of term is %q then return "yes"
  end repeat
  return "no"
end tell`, terminalID)
	if !waitFor(20*time.Second, time.Second, func() bool {
		osa(t, ghosttyWindowHolding(terminalID, `activate window w
        return "ok"`))
		return osa(t, inFront) == "yes"
	}) {
		t.Fatalf("the Ghostty window of terminal %s never came to the front", terminalID)
	}
}

// saveGhosttyWindow saves the window holding a terminal under name and
// returns crex's output and the layout. Ghostty saves its front window, and
// with no caller to go by, a focus change between bringing the window forward
// and crex reading the windows makes it save another one. That is retried,
// and logged, until the layout holds the window's tabs: tabs counts them and
// dir is the directory one of them must be in.
func saveGhosttyWindow(t *testing.T, layouts, name, terminalID string, tabs int, dir string) (string, *model.Layout) {
	t.Helper()

	env := crexEnv("CREX_BACKEND=ghostty", "CREX_DEBUG=1")
	for attempt := 1; attempt <= 3; attempt++ {
		bringGhosttyWindowToFront(t, terminalID)
		out, err := runCrex(t, layouts, env, "save", name)
		if err != nil {
			t.Fatalf("save %s: %v", name, err)
		}

		saved := loadLayout(t, layouts, name)
		if len(saved.Workspaces) == tabs && slices.ContainsFunc(saved.Workspaces, func(ws model.Workspace) bool {
			return slotDir(ws.CWD, "") == dir
		}) {
			return out, saved
		}
		t.Logf("attempt %d: save %s captured another window (%d tabs), the front window changed", attempt, name, len(saved.Workspaces))
	}
	t.Fatalf("save %s never captured the window of terminal %s:\n%s", name, terminalID, layoutText(layouts, name))
	return "", nil
}

// closeGhosttyTerminal ends what runs in a terminal with Ctrl-C and closes
// it; a window goes with its last terminal. Closing terminals one by one,
// each idle at its prompt, is what keeps Ghostty from asking: closing a whole
// window of tabs opens a confirmation sheet that nobody answers and leaves the
// window behind (confirm-close-surface).
func closeGhosttyTerminal(t *testing.T, terminalID string) {
	t.Helper()

	if !ghosttyTerminalExists(t, terminalID) {
		return
	}
	osa(t, fmt.Sprintf(`tell application "Ghostty" to send key "c" modifiers "control" to terminal id %q`, terminalID))
	time.Sleep(500 * time.Millisecond)

	osa(t, fmt.Sprintf(`tell application "Ghostty" to close (terminal id %q)`, terminalID))
	if !waitFor(15*time.Second, 500*time.Millisecond, func() bool { return !ghosttyTerminalExists(t, terminalID) }) {
		t.Errorf("Ghostty terminal %s is still open after the test closed it", terminalID)
	}
}

// claudeInDir returns the live Claude Code process working in dir, spelled
// with symlinks resolved as lsof reports it. Ghostty shows no terminal device
// to look the process up by, and each test gives its conversation a folder of
// its own.
func claudeInDir(t *testing.T, dir string) string {
	t.Helper()

	for _, line := range strings.Split(sh(t, "ps -axo pid=,comm="), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[1]) != "claude" {
			continue
		}
		cwd := sh(t, "lsof -a -p "+fields[0]+" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p'")
		if cwd == dir {
			return fields[0]
		}
	}
	return ""
}

// conversationAsking returns the conversation of dir whose transcript holds
// the user prompt with marker, or "". Outside cmux Claude Code writes no
// session file naming its conversation, so the transcript is the record.
func conversationAsking(dir, marker string) string {
	transcripts, _ := filepath.Glob(filepath.Join(claudeProject(dir), "*.jsonl"))
	for _, transcript := range transcripts {
		if transcriptHas(transcript, "user", marker) {
			return strings.TrimSuffix(filepath.Base(transcript), ".jsonl")
		}
	}
	return ""
}

// startGhosttyClaude starts Claude Code on a Ghostty terminal, typing a cd to
// typedDir, and has one exchange with it, as startClaude does on cmux. dir is
// the same folder spelled the way lsof and Claude record it (symlinks
// resolved), since the process and its transcript are found by it.
func startGhosttyClaude(t *testing.T, terminalID, typedDir, dir, marker string) claudeRun {
	t.Helper()

	t.Cleanup(func() { forgetClaudeProject(t, dir) })
	ghosttyType(t, terminalID, fmt.Sprintf("cd %s && ANTHROPIC_MODEL=%s claude", shellQuoted(typedDir), claudeModel))

	run := claudeRun{marker: marker}
	waitFor(90*time.Second, time.Second, func() bool {
		run.pid = claudeInDir(t, dir)
		return run.pid != ""
	})
	if run.pid == "" {
		t.Fatalf("Claude Code never started in %s on Ghostty terminal %s", dir, terminalID)
	}
	t.Cleanup(func() { stopClaude(t, run) })

	run.sessionID = askClaude(t, marker,
		func(prompt string) { ghosttyType(t, terminalID, prompt) },
		func() string { return conversationAsking(dir, marker) })
	return run
}

// TestGhostty_AISessionStaysInItsWindow is the cmux scenario on Ghostty:
// window A runs a Claude conversation; window B holds a plain shell in the
// same directory, a shell in a path containing "claude-501" and a shell whose
// title says "claude notes". Ghostty reports no terminals, so nothing proves
// where the conversation runs, but its directory is open in window A, and
// saving B must store no Claude command. Saving A must still store the
// conversation's resume on its single pane.
func TestGhostty_AISessionStaysInItsWindow(t *testing.T) {
	requireGhostty(t)
	fenceUserConfig(t)

	shared := realTempDir(t)
	pathTitled := filepath.Join(realTempDir(t), "claude-501", "notes")
	if err := os.MkdirAll(pathTitled, 0o755); err != nil {
		t.Fatal(err)
	}
	named := realTempDir(t)

	windowA := openGhosttyWindow(t, shared)
	run := startGhosttyClaude(t, windowA, shared, shared, newMarker("CREXGOWN"))

	windowB := openGhosttyWindow(t, shared)
	addGhosttyTab(t, windowB, pathTitled)
	namedTerminal := addGhosttyTab(t, windowB, named)
	// A title that is not a path, kept while `cat` waits: the shell would
	// title the terminal with its directory again at its next prompt.
	ghosttyType(t, namedTerminal, `printf '\033]2;claude notes\007'; cat`)
	nameScript := fmt.Sprintf(`tell application "Ghostty" to name of terminal id %q`, namedTerminal)
	if !waitFor(15*time.Second, 500*time.Millisecond, func() bool { return osa(t, nameScript) == "claude notes" }) {
		t.Fatalf("the third shell never took the title %q (it shows %q)", "claude notes", osa(t, nameScript))
	}

	layouts := t.TempDir()
	out, savedB := saveGhosttyWindow(t, layouts, "gown-b", windowB, 3, shared)
	if !strings.Contains(out, run.resumeCommand()) {
		t.Fatalf("save never detected %s, so it could not have been tempted:\n%s", run.resumeCommand(), out)
	}
	requireNoClaudeCommand(t, savedB, run)
	// Window A is still checked after a failure here: the two halves fail
	// for different reasons.
	if t.Failed() {
		t.Logf("evidence for window B.\nsave output:\n%s\nlayout:\n%s", out, layoutText(layouts, "gown-b"))
	}

	out, savedA := saveGhosttyWindow(t, layouts, "gown-a", windowA, 1, shared)
	want := []string{shared + " → " + run.resumeCommand()}
	if got := paneCommands(savedA.Workspaces[0]); !multisetEqual(got, want) {
		t.Fatalf("the Claude pane did not keep its own session.\nwant %v\ngot  %v\nsave output:\n%s\nlayout:\n%s",
			want, got, out, layoutText(layouts, "gown-a"))
	}
}

// TestGhostty_ClaudeInTmpGetsItsSession: on Ghostty the pane's directory comes
// from the shell, /tmp/x, while Claude Code records /private/tmp/x. Without a
// terminal to match on, only the directory ties the two, so the spellings
// must be compared resolved.
func TestGhostty_ClaudeInTmpGetsItsSession(t *testing.T) {
	requireGhostty(t)
	fenceUserConfig(t)

	// /tmp on purpose, unresolved: the spelling is what this test is about.
	dir, err := os.MkdirTemp("/tmp", "crex-audit-gtmp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}

	window := openGhosttyWindow(t, dir)
	run := startGhosttyClaude(t, window, dir, real, newMarker("CREXGTMP"))

	layouts := t.TempDir()
	out, saved := saveGhosttyWindow(t, layouts, "gtmp", window, 1, real)
	want := []string{real + " → " + run.resumeCommand()}
	if got := paneCommands(saved.Workspaces[0]); !multisetEqual(got, want) {
		t.Fatalf("the Claude pane in %s did not get its session.\nwant %v\ngot  %v\nsave output:\n%s\nlayout:\n%s",
			dir, want, got, out, layoutText(layouts, "gtmp"))
	}
}
