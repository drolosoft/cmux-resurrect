package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// popStyles holds the lipgloss styles used by PopModel.
var (
	popHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"})
	popCursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	popNumberStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	popDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	popTitleStyle   = lipgloss.NewStyle().Bold(true)
	popMetaStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	popSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#FFB454", Light: "#D4820A"})
	popFilterStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
)

// PopItem represents a single entry in the pop picker.
type PopItem struct {
	Kind string // "layout" or "template"
	Name string
	Icon string // template icon (e.g. "⧉"), empty for layouts
	Meta string // "3 tabs  May 28" or "editor+git+term"
}

// PopModel is a lightweight bubbletea picker for crex pop.
type PopModel struct {
	items    []PopItem
	filtered []PopItem
	filter   string
	cursor   int
	chosen   *PopItem
	quitting bool
	width    int
	height   int
}

// NewPopModel creates a picker with the given items.
func NewPopModel(items []PopItem, width, height int) *PopModel {
	m := &PopModel{
		items:  items,
		width:  width,
		height: height,
	}
	m.applyFilter()
	return m
}

// Chosen returns the selected item, or nil if cancelled.
func (m *PopModel) Chosen() *PopItem {
	return m.chosen
}

// Init is the Bubble Tea init function.
func (m *PopModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles all incoming messages.
func (m *PopModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.filtered) > 0 {
				item := m.filtered[m.cursor]
				m.chosen = &item
			}
			return m, tea.Quit

		case tea.KeyUp:
			m.cursorUp()
			return m, nil

		case tea.KeyDown:
			m.cursorDown()
			return m, nil

		case tea.KeyBackspace:
			if len(m.filter) > 0 {
				// Remove last rune.
				runes := []rune(m.filter)
				m.filter = string(runes[:len(runes)-1])
				m.applyFilter()
			}
			return m, nil

		default:
			// Number keys 1-9 for instant selection.
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 {
				r := msg.Runes[0]
				if r >= '1' && r <= '9' {
					n := int(r - '0')
					if item := m.selectByNumber(n); item != nil {
						m.chosen = item
						return m, tea.Quit
					}
					return m, nil
				}
				// Regular character — append to filter.
				m.filter += string(r)
				m.applyFilter()
				return m, nil
			}
		}
	}
	return m, nil
}

// View renders the full picker UI.
func (m *PopModel) View() string {
	var b strings.Builder

	// Header: "crex > " + filter
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(popTitleStyle.Render("crex"))
	b.WriteString(" > ")
	if m.filter != "" {
		b.WriteString(popFilterStyle.Render(m.filter))
	}
	b.WriteString(popDimStyle.Render("_"))
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(popDimStyle.Render("  no results"))
		b.WriteString("\n")
	} else {
		// Render items grouped by section.
		idx := 0
		lastKind := ""
		for _, item := range m.filtered {
			// Section header when kind changes.
			if item.Kind != lastKind {
				if lastKind != "" {
					b.WriteString("\n")
				}
				section := "LAYOUTS"
				if item.Kind == "template" {
					section = "TEMPLATES"
				}
				b.WriteString("  ")
				b.WriteString(popSectionStyle.Render(section))
				b.WriteString("\n")
				lastKind = item.Kind
			}

			num := fmt.Sprintf("[%d]", idx+1)
			isCurrent := idx == m.cursor

			var line strings.Builder
			line.WriteString("  ")
			if isCurrent {
				line.WriteString(popCursorStyle.Render("▸"))
				line.WriteString(" ")
				line.WriteString(popNumberStyle.Render(num))
			} else {
				line.WriteString("  ")
				line.WriteString(popDimStyle.Render(num))
			}
			line.WriteString("  ")

			// Icon for templates.
			if item.Icon != "" {
				line.WriteString(item.Icon)
				line.WriteString("  ")
			}

			// Name + meta.
			if isCurrent {
				line.WriteString(popTitleStyle.Render(item.Name))
			} else {
				line.WriteString(item.Name)
			}
			if item.Meta != "" {
				line.WriteString("  ")
				line.WriteString(popMetaStyle.Render(item.Meta))
			}

			b.WriteString(line.String())
			b.WriteString("\n")
			idx++
		}
	}

	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(popDimStyle.Render("↵ launch · type to filter · esc quit"))
	b.WriteString("\n")

	return b.String()
}

// applyFilter rebuilds m.filtered from m.items using m.filter as a substring match.
// The cursor is reset to 0 after filtering.
func (m *PopModel) applyFilter() {
	if m.filter == "" {
		m.filtered = make([]PopItem, len(m.items))
		copy(m.filtered, m.items)
		m.cursor = 0
		return
	}
	lower := strings.ToLower(m.filter)
	m.filtered = m.filtered[:0]
	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.Name), lower) ||
			strings.Contains(strings.ToLower(item.Meta), lower) {
			m.filtered = append(m.filtered, item)
		}
	}
	m.cursor = 0
}

// cursorUp moves the cursor up by one, clamped to 0.
func (m *PopModel) cursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// cursorDown moves the cursor down by one, clamped to the last index.
func (m *PopModel) cursorDown() {
	if len(m.filtered) == 0 {
		return
	}
	if m.cursor < len(m.filtered)-1 {
		m.cursor++
	}
}

// selectByNumber returns the item at 1-based position n, or nil if out of range.
func (m *PopModel) selectByNumber(n int) *PopItem {
	if n < 1 || n > len(m.filtered) {
		return nil
	}
	item := m.filtered[n-1]
	return &item
}
