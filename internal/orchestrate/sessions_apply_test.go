package orchestrate

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/detect"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestApplyDetectedSessions_ReorderedPanesMatchByIndex(t *testing.T) {
	// applySplitGeometry reorders ws.Panes into creation order, so a pane's
	// array position no longer equals its cmux Index. The surface-title map
	// is keyed by Index — looking it up by array position attaches the AI
	// resume command to the WRONG pane (2026-07-11 audit, finding 2).
	//
	// Aside: visual index 2 (full-height right pane, running claude) sits at
	// creation position 1 after the reorder.
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "work",
			CWD:   "/home/u/proj",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0},
				{Type: "terminal", Index: 2, Split: "right"}, // claude pane, creation pos 1
				{Type: "terminal", Index: 1, Split: "down"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "work",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
			{Index: 2, Surfaces: []client.TreeSurface{{Title: "✳ claude — refactor"}}},
		},
	}}
	detected := detect.DetectedSessions{
		ByCWD: map[string][]detect.Session{
			"/home/u/proj": {{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}},
		},
		ByTool: map[string][]detect.Session{
			"claude": {{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}},
		},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	panes := layout.Workspaces[0].Panes
	if got := panes[1].Command; got != "claude --resume abc123" {
		t.Errorf("pane Index 2 (creation pos 1) command = %q, want the claude resume command", got)
	}
	for _, i := range []int{0, 2} {
		if panes[i].Command != "" {
			t.Errorf("pane at pos %d (Index %d) should have no command, got %q", i, panes[i].Index, panes[i].Command)
		}
	}
}

// TestApplyDetectedSessions_ExtraTabsGetTheirOwnSession reproduces the
// workflow from GitHub #8: one workspace, one pane, several TABS, each tab a
// different git worktree of the same project with its own Claude session.
// Detection used to be pane-only, so tabs 2..N came back as bare shells.
func TestApplyDetectedSessions_ExtraTabsGetTheirOwnSession(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj/main",
			Panes: []model.Pane{{
				Type: "terminal", Index: 0, CWD: "/home/u/proj/main",
				Surfaces: []model.Surface{
					{Type: "terminal", CWD: "/home/u/proj/feature-a"},
					{Type: "terminal", CWD: "/home/u/proj/feature-b"},
				},
			}},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{{
			Index: 0,
			Surfaces: []client.TreeSurface{
				{Title: "✳ claude — main"},
				{Title: "✳ claude — feature A"},
				{Title: "✳ claude — feature B"},
			},
		}},
	}}
	mk := func(cwd, id string) detect.Session {
		return detect.Session{Tool: "claude", CWD: cwd, Command: "claude --resume " + id}
	}
	detected := detect.DetectedSessions{
		ByCWD: map[string][]detect.Session{
			"/home/u/proj/main":      {mk("/home/u/proj/main", "aaa")},
			"/home/u/proj/feature-a": {mk("/home/u/proj/feature-a", "bbb")},
			"/home/u/proj/feature-b": {mk("/home/u/proj/feature-b", "ccc")},
		},
		ByTool: map[string][]detect.Session{
			"claude": {
				mk("/home/u/proj/main", "aaa"),
				mk("/home/u/proj/feature-a", "bbb"),
				mk("/home/u/proj/feature-b", "ccc"),
			},
		},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	pane := layout.Workspaces[0].Panes[0]
	if pane.Command != "claude --resume aaa" {
		t.Errorf("pane command = %q, want claude --resume aaa", pane.Command)
	}
	if got := pane.Surfaces[0].Command; got != "claude --resume bbb" {
		t.Errorf("tab 1 command = %q, want claude --resume bbb (its own worktree session)", got)
	}
	if got := pane.Surfaces[1].Command; got != "claude --resume ccc" {
		t.Errorf("tab 2 command = %q, want claude --resume ccc (its own worktree session)", got)
	}
}

// TestApplyDetectedSessions_NoSessionStealingAcrossTabs guards the inverse
// risk: with only ONE running session, exactly one target may claim it.
func TestApplyDetectedSessions_NoSessionStealingAcrossTabs(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj", CWD: "/home/u/proj",
			Panes: []model.Pane{{
				Type: "terminal", Index: 0, CWD: "/home/u/proj",
				Surfaces: []model.Surface{
					{Type: "terminal", CWD: "/home/u/other"},
					{Type: "terminal", CWD: "/home/u/third"},
				},
			}},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{{
			Index: 0,
			Surfaces: []client.TreeSurface{
				{Title: "✳ claude"}, {Title: "✳ claude"}, {Title: "✳ claude"},
			},
		}},
	}}
	one := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume only"}
	detected := detect.DetectedSessions{
		ByCWD:  map[string][]detect.Session{"/home/u/proj": {one}},
		ByTool: map[string][]detect.Session{"claude": {one}},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	pane := layout.Workspaces[0].Panes[0]
	assigned := 0
	for _, c := range []string{pane.Command, pane.Surfaces[0].Command, pane.Surfaces[1].Command} {
		if c == one.Command {
			assigned++
		} else if c != "" {
			t.Errorf("unexpected command %q", c)
		}
	}
	if assigned != 1 {
		t.Errorf("session assigned %d times, want exactly 1", assigned)
	}
}

func TestClearAutoDetectedCommands_ClearsExtraTabs(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			Panes: []model.Pane{{
				Type: "terminal", Command: "claude --resume stale-pane",
				Surfaces: []model.Surface{
					{Type: "terminal", Command: "claude --resume stale-tab"},
					{Type: "terminal", Command: "npm run dev"}, // user command: must survive
				},
			}},
		}},
	}

	clearAutoDetectedCommands(layout)

	pane := layout.Workspaces[0].Panes[0]
	if pane.Command != "" {
		t.Errorf("pane resume command not cleared: %q", pane.Command)
	}
	if pane.Surfaces[0].Command != "" {
		t.Errorf("tab resume command not cleared: %q — stale ids would survive a re-save", pane.Surfaces[0].Command)
	}
	if pane.Surfaces[1].Command != "npm run dev" {
		t.Errorf("user command on tab was wrongly cleared: %q", pane.Surfaces[1].Command)
	}
}

func TestClearAutoDetectedCommands_EnvPrefixedResume(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			Panes: []model.Pane{
				{Type: "terminal", Command: "CLAUDE_CONFIG_DIR=/Users/me/.claude-pro claude --resume stale"},
				{Type: "terminal", Command: "NODE_ENV=dev npm run dev"}, // user command: must survive
			},
		}},
	}

	clearAutoDetectedCommands(layout)

	if got := layout.Workspaces[0].Panes[0].Command; got != "" {
		t.Errorf("env-prefixed resume command not cleared: %q", got)
	}
	if got := layout.Workspaces[0].Panes[1].Command; got != "NODE_ENV=dev npm run dev" {
		t.Errorf("user command wrongly cleared: %q", got)
	}
}

// TestApplyDetectedSessions_SameCWDMatchedByTitle: two Claude sessions run in
// the same directory, in two tabs. Each tab shows its session's title, which
// decides the match — even when the tab listed first would otherwise take
// whichever session was detected first.
func TestApplyDetectedSessions_SameCWDMatchedByTitle(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{
			{Title: "✳ Review code", CWD: "/home/u/proj", Panes: []model.Pane{{Type: "terminal"}}},
			{Title: "⠂ Fix login bug", CWD: "/home/u/proj", Panes: []model.Pane{{Type: "terminal"}}},
		},
	}
	tree := []client.TreeWorkspace{
		{Title: "✳ Review code", Panes: []client.TreePane{{Surfaces: []client.TreeSurface{{Title: "✳ Review code"}}}}},
		{Title: "⠂ Fix login bug", Panes: []client.TreePane{{Surfaces: []client.TreeSurface{{Title: "⠂ Fix login bug"}}}}},
	}
	login := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "CLAUDE_CONFIG_DIR=/home/u/.claude-pro claude --resume login", Title: "Fix login bug"}
	review := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume review", Title: "Review code"}
	detected := detect.DetectedSessions{
		ByCWD:  map[string][]detect.Session{"/home/u/proj": {login, review}},
		ByTool: map[string][]detect.Session{"claude": {login, review}},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	if got := layout.Workspaces[0].Panes[0].Command; got != review.Command {
		t.Errorf("'Review code' tab command = %q, want %q", got, review.Command)
	}
	if got := layout.Workspaces[1].Panes[0].Command; got != login.Command {
		t.Errorf("'Fix login bug' tab command = %q, want %q", got, login.Command)
	}
}

// Pi titles its terminal "π - <session>". The plain "Pi" it used to match
// is inside "api", "pipeline" and "copilot".
func TestTitleMatchLength_Pi(t *testing.T) {
	patterns := detect.TitlePatterns()["pi"]

	for _, title := range []string{"π - crex", "π - refactor the registry"} {
		if titleMatchLength(title, patterns) == 0 {
			t.Errorf("titleMatchLength(%q) = 0 for pi", title)
		}
	}
	for _, title := range []string{"api docs", "deploy pipeline", "GitHub Copilot", "~/Git/pixel"} {
		if got := titleMatchLength(title, patterns); got != 0 {
			t.Errorf("titleMatchLength(%q) = %d for pi, want 0", title, got)
		}
	}
}

// titleTools lists every tool a title names, the longest pattern first and
// ties in registry order, so the pick does not change from one save to the
// next.
func TestTitleTools(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		// CodeBuddy titles its panes with the same ✳ mark as Claude Code.
		{title: "✳ Fix amp timeout", want: []string{"amp", "claude", "codebuddy"}},
		{title: "✳ Claude Code", want: []string{"claude", "codebuddy"}},
		{title: "OpenCode | example", want: []string{"opencode", "amp"}},
		{title: "zsh", want: nil},
		// A shell titles itself with its directory, and a directory can
		// hold a tool's name without the tool running there.
		{title: "/private/tmp/claude-501/scratchpad", want: nil},
		{title: "~/Git/codex-notes", want: nil},
		{title: "txeo@mini: ~/Git/claude-tools", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := titleTools(tt.title); !slices.Equal(got, tt.want) {
				t.Errorf("titleTools(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

// TestApplyDetectedSessions_TitleMatchesTwoTools: a Claude pane titled
// "✳ Fix amp timeout" matches Claude's "✳ " and Amp's "amp". Only Claude has
// a session there. The check used to stop at whichever tool the map handed
// out first, so the pane lost its resume command on some saves and not on
// others. The second pane keeps the workspace out of the cwd-only pass, which
// would otherwise hide the miss.
func TestApplyDetectedSessions_TitleMatchesTwoTools(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0},
				{Type: "terminal", Index: 1, Split: "right"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "✳ Fix amp timeout"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
		},
	}}
	claude := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}
	detected := detect.DetectedSessions{
		ByCWD:  map[string][]detect.Session{"/home/u/proj": {claude}},
		ByTool: map[string][]detect.Session{"claude": {claude}},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	panes := layout.Workspaces[0].Panes
	if panes[0].Command != claude.Command {
		t.Errorf("claude pane command = %q, want %q", panes[0].Command, claude.Command)
	}
	if panes[1].Command != "" {
		t.Errorf("shell pane command = %q, want none", panes[1].Command)
	}
}

// TestApplyDetectedSessions_LongestTitlePatternWins: "OpenCode | example"
// matches OpenCode and, through "example", Amp. Both tools run in the
// directory, so both have a session to give. The longer pattern says more
// about the pane, so OpenCode takes it on every save.
func TestApplyDetectedSessions_LongestTitlePatternWins(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0},
				{Type: "terminal", Index: 1, Split: "right"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "OpenCode | example"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
		},
	}}
	opencode := detect.Session{Tool: "opencode", CWD: "/home/u/proj", Command: "opencode --session ses_1"}
	amp := detect.Session{Tool: "amp", CWD: "/home/u/proj", Command: "amp threads continue T-1"}
	detected := detect.DetectedSessions{
		ByCWD: map[string][]detect.Session{"/home/u/proj": {amp, opencode}},
		ByTool: map[string][]detect.Session{
			"amp":      {amp},
			"opencode": {opencode},
		},
	}

	applyDetectedSessions(layout, tree, nil, detected)

	if got := layout.Workspaces[0].Panes[0].Command; got != opencode.Command {
		t.Errorf("pane command = %q, want %q", got, opencode.Command)
	}
}

// TestApplyDetectedSessions_SameCommandInTwoDirectories: Copilot always
// resumes with "copilot --continue", and a tier-2 tool such as Aider always
// relaunches with its bare name, so two panes in two directories carry the
// same command. A session is the command in its directory: both panes get it.
func TestApplyDetectedSessions_SameCommandInTwoDirectories(t *testing.T) {
	tests := []struct {
		tool    string
		title   string
		command string
	}{
		{tool: "copilot", title: "GitHub Copilot", command: "copilot --continue"},
		{tool: "aider", title: "aider", command: "aider"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			layout := &model.Layout{
				Workspaces: []model.Workspace{{
					Title: "proj",
					CWD:   "/home/u/a",
					Panes: []model.Pane{
						{Type: "terminal", Index: 0, CWD: "/home/u/a"},
						{Type: "terminal", Index: 1, CWD: "/home/u/b", Split: "right"},
					},
				}},
			}
			tree := []client.TreeWorkspace{{
				Title: "proj",
				Panes: []client.TreePane{
					{Index: 0, Surfaces: []client.TreeSurface{{Title: tt.title}}},
					{Index: 1, Surfaces: []client.TreeSurface{{Title: tt.title}}},
				},
			}}
			inA := detect.Session{Tool: tt.tool, CWD: "/home/u/a", Command: tt.command}
			inB := detect.Session{Tool: tt.tool, CWD: "/home/u/b", Command: tt.command}
			detected := detect.DetectedSessions{
				ByCWD: map[string][]detect.Session{
					"/home/u/a": {inA},
					"/home/u/b": {inB},
				},
				ByTool: map[string][]detect.Session{tt.tool: {inA, inB}},
			}

			applyDetectedSessions(layout, tree, nil, detected)

			for i, pane := range layout.Workspaces[0].Panes {
				if pane.Command != tt.command {
					t.Errorf("pane %d (%s) command = %q, want %q", i, pane.CWD, pane.Command, tt.command)
				}
			}
		})
	}
}

// detectedSessions indexes sessions the way detect.AISessions does, keeping
// the order given. ttys maps a session's command to the terminals its process
// runs on; a session left out of it has none.
func detectedSessions(ttys map[string][]string, sessions ...detect.Session) detect.DetectedSessions {
	detected := detect.DetectedSessions{
		ByCWD:  make(map[string][]detect.Session),
		ByTool: make(map[string][]detect.Session),
		TTYs:   make(map[detect.SessionKey][]string),
	}
	for _, s := range sessions {
		detected.ByCWD[s.CWD] = append(detected.ByCWD[s.CWD], s)
		detected.ByTool[s.Tool] = append(detected.ByTool[s.Tool], s)
		if terminals, ok := ttys[s.Command]; ok {
			detected.TTYs[s.Key()] = terminals
		}
	}
	return detected
}

// singlePaneWorkspace returns a saved workspace of one terminal pane in cwd
// and the live tree workspace behind it, titled title on terminal tty.
func singlePaneWorkspace(name, cwd, title, tty string) (model.Workspace, client.TreeWorkspace) {
	saved := model.Workspace{
		Title: name,
		CWD:   cwd,
		Panes: []model.Pane{{Type: "terminal", Index: 0, CWD: cwd}},
	}
	live := client.TreeWorkspace{
		Title: name,
		Panes: []client.TreePane{{Index: 0, Surfaces: []client.TreeSurface{{Title: title, TTY: tty, CWD: cwd}}}},
	}
	return saved, live
}

// commandsOf lists the command of every pane of every workspace in layout.
func commandsOf(layout *model.Layout) []string {
	var commands []string
	for _, ws := range layout.Workspaces {
		for _, pane := range ws.Panes {
			commands = append(commands, pane.Command)
		}
	}
	return commands
}

// TestOtherWindows: AI detection is told about every window but the saved
// one, so it can see where else a session's directory is open.
func TestOtherWindows(t *testing.T) {
	tree := &client.TreeResponse{Windows: []client.TreeWindow{
		{Ref: "window:1", Workspaces: []client.TreeWorkspace{{Title: "mine"}}},
		{Ref: "window:2", Workspaces: []client.TreeWorkspace{{Title: "a"}, {Title: "b"}}},
		{Ref: "window:3", Workspaces: []client.TreeWorkspace{{Title: "c"}}},
	}}

	var titles []string
	for _, ws := range otherWindows(tree, tree.Windows[0]) {
		titles = append(titles, ws.Title)
	}

	if want := []string{"a", "b", "c"}; !slices.Equal(titles, want) {
		t.Errorf("otherWindows = %q, want %q", titles, want)
	}
}

// TestApplyDetectedSessions_PlainShellsGetNoSessionFromAnotherWindow
// reproduces the save of 2026-10-15: two plain shells of a fresh window, in ~
// and in the crex repository, came back with `claude --resume` of three
// conversations running in another window. The cwd-only pass matched them by
// directory alone; their terminals say none of those processes is theirs.
func TestApplyDetectedSessions_PlainShellsGetNoSessionFromAnotherWindow(t *testing.T) {
	home, homeLive := singlePaneWorkspace("home", "/home/u", "zsh", "ttys010")
	crex, crexLive := singlePaneWorkspace("crex", "/home/u/crex", "zsh", "ttys011")
	layout := &model.Layout{Workspaces: []model.Workspace{home, crex}}
	tree := []client.TreeWorkspace{homeLive, crexLive}

	detected := detectedSessions(
		map[string][]string{
			"claude --resume c673208d": {"ttys001"},
			"claude --resume 3773a8f8": {"ttys002"},
			"claude --resume d517100a": {"ttys003"},
		},
		detect.Session{Tool: "claude", CWD: "/home/u", Command: "claude --resume c673208d"},
		detect.Session{Tool: "claude", CWD: "/home/u/crex", Command: "claude --resume 3773a8f8"},
		detect.Session{Tool: "claude", CWD: "/home/u/crex", Command: "claude --resume d517100a"},
	)

	applyDetectedSessions(layout, tree, nil, detected)

	for i, command := range commandsOf(layout) {
		if command != "" {
			t.Errorf("shell %d command = %q, want none: the session runs in another window", i, command)
		}
	}
}

// TestApplyDetectedSessions_ClaudeTitledShellGetsNoSessionFromAnotherWindow
// reproduces the save of 2026-10-14: a shell whose title was a path inside
// Claude Code's scratchpad (claude-501) took the live auris session of
// another window through the title-only pass.
func TestApplyDetectedSessions_ClaudeTitledShellGetsNoSessionFromAnotherWindow(t *testing.T) {
	const scratchpad = "/private/tmp/claude-501/scratchpad"
	auris := detect.Session{Tool: "claude", CWD: "/home/u/auris", Command: "claude --resume auris"}

	t.Run("cmux reports terminals", func(t *testing.T) {
		shell, shellLive := singlePaneWorkspace("scratch", scratchpad, scratchpad, "ttys010")
		layout := &model.Layout{Workspaces: []model.Workspace{shell}}
		detected := detectedSessions(map[string][]string{auris.Command: {"ttys001"}}, auris)

		applyDetectedSessions(layout, []client.TreeWorkspace{shellLive}, nil, detected)

		if got := layout.Workspaces[0].Panes[0].Command; got != "" {
			t.Errorf("shell command = %q, want none", got)
		}
	})

	t.Run("Ghostty, the session shows in another window", func(t *testing.T) {
		shell, shellLive := singlePaneWorkspace("scratch", scratchpad, scratchpad, "")
		_, aurisLive := singlePaneWorkspace("auris", auris.CWD, "✳ Claude Code", "")
		layout := &model.Layout{Workspaces: []model.Workspace{shell}}
		detected := detectedSessions(nil, auris)

		applyDetectedSessions(layout, []client.TreeWorkspace{shellLive}, []client.TreeWorkspace{aurisLive}, detected)

		if got := layout.Workspaces[0].Panes[0].Command; got != "" {
			t.Errorf("shell command = %q, want none", got)
		}
	})

	// The session can run where crex sees no window at all, in another
	// terminal app. The shell's title is its directory, which names no tool.
	t.Run("Ghostty, the session shows in no window", func(t *testing.T) {
		shell, shellLive := singlePaneWorkspace("scratch", scratchpad, scratchpad, "")
		layout := &model.Layout{Workspaces: []model.Workspace{shell}}
		detected := detectedSessions(nil, auris)

		applyDetectedSessions(layout, []client.TreeWorkspace{shellLive}, nil, detected)

		if got := layout.Workspaces[0].Panes[0].Command; got != "" {
			t.Errorf("shell command = %q, want none", got)
		}
	})
}

// TestApplyDetectedSessions_SymlinkedDirectory reproduces the other case of
// 2026-10-14: a Claude pane in /tmp/x missed its own session, which lsof
// reports under /private/tmp/x, and the title-only pass gave it another
// window's session instead. The directories are compared with symlinks
// resolved.
func TestApplyDetectedSessions_SymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}

	// Two panes keep the workspace out of the cwd-only pass, so the title
	// passes alone decide.
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "x",
			CWD:   link,
			Panes: []model.Pane{
				{Type: "terminal", Index: 0, CWD: link},
				{Type: "terminal", Index: 1, CWD: link, Split: "right"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "x",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "✳ Claude Code"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh"}}},
		},
	}}
	other := detect.Session{Tool: "claude", CWD: "/home/u/auris", Command: "claude --resume other"}
	mine := detect.Session{Tool: "claude", CWD: real, Command: "claude --resume mine"}
	detected := detectedSessions(nil, other, mine)

	applyDetectedSessions(layout, tree, nil, detected)

	if got := layout.Workspaces[0].Panes[0].Command; got != mine.Command {
		t.Errorf("claude pane command = %q, want %q", got, mine.Command)
	}
}

// TestApplyDetectedSessions_TerminalPicksThePaneSession: two Claude panes in
// one directory, with no session title to tell them apart. Each process's
// terminal says which pane it runs in, whatever order detection found them.
func TestApplyDetectedSessions_TerminalPicksThePaneSession(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0},
				{Type: "terminal", Index: 1, Split: "right"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "✳ Claude Code", TTY: "/dev/ttys005"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "✳ Claude Code", TTY: "/dev/ttys004"}}},
		},
	}}
	onFour := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume four"}
	onFive := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume five"}
	detected := detectedSessions(
		map[string][]string{onFour.Command: {"ttys004"}, onFive.Command: {"ttys005"}},
		onFour, onFive,
	)

	applyDetectedSessions(layout, tree, nil, detected)

	want := []string{onFive.Command, onFour.Command}
	if got := commandsOf(layout); !slices.Equal(got, want) {
		t.Errorf("pane commands = %q, want %q", got, want)
	}
}

// TestApplyDetectedSessions_TerminalBeatsAnotherWindowInTitleOnlyPass: the
// pane's directory does not match its session's (it was read before a cd),
// so the title-only pass decides. It must take the session on the pane's own
// terminal, not the first Claude session detected anywhere.
func TestApplyDetectedSessions_TerminalBeatsAnotherWindowInTitleOnlyPass(t *testing.T) {
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			CWD:   "/home/u/proj/sub",
			Panes: []model.Pane{
				{Type: "terminal", Index: 0, CWD: "/home/u/proj/sub"},
				{Type: "terminal", Index: 1, CWD: "/home/u/proj/sub", Split: "right"},
			},
		}},
	}
	tree := []client.TreeWorkspace{{
		Title: "proj",
		Panes: []client.TreePane{
			{Index: 0, Surfaces: []client.TreeSurface{{Title: "✳ Claude Code", TTY: "ttys005"}}},
			{Index: 1, Surfaces: []client.TreeSurface{{Title: "zsh", TTY: "ttys006"}}},
		},
	}}
	other := detect.Session{Tool: "claude", CWD: "/home/u/auris", Command: "claude --resume other"}
	mine := detect.Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume mine"}
	detected := detectedSessions(
		map[string][]string{other.Command: {"ttys001"}, mine.Command: {"ttys005"}},
		other, mine,
	)

	applyDetectedSessions(layout, tree, nil, detected)

	want := []string{mine.Command, ""}
	if got := commandsOf(layout); !slices.Equal(got, want) {
		t.Errorf("pane commands = %q, want %q", got, want)
	}
}

// TestApplyDetectedSessions_GhosttyGuessesOnlyWhatNoOtherWindowShows: Ghostty
// reports no terminals, so the guessing passes (title only, cwd only) cannot
// prove a session is the pane's. They still guess, as before, but never with
// a session whose directory is open in another window, where it most likely
// runs.
func TestApplyDetectedSessions_GhosttyGuessesOnlyWhatNoOtherWindowShows(t *testing.T) {
	_, elsewhere := singlePaneWorkspace("crex", "/home/u/crex", "✳ Claude Code", "")

	t.Run("cwd-only pass", func(t *testing.T) {
		session := detect.Session{Tool: "codex", CWD: "/home/u/crex", Command: "codex resume thread-1"}
		for _, tt := range []struct {
			name   string
			others []client.TreeWorkspace
			want   string
		}{
			{name: "open in another window", others: []client.TreeWorkspace{elsewhere}, want: ""},
			{name: "open nowhere else", others: nil, want: session.Command},
		} {
			t.Run(tt.name, func(t *testing.T) {
				shell, shellLive := singlePaneWorkspace("crex", "/home/u/crex", "zsh", "")
				layout := &model.Layout{Workspaces: []model.Workspace{shell}}

				applyDetectedSessions(layout, []client.TreeWorkspace{shellLive}, tt.others, detectedSessions(nil, session))

				if got := layout.Workspaces[0].Panes[0].Command; got != tt.want {
					t.Errorf("shell command = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("title-only pass", func(t *testing.T) {
		session := detect.Session{Tool: "claude", CWD: "/home/u/crex", Command: "claude --resume crex"}
		for _, tt := range []struct {
			name   string
			others []client.TreeWorkspace
			want   string
		}{
			{name: "open in another window", others: []client.TreeWorkspace{elsewhere}, want: ""},
			{name: "open nowhere else", others: nil, want: session.Command},
		} {
			t.Run(tt.name, func(t *testing.T) {
				pane, paneLive := singlePaneWorkspace("notes", "/home/u/notes", "✳ Claude Code", "")
				layout := &model.Layout{Workspaces: []model.Workspace{pane}}

				applyDetectedSessions(layout, []client.TreeWorkspace{paneLive}, tt.others, detectedSessions(nil, session))

				if got := layout.Workspaces[0].Panes[0].Command; got != tt.want {
					t.Errorf("pane command = %q, want %q", got, tt.want)
				}
			})
		}
	})
}
