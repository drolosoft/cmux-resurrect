package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestShellModel_WelcomeInInit(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if !strings.Contains(m.welcome, "crex") {
		t.Error("welcome should contain 'crex'")
	}
	if !strings.Contains(m.welcome, "help") {
		t.Error("welcome should mention 'help'")
	}
}

func TestShellModel_StartsInPromptMode(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if m.mode != modePrompt {
		t.Errorf("expected modePrompt, got %v", m.mode)
	}
}

func TestShellModel_ViewOnlyShowsPrompt(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	view := m.View()
	if !strings.Contains(view, "crex❯") {
		t.Error("view should show the prompt")
	}
	// View should NOT contain welcome (that's printed via Init)
	if strings.Contains(view, "interactive shell") {
		t.Error("view should not contain welcome text (printed via tea.Println)")
	}
}

func TestShellModel_ExitQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("exit")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if !sm.quitting {
		t.Error("expected quitting=true after 'exit'")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}

func TestShellModel_HelpProducesOutput(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("help")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	// Output is flushed via tea.Println cmd, not in View
	if cmd == nil {
		t.Error("expected tea.Println command for help output")
	}
	// View should only show prompt, not help content
	view := sm.View()
	if strings.Contains(view, "Layouts") {
		t.Error("help content should be printed via tea.Println, not in View()")
	}
}

func TestShellModel_UnknownCommand(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("wat")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Unknown command output is flushed via tea.Println
	if cmd == nil {
		t.Error("expected tea.Println command for error output")
	}
}

func TestShellModel_EmptyEnterDoesNothing(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if sm.quitting {
		t.Error("empty enter should not quit")
	}
	if sm.mode != modePrompt {
		t.Error("should stay in prompt mode")
	}
}

func TestShellModel_HistoryRecordsCommands(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	m.prompt.SetValue("help")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if len(sm.history) != 1 {
		t.Errorf("history length = %d, want 1", len(sm.history))
	}
	if sm.history[0] != "help" {
		t.Errorf("history[0] = %q, want %q", sm.history[0], "help")
	}
}

func TestShellModel_CtrlCQuits(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	sm := result.(ShellModel)

	if !sm.quitting {
		t.Error("ctrl+c should quit")
	}
	if cmd == nil {
		t.Error("expected tea.Quit command")
	}
}
