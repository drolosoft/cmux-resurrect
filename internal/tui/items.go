package tui

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// ItemKind distinguishes between saved layouts and gallery templates.
type ItemKind int

const (
	KindLayout ItemKind = iota
	KindTemplate
	KindWorkspace
	KindAllWs
)

// Item is a single entry in the TUI list, representing either a saved layout
// or a gallery template.
type Item struct {
	Kind        ItemKind
	Name        string
	Description string
	Workspaces  int
	Icon        string
	Category    string
	SubItems    []Item

	// Group is the sidebar group a workspace item belongs to, nil when it is
	// loose. The browse view prints a header line above each run of items
	// that share a group, so the picker groups workspaces like `crex show`.
	Group *model.Group

	// Anchor marks the workspace that is its group's header in cmux.
	Anchor bool
}

// FilterValue returns the string used for fuzzy filtering.
// Implements list.Item from charmbracelet/bubbles.
func (i Item) FilterValue() string {
	return i.Name + " " + i.Description
}

// Title returns the display title for the list row.
// Templates include the icon prefix; layouts show the name only.
func (i Item) Title() string {
	if i.Kind == KindTemplate && i.Icon != "" {
		return i.Icon + " " + i.Name
	}
	return i.Name
}

// Desc returns the subtitle shown below the title in the list.
// Layouts show the workspace count and description; templates show description only.
func (i Item) Desc() string {
	switch i.Kind {
	case KindLayout:
		if i.Description != "" {
			return fmt.Sprintf("%d workspaces — %s", i.Workspaces, i.Description)
		}
		return fmt.Sprintf("%d workspaces", i.Workspaces)
	case KindWorkspace:
		if i.Description != "" {
			return i.Description
		}
		if i.Workspaces == 1 {
			return "1 pane"
		}
		return fmt.Sprintf("%d panes", i.Workspaces)
	case KindAllWs:
		return ""
	default:
		return i.Description
	}
}

// ItemsFromLayouts converts a slice of LayoutMeta into TUI Items.
func ItemsFromLayouts(metas []model.LayoutMeta) []Item {
	items := make([]Item, len(metas))
	for idx, m := range metas {
		// Sub-items follow the same grouping as `crex show`; a layout without
		// groups yields one loose section in file order, as before.
		var subItems []Item
		for _, section := range m.Sections() {
			for _, i := range section.Members {
				subItems = append(subItems, workspaceItem(m, i, section))
			}
		}
		items[idx] = Item{
			Kind:        KindLayout,
			Name:        m.Name,
			Description: m.Description,
			Workspaces:  m.WorkspaceCount,
			SubItems:    subItems,
		}
	}
	return items
}

// workspaceItem builds the picker row for the workspace at position i of the
// layout metadata, tagged with the section it was grouped into.
func workspaceItem(m model.LayoutMeta, i int, section model.Section) Item {
	panes := 0
	if i < len(m.WorkspacePanes) {
		panes = m.WorkspacePanes[i]
	}

	summary := ""
	if i < len(m.WorkspaceSummaries) {
		summary = m.WorkspaceSummaries[i]
	}

	return Item{
		Kind:        KindWorkspace,
		Name:        m.WorkspaceTitles[i],
		Description: summary,
		Workspaces:  panes,
		Group:       section.Group,
		Anchor:      section.IsAnchor(i),
	}
}

// ItemsFromTemplates converts a slice of Template pointers into TUI Items.
func ItemsFromTemplates(templates []*model.Template) []Item {
	items := make([]Item, len(templates))
	for idx, t := range templates {
		items[idx] = Item{
			Kind:        KindTemplate,
			Name:        t.Name,
			Description: t.Description,
			Icon:        t.Icon,
			Category:    t.Category,
		}
	}
	return items
}
