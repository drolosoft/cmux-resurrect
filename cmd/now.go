package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var nowCmd = &cobra.Command{
	Use:   "now",
	Short: "Show the current live workspaces and panes",
	Long:  "Renders the live workspace/pane tree from the running backend. Read-only — does not modify anything.\n\nThis is the CLI equivalent of the TUI 'now' command.",
	Args:  cobra.NoArgs,
	RunE:  runNow,
}

// nowIndent is the left margin of every line `crex now` prints.
const nowIndent = "  "

// nowGroupIndent is the extra margin of a workspace inside a group, the same
// step `crex show` uses.
const nowGroupIndent = "  "

func init() {
	rootCmd.AddCommand(nowCmd)
}

func runNow(cmd *cobra.Command, args []string) error {
	return renderNow(cmd.OutOrStdout(), newClient())
}

// renderNow prints the caller's live window. It is split from runNow so tests
// can hand it a fake backend instead of the live app.
func renderNow(w io.Writer, cl client.Backend) error {
	// The window this command was typed in, not whichever is frontmost —
	// the same rule save follows.
	win, err := orchestrate.LiveWindow(cl)
	if err != nil {
		return fmt.Errorf("read live state: %w", err)
	}

	o := newWF(w)
	o.f("%s\n", headingStyle.Render("  Current "+unitNameCap(2)))

	home, _ := os.UserHomeDir()

	// Group the window like the cmux sidebar, with the same helper `crex
	// show` and the TUI use; without group support it is one flat section.
	for _, section := range orchestrate.LiveSections(cl, win) {
		indent := nowIndent
		if section.Group != nil {
			o.f("%s%s\n", nowIndent, showGroupHeader(*section.Group))
			indent = nowIndent + nowGroupIndent
		}

		for _, position := range section.Members {
			nowWorkspace(o, cl, win.Workspaces[position], indent, section.IsAnchor(position), home)
		}
	}

	if len(win.Workspaces) == 0 {
		o.f("  %s\n", dimStyle.Render("No "+unitName(2)+" found."))
	}
	o.ln()
	return nil
}

// nowWorkspace prints one live workspace on a single line: title, badges,
// cwd and pane count. isAnchor adds the same "anchor" tag `crex show` uses,
// because the anchor's own title rarely matches its group's name.
func nowWorkspace(o wf, cl client.Backend, ws client.TreeWorkspace, indent string, isAnchor bool, home string) {
	var badges []string
	if isAnchor {
		badges = append(badges, dimStyle.Render("anchor"))
	}
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
	o.f("%s%s%s", indent, greenStyle.Render(ws.Title), badgeStr)

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
