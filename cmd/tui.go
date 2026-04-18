package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/drolosoft/cmux-resurrect/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive shell",
	Long:  "Launch the crex interactive shell for browsing layouts, templates, and live state.",
	Args:  cobra.NoArgs,
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	cl := newClient()
	backend := cachedBackend

	m := tui.NewShellModel(store, cl, backend, cfg.WorkspaceFile)
	p := tea.NewProgram(m) // no AltScreen — inline shell
	_, err = p.Run()
	return err
}
