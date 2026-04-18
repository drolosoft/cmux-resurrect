package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive workspace launcher",
	Long:  "Browse and launch saved layouts and gallery templates in a fuzzy-search TUI.",
	Args:  cobra.NoArgs,
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return err
	}
	metas, err := store.List()
	if err != nil {
		return err
	}

	templates := gallery.List()

	layoutItems := tui.ItemsFromLayouts(metas)
	templateItems := tui.ItemsFromTemplates(templates)

	m := tui.NewModel(layoutItems, templateItems)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	final, ok := finalModel.(tui.Model)
	if !ok {
		return nil
	}

	action := final.Action()
	if action == nil {
		return nil
	}

	switch a := action.(type) {
	case tui.SelectedAction:
		return handleTUISelect(a.Item)
	case tui.SaveAction:
		return handleTUISave()
	case tui.DeleteAction:
		return handleTUIDelete(a.Item)
	}
	return nil
}

func handleTUISelect(item tui.Item) error {
	switch item.Kind {
	case tui.KindLayout:
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
					fmt.Fprintf(os.Stderr, "  %s  %s: %v\n", yellowStyle.Render("FAIL"), t, err)
				} else {
					fmt.Fprintf(os.Stderr, "  %s  %s (%d panes)\n", greenStyle.Render("OK"), t, panes)
				}
			},
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("Adding"), greenStyle.Render(item.Name))
		result, err := restorer.Restore(item.Name, false, orchestrate.RestoreModeAdd)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("Restored %d/%d %s", result.WorkspacesOK, result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
		return nil

	case tui.KindTemplate:
		return runTemplateUse(nil, []string{item.Name})
	}
	return nil
}

func handleTUISave() error {
	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	saver := &orchestrate.Saver{Client: cl, Store: store}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("Saving layout"), greenStyle.Render("default"))

	layout, err := saver.Save("default", "")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	for _, ws := range layout.Workspaces {
		pin := ""
		if ws.Pinned {
			pin = " pinned"
		}
		panes := ""
		if len(ws.Panes) > 1 {
			panes = fmt.Sprintf(" (%d panes)", len(ws.Panes))
		}
		fmt.Fprintf(os.Stderr, "  %s  %s%s%s\n",
			greenStyle.Render("OK"),
			padTitle(ws.Title),
			dimStyle.Render(panes),
			pin)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("Saved %d %s to %s", len(layout.Workspaces), unitName(len(layout.Workspaces)), store.Path("default"))))
	return nil
}

func handleTUIDelete(item tui.Item) error {
	store, err := newStore()
	if err != nil {
		return err
	}
	if !store.Exists(item.Name) {
		return fmt.Errorf("layout %q not found", item.Name)
	}
	if err := store.Delete(item.Name); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render(fmt.Sprintf("Deleted layout %q", item.Name)))
	return nil
}
