package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBrowseModel_NavigateDown(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
		{Kind: KindLayout, Name: "c"},
	}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", bm.cursor)
	}
}

func TestBrowseModel_NavigateUp(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "a"},
		{Kind: KindLayout, Name: "b"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", bm.cursor)
	}
}

func TestBrowseModel_ClampTop(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyUp})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_ClampBottom(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bm.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", bm.cursor)
	}
}

func TestBrowseModel_EnterSelectsItem(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
	}
	bm := NewBrowseModel(items, "restore")
	bm.cursor = 1

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !bm.selected {
		t.Error("expected selected=true after Enter")
	}
	if bm.SelectedItem().Name != "afternoon" {
		t.Errorf("selected item = %q, want %q", bm.SelectedItem().Name, "afternoon")
	}
}

func TestBrowseModel_QuitReturnsToPrompt(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !bm.done {
		t.Error("expected done=true after q")
	}
	if bm.selected {
		t.Error("expected selected=false after q")
	}
}

func TestBrowseModel_LetterExitsBrowse(t *testing.T) {
	items := []Item{{Kind: KindLayout, Name: "a"}}
	bm := NewBrowseModel(items, "restore")

	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !bm.done {
		t.Error("expected done=true after typing a letter")
	}
	if bm.passthrough != 's' {
		t.Errorf("passthrough = %q, want 's'", bm.passthrough)
	}
}

func TestBrowseModel_FilterNarrows(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning"},
		{Kind: KindLayout, Name: "afternoon"},
		{Kind: KindLayout, Name: "evening"},
	}
	bm := NewBrowseModel(items, "restore")

	// Press / to enter filter
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !bm.filtering {
		t.Error("expected filtering=true after /")
	}

	// Type 'm' — should match "morning"
	bm, _ = bm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if len(bm.visible) != 1 {
		t.Errorf("after filter 'm': visible = %d, want 1", len(bm.visible))
	}
}

func TestBrowseModel_View_ContainsCursor(t *testing.T) {
	items := []Item{
		{Kind: KindLayout, Name: "morning", Description: "test", Workspaces: 2},
	}
	bm := NewBrowseModel(items, "restore")
	view := bm.View()

	if !strings.Contains(view, "▸") {
		t.Error("browse view should contain cursor marker ▸")
	}
	if !strings.Contains(view, "[1]") {
		t.Error("browse view should contain numbered index [1]")
	}
}
