package tui

import (
	"strings"
	"testing"
)

// testPopItems returns a standard set of test items: 2 layouts + 2 templates.
func testPopItems() []PopItem {
	return []PopItem{
		{Kind: "layout", Name: "morning", Meta: "3 tabs  May 28"},
		{Kind: "layout", Name: "afternoon", Meta: "2 tabs  May 27"},
		{Kind: "template", Name: "ide", Icon: "⧉", Meta: "editor+git+term"},
		{Kind: "template", Name: "claude", Icon: "🤖", Meta: "claude code setup"},
	}
}

func TestPopItems_AllShown(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)
	view := m.View()
	for _, name := range []string{"morning", "afternoon", "ide", "claude"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing item %q", name)
		}
	}
}

func TestPopItems_FilterNarrows(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)
	m.filter = "mor"
	m.applyFilter()
	if len(m.filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(m.filtered))
	}
	if m.filtered[0].Name != "morning" {
		t.Errorf("filtered[0].Name = %q, want %q", m.filtered[0].Name, "morning")
	}
	view := m.View()
	if !strings.Contains(view, "morning") {
		t.Error("View() should contain 'morning' after filter 'mor'")
	}
	if strings.Contains(view, "afternoon") {
		t.Error("View() should not contain 'afternoon' after filter 'mor'")
	}
}

func TestPopItems_FilterEmpty(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)
	m.filter = "zzz"
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}
}

func TestPopItems_NumberSelection(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)
	item := m.selectByNumber(2)
	if item == nil {
		t.Fatal("selectByNumber(2) returned nil, want second item")
	}
	if item.Name != "afternoon" {
		t.Errorf("selectByNumber(2).Name = %q, want %q", item.Name, "afternoon")
	}
}

func TestPopItems_NumberOutOfRange(t *testing.T) {
	items := []PopItem{
		{Kind: "layout", Name: "only", Meta: "1 tab  May 28"},
	}
	m := NewPopModel(items, 80, 24)
	if got := m.selectByNumber(5); got != nil {
		t.Errorf("selectByNumber(5) = %v, want nil", got)
	}
	if got := m.selectByNumber(0); got != nil {
		t.Errorf("selectByNumber(0) = %v, want nil", got)
	}
}

func TestPopItems_CursorBounds(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)

	// Move up from zero — cursor should stay at 0.
	m.cursorUp()
	if m.cursor != 0 {
		t.Errorf("cursor after up from 0 = %d, want 0", m.cursor)
	}

	// Move down past the end — cursor should stop at last index.
	for i := 0; i < 10; i++ {
		m.cursorDown()
	}
	max := len(m.filtered) - 1
	if m.cursor != max {
		t.Errorf("cursor after many downs = %d, want %d", m.cursor, max)
	}
}

func TestPopView_SectionHeaders(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24)
	view := m.View()
	if !strings.Contains(view, "LAYOUTS") {
		t.Error("View() missing 'LAYOUTS' section header")
	}
	if !strings.Contains(view, "TEMPLATES") {
		t.Error("View() missing 'TEMPLATES' section header")
	}
}
