//go:build live

package live

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// The scenarios in this file prove on the real cmux that save gives an AI
// session only to the pane its process runs in, never to a shell of another
// window that happens to share its directory or to name the tool in its
// title. Each one starts real Claude Code conversations, because only a
// conversation with a transcript has a session to resume. They spend the
// user's real quota, so every conversation is one short exchange with the
// cheapest model.

// claudeModel is the model the conversations run on. It goes through the
// environment instead of --model so the pane runs a plain `claude`, the
// command line a user types.
const claudeModel = "haiku"

// claudeSessionID matches the UUID Claude Code names a conversation with.
// Cleanup deletes files by that name, so nothing else may ever pass for one.
var claudeSessionID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// claudeRun is a live Claude Code conversation a test started.
type claudeRun struct {
	pid       string // the claude process
	sessionID string // the conversation, which names its transcript file
	marker    string // the word its single exchange asked Claude to reply with
}

// resumeCommand is what save must store for the pane the conversation runs in.
func (run claudeRun) resumeCommand() string {
	return "claude --resume " + run.sessionID
}

// fenceUserConfig fails the test when anything under ~/.config/crex changed
// while it ran. The shells of the audit windows cannot be kept away from the
// user's real crex (cmux hands them no environment of ours), so the scenarios
// type only the binary under test with a scratch layouts dir and config, and
// this fence proves that nothing reached the real one.
func fenceUserConfig(t *testing.T) {
	t.Helper()

	before := userConfigDigest(t)
	t.Cleanup(func() {
		if after := userConfigDigest(t); after != before {
			t.Errorf("~/.config/crex changed while the test ran: a crex call reached the user's real config")
		}
	})
}

// userConfigDigest hashes the name and content of every file under
// ~/.config/crex.
func userConfigDigest(t *testing.T) string {
	t.Helper()

	hash := sha256.New()
	root := filepath.Join(homeDir, ".config", "crex")
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		// A hash never fails to take bytes.
		_, _ = fmt.Fprintf(hash, "%s %d\n", path, len(data))
		hash.Write(data)
		return nil
	})

	return hex.EncodeToString(hash.Sum(nil))
}

// realTempDir returns a fresh temporary directory with symlinks resolved,
// which is how lsof and Claude Code spell the directory of a process.
func realTempDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// newMarker returns a word no earlier run can have produced, so finding it
// in a transcript or on a screen proves this run's exchange.
func newMarker(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1_000_000_000)
}

// shellQuoted single-quotes a word for the shell it is typed into.
func shellQuoted(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// typeLine types one line into a cmux surface and presses Enter.
func typeLine(t *testing.T, surfaceRef, line string) {
	t.Helper()

	if _, err := cmuxRun(t, "send", "--surface", surfaceRef, line); err != nil {
		t.Fatalf("send to %s: %v", surfaceRef, err)
	}
	if _, err := cmuxRun(t, "send-key", "--surface", surfaceRef, "enter"); err != nil {
		t.Fatalf("send-key to %s: %v", surfaceRef, err)
	}
}

// addAuditWorkspace opens a workspace titled title in a window, with its shell
// in dir, and returns it with its terminal once that shell takes input. cmux
// spawns a shell only when its workspace is shown, so it is selected until
// then. The window's own teardown closes it.
func addAuditWorkspace(t *testing.T, windowRef, title, dir string) (workspaceRef, surfaceRef string) {
	t.Helper()

	out, err := cmuxRun(t, "new-workspace", "--window", windowRef, "--name", title, "--cwd", dir)
	fields := strings.Fields(out)
	if err != nil || len(fields) < 2 {
		t.Fatalf("new-workspace %q: %v (%q)", title, err, out)
	}
	workspaceRef = fields[1]

	waitFor(30*time.Second, 500*time.Millisecond, func() bool {
		ws := wsByRef(t, workspaceRef)
		if ws == nil || len(ws.Panes) == 0 || len(ws.Panes[0].Surfaces) == 0 {
			return false
		}
		surfaceRef = ws.Panes[0].Surfaces[0].Ref
		return true
	})
	if surfaceRef == "" {
		t.Fatalf("workspace %s (%q) never showed a terminal", workspaceRef, title)
	}

	waitShellReady(t, workspaceRef, surfaceRef)
	return workspaceRef, surfaceRef
}

// splitAuditPane splits a pane to the right and returns the new terminal
// once its shell takes input.
func splitAuditPane(t *testing.T, workspaceRef, surfaceRef string) string {
	t.Helper()

	before := map[string]bool{}
	for _, ref := range surfaceRefs(wsByRef(t, workspaceRef)) {
		before[ref] = true
	}
	if _, err := cmuxRun(t, "new-split", "right", "--workspace", workspaceRef, "--surface", surfaceRef); err != nil {
		t.Fatalf("new-split: %v", err)
	}

	var created string
	waitFor(30*time.Second, 500*time.Millisecond, func() bool {
		for _, ref := range surfaceRefs(wsByRef(t, workspaceRef)) {
			if !before[ref] {
				created = ref
				return true
			}
		}
		return false
	})
	if created == "" {
		t.Fatalf("the split of %s never showed a terminal", workspaceRef)
	}

	waitShellReady(t, workspaceRef, created)
	return created
}

// waitShellReady selects a workspace until the shell behind a surface takes
// input. A workspace nobody looks at has no shell, and text sent to it is lost.
func waitShellReady(t *testing.T, workspaceRef, surfaceRef string) {
	t.Helper()

	polls := 0
	ready := waitFor(60*time.Second, time.Second, func() bool {
		if polls%10 == 0 {
			_, _ = cmuxRun(t, "select-workspace", "--workspace", workspaceRef)
		}
		polls++
		_, readySet := terminals(t)
		return readySet[surfaceRef]
	})
	if !ready {
		t.Fatalf("the shell of %s in %s never became ready", surfaceRef, workspaceRef)
	}
}

// surfaceTTY returns the terminal cmux reports behind a surface ("ttys004"),
// or "" when it reports none.
func surfaceTTY(t *testing.T, surfaceRef string) string {
	t.Helper()

	out, err := cmuxRun(t, "--json", "tree", "--all")
	if err != nil {
		return ""
	}
	var resp struct {
		Windows []struct {
			Workspaces []struct {
				Panes []struct {
					Surfaces []struct {
						Ref string `json:"ref"`
						TTY string `json:"tty"`
					} `json:"surfaces"`
				} `json:"panes"`
			} `json:"workspaces"`
		} `json:"windows"`
	}
	if json.Unmarshal([]byte(out), &resp) != nil {
		return ""
	}

	for _, window := range resp.Windows {
		for _, ws := range window.Workspaces {
			for _, pane := range ws.Panes {
				for _, surface := range pane.Surfaces {
					if surface.Ref == surfaceRef {
						return surface.TTY
					}
				}
			}
		}
	}
	return ""
}

// claudeOnTTY returns the Claude Code process running on a terminal and the
// conversation its session file names, or empty strings while there is none.
func claudeOnTTY(t *testing.T, tty string) (pid, sessionID string) {
	t.Helper()

	for _, line := range strings.Split(sh(t, "ps -t "+tty+" -o pid=,comm="), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || filepath.Base(fields[1]) != "claude" {
			continue
		}
		if id := claudeSessionOf(fields[0]); id != "" {
			return fields[0], id
		}
	}
	return "", ""
}

// claudeSessionOf reads the conversation a Claude Code process records in
// ~/.claude/sessions/<pid>.json, or "" when it records none.
func claudeSessionOf(pid string) string {
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "sessions", pid+".json"))
	if err != nil {
		return ""
	}
	var meta struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(data, &meta) != nil || !claudeSessionID.MatchString(meta.SessionID) {
		return ""
	}
	return meta.SessionID
}

// claudeTranscript returns the transcript file of a conversation, or "" while
// Claude Code has not written one.
func claudeTranscript(sessionID string) string {
	matches, _ := filepath.Glob(filepath.Join(homeDir, ".claude", "projects", "*", sessionID+".jsonl"))
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// transcriptHas reports whether a transcript holds an entry of the given type
// ("user", "assistant") whose message contains text.
func transcriptHas(path, entryType, text string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		var entry struct {
			Type    string          `json:"type"`
			Message json.RawMessage `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Type != entryType {
			continue
		}
		if strings.Contains(string(entry.Message), text) {
			return true
		}
	}
	return false
}

// startClaude starts Claude Code in dir on a surface's shell and has one
// exchange with it. It returns once the reply is in the transcript: before
// that, Claude has no session anyone could resume.
func startClaude(t *testing.T, surfaceRef, dir, marker string) claudeRun {
	t.Helper()

	tty := surfaceTTY(t, surfaceRef)
	if tty == "" {
		t.Fatalf("cmux reports no terminal behind %s", surfaceRef)
	}
	t.Cleanup(func() { forgetClaudeProject(t, dir) })
	typeLine(t, surfaceRef, fmt.Sprintf("cd %s && ANTHROPIC_MODEL=%s claude", shellQuoted(dir), claudeModel))

	run := claudeRun{marker: marker}
	waitFor(90*time.Second, time.Second, func() bool {
		run.pid, run.sessionID = claudeOnTTY(t, tty)
		return run.sessionID != ""
	})
	if run.sessionID == "" {
		t.Fatalf("Claude Code never started on %s (%s)", surfaceRef, tty)
	}
	t.Cleanup(func() { stopClaude(t, run) })

	// Under cmux Claude names its conversation in a session file from the
	// start, so the transcript to watch is known before the prompt.
	asked := func() string {
		if transcriptHas(claudeTranscript(run.sessionID), "user", marker) {
			return run.sessionID
		}
		return ""
	}
	askClaude(t, marker, func(prompt string) { typeLine(t, surfaceRef, prompt) }, asked)
	return run
}

// askClaude types the one prompt of a conversation through send and waits for
// Claude's reply in its transcript. asked returns the conversation whose
// transcript holds the prompt, or "" until there is one. The prompt is typed
// again when Claude never got it. It returns the conversation that answered.
func askClaude(t *testing.T, marker string, send func(prompt string), asked func() string) string {
	t.Helper()

	prompt := "Reply with exactly: " + marker
	for attempt := 1; attempt <= 3; attempt++ {
		// Claude starts a moment before its prompt reads keys, and keys
		// sent in between are lost.
		time.Sleep(3 * time.Second)
		send(prompt)

		var sessionID string
		waitFor(45*time.Second, time.Second, func() bool {
			sessionID = asked()
			return sessionID != ""
		})
		if sessionID == "" {
			t.Logf("attempt %d: Claude did not take the prompt yet", attempt)
			continue
		}

		replied := waitFor(4*time.Minute, 2*time.Second, func() bool {
			return transcriptHas(claudeTranscript(sessionID), "assistant", marker)
		})
		if !replied {
			t.Fatalf("Claude never replied with %s in session %s", marker, sessionID)
		}
		return sessionID
	}
	t.Fatalf("Claude never took the prompt for %s", marker)
	return ""
}

// stopClaude ends a conversation's process and deletes what Claude Code
// stored for it.
func stopClaude(t *testing.T, run claudeRun) {
	t.Helper()

	stopProcess(t, run.pid, "claude")
	forgetClaudeSession(t, run.sessionID)
}

// stopProcess sends SIGTERM to pid when it still runs name, and waits for it
// to go. The name check keeps a reused pid from ever hitting a stranger.
func stopProcess(t *testing.T, pid, name string) {
	t.Helper()

	running := func() bool {
		return filepath.Base(sh(t, "ps -p "+pid+" -o comm=")) == name
	}
	if pid == "" || !running() {
		return
	}

	var number int
	if _, err := fmt.Sscan(pid, &number); err != nil {
		return
	}
	_ = syscall.Kill(number, syscall.SIGTERM)
	if !waitFor(15*time.Second, 500*time.Millisecond, func() bool { return !running() }) {
		_ = syscall.Kill(number, syscall.SIGKILL)
	}
}

// forgetClaudeSession deletes the files Claude Code keeps for a conversation
// the test started: everything named after its id one level below ~/.claude
// (the per-session side folders) and in the project folders. The id is
// checked first, since a bad one would turn the patterns into wildcards.
func forgetClaudeSession(t *testing.T, sessionID string) {
	t.Helper()

	if !claudeSessionID.MatchString(sessionID) {
		return
	}

	for _, pattern := range []string{
		filepath.Join(homeDir, ".claude", "projects", "*", sessionID+"*"),
		filepath.Join(homeDir, ".claude", "*", sessionID+"*"),
	} {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			_ = os.RemoveAll(match)
		}
	}
}

// nonAlphanumeric matches what Claude Code turns into "-" when it names a
// project folder after a directory.
var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// harnessProject matches the project folders Claude Code names after the
// directories this harness creates: a test's temporary directory, or a
// crex-audit folder under /tmp. No other project folder is ever deleted.
var harnessProject = regexp.MustCompile(`-T-Test(Cmux|Ghostty)-[A-Za-z]+[0-9]+-|-tmp-crex-audit-`)

// claudeProject returns the folder where Claude Code keeps the conversations
// of a directory: its real path with every character but letters and digits
// turned into "-".
func claudeProject(dir string) string {
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	return filepath.Join(homeDir, ".claude", "projects", nonAlphanumeric.ReplaceAllString(dir, "-"))
}

// forgetClaudeProject deletes the project folder of a directory the test
// created, with what the user's hooks left in it besides the transcripts.
func forgetClaudeProject(t *testing.T, dir string) {
	t.Helper()

	project := claudeProject(dir)
	if !harnessProject.MatchString(filepath.Base(project)) {
		t.Logf("kept %s: not a folder this harness creates", project)
		return
	}
	_ = os.RemoveAll(project)
}

// paneCommands lists every terminal slot of a saved workspace as
// "<directory> → <command>", tabs included.
func paneCommands(ws model.Workspace) []string {
	var slots []string
	for _, pane := range ws.Panes {
		slots = append(slots, slotDir(pane.CWD, ws.CWD)+" → "+pane.Command)
		for _, surface := range pane.Surfaces {
			slots = append(slots, slotDir(surface.CWD, ws.CWD)+" → "+surface.Command)
		}
	}
	return slots
}

// slotDir is the directory a saved slot runs in, symlinks resolved: its own,
// or its workspace's when it stores none.
func slotDir(dir, workspaceDir string) string {
	if dir == "" {
		dir = workspaceDir
	}
	dir = expandFixtureHome(dir)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		return real
	}
	return dir
}

// layoutText returns a saved layout file for the failure message.
func layoutText(layouts, name string) string {
	data, err := os.ReadFile(filepath.Join(layouts, name+".toml"))
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// requireNoClaudeCommand fails for every slot of a saved layout that would
// start or resume Claude Code, none of which runs there. The message tells the
// other window's conversation apart from a session running anywhere else.
func requireNoClaudeCommand(t *testing.T, saved *model.Layout, otherWindow claudeRun) {
	t.Helper()

	for _, ws := range saved.Workspaces {
		for _, slot := range paneCommands(ws) {
			// Only the command counts: the directory of the slot can hold
			// the word on purpose (claude-501).
			_, command, _ := strings.Cut(slot, " → ")
			switch {
			case strings.Contains(command, otherWindow.sessionID):
				t.Errorf("workspace %q took the conversation of the other window: %s", ws.Title, slot)
			case strings.Contains(command, "claude"):
				t.Errorf("workspace %q took a Claude session that does not run in it: %s", ws.Title, slot)
			}
		}
	}
}

// workspaceIn returns the saved workspace with the given title, or nil.
func workspaceIn(saved *model.Layout, title string) *model.Workspace {
	for i := range saved.Workspaces {
		if saved.Workspaces[i].Title == title {
			return &saved.Workspaces[i]
		}
	}
	return nil
}

// TestCmux_AISessionStaysInItsWindow: window A runs a Claude conversation in a
// directory; window B holds a plain shell in the same directory, a shell in a
// path containing "claude-501" and a tab retitled "claude notes". Saving B
// must store no Claude command at all: the cwd-only pass used to hand the
// plain shell the conversation, and the title-only pass the two others. Saving
// A must still store the conversation's resume on its own pane.
func TestCmux_AISessionStaysInItsWindow(t *testing.T) {
	requireCmux(t)
	fenceUserConfig(t)

	const prefix = "crex-audit-own"
	refuseLeftovers(t, prefix+"-claude", prefix+"-plain", prefix+"-path", prefix+"-named", "claude notes")

	shared := realTempDir(t)
	pathTitled := filepath.Join(realTempDir(t), "claude-501", "notes")
	if err := os.MkdirAll(pathTitled, 0o755); err != nil {
		t.Fatal(err)
	}
	named := realTempDir(t)

	windowA, shellA := openAuditWindow(t)
	_, claudeSurface := addAuditWorkspace(t, windowA, prefix+"-claude", shared)
	run := startClaude(t, claudeSurface, shared, newMarker("CREXOWN"))

	windowB, shellB := openAuditWindow(t)
	addAuditWorkspace(t, windowB, prefix+"-plain", shared)
	addAuditWorkspace(t, windowB, prefix+"-path", pathTitled)
	_, namedSurface := addAuditWorkspace(t, windowB, prefix+"-named", named)
	if _, err := cmuxRun(t, "rename-tab", "--surface", namedSurface, "claude notes"); err != nil {
		t.Fatalf("rename-tab: %v", err)
	}

	layouts := t.TempDir()
	out := crexInWindow(t, shellB, layouts, crexConf, "save", "own-b")
	// The save must have seen the conversation, or an empty result would
	// prove nothing.
	if !strings.Contains(out, run.resumeCommand()) {
		t.Fatalf("save never detected %s, so it could not have been tempted:\n%s", run.resumeCommand(), out)
	}
	savedB := loadLayout(t, layouts, "own-b")
	if workspaceIn(savedB, prefix+"-plain") == nil {
		t.Fatalf("save of window B did not capture %s-plain:\n%s", prefix, layoutText(layouts, "own-b"))
	}
	requireNoClaudeCommand(t, savedB, run)
	// Window A is still checked after a failure here: the two halves fail
	// for different reasons.
	if t.Failed() {
		t.Logf("evidence for window B.\nsave output:\n%s\nlayout:\n%s", out, layoutText(layouts, "own-b"))
	}

	out = crexInWindow(t, shellA, layouts, crexConf, "save", "own-a")
	savedA := loadLayout(t, layouts, "own-a")
	own := workspaceIn(savedA, prefix+"-claude")
	if own == nil {
		t.Fatalf("save of window A did not capture %s-claude:\n%s", prefix, layoutText(layouts, "own-a"))
	}
	want := []string{shared + " → " + run.resumeCommand()}
	if got := paneCommands(*own); !multisetEqual(got, want) {
		t.Fatalf("the Claude pane did not keep its own session.\nwant %v\ngot  %v\nsave output:\n%s\nlayout:\n%s",
			want, got, out, layoutText(layouts, "own-a"))
	}
}

// TestCmux_TwoClaudePanesKeepTheirOwnSessions: one workspace, two panes, each
// with its own Claude conversation in its own directory, the second pane's
// tab titled "✳ amp example". That title names Amp more specifically than
// Claude, as Claude's own "✳ <topic>" titles can, so Amp is tried first and
// must not end the search. Ten saves in a row must each store both resumes,
// each on its own pane: one save that drops or swaps them is the flake this
// guards against.
func TestCmux_TwoClaudePanesKeepTheirOwnSessions(t *testing.T) {
	requireCmux(t)
	fenceUserConfig(t)

	const prefix = "crex-audit-pair"
	refuseLeftovers(t, prefix)

	dirOne := realTempDir(t)
	dirTwo := realTempDir(t)

	window, shell := openAuditWindow(t)
	workspaceRef, first := addAuditWorkspace(t, window, prefix, dirOne)
	second := splitAuditPane(t, workspaceRef, first)
	runOne := startClaude(t, first, dirOne, newMarker("CREXONE"))
	runTwo := startClaude(t, second, dirTwo, newMarker("CREXTWO"))
	if _, err := cmuxRun(t, "rename-tab", "--surface", second, "✳ amp example"); err != nil {
		t.Fatalf("rename-tab: %v", err)
	}

	want := []string{
		dirOne + " → " + runOne.resumeCommand(),
		dirTwo + " → " + runTwo.resumeCommand(),
	}
	layouts := t.TempDir()
	const saves = 10
	good := 0
	// Each pane is counted on its own as well, so the report tells a pane
	// that never gets its session from one that loses it now and then.
	kept := make(map[string]int)
	for attempt := 1; attempt <= saves; attempt++ {
		name := fmt.Sprintf("pair-%d", attempt)
		out := crexInWindow(t, shell, layouts, crexConf, "save", name)

		var got []string
		for _, ws := range loadLayout(t, layouts, name).Workspaces {
			if len(ws.Panes) == 2 {
				got = paneCommands(ws)
			}
		}
		for _, slot := range got {
			if slices.Contains(want, slot) {
				kept[slot]++
			}
		}
		if !multisetEqual(got, want) {
			t.Errorf("save %d of %d lost a pane's session.\nwant %v\ngot  %v\nsave output:\n%s\nlayout:\n%s",
				attempt, saves, want, got, out, layoutText(layouts, name))
			continue
		}
		good++
	}
	t.Logf("%d/%d saves kept both sessions on their own panes", good, saves)
	for _, slot := range want {
		t.Logf("%d/%d saves kept %s", kept[slot], saves, slot)
	}
}

// TestCmux_ClaudeInTmpGetsItsSession: Claude Code records a conversation
// started in /tmp/x under /private/tmp/x, the real path, while the shell and
// cmux call the folder /tmp/x. The pane must still get its own session.
func TestCmux_ClaudeInTmpGetsItsSession(t *testing.T) {
	requireCmux(t)
	fenceUserConfig(t)

	const prefix = "crex-audit-tmp"
	refuseLeftovers(t, prefix)

	// /tmp on purpose, unresolved: the spelling is what this test is about.
	dir, err := os.MkdirTemp("/tmp", prefix+"-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	window, shell := openAuditWindow(t)
	_, surface := addAuditWorkspace(t, window, prefix, dir)
	run := startClaude(t, surface, dir, newMarker("CREXTMP"))
	if transcript := claudeTranscript(run.sessionID); !strings.Contains(transcript, "-private-tmp-") {
		t.Logf("Claude stored the conversation at %s, not under the /private/tmp spelling", transcript)
	}

	layouts := t.TempDir()
	out := crexInWindow(t, shell, layouts, crexConf, "save", "tmp")
	own := workspaceIn(loadLayout(t, layouts, "tmp"), prefix)
	if own == nil {
		t.Fatalf("save did not capture %s:\n%s", prefix, layoutText(layouts, "tmp"))
	}
	want := []string{slotDir(dir, "") + " → " + run.resumeCommand()}
	if got := paneCommands(*own); !multisetEqual(got, want) {
		t.Fatalf("the Claude pane in %s did not get its session.\nwant %v\ngot  %v\nsave output:\n%s\nlayout:\n%s",
			dir, want, got, out, layoutText(layouts, "tmp"))
	}
}

// crexInWindow types a crex command into a window's shell, as a user does,
// and returns what it printed. Save and restore act on the window they run
// in, which cmux takes from the connection, so they must run inside it. Only
// the binary under test is typed, always with a scratch layouts dir and
// config, and with CREX_DEBUG=1 so a failure shows what detection saw.
func crexInWindow(t *testing.T, surfaceRef, layouts, config string, args ...string) string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "log")
	done := filepath.Join(dir, "done")
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuoted(arg))
	}
	line := fmt.Sprintf("CREX_DEBUG=1 %s --layouts-dir %s --config %s %s >%s 2>&1; touch %s",
		shellQuoted(crexBin), shellQuoted(layouts), shellQuoted(config), strings.Join(quoted, " "),
		shellQuoted(logPath), shellQuoted(done))
	typeLine(t, surfaceRef, line)

	finished := waitFor(4*time.Minute, time.Second, func() bool {
		_, err := os.Stat(done)
		return err == nil
	})
	log, _ := os.ReadFile(logPath)
	if !finished {
		t.Fatalf("crex %s typed into %s never finished:\n%s", strings.Join(args, " "), surfaceRef, log)
	}
	t.Logf("crex %s:\n%s", strings.Join(args, " "), log)
	return string(log)
}
