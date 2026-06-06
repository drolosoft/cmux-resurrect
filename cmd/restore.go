package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

// readKey reads a single keypress in raw terminal mode.
func readKey() (byte, error) {
	state, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return 0, fmt.Errorf("cancelled")
	}
	buf := make([]byte, 3)
	n, _ := os.Stdin.Read(buf)
	term.Restore(os.Stdin.Fd(), state)
	if n == 0 {
		return 0, fmt.Errorf("cancelled")
	}
	return buf[0], nil
}

var restoreDryRun bool
var restoreMode string

var restoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Restore a saved layout",
	Long:  "Recreates tabs, pane arrangements, and sends commands from a saved layout.\n\nYou will be asked whether to replace your current tabs or add to them.\nUse --mode to skip the interactive prompt (useful for scripts).\n\nIf no layout name is given, an interactive picker is shown.",
	Args:  cobra.MaximumNArgs(2),
	RunE:  runRestore,
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "show commands without executing")
	restoreCmd.Flags().StringVar(&restoreMode, "mode", "", "restore mode: \"replace\" or \"add\" (skip interactive prompt)")
	restoreCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeLayoutNames(cmd, args, toComplete)
		case 1:
			return completeWorkspaceNames(cmd, args, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	_ = restoreCmd.RegisterFlagCompletionFunc("mode", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"replace\tClose existing " + unitName(2) + " first",
			"add\tKeep existing " + unitName(2),
		}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	var name string
	var workspaceFilter string
	var pickerMode orchestrate.RestoreMode
	var pickerModeSet bool
	switch len(args) {
	case 2:
		name = args[0]
		workspaceFilter = args[1]
	case 1:
		name = args[0]
	default:
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
		// Skip the mode step inside the picker if it's already determined.
		skipMode := restoreMode != "" || restoreDryRun || cfg.RestoreMode != ""
		pick, err := pickLayout(metas, skipMode)
		if err != nil {
			return err
		}
		name = pick.Layout
		workspaceFilter = pick.Workspace
		if !skipMode {
			pickerMode = pick.Mode
			pickerModeSet = true
		}
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
		Client:     cl,
		Store:      store,
		AutoAccept: cfg.AutoAccept,
		SkipPing:   os.Getenv("CREX_BACKEND") != "",
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
	skipMatching := true // default: keep matching tabs
	switch {
	case restoreMode == "replace":
		mode = orchestrate.RestoreModeReplace
	case restoreMode == "add":
		mode = orchestrate.RestoreModeAdd
	case restoreDryRun:
		mode = orchestrate.RestoreModeAdd
	case pickerModeSet:
		mode = pickerMode
	default:
		switch cfg.RestoreMode {
		case "replace":
			mode = orchestrate.RestoreModeReplace
		case "add":
			mode = orchestrate.RestoreModeAdd
		default:
			if workspaceFilter != "" {
				mode = orchestrate.RestoreModeAdd
			} else {
				layout, loadErr := store.Load(name)
				if loadErr != nil {
					return loadErr
				}
				titles := make([]string, len(layout.Workspaces))
				for i, ws := range layout.Workspaces {
					titles[i] = ws.Title
				}
				detection := orchestrate.DetectRestoreState(cl, titles)

				switch detection.Hint {
				case orchestrate.HintNoop:
					fmt.Fprintln(os.Stderr)
					fmt.Fprintln(os.Stderr, dimStyle.Render("Layout already matches current tabs. Nothing to do."))
					return nil
				case orchestrate.HintAutoAdd:
					mode = orchestrate.RestoreModeAdd
				case orchestrate.HintAskFresh:
					// Matching tabs exist but no extras — Replace/Add are equivalent
					// for closing. Just ask Skip/Fresh directly.
					mode = orchestrate.RestoreModeAdd
					skip, err := promptSkipMatching()
					if err != nil {
						fmt.Fprintln(os.Stderr, "\n"+dimStyle.Render("Cancelled."))
						return nil
					}
					skipMatching = skip
				case orchestrate.HintAskMode:
					prompted, err := promptRestoreMode()
					if err != nil {
						fmt.Fprintln(os.Stderr, "\n"+dimStyle.Render("Cancelled."))
						return nil
					}
					mode = prompted
					skipMatching = false
				case orchestrate.HintAskBoth:
					prompted, err := promptRestoreMode()
					if err != nil {
						fmt.Fprintln(os.Stderr, "\n"+dimStyle.Render("Cancelled."))
						return nil
					}
					mode = prompted
					if mode == orchestrate.RestoreModeReplace {
						skip, err := promptSkipMatching()
						if err != nil {
							fmt.Fprintln(os.Stderr, "\n"+dimStyle.Render("Cancelled."))
							return nil
						}
						skipMatching = skip
					}
				}
			}
		}
	}

	if restoreDryRun {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render("👁️  Dry-run restore of"), greenStyle.Render(name))
	} else {
		action := "🔄 Syncing (replace)"
		if mode == orchestrate.RestoreModeAdd {
			action = "➕ Syncing (add)"
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s %s\n", yellowStyle.Render(action), greenStyle.Render(name))
	}

	result, err := restorer.Restore(name, restoreDryRun, mode, workspaceFilter, skipMatching)
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
				fmt.Println("  " + yellowStyle.Render(c))
			default:
				// Color the cmux prefix dim, highlight the action
				parts := strings.SplitN(c, " ", 3)
				if len(parts) >= 2 {
					fmt.Printf("  %s %s", dimStyle.Render(parts[0]), cyanStyle.Render(parts[1]))
					if len(parts) == 3 {
						fmt.Printf(" %s", dimStyle.Render(parts[2]))
					}
					fmt.Println()
				} else {
					fmt.Println("  " + c)
				}
			}
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  %s\n\n",
			greenStyle.Render(fmt.Sprintf("✅ %d commands for %d %s", len(result.Commands)-countBlanks(result.Commands), result.WorkspacesTotal, unitName(result.WorkspacesTotal))))
		return nil
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

func promptRestoreMode() (orchestrate.RestoreMode, error) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "How do you want to restore?\n\n")
	fmt.Fprintf(os.Stderr, "  %s  Close non-matching %s, keep matching\n", cyanStyle.Render("[r]eplace"), unitName(2))
	fmt.Fprintf(os.Stderr, "  %s  Keep all existing %s, add missing\n\n", cyanStyle.Render("[a]dd"), unitName(2))
	fmt.Fprintf(os.Stderr, "Choice [r/a]: ")

	for {
		key, err := readKey()
		if err != nil {
			return 0, err
		}
		switch key {
		case 'r', 'R':
			fmt.Fprintln(os.Stderr)
			return orchestrate.RestoreModeReplace, nil
		case 'a', 'A':
			fmt.Fprintln(os.Stderr)
			return orchestrate.RestoreModeAdd, nil
		case 0x1b, 'q', 'Q', 0x03:
			fmt.Fprintln(os.Stderr)
			return 0, fmt.Errorf("cancelled")
		}
	}
}

func promptSkipMatching() (bool, error) {
	fmt.Fprintf(os.Stderr, "\nTabs already open match the layout.\n\n")
	fmt.Fprintf(os.Stderr, "  %s  Leave matching tabs as they are\n", cyanStyle.Render("[s]kip"))
	fmt.Fprintf(os.Stderr, "  %s  Close and recreate from layout\n\n", cyanStyle.Render("[f]resh"))
	fmt.Fprintf(os.Stderr, "Choice [s/f]: ")

	for {
		key, err := readKey()
		if err != nil {
			return true, err
		}
		switch key {
		case 's', 'S':
			fmt.Fprintln(os.Stderr)
			return true, nil
		case 'f', 'F':
			fmt.Fprintln(os.Stderr)
			return false, nil
		case 0x1b, 'q', 'Q', 0x03:
			fmt.Fprintln(os.Stderr)
			return true, fmt.Errorf("cancelled")
		}
	}
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
