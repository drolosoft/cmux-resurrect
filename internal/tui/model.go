package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// state represents which mode the TUI is in.
type state int

const (
	stateList   state = iota // browsing the item list
	stateFilter              // typing a filter string
)

// Action types returned after the TUI exits.

// SelectedAction is returned when the user selects an item (Enter).
type SelectedAction struct {
	Item Item
}

// SaveAction is returned when the user presses 's' to save the current layout.
type SaveAction struct{}

// DeleteAction is returned when the user presses 'd' to delete the selected layout.
type DeleteAction struct {
	Item Item
}

// Model is the Bubble Tea model for the crex TUI launcher.
type Model struct {
	// all items (layouts first, then templates)
	items []Item
	// items after applying the current filter
	filtered []Item

	state  state
	cursor int

	filter textinput.Model
	keys   KeyMap

	action   interface{}
	quitting bool
}

// NewModel creates a new Model from the provided layouts and templates.
// Layouts are prepended so they appear before templates in the list.
func NewModel(layouts []Item, templates []Item) Model {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.CharLimit = 64

	all := make([]Item, 0, len(layouts)+len(templates))
	all = append(all, layouts...)
	all = append(all, templates...)

	m := Model{
		items:  all,
		keys:   DefaultKeyMap(),
		filter: ti,
	}
	m.filtered = m.applyFilter("")
	return m
}

// Action returns the action that caused the TUI to exit, or nil if none yet.
func (m Model) Action() interface{} {
	return m.action
}

// Quitting reports whether the user quit without selecting an item.
func (m Model) Quitting() bool {
	return m.quitting
}

// applyFilter returns items whose FilterValue contains s (case-insensitive).
// When s is empty every item passes.
func (m Model) applyFilter(s string) []Item {
	if s == "" {
		result := make([]Item, len(m.items))
		copy(result, m.items)
		return result
	}
	lower := strings.ToLower(s)
	var out []Item
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.FilterValue()), lower) {
			out = append(out, item)
		}
	}
	return out
}

// clampCursor keeps the cursor within [0, len(filtered)-1].
func (m *Model) clampCursor() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

// selectedItem returns the currently highlighted item, if any.
func (m Model) selectedItem() (Item, bool) {
	if len(m.filtered) == 0 {
		return Item{}, false
	}
	return m.filtered[m.cursor], true
}

// --- tea.Model interface ---

// Init is the Bubble Tea init function; no I/O commands needed at startup.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles all incoming messages and key events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateList:
			return m.updateList(msg)
		case stateFilter:
			return m.updateFilter(msg)
		}
	}
	return m, nil
}

// updateList handles key events while in stateList.
func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Quit):
		m.quitting = true
		return m, tea.Quit

	case keyMatches(msg, m.keys.Filter):
		m.state = stateFilter
		m.filter.SetValue("")
		m.filter.Focus()
		return m, nil

	case keyMatches(msg, m.keys.Enter):
		item, ok := m.selectedItem()
		if ok {
			m.action = SelectedAction{Item: item}
		}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Save):
		m.action = SaveAction{}
		return m, tea.Quit

	case keyMatches(msg, m.keys.Delete):
		item, ok := m.selectedItem()
		if ok && item.Kind == KindLayout {
			m.action = DeleteAction{Item: item}
			return m, tea.Quit
		}
		return m, nil

	case msg.String() == "j" || msg.Type == tea.KeyDown:
		m.cursor++
		m.clampCursor()
		return m, nil

	case msg.String() == "k" || msg.Type == tea.KeyUp:
		m.cursor--
		m.clampCursor()
		return m, nil
	}
	return m, nil
}

// updateFilter handles key events while in stateFilter.
func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, m.keys.Escape):
		// Exit filter, clear text
		m.state = stateList
		m.filter.SetValue("")
		m.filter.Blur()
		m.filtered = m.applyFilter("")
		m.cursor = 0
		return m, nil

	case keyMatches(msg, m.keys.Enter):
		// Select currently highlighted item and exit
		m.state = stateList
		m.filter.Blur()
		item, ok := m.selectedItem()
		if ok {
			m.action = SelectedAction{Item: item}
		}
		return m, tea.Quit
	}

	// All other keys go to the text input
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filtered = m.applyFilter(m.filter.Value())
	m.cursor = 0
	return m, cmd
}

// keyMatches reports whether msg matches any key in b.
func keyMatches(msg tea.KeyMsg, b interface{ Keys() []string }) bool {
	for _, k := range b.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// View renders the model. Full rendering lives in view.go (Task 9).
// This stub is enough for the build to compile.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return "crex — use arrow keys / j k to navigate"
}
