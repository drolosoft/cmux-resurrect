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

	// Two-level drill-in state (restore action only).
	inDetail     bool
	parentItems  []Item
	parentCursor int
	layoutName   string
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

// drillIn enters the detail view for the currently selected layout item.
func (bm *BrowseModel) drillIn() {
	if bm.cursor >= len(bm.visible) {
		return
	}
	item := bm.visible[bm.cursor]
	if len(item.SubItems) == 0 {
		return
	}

	bm.parentItems = bm.visible
	bm.parentCursor = bm.cursor
	bm.layoutName = item.Name

	detail := make([]Item, 0, len(item.SubItems)+1)
	detail = append(detail, Item{
		Kind:       KindAllWs,
		Name:       fmt.Sprintf("All workspaces (%d)", len(item.SubItems)),
		Workspaces: len(item.SubItems),
	})
	detail = append(detail, item.SubItems...)

	bm.visible = detail
	bm.items = detail
	bm.cursor = 0
	bm.inDetail = true
	bm.filtering = false
	bm.filterText = ""
}

// drillOut returns from the detail view to the parent layout list.
func (bm *BrowseModel) drillOut() {
	bm.visible = bm.parentItems
	bm.items = bm.parentItems
	bm.cursor = bm.parentCursor
	bm.inDetail = false
	bm.layoutName = ""
	bm.parentItems = nil
	bm.filtering = false
	bm.filterText = ""
}

// SelectedItem returns the currently selected item.
func (bm BrowseModel) SelectedItem() Item {
	if bm.cursor < len(bm.visible) {
		return bm.visible[bm.cursor]
	}
	return Item{}
}

// Done reports whether the browse interaction has completed.
func (bm BrowseModel) Done() bool { return bm.done }

// Selected reports whether an item was selected (vs cancelled).
func (bm BrowseModel) Selected() bool { return bm.selected }

// LayoutName returns the layout name when in detail view (set by drillIn).
func (bm BrowseModel) LayoutName() string { return bm.layoutName }

// Update processes key events in browse mode.
func (bm BrowseModel) Update(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
	if bm.filtering {
		return bm.updateFilter(msg)
	}

	if bm.inDetail {
		return bm.updateDetail(msg)
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

	case tea.KeyRight, tea.KeyTab:
		if bm.action == "restore" && len(bm.visible) > 0 {
			bm.drillIn()
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
			switch {
			case r == 'q':
				bm.done = true
				return bm, nil
			case r == '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			case r >= '1' && r <= '9':
				idx := int(r - '1') // '1' → 0, '2' → 1, etc.
				if idx < len(bm.visible) {
					bm.cursor = idx
				}
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

// updateDetail processes key events when in the detail (workspace picker) view.
func (bm BrowseModel) updateDetail(msg tea.KeyMsg) (BrowseModel, tea.Cmd) {
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
	case tea.KeyLeft, tea.KeyEsc, tea.KeyShiftTab, tea.KeyBackspace:
		bm.drillOut()
		return bm, nil
	case tea.KeyEnter:
		if len(bm.visible) > 0 {
			bm.selected = true
			bm.done = true
		}
		return bm, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			r := msg.Runes[0]
			switch {
			case r == 'q':
				bm.drillOut()
				return bm, nil
			case r == '/':
				bm.filtering = true
				bm.filterText = ""
				return bm, nil
			case r >= '1' && r <= '9':
				idx := int(r - '1')
				if idx < len(bm.visible) {
					bm.cursor = idx
				}
				return bm, nil
			default:
				bm.drillOut()
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

	// Show breadcrumb title when in detail view.
	if bm.inDetail && bm.layoutName != "" {
		fmt.Fprintf(&b, "  %s\n\n", shellDimStyle.Render(fmt.Sprintf("Restore from %q:", bm.layoutName)))
	}

	for i, item := range bm.visible {
		idx := shellDimStyle.Render(fmt.Sprintf("[%d]", i+1))
		name := item.Title()
		desc := item.Desc()

		if i == bm.cursor {
			fmt.Fprintf(&b, "  %s %s %s", shellCursorStyle.Render("▸"), idx, shellSuccessStyle.Render(name))
		} else {
			fmt.Fprintf(&b, "    %s %s", idx, name)
		}
		if desc != "" {
			b.WriteString("  ")
			b.WriteString(shellDimStyle.Render(desc))
		}
		b.WriteString("\n")
	}

	switch {
	case bm.filtering:
		fmt.Fprintf(&b, "  / %s", bm.filterText)
		b.WriteString(shellDimStyle.Render("▌"))
		b.WriteString("\n")
	case bm.inDetail:
		hint := "  ↑/↓ select · ↵ restore · / filter · ←/esc back"
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	default:
		hint := fmt.Sprintf("  ↑/↓ select · ↵ %s · / filter · q back", bm.action)
		if bm.action == "restore" {
			hint = "  ↑/↓ select · ↵ restore · →/tab pick workspace · / filter · esc cancel"
		}
		b.WriteString(shellDimStyle.Render(hint))
		b.WriteString("\n")
	}
	b.WriteString(" \n")

	return b.String()
}
