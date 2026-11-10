//go:build live

package live

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The scenarios in this file close the loop on a saved AI session: a real
// conversation is saved, its pane is closed, the layout is restored, and the
// restored pane must be back in the same conversation. They also restore it
// once more with auto_accept = ["all"], which must add the tool's flag to the
// command and still start the tool. Each pass is one short exchange with the
// cheapest model, since it spends the user's real quota (Claude) or loads a
// local model (OpenCode on Ollama).

// openCodeModel is the model the OpenCode conversations run on: the local
// Ollama one this machine's OpenCode is configured with, so no account and
// no quota are involved.
const openCodeModel = "ollama/gemma4-opencode"

// openCodeAgent is the agent the OpenCode conversations use. It is named
// explicitly because the configured default agent may not exist.
const openCodeAgent = "build"

// autoAcceptConfig writes a scratch crex config that turns on auto-accept for
// every tool and returns its path.
func autoAcceptConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("auto_accept = [\"all\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// processOn returns the pid and command line of the process on a terminal
// whose program is name, or empty strings while there is none.
func processOn(t *testing.T, tty, name string) (pid, args string) {
	t.Helper()

	return findProcess(sh(t, "ps -t "+tty+" -o pid=,args="), name)
}

// findProcess picks, from ps lines of "<pid> <args>", the first process whose
// program is name.
func findProcess(psOutput, name string) (pid, args string) {
	for _, line := range strings.Split(psOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[1]) != name {
			continue
		}
		return fields[0], strings.Join(fields[1:], " ")
	}
	return "", ""
}

// processInDir returns the pid and command line of a process named name whose
// working directory is dir (symlinks resolved), or empty strings. Ghostty
// shows no terminal device to look a process up by, so each pass gives its
// conversation a folder of its own.
func processInDir(t *testing.T, dir, name string) (pid, args string) {
	t.Helper()

	for _, line := range strings.Split(sh(t, "ps -axo pid=,args="), "\n") {
		candidate, candidateArgs := findProcess(line, name)
		if candidate == "" {
			continue
		}
		cwd := sh(t, "lsof -a -p "+candidate+" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p'")
		if cwd == dir {
			return candidate, candidateArgs
		}
	}
	return "", ""
}

// screenShows reports whether a cmux surface's screen holds text.
func screenShows(t *testing.T, surfaceRef, text string) bool {
	t.Helper()

	screen, _ := cmuxRun(t, "read-screen", "--surface", surfaceRef)
	return strings.Contains(screen, text)
}

// requireArguments fails unless a command line holds every one of words.
func requireArguments(t *testing.T, what, args string, words ...string) {
	t.Helper()

	for _, word := range words {
		if !strings.Contains(args, word) {
			t.Errorf("%s runs %q, which lacks %q", what, args, word)
		}
	}
}

// requireStillRunning fails when the process is gone a few seconds after it
// started: a tool that refuses a flag exits at once.
func requireStillRunning(t *testing.T, what, pid, name string) {
	t.Helper()

	time.Sleep(8 * time.Second)
	if filepath.Base(sh(t, "ps -p "+pid+" -o comm=")) != name {
		t.Errorf("%s (pid %s) exited within seconds of starting", what, pid)
	}
}

// restoredCmuxSurface waits for the workspace a restore brought back under
// title, shows it so its shell spawns and runs its command, and returns it
// with its terminal and that terminal's tty.
func restoredCmuxSurface(t *testing.T, title string) (workspaceRef, surfaceRef, tty string) {
	t.Helper()

	waitFor(60*time.Second, time.Second, func() bool {
		ws := wsByTitle(t, title)
		if ws == nil || len(ws.Panes) == 0 || len(ws.Panes[0].Surfaces) == 0 {
			return false
		}
		workspaceRef, surfaceRef = ws.Ref, ws.Panes[0].Surfaces[0].Ref
		return true
	})
	if surfaceRef == "" {
		t.Fatalf("restore brought back no workspace titled %q", title)
	}
	// Forced: a tool left running in it would make cmux refuse the close.
	t.Cleanup(func() { _, _ = cmuxRun(t, "close-workspace", "--workspace", workspaceRef, "--force") })

	polls := 0
	waitFor(60*time.Second, time.Second, func() bool {
		if polls%10 == 0 {
			_, _ = cmuxRun(t, "select-workspace", "--workspace", workspaceRef)
		}
		polls++
		tty = surfaceTTY(t, surfaceRef)
		return tty != ""
	})
	if tty == "" {
		t.Fatalf("the restored workspace %s never got a terminal", workspaceRef)
	}
	return workspaceRef, surfaceRef, tty
}

// closeCmuxPane ends the tool in a workspace and closes it, as a user closing
// the pane does, and waits until cmux no longer lists it.
func closeCmuxPane(t *testing.T, workspaceRef, pid, name string) {
	t.Helper()

	stopProcess(t, pid, name)

	// cmux refuses to close a workspace while a process runs in it, and a
	// tool's helpers (MCP servers) can outlive it for a moment. --force is
	// what closing the pane by hand does: it ends whatever is left. The
	// close is repeated in case cmux was still busy with the first one.
	attempts := 0
	gone := waitFor(30*time.Second, time.Second, func() bool {
		if attempts%5 == 0 {
			_, _ = cmuxRun(t, "close-workspace", "--workspace", workspaceRef, "--force")
		}
		attempts++
		return wsByRef(t, workspaceRef) == nil
	})
	if !gone {
		t.Fatalf("workspace %s is still open after closing it", workspaceRef)
	}
}

// renameShellWorkspace gives the workspace of a window's shell a fixed title.
// A restore in add mode skips titles already open, and the shell's own title
// changes with each command, so a fixed one keeps it from being restored twice.
func renameShellWorkspace(t *testing.T, windowRef, title string) {
	t.Helper()

	workspaceRef, _ := windowWorkspace(t, windowRef)
	if _, err := cmuxRun(t, "rename-workspace", "--workspace", workspaceRef, title); err != nil {
		t.Fatalf("rename-workspace: %v", err)
	}
}

// TestCmux_ClaudeConversationSurvivesRestore saves a pane with a live Claude
// conversation, closes the pane, restores the layout, and requires the
// restored pane to run `claude --resume` of the same conversation and to show
// its exchange on screen. Restoring it again with auto_accept = ["all"] must
// add --dangerously-skip-permissions, and Claude must start with it.
func TestCmux_ClaudeConversationSurvivesRestore(t *testing.T) {
	requireCmux(t)
	fenceUserConfig(t)

	const prefix = "crex-audit-resume-claude"
	refuseLeftovers(t, prefix, prefix+"-shell")

	dir := realTempDir(t)
	window, shell := openAuditWindow(t)
	renameShellWorkspace(t, window, prefix+"-shell")
	workspaceRef, surface := addAuditWorkspace(t, window, prefix, dir)
	run := startClaude(t, surface, dir, newMarker("CREXRES"))

	layouts := t.TempDir()
	out := crexInWindow(t, shell, layouts, crexConf, "save", "resume")
	saved := workspaceIn(loadLayout(t, layouts, "resume"), prefix)
	want := []string{dir + " → " + run.resumeCommand()}
	if saved == nil || !multisetEqual(paneCommands(*saved), want) {
		t.Fatalf("save did not store the conversation's resume.\nwant %v\nsave output:\n%s\nlayout:\n%s",
			want, out, layoutText(layouts, "resume"))
	}

	closeCmuxPane(t, workspaceRef, run.pid, "claude")
	crexInWindow(t, shell, layouts, crexConf, "restore", "resume", "--mode", "add")
	workspaceRef = requireCmuxClaudeBack(t, prefix, run, "--resume "+run.sessionID)

	t.Run("auto_accept", func(t *testing.T) {
		pid, _ := claudeOnTTY(t, surfaceTTY(t, firstSurface(t, workspaceRef)))
		closeCmuxPane(t, workspaceRef, pid, "claude")
		crexInWindow(t, shell, layouts, autoAcceptConfig(t), "restore", "resume", "--mode", "add")
		requireCmuxClaudeBack(t, prefix, run, "--dangerously-skip-permissions", "--resume "+run.sessionID)
	})
}

// firstSurface returns the first terminal of a workspace.
func firstSurface(t *testing.T, workspaceRef string) string {
	t.Helper()

	ws := wsByRef(t, workspaceRef)
	if ws == nil || len(ws.Panes) == 0 || len(ws.Panes[0].Surfaces) == 0 {
		t.Fatalf("workspace %s has no terminal", workspaceRef)
	}
	return ws.Panes[0].Surfaces[0].Ref
}

// requireCmuxClaudeBack waits for the restored workspace titled title to run
// Claude in the conversation of run, with words on its command line, alive a
// few seconds later and showing the exchange on screen. It returns the
// restored workspace.
func requireCmuxClaudeBack(t *testing.T, title string, run claudeRun, words ...string) string {
	t.Helper()

	workspaceRef, surface, tty := restoredCmuxSurface(t, title)

	var pid, sessionID string
	waitFor(90*time.Second, time.Second, func() bool {
		pid, sessionID = claudeOnTTY(t, tty)
		return pid != ""
	})
	if pid == "" {
		screen, _ := cmuxRun(t, "read-screen", "--surface", surface)
		t.Fatalf("no Claude started in the restored pane.\nprocesses on %s:\n%s\nscreen:\n%s",
			tty, sh(t, "ps -t "+tty+" -o pid=,stat=,args="), screen)
	}
	t.Cleanup(func() { stopProcess(t, pid, "claude") })

	_, args := processOn(t, tty, "claude")
	requireArguments(t, "the restored pane", args, words...)
	if sessionID != run.sessionID {
		t.Errorf("the restored pane is in conversation %s, want %s", sessionID, run.sessionID)
	}
	requireStillRunning(t, "the restored Claude", pid, "claude")
	if !waitFor(60*time.Second, 2*time.Second, func() bool { return screenShows(t, surface, run.marker) }) {
		t.Errorf("the restored pane never showed the exchange (%s) of its conversation", run.marker)
	}
	return workspaceRef
}

// claudeConversationTitle returns the title Claude Code generated for a
// conversation, which it also gives the terminal, or "".
func claudeConversationTitle(sessionID string) string {
	data, err := os.ReadFile(claudeTranscript(sessionID))
	if err != nil {
		return ""
	}

	title := ""
	for _, line := range strings.Split(string(data), "\n") {
		const field = `"aiTitle":"`
		start := strings.Index(line, field)
		if start < 0 {
			continue
		}
		rest := line[start+len(field):]
		if end := strings.Index(rest, `"`); end > 0 {
			title = rest[:end]
		}
	}
	return title
}

// TestGhostty_ClaudeConversationSurvivesRestore is the cmux scenario on
// Ghostty, whose screen the harness cannot read: the restored terminal must
// run `claude --resume` of the same conversation and show its title, which
// Claude Code takes from the conversation it loaded.
func TestGhostty_ClaudeConversationSurvivesRestore(t *testing.T) {
	requireGhostty(t)
	fenceUserConfig(t)

	dir := realTempDir(t)
	anchor := openGhosttyWindow(t, homeDir)
	claudeTerminal := addGhosttyTab(t, anchor, dir)
	run := startGhosttyClaude(t, claudeTerminal, dir, dir, newMarker("CREXGRES"))

	layouts := t.TempDir()
	out, saved := saveGhosttyWindow(t, layouts, "gresume", anchor, 2, dir)
	var found bool
	for _, ws := range saved.Workspaces {
		if multisetEqual(paneCommands(ws), []string{dir + " → " + run.resumeCommand()}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("save did not store the conversation's resume %s:\n%s\nlayout:\n%s", run.resumeCommand(), out, layoutText(layouts, "gresume"))
	}

	stopProcess(t, run.pid, "claude")
	closeGhosttyTerminal(t, claudeTerminal)
	restored := restoreGhosttyTab(t, layouts, crexConf, "gresume", anchor)
	requireGhosttyClaudeBack(t, restored, dir, run, "--resume "+run.sessionID)

	t.Run("auto_accept", func(t *testing.T) {
		pid, _ := processInDir(t, dir, "claude")
		stopProcess(t, pid, "claude")
		closeGhosttyTerminal(t, restored)
		again := restoreGhosttyTab(t, layouts, autoAcceptConfig(t), "gresume", anchor)
		requireGhosttyClaudeBack(t, again, dir, run, "--dangerously-skip-permissions", "--resume "+run.sessionID)
	})
}

// restoreGhosttyTab restores a layout in add mode into the window holding
// anchor, where it brings back the one tab that is no longer open, and
// returns that tab's terminal with its teardown registered.
func restoreGhosttyTab(t *testing.T, layouts, config, name, anchor string) string {
	t.Helper()

	listTerminals := ghosttyWindowHolding(anchor, `set outp to ""
        repeat with term in terminals of w
          set outp to outp & (id of term) & linefeed
        end repeat
        return outp`)
	before := map[string]bool{}
	for _, terminalID := range strings.Fields(osa(t, listTerminals)) {
		before[terminalID] = true
	}

	bringGhosttyWindowToFront(t, anchor)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, crexBin, "--layouts-dir", layouts, "--config", config, "restore", name, "--mode", "add")
	command.Env = crexEnv("CREX_BACKEND=ghostty", "CREX_DEBUG=1")
	out, err := command.CombinedOutput()
	t.Logf("crex restore %s → err=%v\n%s", name, err, out)
	if err != nil {
		t.Fatalf("restore %s: %v", name, err)
	}

	var restored string
	waitFor(30*time.Second, time.Second, func() bool {
		for _, terminalID := range strings.Fields(osa(t, listTerminals)) {
			if !before[terminalID] {
				restored = terminalID
				return true
			}
		}
		return false
	})
	if restored == "" {
		t.Fatalf("restore %s added no terminal to the test's window", name)
	}
	t.Cleanup(func() { closeGhosttyTerminal(t, restored) })
	return restored
}

// requireGhosttyClaudeBack waits for Claude to run in dir in the conversation
// of run, with words on its command line, alive a few seconds later, and for
// the restored terminal to show the conversation's title.
func requireGhosttyClaudeBack(t *testing.T, terminalID, dir string, run claudeRun, words ...string) {
	t.Helper()

	var pid, args string
	waitFor(90*time.Second, time.Second, func() bool {
		pid, args = processInDir(t, dir, "claude")
		return pid != "" && pid != run.pid
	})
	if pid == "" || pid == run.pid {
		t.Fatalf("no Claude started in %s after the restore", dir)
	}
	t.Cleanup(func() { stopProcess(t, pid, "claude") })

	requireArguments(t, "the restored terminal", args, words...)
	requireStillRunning(t, "the restored Claude", pid, "claude")

	title := claudeConversationTitle(run.sessionID)
	if title == "" {
		title = run.marker
	}
	nameScript := fmt.Sprintf(`tell application "Ghostty" to name of terminal id %q`, terminalID)
	if !waitFor(60*time.Second, 2*time.Second, func() bool { return strings.Contains(osa(t, nameScript), title) }) {
		t.Errorf("the restored terminal shows %q, not the conversation's title %q", osa(t, nameScript), title)
	}
}

// openCodeDB is the database OpenCode keeps its sessions in.
func openCodeDB() string {
	return filepath.Join(homeDir, ".local", "share", "opencode", "opencode.db")
}

// openCodeSessionAnswered returns the OpenCode session in dir whose assistant
// reply holds marker, or "".
func openCodeSessionAnswered(t *testing.T, dir, marker string) string {
	t.Helper()

	query := fmt.Sprintf(`SELECT s.id FROM session s
JOIN part p ON p.session_id = s.id
JOIN message m ON m.id = p.message_id
WHERE s.directory = '%s' AND json_extract(m.data, '$.role') = 'assistant' AND p.data LIKE '%%%s%%'
LIMIT 1;`, strings.ReplaceAll(dir, "'", "''"), marker)
	out, err := exec.Command("sqlite3", "-readonly", openCodeDB(), query).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// openCodeSessionsIn lists every OpenCode session recorded for dir.
func openCodeSessionsIn(t *testing.T, dir string) []string {
	t.Helper()

	query := fmt.Sprintf(`SELECT id FROM session WHERE directory = '%s';`, strings.ReplaceAll(dir, "'", "''"))
	out, _ := exec.Command("sqlite3", "-readonly", openCodeDB(), query).Output()
	return strings.Fields(string(out))
}

// forgetOpenCodeSessions deletes, through OpenCode itself, every session it
// recorded for a directory the test created.
func forgetOpenCodeSessions(t *testing.T, dir string) {
	t.Helper()

	for _, id := range openCodeSessionsIn(t, dir) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		command := exec.CommandContext(ctx, "opencode", "session", "delete", id)
		command.Dir = os.TempDir()
		if out, err := command.CombinedOutput(); err != nil {
			t.Logf("opencode session delete %s: %v\n%s", id, err, out)
		}
		cancel()
	}
}

// openCodeStartLine is the command that starts OpenCode in dir with its one
// prompt.
func openCodeStartLine(dir, marker string) string {
	return fmt.Sprintf("cd %s && opencode --agent %s --model %s --prompt %s",
		shellQuoted(dir), openCodeAgent, openCodeModel, shellQuoted("Reply with exactly: "+marker))
}

// waitOpenCodeAnswer waits for OpenCode's reply in dir and returns its session.
func waitOpenCodeAnswer(t *testing.T, dir, marker string) string {
	t.Helper()

	var sessionID string
	waitFor(6*time.Minute, 3*time.Second, func() bool {
		sessionID = openCodeSessionAnswered(t, dir, marker)
		return sessionID != ""
	})
	if sessionID == "" {
		t.Fatalf("OpenCode never replied with %s in %s", marker, dir)
	}
	return sessionID
}

// TestCmux_OpenCodeConversationSurvivesRestore is the Claude scenario with
// OpenCode on a local model: the restored pane must run `opencode --session`
// of the same session and show its exchange.
func TestCmux_OpenCodeConversationSurvivesRestore(t *testing.T) {
	requireCmux(t)
	fenceUserConfig(t)

	const prefix = "crex-audit-resume-opencode"
	refuseLeftovers(t, prefix, prefix+"-shell")

	dir := realTempDir(t)
	t.Cleanup(func() { forgetOpenCodeSessions(t, dir) })
	marker := newMarker("CREXOC")

	window, shell := openAuditWindow(t)
	renameShellWorkspace(t, window, prefix+"-shell")
	workspaceRef, surface := addAuditWorkspace(t, window, prefix, dir)
	tty := surfaceTTY(t, surface)
	typeLine(t, surface, openCodeStartLine(dir, marker))
	sessionID := waitOpenCodeAnswer(t, dir, marker)
	pid, _ := processOn(t, tty, "opencode")
	t.Cleanup(func() { stopProcess(t, pid, "opencode") })
	resume := "opencode --session " + sessionID

	layouts := t.TempDir()
	out := crexInWindow(t, shell, layouts, crexConf, "save", "ocresume")
	saved := workspaceIn(loadLayout(t, layouts, "ocresume"), prefix)
	want := []string{dir + " → " + resume}
	if saved == nil || !multisetEqual(paneCommands(*saved), want) {
		t.Fatalf("save did not store the session's resume.\nwant %v\nsave output:\n%s\nlayout:\n%s",
			want, out, layoutText(layouts, "ocresume"))
	}

	closeCmuxPane(t, workspaceRef, pid, "opencode")
	crexInWindow(t, shell, layouts, crexConf, "restore", "ocresume", "--mode", "add")
	_, restoredSurface, restoredTTY := restoredCmuxSurface(t, prefix)

	var restoredPID, args string
	waitFor(90*time.Second, time.Second, func() bool {
		restoredPID, args = processOn(t, restoredTTY, "opencode")
		return restoredPID != ""
	})
	if restoredPID == "" {
		t.Fatalf("no OpenCode started in the restored pane:\n%s", sh(t, "ps -t "+restoredTTY+" -o pid=,args="))
	}
	t.Cleanup(func() { stopProcess(t, restoredPID, "opencode") })
	requireArguments(t, "the restored pane", args, "--session "+sessionID)
	requireStillRunning(t, "the restored OpenCode", restoredPID, "opencode")
	if !waitFor(90*time.Second, 2*time.Second, func() bool { return screenShows(t, restoredSurface, marker) }) {
		t.Errorf("the restored pane never showed the exchange (%s) of its session", marker)
	}
}

// TestGhostty_OpenCodeConversationSurvivesRestore is the OpenCode scenario on
// Ghostty: the restored terminal must run `opencode --session` of the same
// session and stay up.
func TestGhostty_OpenCodeConversationSurvivesRestore(t *testing.T) {
	requireGhostty(t)
	fenceUserConfig(t)

	dir := realTempDir(t)
	t.Cleanup(func() { forgetOpenCodeSessions(t, dir) })
	marker := newMarker("CREXGOC")

	anchor := openGhosttyWindow(t, homeDir)
	terminal := addGhosttyTab(t, anchor, dir)
	ghosttyType(t, terminal, openCodeStartLine(dir, marker))
	sessionID := waitOpenCodeAnswer(t, dir, marker)
	pid, _ := processInDir(t, dir, "opencode")
	t.Cleanup(func() { stopProcess(t, pid, "opencode") })
	resume := "opencode --session " + sessionID

	layouts := t.TempDir()
	out, saved := saveGhosttyWindow(t, layouts, "gocresume", anchor, 2, dir)
	var found bool
	for _, ws := range saved.Workspaces {
		if multisetEqual(paneCommands(ws), []string{dir + " → " + resume}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("save did not store the session's resume %s:\n%s\nlayout:\n%s", resume, out, layoutText(layouts, "gocresume"))
	}

	stopProcess(t, pid, "opencode")
	closeGhosttyTerminal(t, terminal)
	restoreGhosttyTab(t, layouts, crexConf, "gocresume", anchor)

	var restoredPID, args string
	waitFor(90*time.Second, time.Second, func() bool {
		restoredPID, args = processInDir(t, dir, "opencode")
		return restoredPID != "" && restoredPID != pid
	})
	if restoredPID == "" || restoredPID == pid {
		t.Fatalf("no OpenCode started in %s after the restore", dir)
	}
	t.Cleanup(func() { stopProcess(t, restoredPID, "opencode") })
	requireArguments(t, "the restored terminal", args, "--session "+sessionID)
	requireStillRunning(t, "the restored OpenCode", restoredPID, "opencode")
}
