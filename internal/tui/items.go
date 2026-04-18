package tui

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// ItemKind distinguishes between saved layouts and gallery templates.
type ItemKind int

const (
	KindLayout   ItemKind = iota
	KindTemplate ItemKind = iota
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
	if i.Kind == KindLayout {
		if i.Description != "" {
			return fmt.Sprintf("%d workspaces — %s", i.Workspaces, i.Description)
		}
		return fmt.Sprintf("%d workspaces", i.Workspaces)
	}
	return i.Description
}

// ItemsFromLayouts converts a slice of LayoutMeta into TUI Items.
func ItemsFromLayouts(metas []model.LayoutMeta) []Item {
	items := make([]Item, len(metas))
	for idx, m := range metas {
		items[idx] = Item{
			Kind:        KindLayout,
			Name:        m.Name,
			Description: m.Description,
			Workspaces:  m.WorkspaceCount,
		}
	}
	return items
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
