package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
	normalStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#333333"})
	descStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	categoryLabel = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}).Bold(true)
)

// View renders the full TUI layout.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString("\n  ")
	b.WriteString(titleStyle.Render("crex — select a layout to restore"))
	b.WriteString("\n\n")

	// Filter input (only in stateFilter mode)
	if m.state == stateFilter {
		b.WriteString("  ")
		b.WriteString(m.filter.View())
		b.WriteString("\n\n")
	}

	// Determine if any layouts or templates exist in the filtered list
	hasLayouts := false
	hasTemplates := false
	for _, item := range m.filtered {
		if item.Kind == KindLayout {
			hasLayouts = true
		} else if item.Kind == KindTemplate {
			hasTemplates = true
		}
	}

	// Empty state
	if len(m.filtered) == 0 {
		if m.filter.Value() != "" {
			b.WriteString("  ")
			b.WriteString(normalStyle.Render("No matches"))
			b.WriteString("\n")
		} else {
			b.WriteString("  ")
			b.WriteString(normalStyle.Render("No saved layouts. Press 's' to save."))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.statusBar())
		return b.String()
	}

	// Layouts section
	if hasLayouts {
		b.WriteString("  ")
		b.WriteString(categoryLabel.Render("Saved Layouts"))
		b.WriteString("\n")
		for idx, item := range m.filtered {
			if item.Kind != KindLayout {
				continue
			}
			b.WriteString(m.renderItem(idx, item))
		}
		b.WriteString("\n")
	}

	// Templates section
	if hasTemplates {
		b.WriteString("  ")
		b.WriteString(categoryLabel.Render("Templates"))
		b.WriteString("\n")
		for idx, item := range m.filtered {
			if item.Kind != KindTemplate {
				continue
			}
			b.WriteString(m.renderItem(idx, item))
		}
		b.WriteString("\n")
	}

	b.WriteString(m.statusBar())
	return b.String()
}

// renderItem renders a single list row with cursor, name, and description.
func (m Model) renderItem(idx int, item Item) string {
	var b strings.Builder
	if idx == m.cursor {
		b.WriteString("  ")
		b.WriteString(selectedStyle.Render("▸ "))
		b.WriteString(selectedStyle.Render(item.Title()))
	} else {
		b.WriteString("    ")
		b.WriteString(normalStyle.Render(item.Title()))
	}
	desc := item.Desc()
	if desc != "" {
		b.WriteString("  ")
		b.WriteString(descStyle.Render(desc))
	}
	b.WriteString("\n")
	return b.String()
}

// statusBar returns the key-hint line at the bottom.
func (m Model) statusBar() string {
	hints := []string{
		"↑/↓ navigate",
		"↵ restore",
		"/ filter",
		"s save",
		"d delete",
		"q quit",
	}
	return "  " + statusStyle.Render(strings.Join(hints, " · ")) + "\n"
}
