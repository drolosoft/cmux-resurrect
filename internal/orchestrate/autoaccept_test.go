package orchestrate

import (
	"testing"
)

func TestInjectAutoAccept_Claude(t *testing.T) {
	cmd, tool := InjectAutoAccept("claude --resume abc123", []string{"claude"})
	if cmd != "claude --dangerously-skip-permissions --resume abc123" {
		t.Errorf("got %q", cmd)
	}
	if tool != "claude" {
		t.Errorf("tool = %q, want claude", tool)
	}
}

func TestInjectAutoAccept_OpenCode(t *testing.T) {
	cmd, tool := InjectAutoAccept("opencode --session ses_xyz", []string{"opencode"})
	if cmd != "opencode --auto --session ses_xyz" {
		t.Errorf("got %q", cmd)
	}
	if tool != "opencode" {
		t.Errorf("tool = %q, want opencode", tool)
	}
}

func TestInjectAutoAccept_Codex(t *testing.T) {
	cmd, tool := InjectAutoAccept("codex resume abc123", []string{"codex"})
	if cmd != "codex --dangerously-bypass-approvals-and-sandbox resume abc123" {
		t.Errorf("got %q", cmd)
	}
	if tool != "codex" {
		t.Errorf("tool = %q, want codex", tool)
	}
}

func TestInjectAutoAccept_NonAICommand(t *testing.T) {
	cmd, tool := InjectAutoAccept("npm run dev", []string{"claude"})
	if cmd != "npm run dev" {
		t.Errorf("got %q, want unchanged", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestInjectAutoAccept_NotInList(t *testing.T) {
	cmd, tool := InjectAutoAccept("claude --resume abc123", []string{"codex"})
	if cmd != "claude --resume abc123" {
		t.Errorf("got %q, want unchanged", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestInjectAutoAccept_All(t *testing.T) {
	cmd, tool := InjectAutoAccept("claude --resume abc123", []string{"all"})
	if cmd != "claude --dangerously-skip-permissions --resume abc123" {
		t.Errorf("got %q", cmd)
	}
	if tool != "claude" {
		t.Errorf("tool = %q, want claude", tool)
	}
}

func TestInjectAutoAccept_EmptyList(t *testing.T) {
	cmd, tool := InjectAutoAccept("claude --resume abc123", nil)
	if cmd != "claude --resume abc123" {
		t.Errorf("got %q, want unchanged", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestInjectAutoAccept_FlagAlreadyPresent(t *testing.T) {
	cmd, tool := InjectAutoAccept("claude --dangerously-skip-permissions --resume abc123", []string{"claude"})
	if cmd != "claude --dangerously-skip-permissions --resume abc123" {
		t.Errorf("got %q, want unchanged (flag already present)", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty (no injection needed)", tool)
	}
}

func TestInjectAutoAccept_NoFlag(t *testing.T) {
	// Pi has no prompt to skip: it never asks before running a tool.
	cmd, tool := InjectAutoAccept("pi --continue", []string{"all"})
	if cmd != "pi --continue" {
		t.Errorf("got %q, want unchanged", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestInjectAutoAccept_EmptyCommand(t *testing.T) {
	cmd, tool := InjectAutoAccept("", []string{"all"})
	if cmd != "" {
		t.Errorf("got %q, want empty", cmd)
	}
	if tool != "" {
		t.Errorf("tool = %q, want empty", tool)
	}
}

func TestInjectAutoAccept_MultiWordLaunch(t *testing.T) {
	cmd, tool := InjectAutoAccept("acli rovodev run", []string{"rovo"})
	if cmd != "acli rovodev run --yolo" {
		t.Errorf("got %q", cmd)
	}
	if tool != "rovo" {
		t.Errorf("tool = %q, want rovo", tool)
	}
}

func TestInjectAutoAccept_Gemini(t *testing.T) {
	cmd, tool := InjectAutoAccept("gemini --resume abc12345", []string{"gemini"})
	if cmd != "gemini --approval-mode=yolo --resume abc12345" {
		t.Errorf("got %q", cmd)
	}
	if tool != "gemini" {
		t.Errorf("tool = %q, want gemini", tool)
	}
}

func TestInjectAutoAccept_Cursor(t *testing.T) {
	cmd, tool := InjectAutoAccept("agent", []string{"cursor"})
	if cmd != "agent --force" {
		t.Errorf("got %q", cmd)
	}
	if tool != "cursor" {
		t.Errorf("tool = %q, want cursor", tool)
	}
}

func TestInjectAutoAccept_EnvPrefix(t *testing.T) {
	cmd, tool := InjectAutoAccept("CLAUDE_CONFIG_DIR=/Users/me/.claude-pro claude --resume abc123", []string{"claude"})
	if cmd != "CLAUDE_CONFIG_DIR=/Users/me/.claude-pro claude --dangerously-skip-permissions --resume abc123" {
		t.Errorf("got %q", cmd)
	}
	if tool != "claude" {
		t.Errorf("tool = %q, want claude", tool)
	}
}

// Factory's binary is droid; auto_accept still lists the tool by its name.
func TestInjectAutoAccept_LaunchCommandNotNamedAfterTool(t *testing.T) {
	cmd, tool := InjectAutoAccept("droid", []string{"factory"})
	if cmd != "droid --auto high" {
		t.Errorf("got %q", cmd)
	}
	if tool != "factory" {
		t.Errorf("tool = %q, want factory", tool)
	}

	cmd, tool = InjectAutoAccept("droid", []string{"all"})
	if cmd != "droid --auto high" || tool != "factory" {
		t.Errorf("with all: got (%q, %q)", cmd, tool)
	}
}

// TestInjectAutoAccept_DroidInteractiveFlag: --skip-permissions-unsafe
// exists only on `droid exec`. The interactive droid ignores options it does
// not know, so the old flag started a session that still asked. --auto is a
// root option, and `droid --auto high --resume <id> --version` prints the
// version, which droid does only when every option before it parsed.
func TestInjectAutoAccept_DroidInteractiveFlag(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"droid --resume 7f8e2a10-0000-4000-8000-000000000001", "droid --auto high --resume 7f8e2a10-0000-4000-8000-000000000001"},
		{"FACTORY_HOME_OVERRIDE=/Users/me/work droid --resume abc123", "FACTORY_HOME_OVERRIDE=/Users/me/work droid --auto high --resume abc123"},
		{"droid --auto high --resume abc123", "droid --auto high --resume abc123"},
	}
	for _, tt := range tests {
		cmd, _ := InjectAutoAccept(tt.command, []string{"factory"})
		if cmd != tt.want {
			t.Errorf("InjectAutoAccept(%q) = %q, want %q", tt.command, cmd, tt.want)
		}
	}
}

// auto_accept takes registry names, the same names a saved layout uses.
func TestInjectAutoAccept_BinaryNameInList(t *testing.T) {
	cmd, tool := InjectAutoAccept("droid", []string{"droid"})
	if cmd != "droid" || tool != "" {
		t.Errorf("got (%q, %q), want droid unchanged", cmd, tool)
	}
}

// Older versions saved the registry name as the command of a tier-2 pane.
// "factory" starts nothing, so no flag is added to it.
func TestInjectAutoAccept_StaleRegistryName(t *testing.T) {
	cmd, tool := InjectAutoAccept("factory", []string{"all"})
	if cmd != "factory" || tool != "" {
		t.Errorf("got (%q, %q), want factory unchanged", cmd, tool)
	}
}

// TestInjectAutoAccept_KiroFlagAfterChat: kiro-cli accepts --trust-all-tools
// only after its chat subcommand. `kiro-cli --trust-all-tools chat …` exits
// with status 2 ("unexpected argument '--trust-all-tools' found"), so the
// pane would never start. A command without chat gets it, since bare kiro-cli
// opens a chat anyway.
func TestInjectAutoAccept_KiroFlagAfterChat(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"kiro-cli chat --resume-id 00000000-0000-0000-0000-000000000000", "kiro-cli chat --trust-all-tools --resume-id 00000000-0000-0000-0000-000000000000"},
		{"kiro-cli chat --resume", "kiro-cli chat --trust-all-tools --resume"},
		{"kiro-cli chat", "kiro-cli chat --trust-all-tools"},
		{"kiro-cli", "kiro-cli chat --trust-all-tools"},
		{"kiro-cli --resume-id sess_0001", "kiro-cli chat --trust-all-tools --resume-id sess_0001"},
		{"KIRO_LOG_LEVEL=debug kiro-cli chat --resume", "KIRO_LOG_LEVEL=debug kiro-cli chat --trust-all-tools --resume"},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			cmd, tool := InjectAutoAccept(tt.command, []string{"kiro"})
			if cmd != tt.want || tool != "kiro" {
				t.Errorf("got (%q, %q), want (%q, kiro)", cmd, tool, tt.want)
			}
		})
	}
}

// TestInjectAutoAccept_KiroOtherSubcommand: a kiro-cli command that runs
// another subcommand has no chat to trust tools in, and putting chat in front
// of it would turn the subcommand into a prompt.
func TestInjectAutoAccept_KiroOtherSubcommand(t *testing.T) {
	for _, command := range []string{"kiro-cli login", "kiro-cli settings list", "kiro-cli chatty", "kiro-cli chat --trust-all-tools --resume"} {
		cmd, tool := InjectAutoAccept(command, []string{"all"})
		if cmd != command || tool != "" {
			t.Errorf("InjectAutoAccept(%q) = (%q, %q), want it unchanged", command, cmd, tool)
		}
	}
}

// TestInjectAutoAccept_HermesResumeKeepsItsMode checks that a resumed Hermes
// session is left alone: Hermes exits with an error when --resume carries a
// --yolo the session was not created with.
func TestInjectAutoAccept_HermesResumeKeepsItsMode(t *testing.T) {
	resumed, tool := InjectAutoAccept("hermes --resume 20261110_091523_a1b2c3", []string{"all"})
	if resumed != "hermes --resume 20261110_091523_a1b2c3" || tool != "" {
		t.Errorf("resume = %q (tool %q), want the command unchanged", resumed, tool)
	}

	fresh, tool := InjectAutoAccept("hermes", []string{"hermes"})
	if fresh != "hermes --yolo" || tool != "hermes" {
		t.Errorf("fresh = %q (tool %q), want hermes --yolo", fresh, tool)
	}
}
