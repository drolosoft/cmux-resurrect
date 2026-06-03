package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
	"github.com/spf13/cobra"
)

var popLast bool

var popCmd = &cobra.Command{
	Use:   "pop [name] [path]",
	Short: "Quick workspace launcher",
	Long: `Open a picker to launch saved layouts or templates instantly.

With no arguments, shows a filterable picker. With arguments, launches directly:
  crex pop morning        restore a saved layout
  crex pop ide .          apply a template to a directory
  crex pop --last         restore the most recently saved layout`,
	Args:              cobra.MaximumNArgs(2),
	RunE:              runPop,
	ValidArgsFunction: completePopArgs,
}

func init() {
	popCmd.Flags().BoolVar(&popLast, "last", false, "restore the most recently saved layout")
	rootCmd.AddCommand(popCmd)
}

func runPop(cmd *cobra.Command, args []string) error {
	if popLast {
		return popRestoreLast()
	}
	if len(args) > 0 {
		return popDirect(args)
	}
	return popPicker()
}

// popPicker launches the bubbletea PopModel and dispatches the chosen item.
func popPicker() error {
	items, err := buildPopItems()
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No layouts or templates found."))
		return nil
	}

	m := tui.NewPopModel(items, 0, 0)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("picker: %w", err)
	}

	pm, ok := finalModel.(*tui.PopModel)
	if !ok {
		return nil
	}

	chosen := pm.Chosen()
	if chosen == nil {
		return nil
	}

	switch chosen.Kind {
	case "layout":
		return doRestore(chosen.Name)
	case "template":
		return doTemplateUse(chosen.Name, ".")
	}
	return nil
}

// popDirect tries name as a layout first, then as a template.
func popDirect(args []string) error {
	name := args[0]
	path := "."
	if len(args) >= 2 {
		path = args[1]
	}

	store, err := newStore()
	if err != nil {
		return err
	}

	if store.Exists(name) {
		return doRestore(name)
	}

	if _, ok := gallery.Get(name); ok {
		return doTemplateUse(name, path)
	}

	return fmt.Errorf("no layout or template named %q", name)
}

// popRestoreLast finds the most recently saved layout and restores it.
func popRestoreLast() error {
	store, err := newStore()
	if err != nil {
		return err
	}

	metas, err := store.List()
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No saved layouts. Use 'crex save <name>' to create one."))
		return nil
	}

	// Sort by SavedAt descending to find most recent.
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].SavedAt.After(metas[j].SavedAt)
	})

	latest := metas[0]
	fmt.Fprintf(os.Stderr, "%s %s %s\n",
		yellowStyle.Render("➕ Syncing (add)"),
		greenStyle.Render(latest.Name),
		dimStyle.Render("(last saved "+formatAge(latest.SavedAt)+")"))

	return doRestore(latest.Name)
}

// doRestore creates a Restorer and calls Restore with mode Add and skipMatching=true.
func doRestore(name string) error {
	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	restorer := &orchestrate.Restorer{
		Client: cl,
		Store:  store,
		OnProgress: func(title string, panes int, err error) {
			t := padTitle(title)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "skipped") {
					fmt.Fprintf(os.Stderr, "  %s  %s %s\n", dimStyle.Render("SKIP"), t, dimStyle.Render("("+errMsg+")"))
				} else {
					fmt.Fprintf(os.Stderr, "  %s  %s: %v\n", yellowStyle.Render("FAIL"), t, err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n", greenStyle.Render("OK"), t, panes)
			}
		},
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("➕ Syncing (add)"), greenStyle.Render(name))

	result, err := restorer.Restore(name, false, orchestrate.RestoreModeAdd, "", true)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	if result.WorkspacesClosed > 0 {
		fmt.Fprintf(os.Stderr, "  %s\n", dimStyle.Render(fmt.Sprintf("Closed %d existing %s", result.WorkspacesClosed, unitName(result.WorkspacesClosed))))
	}
	fmt.Fprintf(os.Stderr, "  %s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", yellowStyle.Render("⚠️  Errors:"))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", dimStyle.Render("• "+e))
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

// doTemplateUse applies a gallery template to a directory.
func doTemplateUse(templateName, path string) error {
	tmpl, ok := gallery.Get(templateName)
	if !ok {
		return fmt.Errorf("template %q not found in gallery", templateName)
	}

	panes := gallery.BuildPanes(tmpl)

	cwd := config.ExpandHome(path)
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	icon := ""
	if tmpl.Category == "workflow" {
		icon = tmpl.Icon
	}

	cl := newClient()
	if err := cl.Ping(); err != nil {
		return fmt.Errorf("backend not reachable: %w", err)
	}

	user := &orchestrate.TemplateUser{
		Client: cl,
		OnProgress: func(msg string) {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", dimStyle.Render("INFO"), msg)
		},
	}

	result, err := user.Use(panes, orchestrate.TemplateUseOpts{
		Icon: icon,
		CWD:  absCWD,
	}, false)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s  %s %s (%d panes)\n\n",
		greenStyle.Render("OK"),
		greenStyle.Render(padTitle(result.Title)),
		dimStyle.Render("from "+tmpl.Name),
		result.Panes)
	return nil
}

// buildPopItems collects saved layouts and gallery templates into []tui.PopItem.
func buildPopItems() ([]tui.PopItem, error) {
	var items []tui.PopItem

	// Layouts first.
	store, err := newStore()
	if err != nil {
		return nil, err
	}
	metas, err := store.List()
	if err != nil {
		return nil, err
	}
	for _, m := range metas {
		meta := fmt.Sprintf("%d %s  %s", m.WorkspaceCount, unitName(m.WorkspaceCount), formatAge(m.SavedAt))
		items = append(items, tui.PopItem{
			Kind: "layout",
			Name: m.Name,
			Meta: meta,
		})
	}

	// Templates second.
	for _, tmpl := range gallery.List() {
		meta := tmpl.Description
		items = append(items, tui.PopItem{
			Kind: "template",
			Name: tmpl.Name,
			Icon: tmpl.Icon,
			Meta: meta,
		})
	}

	return items, nil
}

// completePopArgs provides tab completion for `crex pop`.
func completePopArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// First arg: layout names + template names.
		var names []string

		store, err := newStore()
		if err == nil {
			metas, err := store.List()
			if err == nil {
				for _, m := range metas {
					desc := fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount))
					names = append(names, fmt.Sprintf("%s\t%s", m.Name, desc))
				}
			}
		}
		for _, tmpl := range gallery.List() {
			names = append(names, fmt.Sprintf("%s\t%s %s", tmpl.Name, tmpl.Icon, tmpl.Description))
		}
		return names, cobra.ShellCompDirectiveNoFileComp

	case 1:
		// Second arg: directory (only relevant if first arg is a template).
		return nil, cobra.ShellCompDirectiveFilterDirs

	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// formatAge returns a human-readable time since t: "3h ago", "2d ago", or "Jan 28".
func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}
