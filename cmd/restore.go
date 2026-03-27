package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var restoreDryRun bool
var restoreMode string

var restoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Restore a saved cmux layout",
	Long:  "Recreates workspaces, splits, and sends commands from a saved layout.\n\nYou will be asked whether to replace your current workspaces or add to them.\nUse --mode to skip the interactive prompt (useful for scripts).\n\nIf no layout name is given, an interactive picker is shown.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show commands without executing")
	restoreCmd.Flags().StringVar(&restoreMode, "mode", "", "restore mode: \"replace\" or \"add\" (skip interactive prompt)")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	var name string
	if len(args) == 1 {
		name = args[0]
	} else {
		// Interactive picker.
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
		picked, err := pickLayout(metas)
		if err != nil {
			return err
		}
		name = picked
	}

	// Validate --mode flag value early.
	if restoreMode != "" && restoreMode != "replace" && restoreMode != "add" {
		return fmt.Errorf("invalid --mode %q: must be \"replace\" or \"add\"", restoreMode)
	}

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

	// Determine restore mode.
	var mode orchestrate.RestoreMode
	switch {
	case restoreMode == "replace":
		mode = orchestrate.RestoreModeReplace
	case restoreMode == "add":
		mode = orchestrate.RestoreModeAdd
	case restoreDryRun:
		// Dry-run without explicit --mode defaults to "add" (non-destructive preview).
		mode = orchestrate.RestoreModeAdd
	default:
		// Interactive prompt.
		mode, err = askRestoreMode()
		if err != nil {
			return err
		}
	}

	if restoreDryRun {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("👁️  Dry-run restore of"), greenStyle.Render(name))
	} else {
		action := "🔄 Replacing with"
		if mode == orchestrate.RestoreModeAdd {
			action = "➕ Adding from"
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render(action), greenStyle.Render(name))
	}

	result, err := restorer.Restore(name, restoreDryRun, mode)
	if err != nil {
		return err
	}

	if restoreDryRun {
		fmt.Fprintln(os.Stderr)
		for _, c := range result.Commands {
			switch {
			case c == "":
				fmt.Println()
			case strings.HasPrefix(c, "#"):
				fmt.Println(yellowStyle.Render(c))
			default:
				// Color the cmux prefix dim, highlight the action
				parts := strings.SplitN(c, " ", 3)
				if len(parts) >= 2 {
					fmt.Printf("%s %s", dimStyle.Render(parts[0]), cyanStyle.Render(parts[1]))
					if len(parts) == 3 {
						fmt.Printf(" %s", dimStyle.Render(parts[2]))
					}
					fmt.Println()
				} else {
					fmt.Println(c)
				}
			}
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ %d commands for %d workspaces", len(result.Commands)-countBlanks(result.Commands), result.WorkspacesTotal)))
		return nil
	}

	fmt.Fprintln(os.Stderr)
	if result.WorkspacesClosed > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render(fmt.Sprintf("  Closed %d existing workspaces", result.WorkspacesClosed)))
	}
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render(fmt.Sprintf("✅ Restored %d/%d workspaces", result.WorkspacesOK, result.WorkspacesTotal)))
	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", yellowStyle.Render("⚠️  Errors:"))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", dimStyle.Render("• "+e))
		}
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

func countBlanks(cmds []string) int {
	n := 0
	for _, c := range cmds {
		if c == "" || strings.HasPrefix(c, "#") {
			n++
		}
	}
	return n
}

// askRestoreMode prompts the user to choose between replacing or adding workspaces.
func askRestoreMode() (orchestrate.RestoreMode, error) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n", headingStyle.Render("How do you want to restore?"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cyanStyle.Render("[r]"), "Replace — close all current workspaces, then restore")
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cyanStyle.Render("[a]"), "Add     — keep current workspaces, add restored ones")
	fmt.Fprintf(os.Stderr, "\n%s ", dimStyle.Render("Choice [r/a]:"))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return orchestrate.RestoreModeReplace, fmt.Errorf("read input: %w", err)
	}

	switch strings.TrimSpace(strings.ToLower(input)) {
	case "a", "add":
		return orchestrate.RestoreModeAdd, nil
	case "r", "replace", "":
		return orchestrate.RestoreModeReplace, nil
	default:
		return orchestrate.RestoreModeReplace, fmt.Errorf("invalid choice %q — use 'r' or 'a'", strings.TrimSpace(input))
	}
}
