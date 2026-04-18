package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel_EmptyState(t *testing.T) {
	m := NewModel(nil, nil)

	if m.state != stateList {
		t.Errorf("expected stateList, got %v", m.state)
	}
	if len(m.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.items))
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.Action() != nil {
		t.Errorf("expected nil action, got %v", m.Action())
	}
	if m.Quitting() {
		t.Error("expected Quitting() == false on new model")
	}
}

func TestNewModel_WithItems(t *testing.T) {
	layouts := []Item{
		{Kind: KindLayout, Name: "work", Description: "work layout", Workspaces: 3},
		{Kind: KindLayout, Name: "home", Description: "home layout", Workspaces: 2},
	}
	templates := []Item{
		{Kind: KindTemplate, Name: "dev", Description: "dev template", Icon: "🔧"},
	}

	m := NewModel(layouts, templates)

	if len(m.items) != 3 {
		t.Errorf("expected 3 items (2 layouts + 1 template), got %d", len(m.items))
	}
	// layouts come first
	if m.items[0].Name != "work" {
		t.Errorf("expected first item 'work', got %q", m.items[0].Name)
	}
	if m.items[1].Name != "home" {
		t.Errorf("expected second item 'home', got %q", m.items[1].Name)
	}
	if m.items[2].Name != "dev" {
		t.Errorf("expected third item 'dev', got %q", m.items[2].Name)
	}
}

func TestModel_QuitKey(t *testing.T) {
	m := NewModel(nil, nil)

	updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_ = updatedModel

	if cmd == nil {
		t.Fatal("expected a command from 'q' key, got nil")
	}
	// Execute the command and check it's a QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg from 'q' key, got %T", msg)
	}

	// The model should be quitting
	mm, ok := updatedModel.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if !mm.Quitting() {
		t.Error("expected Quitting() == true after 'q' key")
	}
}

func TestModel_FilterToggle(t *testing.T) {
	m := NewModel(nil, nil)

	if m.state != stateList {
		t.Errorf("pre-condition: expected stateList, got %v", m.state)
	}

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	mm, ok := updatedModel.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if mm.state != stateFilter {
		t.Errorf("expected stateFilter after '/', got %v", mm.state)
	}
}
