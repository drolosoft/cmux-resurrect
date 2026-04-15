package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

// completeLayoutNames provides dynamic completion of saved layout names.
// Used by: save, restore, delete, show, edit, watch.
func completeLayoutNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	store, err := newStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	metas, err := store.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(metas))
	for _, m := range metas {
		desc := m.Description
		if desc == "" {
			desc = fmt.Sprintf("%d workspaces", m.WorkspaceCount)
		}
		names = append(names, fmt.Sprintf("%s\t%s", m.Name, desc))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeTemplateNames provides dynamic completion of gallery template names.
// Used by: template show.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, tmpl := range gallery.List() {
		names = append(names, fmt.Sprintf("%s\t%s %s", tmpl.Name, tmpl.Icon, tmpl.Description))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeWorkspaceNames provides dynamic completion of project names
// from the Workspace Blueprint.
// Used by: ws remove, ws toggle.
func completeWorkspaceNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names := make([]string, 0, len(wf.Projects))
	for _, p := range wf.Projects {
		state := "enabled"
		if !p.Enabled {
			state = "disabled"
		}
		names = append(names, fmt.Sprintf("%s\t%s %s (%s)", p.Name, p.Icon, p.Template, state))
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
