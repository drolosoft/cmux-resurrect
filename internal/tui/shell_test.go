package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestShellModel_InitShowsWelcome(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	view := m.View()

	if !strings.Contains(view, "crex") {
		t.Error("initial view should contain 'crex'")
	}
	if !strings.Contains(view, "help") {
		t.Error("initial view should mention 'help'")
	}
}

func TestShellModel_StartsInPromptMode(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	if m.mode != modePrompt {
		t.Errorf("expected modePrompt, got %v", m.mode)
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

func TestShellModel_HelpShowsOutput(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("help")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	view := sm.View()
	if !strings.Contains(view, "Live") {
		t.Error("help output should contain 'Live' group")
	}
	if !strings.Contains(view, "Layouts") {
		t.Error("help output should contain 'Layouts' group")
	}
}

func TestShellModel_UnknownCommand(t *testing.T) {
	m := NewShellModel(nil, nil, client.BackendGhostty, "")
	m.prompt.SetValue("wat")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := result.(ShellModel)

	if !strings.Contains(sm.output.String(), "Unknown command") {
		t.Error("unknown command should show error message")
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
