package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var nowCmd = &cobra.Command{
	Use:   "now",
	Short: "Show the current live workspaces and panes",
	Long:  "Renders the live workspace/pane tree from the running backend. Read-only — does not modify anything.\n\nThis is the CLI equivalent of the TUI 'now' command.",
	Args:  cobra.NoArgs,
	RunE:  runNow,
}

func init() {
	rootCmd.AddCommand(nowCmd)
}

func runNow(cmd *cobra.Command, args []string) error {
	cl := newClient()
	tree, err := cl.Tree()
	if err != nil {
		return fmt.Errorf("read live state: %w", err)
	}

	o := newWF(cmd.OutOrStdout())
	o.f("%s\n", headingStyle.Render("  Current "+unitNameCap(2)))

	home, _ := os.UserHomeDir()
	total := 0
	for _, win := range tree.Windows {
		for _, ws := range win.Workspaces {
			total++

			var badges []string
			if ws.Pinned {
				badges = append(badges, "📌")
			}
			if ws.Active || ws.Selected {
				badges = append(badges, "●")
			}
			badgeStr := ""
			if len(badges) > 0 {
				badgeStr = " " + strings.Join(badges, " ")
			}
			o.f("  %s%s", greenStyle.Render(ws.Title), badgeStr)

			// CWD from sidebar-state (best effort).
			if ws.Ref != "" {
				if sidebar, err := cl.SidebarState(ws.Ref); err == nil && sidebar.CWD != "" {
					cwd := sidebar.CWD
					if home != "" {
						cwd = strings.Replace(cwd, home, "~", 1)
					}
					o.f("  %s", dimStyle.Render(cwd))
				}
			}

			if len(ws.Panes) > 0 {
				o.f("  %s", dimStyle.Render(fmt.Sprintf("%d pane(s)", len(ws.Panes))))
			}
			o.ln()
		}
	}

	if total == 0 {
		o.f("  %s\n", dimStyle.Render("No "+unitName(2)+" found."))
	}
	o.ln()
	return nil
}
