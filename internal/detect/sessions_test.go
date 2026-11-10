package detect

import (
	"slices"
	"testing"
)

func TestValidSessionID(t *testing.T) {
	valid := []string{
		"c52c23e5-5cbe-4786-b046-528839201e7a",
		"019dfd00-d9d9-7780-a206-e338595fc436",
		"ses_20590cf55ffe6n9PUYlGssP4HP",
		"simple-id",
		"abc123",
	}
	invalid := []string{
		"foo; rm -rf /",
		"id with spaces",
		"id\nwith\nnewlines",
		"$(whoami)",
		"",
		"id|pipe",
		"id>redirect",
	}
	for _, id := range valid {
		if !validSessionID.MatchString(id) {
			t.Errorf("expected valid: %q", id)
		}
	}
	for _, id := range invalid {
		if validSessionID.MatchString(id) {
			t.Errorf("expected invalid: %q", id)
		}
	}
}

func TestEscapeSQLite(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"simple", "simple"},
		{"/Users/test/project", "/Users/test/project"},
		{"it's", "it''s"},
		{"it''s", "it''''s"},
		{"no'quotes'here", "no''quotes''here"},
	}
	for _, tt := range tests {
		got := escapeSQLite(tt.input)
		if got != tt.want {
			t.Errorf("escapeSQLite(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTitlePatterns(t *testing.T) {
	patterns := TitlePatterns()
	if len(patterns) != len(registry) {
		t.Fatalf("TitlePatterns returned %d tools, registry has %d", len(patterns), len(registry))
	}
	for _, d := range registry {
		p, ok := patterns[d.Name]
		if !ok {
			t.Errorf("missing patterns for tool %q", d.Name)
		}
		if len(p) == 0 {
			t.Errorf("empty patterns for tool %q", d.Name)
		}
	}
}

// The save flow tries tools in this order when a title matches several, so it
// has to be the registry's, every time.
func TestOrderedTitlePatterns(t *testing.T) {
	ordered := OrderedTitlePatterns()
	if len(ordered) != len(registry) {
		t.Fatalf("OrderedTitlePatterns returned %d tools, registry has %d", len(ordered), len(registry))
	}
	for i, d := range registry {
		if ordered[i].Tool != d.Name {
			t.Errorf("position %d: tool %q, want %q", i, ordered[i].Tool, d.Name)
		}
		if !slices.Equal(ordered[i].Patterns, d.TitlePatterns) {
			t.Errorf("%s: patterns %q, want %q", d.Name, ordered[i].Patterns, d.TitlePatterns)
		}
	}
}

func TestBatchCWDs_EmptyInput(t *testing.T) {
	result := batchCWDs(nil)
	if len(result) != 0 {
		t.Error("expected empty map for nil input")
	}
}

func TestRegistryNames(t *testing.T) {
	names := make(map[string]bool)
	processOwners := make(map[string]string)
	for _, d := range registry {
		if names[d.Name] {
			t.Errorf("duplicate tool name: %q", d.Name)
		}
		names[d.Name] = true

		if len(d.ProcessNames) == 0 {
			t.Errorf("no process names for %q", d.Name)
		}
		for _, process := range d.ProcessNames {
			if owner, taken := processOwners[process]; taken {
				t.Errorf("process name %q belongs to both %q and %q", process, owner, d.Name)
			}
			processOwners[process] = d.Name
		}

		if d.Detect == nil {
			t.Errorf("nil Detect func for %q", d.Name)
		}
		if len(d.TitlePatterns) == 0 {
			t.Errorf("no title patterns for %q", d.Name)
		}
	}
}

// registryRow is what the registry promises about one tool: the processes
// crex recognises it by, what a restore types to start it fresh and the flag
// auto_accept adds.
type registryRow struct {
	processNames   []string
	launchCommand  string
	autoAcceptFlag string
}

// wantRegistry pins every registered tool. A change to any of these values
// changes what crex types into users' terminals, so it has to show up here.
var wantRegistry = map[string]registryRow{
	"claude":      {[]string{"claude"}, "claude", "--dangerously-skip-permissions"},
	"opencode":    {[]string{"opencode"}, "opencode", "--auto"},
	"codex":       {[]string{"codex"}, "codex", "--dangerously-bypass-approvals-and-sandbox"},
	"amp":         {[]string{"amp"}, "amp", "--dangerously-allow-all"},
	"gemini":      {[]string{"gemini"}, "gemini", "--approval-mode=yolo"},
	"copilot":     {[]string{"copilot"}, "copilot", "--allow-all"},
	"grok":        {[]string{"grok"}, "grok", "--always-approve"},
	"cursor":      {[]string{"agent", "cursor-agent"}, "agent", "--force"},
	"aider":       {[]string{"aider"}, "aider", "--yes-always"},
	"pi":          {[]string{"pi"}, "pi", ""},
	"rovo":        {[]string{"atlassian_cli_rovodev"}, "acli rovodev run", "--yolo"},
	"hermes":      {[]string{"hermes"}, "hermes", "--yolo"},
	"codebuddy":   {[]string{"codebuddy", "cbc"}, "codebuddy", "--dangerously-skip-permissions"},
	"factory":     {[]string{"droid"}, "droid", "--skip-permissions-unsafe"},
	"qoder":       {[]string{"qoder", "qodercli"}, "qoder", "--yolo"},
	"qwen":        {[]string{"qwen"}, "qwen", "--yolo"},
	"goose":       {[]string{"goose"}, "goose", ""},
	"crush":       {[]string{"crush"}, "crush", "--yolo"},
	"kiro":        {[]string{"kiro-cli", "kiro-cli-chat"}, "kiro-cli", "--trust-all-tools"},
	"antigravity": {[]string{"agy"}, "agy", "--dangerously-skip-permissions"},
	"cline":       {[]string{"cline", ".cline"}, "cline", ""},
	"vibe":        {[]string{"vibe-rs", "vibe"}, "vibe", "--auto-approve"},
	"omp":         {[]string{"omp"}, "omp", "--yolo"},
	"kimi":        {[]string{"kimi", "kimi-code"}, "kimi", "--auto"},
}

func TestRegistryTable(t *testing.T) {
	if len(registry) != len(wantRegistry) {
		t.Errorf("registry has %d tools, the table pins %d", len(registry), len(wantRegistry))
	}

	flags := AutoAcceptFlags()
	for _, d := range registry {
		want, pinned := wantRegistry[d.Name]
		if !pinned {
			t.Errorf("tool %q is not pinned in wantRegistry", d.Name)
			continue
		}

		if !slices.Equal(d.ProcessNames, want.processNames) {
			t.Errorf("%s: process names = %q, want %q", d.Name, d.ProcessNames, want.processNames)
		}
		if got := d.launchCommand(); got != want.launchCommand {
			t.Errorf("%s: launch command = %q, want %q", d.Name, got, want.launchCommand)
		}
		if d.AutoAcceptFlag != want.autoAcceptFlag {
			t.Errorf("%s: auto-accept flag = %q, want %q", d.Name, d.AutoAcceptFlag, want.autoAcceptFlag)
		}

		// AutoAcceptFlags lists only the tools that have a flag.
		flag, listed := flags[d.Name]
		if listed != (want.autoAcceptFlag != "") || flag != want.autoAcceptFlag {
			t.Errorf("%s: AutoAcceptFlags entry = (%q, %v), want %q", d.Name, flag, listed, want.autoAcceptFlag)
		}
	}
}

// Tools crex cannot resume are restarted with the command that was running,
// never with the registry name: "factory" or "rovo" is not a command anyone
// has installed.
func TestRegistry_ProcessAwareRelaunchCommand(t *testing.T) {
	// An empty home keeps the tier-1 detectors from finding real sessions
	// of this machine; only the process-aware ones answer.
	t.Setenv("HOME", t.TempDir())

	for _, d := range registry {
		want, pinned := wantRegistry[d.Name]
		if !pinned {
			continue
		}
		session := d.Detect("/tmp/project", "")
		if session == nil {
			// Tier-1 detectors find nothing in a made-up directory.
			continue
		}
		if session.Tool != d.Name {
			t.Errorf("%s: session tool = %q", d.Name, session.Tool)
		}
		if session.Command != want.launchCommand {
			t.Errorf("%s: relaunch command = %q, want %q", d.Name, session.Command, want.launchCommand)
		}
	}
}

// TestFlagSubcommands: only Kiro takes its flag after a subcommand. Every
// other tool's flag goes right after its launch command.
func TestFlagSubcommands(t *testing.T) {
	want := map[string]string{"kiro": "chat"}

	got := FlagSubcommands()
	if len(got) != len(want) {
		t.Errorf("FlagSubcommands() = %q, want %q", got, want)
	}
	for tool, subcommand := range want {
		if got[tool] != subcommand {
			t.Errorf("FlagSubcommands()[%q] = %q, want %q", tool, got[tool], subcommand)
		}
	}
}

func TestBareCommands(t *testing.T) {
	bare := BareCommands()

	// Process names, launch commands, and the registry names older crex
	// versions saved as the command of a tier-2 pane.
	for _, command := range []string{"claude", "agent", "cursor-agent", "droid", "cbc", "qodercli", "atlassian_cli_rovodev", "acli rovodev run", "cursor", "factory", "rovo"} {
		if !bare[command] {
			t.Errorf("BareCommands() is missing %q", command)
		}
	}
	for _, command := range []string{"htop", "npm run dev", "claude --resume abc", "acli"} {
		if bare[command] {
			t.Errorf("BareCommands() should not contain %q", command)
		}
	}
}

func TestToolLaunch(t *testing.T) {
	tests := []struct {
		cmdline    string
		wantTool   string
		wantLaunch string
		wantOK     bool
	}{
		{"claude --resume abc123", "claude", "claude", true},
		{"claude", "claude", "claude", true},
		{"codex resume abc123", "codex", "codex", true},
		{"droid", "factory", "droid", true},
		{"agent --model auto", "cursor", "agent", true},
		{"acli rovodev run", "rovo", "acli rovodev run", true},
		{"acli rovodev run --restore", "rovo", "acli rovodev run", true},

		{"acli rovodev", "", "", false},
		{"acli jira workitem view", "", "", false},
		{"claudette", "", "", false},
		// The name older versions saved for cursor is the Cursor editor's
		// own command; it does not start the agent.
		{"cursor", "", "", false},
		{"factory", "", "", false},
		{"npm run dev", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmdline, func(t *testing.T) {
			tool, launch, ok := ToolLaunch(tt.cmdline)
			if ok != tt.wantOK || tool != tt.wantTool || launch != tt.wantLaunch {
				t.Errorf("ToolLaunch(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.cmdline, tool, launch, ok, tt.wantTool, tt.wantLaunch, tt.wantOK)
			}
		})
	}
}

// newDetectedSessions returns an empty result for addSession to fill.
func newDetectedSessions() DetectedSessions {
	return DetectedSessions{
		ByCWD:  make(map[string][]Session),
		ByTool: make(map[string][]Session),
		TTYs:   make(map[SessionKey][]string),
	}
}

// TestAddSession_SameCommandInTwoDirectories: Copilot resumes with
// "copilot --continue" and Aider relaunches as "aider" wherever they run. Two
// processes in two directories are two sessions, and both are kept.
func TestAddSession_SameCommandInTwoDirectories(t *testing.T) {
	tests := []struct {
		tool    string
		command string
	}{
		{tool: "copilot", command: "copilot --continue"},
		{tool: "aider", command: "aider"},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			result := newDetectedSessions()
			seen := make(map[SessionKey]bool)

			addSession(result, seen, &Session{Tool: tt.tool, CWD: "/home/u/a", Command: tt.command})
			addSession(result, seen, &Session{Tool: tt.tool, CWD: "/home/u/b", Command: tt.command})

			if got := len(result.ByTool[tt.tool]); got != 2 {
				t.Errorf("ByTool[%q] holds %d sessions, want 2", tt.tool, got)
			}
			for _, cwd := range []string{"/home/u/a", "/home/u/b"} {
				if got := len(result.ByCWD[cwd]); got != 1 {
					t.Errorf("ByCWD[%q] holds %d sessions, want 1", cwd, got)
				}
			}
		})
	}
}

// TestAddSession_SameSessionTwice guards what the filter was written for: two
// Claude processes in one directory that resolve to the same session id (the
// most-recent-transcript fallback) are one session, not two panes resuming
// the same conversation.
func TestAddSession_SameSessionTwice(t *testing.T) {
	result := newDetectedSessions()
	seen := make(map[SessionKey]bool)
	session := Session{Tool: "claude", CWD: "/home/u/proj", Command: "claude --resume abc123"}

	addSession(result, seen, &session)
	addSession(result, seen, &session)
	addSession(result, seen, nil)

	if got := len(result.ByTool["claude"]); got != 1 {
		t.Errorf("ByTool[claude] holds %d sessions, want 1", got)
	}
	if got := len(result.ByCWD["/home/u/proj"]); got != 1 {
		t.Errorf("ByCWD[/home/u/proj] holds %d sessions, want 1", got)
	}
}

// TestCollectSessions_SameSessionTwiceKeepsBothTerminals: two Claude
// processes that resolve to one session leave it once, on both their
// terminals, so a pane on either one can claim it.
func TestCollectSessions_SameSessionTwiceKeepsBothTerminals(t *testing.T) {
	detectors := map[string]detector{
		"claude": {Name: "claude", Detect: func(cwd, pid string) *Session {
			return &Session{Tool: "claude", CWD: cwd, Command: "claude --resume abc123"}
		}},
	}
	procs := []aiProcess{
		{tool: "claude", pid: "20", cwd: "/home/u/proj", tty: "ttys001"},
		{tool: "claude", pid: "21", cwd: "/home/u/proj", tty: "ttys002"},
	}

	result := collectSessions(procs, detectors)

	if got := len(result.ByTool["claude"]); got != 1 {
		t.Errorf("ByTool[claude] holds %d sessions, want 1", got)
	}
	key := SessionKey{Command: "claude --resume abc123", CWD: "/home/u/proj"}
	if got, want := result.TTYs[key], []string{"ttys001", "ttys002"}; !slices.Equal(got, want) {
		t.Errorf("session terminals = %q, want %q", got, want)
	}
}

// TestCollectSessions_ProcessWithoutTerminal: a process ps shows with no
// terminal leaves its session with no entry, never an empty name a pane
// without a terminal could match.
func TestCollectSessions_ProcessWithoutTerminal(t *testing.T) {
	detectors := map[string]detector{
		"codex": {Name: "codex", Detect: func(cwd, pid string) *Session {
			return &Session{Tool: "codex", CWD: cwd, Command: "codex resume thread-1"}
		}},
	}
	procs := []aiProcess{{tool: "codex", pid: "10", cwd: "/home/u/proj"}}

	result := collectSessions(procs, detectors)

	if got := len(result.ByTool["codex"]); got != 1 {
		t.Fatalf("ByTool[codex] holds %d sessions, want 1", got)
	}
	if ttys, ok := result.TTYs[result.ByTool["codex"][0].Key()]; ok {
		t.Errorf("session terminals = %q, want no entry", ttys)
	}
}

// TestParseProcessLine reads one line of `ps -o pid,tty,…`. A process with
// no terminal shows "??" on macOS and "?" on Linux; both mean none.
func TestParseProcessLine(t *testing.T) {
	tests := []struct {
		line     string
		wantPID  string
		wantTTY  string
		wantRest []string
		wantOK   bool
	}{
		{line: "  4242 ttys004  /usr/local/bin/claude", wantPID: "4242", wantTTY: "ttys004", wantRest: []string{"/usr/local/bin/claude"}, wantOK: true},
		{line: "4242 pts/3 node /opt/claude/cli.js --resume", wantPID: "4242", wantTTY: "pts/3", wantRest: []string{"node", "/opt/claude/cli.js", "--resume"}, wantOK: true},
		{line: "4242 ?? /usr/local/bin/claude", wantPID: "4242", wantTTY: "", wantRest: []string{"/usr/local/bin/claude"}, wantOK: true},
		{line: "4242 ? claude", wantPID: "4242", wantTTY: "", wantRest: []string{"claude"}, wantOK: true},
		{line: "4242 ttys004", wantOK: false},
		{line: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			pid, tty, rest, ok := parseProcessLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("parseProcessLine(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if pid != tt.wantPID || tty != tt.wantTTY || !slices.Equal(rest, tt.wantRest) {
				t.Errorf("parseProcessLine(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tt.line, pid, tty, rest, tt.wantPID, tt.wantTTY, tt.wantRest)
			}
		})
	}
}

// TestTTYName: cmux and ps name a terminal with or without /dev/, and a
// pane matches a process only when both are spelled the same way.
func TestTTYName(t *testing.T) {
	tests := map[string]string{
		"/dev/ttys004": "ttys004",
		"ttys004":      "ttys004",
		"/dev/pts/3":   "pts/3",
		"??":           "",
		"?":            "",
		"":             "",
	}
	for device, want := range tests {
		if got := TTYName(device); got != want {
			t.Errorf("TTYName(%q) = %q, want %q", device, got, want)
		}
	}
}

// TestCollectSessions_EveryProcessTerminal: a tool keyed by directory is
// asked once for two processes in one folder, but both processes' terminals
// end up on the session. Claude is asked once per process, and each session
// carries only its own terminal.
func TestCollectSessions_EveryProcessTerminal(t *testing.T) {
	detectors := map[string]detector{
		"codex": {Name: "codex", Detect: func(cwd, pid string) *Session {
			return &Session{Tool: "codex", CWD: cwd, Command: "codex resume thread-1"}
		}},
		"claude": {Name: "claude", Detect: func(cwd, pid string) *Session {
			return &Session{Tool: "claude", CWD: cwd, Command: "claude --resume " + pid}
		}},
	}
	procs := []aiProcess{
		{tool: "codex", pid: "10", cwd: "/home/u/proj", tty: "ttys001"},
		{tool: "codex", pid: "11", cwd: "/home/u/proj", tty: "ttys002"},
		{tool: "claude", pid: "20", cwd: "/home/u/proj", tty: "ttys003"},
		{tool: "claude", pid: "21", cwd: "/home/u/proj", tty: "ttys004"},
	}

	result := collectSessions(procs, detectors)

	codex := result.ByTool["codex"]
	if len(codex) != 1 {
		t.Fatalf("codex sessions = %+v, want one", codex)
	}
	if got := result.TTYs[codex[0].Key()]; !slices.Equal(got, []string{"ttys001", "ttys002"}) {
		t.Errorf("codex terminals = %q, want ttys001 and ttys002", got)
	}

	claude := result.ByTool["claude"]
	if len(claude) != 2 {
		t.Fatalf("claude sessions = %+v, want two", claude)
	}
	first := result.TTYs[claude[0].Key()]
	second := result.TTYs[claude[1].Key()]
	if !slices.Equal(first, []string{"ttys003"}) || !slices.Equal(second, []string{"ttys004"}) {
		t.Errorf("claude terminals = %q and %q, want ttys003 and ttys004", first, second)
	}
}
