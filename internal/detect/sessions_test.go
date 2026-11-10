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
	"claude":    {[]string{"claude"}, "claude", "--dangerously-skip-permissions"},
	"opencode":  {[]string{"opencode"}, "opencode", "--auto"},
	"codex":     {[]string{"codex"}, "codex", "--dangerously-bypass-approvals-and-sandbox"},
	"amp":       {[]string{"amp"}, "amp", "--dangerously-allow-all"},
	"gemini":    {[]string{"gemini"}, "gemini", "--approval-mode=yolo"},
	"copilot":   {[]string{"copilot"}, "copilot", "--allow-all"},
	"grok":      {[]string{"grok"}, "grok", "--always-approve"},
	"cursor":    {[]string{"agent", "cursor-agent"}, "agent", "--force"},
	"aider":     {[]string{"aider"}, "aider", "--yes-always"},
	"pi":        {[]string{"pi"}, "pi", ""},
	"rovo":      {[]string{"atlassian_cli_rovodev"}, "acli rovodev run", "--yolo"},
	"hermes":    {[]string{"hermes"}, "hermes", "--yolo"},
	"codebuddy": {[]string{"codebuddy", "cbc"}, "codebuddy", "--dangerously-skip-permissions"},
	"factory":   {[]string{"droid"}, "droid", "--skip-permissions-unsafe"},
	"qoder":     {[]string{"qoder", "qodercli"}, "qoder", "--yolo"},
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
