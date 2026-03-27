package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// pickLayout shows an interactive selector and lets the user pick a layout.
func pickLayout(metas []model.LayoutMeta) (string, error) {
	options := make([]huh.Option[string], len(metas))
	for i, m := range metas {
		label := fmt.Sprintf("%s  %d workspaces", m.Name, m.WorkspaceCount)
		if m.Description != "" {
			desc := m.Description
			if len(desc) > 35 {
				desc = desc[:32] + "..."
			}
			label += "  " + desc
		}
		options[i] = huh.NewOption(label, m.Name)
	}

	var selected string
	err := huh.NewSelect[string]().
		Title("📦 Select a layout to restore").
		Options(options...).
		Value(&selected).
		Run()
	if err != nil {
		return "", err
	}

	return selected, nil
}
