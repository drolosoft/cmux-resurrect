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
	if cmd != "opencode --yolo --session ses_xyz" {
		t.Errorf("got %q", cmd)
	}
	if tool != "opencode" {
		t.Errorf("tool = %q, want opencode", tool)
	}
}

func TestInjectAutoAccept_Codex(t *testing.T) {
	cmd, tool := InjectAutoAccept("codex resume abc123", []string{"codex"})
	if cmd != "codex --full-auto resume abc123" {
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
	cmd, tool := InjectAutoAccept("cursor --resume abc123", []string{"all"})
	if cmd != "cursor --resume abc123" {
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

func TestInjectAutoAccept_MultiWordFlag(t *testing.T) {
	cmd, tool := InjectAutoAccept("qoder resume abc123", []string{"qoder"})
	if cmd != "qoder --permission-mode auto resume abc123" {
		t.Errorf("got %q", cmd)
	}
	if tool != "qoder" {
		t.Errorf("tool = %q, want qoder", tool)
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
	if cmd != "droid --skip-permissions-unsafe" {
		t.Errorf("got %q", cmd)
	}
	if tool != "factory" {
		t.Errorf("tool = %q, want factory", tool)
	}

	cmd, tool = InjectAutoAccept("droid", []string{"all"})
	if cmd != "droid --skip-permissions-unsafe" || tool != "factory" {
		t.Errorf("with all: got (%q, %q)", cmd, tool)
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
