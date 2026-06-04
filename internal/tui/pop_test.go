package tui

import (
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// testPopItems returns a standard set of test items: 2 layouts + 2 templates.
func testPopItems() []PopItem {
	return []PopItem{
		{Kind: "layout", Name: "morning", Meta: "3 tabs  2h ago"},
		{Kind: "layout", Name: "afternoon", Meta: "2 tabs  1d ago"},
		{Kind: "template", Name: "ide", Icon: "⧉", Meta: "editor+git+term"},
		{Kind: "template", Name: "claude", Icon: "🤖", Meta: "claude code setup"},
	}
}

func mockLoader(name string) (*model.Layout, error) {
	if name == "morning" {
		return &model.Layout{
			Name: "morning",
			Workspaces: []model.Workspace{
				{Title: "webapp", CWD: "~/projects/webapp", Index: 0, Pinned: true,
					Panes: []model.Pane{{Type: "terminal", Command: "npm run dev"}, {Type: "terminal"}}},
				{Title: "api", CWD: "~/projects/api", Index: 1,
					Panes: []model.Pane{{Type: "terminal", Command: "go test"}}},
			},
		}, nil
	}
	return nil, nil
}

// --- Fuzzy filter tests (5) ---

func TestPop_FilterFuzzyMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.filter = "mor"
	m.applyFilter()
	if len(m.filtered) == 0 {
		t.Fatal("expected at least 1 result for 'mor'")
	}
	if m.filtered[0].Name != "morning" {
		t.Errorf("filtered[0].Name = %q, want %q", m.filtered[0].Name, "morning")
	}
}

func TestPop_FilterScatteredMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.filter = "cde"
	m.applyFilter()
	found := false
	for _, item := range m.filtered {
		if item.Name == "claude" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'claude' to match fuzzy 'cde', got %d results", len(m.filtered))
	}
}

func TestPop_FilterNoMatch(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.filter = "zzz"
	m.applyFilter()
	if len(m.filtered) != 0 {
		t.Fatalf("filtered len = %d, want 0", len(m.filtered))
	}
}

func TestPop_FilterEmpty(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	// No filter set — should show all 4 items.
	if len(m.filtered) != 4 {
		t.Fatalf("filtered len = %d, want 4", len(m.filtered))
	}
}

func TestPop_MatchPositions(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.filter = "mor"
	m.applyFilter()
	if len(m.matchPositions) == 0 {
		t.Fatal("matchPositions should be non-empty after a match")
	}
	if len(m.matchPositions[0]) == 0 {
		t.Error("first item matchPositions should have matched indices")
	}
}

// --- Navigation tests (4) ---

func TestPop_CursorBounds(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)

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

func TestPop_NumberSelect(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	item := m.selectByNumber(2)
	if item == nil {
		t.Fatal("selectByNumber(2) returned nil, want second item")
	}
	if item.Name != "afternoon" {
		t.Errorf("selectByNumber(2).Name = %q, want %q", item.Name, "afternoon")
	}
}

func TestPop_NumberOutOfRange(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	if got := m.selectByNumber(9); got != nil {
		t.Errorf("selectByNumber(9) = %v, want nil", got)
	}
	if got := m.selectByNumber(0); got != nil {
		t.Errorf("selectByNumber(0) = %v, want nil", got)
	}
}

func TestPop_CursorResetsOnFilter(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	// Move cursor to position 2.
	m.cursorDown()
	m.cursorDown()
	if m.cursor != 2 {
		t.Fatalf("cursor should be 2, got %d", m.cursor)
	}
	// Applying a filter should reset cursor.
	m.filter = "a"
	m.applyFilter()
	if m.cursor != 0 {
		t.Errorf("cursor after filter = %d, want 0", m.cursor)
	}
}

// --- Drill-in tests (5) ---

func TestPop_DrillEnter(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	if m.mode != modeDrill {
		t.Errorf("mode = %d, want modeDrill (%d)", m.mode, modeDrill)
	}
	if len(m.drillItems) != 2 {
		t.Errorf("drillItems len = %d, want 2", len(m.drillItems))
	}
}

func TestPop_DrillExit(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	m.exitDrill()
	if m.mode != modeList {
		t.Errorf("mode = %d, want modeList (%d)", m.mode, modeList)
	}
}

func TestPop_DrillOnTemplate_Noop(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("ide") // mockLoader returns nil for "ide"
	if m.mode != modeList {
		t.Errorf("mode = %d, want modeList (%d) — drill into template should be noop", m.mode, modeList)
	}
}

func TestPop_DrillPaneSummary(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	if len(m.drillItems) < 1 {
		t.Fatal("expected at least 1 drill item")
	}
	got := m.drillItems[0].PaneSummary
	want := "npm | shell"
	if got != want {
		t.Errorf("PaneSummary = %q, want %q", got, want)
	}
}

func TestPop_DrillFilterWorks(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	m.filter = "web"
	m.applyDrillFilter()
	if len(m.drillFiltered) != 1 {
		t.Fatalf("drillFiltered len = %d, want 1", len(m.drillFiltered))
	}
	if m.drillFiltered[0].Title != "webapp" {
		t.Errorf("drillFiltered[0].Title = %q, want %q", m.drillFiltered[0].Title, "webapp")
	}
}

// --- View rendering tests (7) ---

func TestPop_ViewSections(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	view := m.View()
	if !strings.Contains(view, "LAYOUTS") {
		t.Error("View() missing 'LAYOUTS' section header")
	}
	if !strings.Contains(view, "TEMPLATES") {
		t.Error("View() missing 'TEMPLATES' section header")
	}
}

func TestPop_ViewFooterList(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	view := m.View()
	if !strings.Contains(view, "tab") && !strings.Contains(view, "drill") {
		t.Error("list footer should mention 'tab' or 'drill'")
	}
}

func TestPop_ViewFooterDrill(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	view := m.View()
	if !strings.Contains(view, "back") {
		t.Error("drill footer should mention 'back'")
	}
}

func TestPop_ViewBreadcrumb(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	m.enterDrill("morning")
	view := m.View()
	if !strings.Contains(view, "morning") {
		t.Error("drill view should contain layout name in breadcrumb")
	}
}

func TestPop_ViewHasBorder(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	view := m.View()
	if !strings.Contains(view, "╔") || !strings.Contains(view, "╝") {
		t.Error("View() should contain double border chars ╔ and ╝")
	}
}

func TestPop_ViewDrillArrow(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	// Cursor is on first item (a layout) — should show →
	view := m.View()
	if !strings.Contains(view, "→") {
		t.Error("layout items with cursor should show → drill indicator")
	}
}

func TestPop_ViewAllItems(t *testing.T) {
	m := NewPopModel(testPopItems(), 80, 24, mockLoader)
	view := m.View()
	for _, name := range []string{"morning", "afternoon", "ide", "claude"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing item %q", name)
		}
	}
}

// --- termWidth accuracy tests (ZWJ emoji regression guard) ---

func TestTermWidth_ZWJEmoji(t *testing.T) {
	// ZWJ emoji sequences that lipgloss.Width miscounts.
	// termWidth must return accurate terminal cell widths.
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"detective+skin+ZWJ+female", "🕵🏼\u200d♀️", 2},
		{"bird+ZWJ+fire", "🐦\u200d🔥", 2},
		{"genie+ZWJ+male", "🧞\u200d♂️", 2},
		{"brain (simple)", "🧠", 2},
		{"moai (simple)", "🗿", 2},
		{"skull+VS16", "☠️", 2},
		{"ASCII only", "hello", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := termWidth(tt.input)
			if got != tt.want {
				t.Errorf("termWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestDrill_ZWJTitleNoWrap(t *testing.T) {
	// Workspace titles with complex ZWJ emojis must not exceed innerWidth
	// after padLine. This test prevents regression of the "duplicate first item"
	// and "phantom gap" rendering bugs caused by line wrapping.
	loader := func(name string) (*model.Layout, error) {
		if name == "zjw-test" {
			return &model.Layout{
				Name: "zjw-test",
				Workspaces: []model.Workspace{
					{Title: "0 🕵🏼\u200d♀️ Lazy Agent", Index: 0,
						Panes: []model.Pane{{Type: "terminal"}}},
					{Title: "3  🐦\u200d🔥crex", Index: 1,
						Panes: []model.Pane{{Type: "terminal"}, {Type: "terminal"}}},
					{Title: "5 🧞\u200d♂️ Prompto", Index: 2,
						Panes: []model.Pane{{Type: "terminal"}}},
				},
			}, nil
		}
		return nil, nil
	}

	items := []PopItem{{Kind: "layout", Name: "zjw-test", Meta: "3 tabs"}}
	m := NewPopModel(items, 80, 24, loader)
	m.enterDrill("zjw-test")

	innerWidth := 66 // (80-6=74, clamped to 74) - 8 = 66
	view := m.viewDrill(innerWidth, 13)

	for i, line := range strings.Split(view, "\n") {
		w := termWidth(line)
		if w > innerWidth {
			t.Errorf("line %d exceeds innerWidth: termWidth=%d > %d\n  line: %q", i, w, innerWidth, line)
		}
	}
}
