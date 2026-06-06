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
