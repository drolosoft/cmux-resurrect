package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BrowseModel handles arrow-key navigation on a listing.
type BrowseModel struct {
	items       []Item
	visible     []Item
	cursor      int
	action      string // "restore", "use", "toggle" — the Enter action label
	filtering   bool
	filterText  string
	selected    bool
	done        bool
	passthrough rune // non-zero if user typed a letter (pass to prompt)
}

// NewBrowseModel creates a browse model from a list of items.
func NewBrowseModel(items []Item, action string) BrowseModel {
	vis := make([]Item, len(items))
	copy(vis, items)
	return BrowseModel{
		items:   items,
		visible: vis,
		action:  action,
	}
}

// SelectedItem returns the currently selected item.
func (bm BrowseModel) SelectedItem() Item {
	if bm.cursor < len(bm.visible) {
		return bm.visible[bm.cursor]
	}
	return Item{}
}

// Update processes key events in browse mode.
func (bm BrowseModel) Update(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	if bm.filtering {
		return bm.updateFilter(msg)
	}

	switch msg.Type {
	case tea.KeyDown:
		if bm.cursor < len(bm.visible)-1 {
			bm.cursor++
		}
		return bm, nil

	case tea.KeyUp:
		if bm.cursor > 0 {
			bm.cursor--
		}
		return bm, nil

	case tea.KeyEnter:
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyEsc:
		bm.done = true
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			switch r {
			case 'q':
				bm.done = true
				return bm, nil
			case '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			default:
				bm.done = true
				bm.passthrough = r
				return bm, nil
			}
		}
	}
	return bm, nil
}

func (bm BrowseModel) updateFilter(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		bm.filtering = false
		bm.filterText = ""
		bm.visible = make([]Item, len(bm.items))
		copy(bm.visible, bm.items)
		bm.cursor = 0
		return bm, nil

	case tea.KeyEnter:
		bm.filtering = false
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil

	case tea.KeyBackspace:
		if len(bm.filterText) > 0 {
			bm.filterText = bm.filterText[:len(bm.filterText)-1]
			bm.applyFilter()
		}
		return bm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			bm.filterText += string(msg.Runes[0])
			bm.applyFilter()
		}
		return bm, nil
	}
	return bm, nil
}

func (bm *BrowseModel) applyFilter() {
	if bm.filterText == "" {
		bm.visible = make([]Item, len(bm.items))
		copy(bm.visible, bm.items)
	} else {
		lower := strings.ToLower(bm.filterText)
		bm.visible = nil
		for _, item := range bm.items {
			if strings.Contains(strings.ToLower(item.FilterValue()), lower) {
				bm.visible = append(bm.visible, item)
			}
		}
	}
	bm.cursor = 0
}

// View renders the browse list with cursor and indices.
func (bm BrowseModel) View() string {
	var b strings.Builder

	for i, item := range bm.visible {
		idx := shellDimStyle.Render(fmt.Sprintf("[%d]", i+1))
		name := item.Title()
		desc := item.Desc()

		if i == bm.cursor {
			b.WriteString(fmt.Sprintf("  %s %s %s", shellCursorStyle.Render("▸"), idx, shellSuccessStyle.Render(name)))
		} else {
			b.WriteString(fmt.Sprintf("    %s %s", idx, name))
		}
		if desc != "" {
			b.WriteString("  ")
			b.WriteString(shellDimStyle.Render(desc))
		}
		b.WriteString("\n")
	}

	if bm.filtering {
		b.WriteString(fmt.Sprintf("  / %s", bm.filterText))
		b.WriteString(shellDimStyle.Render("▌"))
		b.WriteString("\n")
	} else {
		hint := fmt.Sprintf("  ↑/↓ select · ↵ %s · / filter · q back", bm.action)
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	}

	return b.String()
}
