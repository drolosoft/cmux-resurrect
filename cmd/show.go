package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var showRaw bool

// showIndent is the left margin of every workspace block in `crex show`.
const showIndent = "   "

// showGroupIndent is the extra margin of a workspace inside a group, enough
// to read it as nested under the header without wasting width.
const showGroupIndent = "  "

var showCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a saved layout",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showRaw, "raw", false, "show raw TOML content")
	showCmd.ValidArgsFunction = completeLayoutNames
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	store, err := newStore()
	if err != nil {
		return err
	}

	o := newWF(cmd.OutOrStdout())

	if showRaw {
		data, err := os.ReadFile(store.Path(name))
		if err != nil {
			return fmt.Errorf("layout %q not found", name)
		}
		o.f("%s", string(data))
		return nil
	}

	layout, err := store.Load(name)
	if err != nil {
		return err
	}

	// Header
	o.f("\n%s\n", headingStyle.Render("📦 "+layout.Name))
	if layout.Description != "" {
		o.f("   %s\n", dimStyle.Render(layout.Description))
	}
	saved := layout.SavedAt.Local().Format("Jan 02, 2006 15:04")
	o.f("   %s\n", dimStyle.Render(fmt.Sprintf("Saved %s · %d %s", saved, len(layout.Workspaces), unitName(len(layout.Workspaces)))))
	o.ln()

	// Workspaces sit under their group headers like in the cmux sidebar; a
	// layout without groups comes back as one loose section, printed exactly
	// as before groups existed.
	for _, section := range layout.Sections() {
		indent := showIndent
		if section.Group != nil {
			o.f("%s%s\n", showIndent, showGroupHeader(*section.Group))
			indent = showIndent + showGroupIndent
		}

		for _, position := range section.Members {
			showWorkspace(o, layout.Workspaces[position], indent, section.IsAnchor(position))
		}
	}

	return nil
}

// showWorkspace prints one workspace block: title and badges, description,
// cwd and the pane tree. indent is today's three spaces for loose workspaces
// and one level deeper for group members, so a layout without groups prints
// byte for byte what it printed before. isAnchor tags the workspace that is
// its group's header in cmux, whose own title often differs from the group
// name ("Group 7" heading "🎮 Cmux").
func showWorkspace(o wf, ws model.Workspace, indent string, isAnchor bool) {
	// Workspace title with badges
	title := greenStyle.Render(ws.Title)
	badges := ""

	// An anchor the group header already names prints as "(anchor)", the
	// way save and restore list it; one the user titled keeps its title.
	if isAnchor && model.AnchorShownAsGroup(ws.Title, ws.Group) {
		title = dimStyle.Render("(anchor)")
	} else if isAnchor {
		badges += " " + dimStyle.Render("anchor")
	}
	if ws.Pinned {
		badges += " " + cyanStyle.Render("📌")
	}
	if ws.Active {
		badges += " " + yellowStyle.Render("◀ active")
	}
	o.f("%s%s%s\n", indent, title, badges)
	if ws.Description != "" {
		o.f("%s%s\n", indent, dimStyle.Render(ws.Description))
	}

	// CWD
	o.f("%s%s\n", indent, dimStyle.Render("cwd "+ws.CWD))

	// Panes as a tree
	for i, p := range ws.Panes {
		isLast := i == len(ws.Panes)-1
		prefix := "├──"
		if isLast {
			prefix = "└──"
		}
		prefix = dimStyle.Render(prefix)

		// Build pane description
		var desc string
		if p.Split != "" {
			desc = magentaStyle.Render("→"+p.Split) + " "
		}
		switch {
		case p.Type == "browser":
			url := p.URL
			if len(url) > 50 {
				url = url[:47] + "..."
			}
			if url != "" {
				desc += cyanStyle.Render("🌐 " + url)
			} else {
				desc += dimStyle.Render("🌐 browser")
			}
		case p.Command != "":
			cmd := p.Command
			if len(cmd) > 50 {
				cmd = cmd[:47] + "..."
			}
			desc += cyanStyle.Render(cmd)
		default:
			desc += dimStyle.Render("shell")
		}
		if p.Focus {
			desc += " " + yellowStyle.Render("★")
		}

		o.f("%s%s %s\n", indent, prefix, desc)
	}
	o.ln()
}

// showGroupHeader renders a group's header line: its name and the same
// badges the TUI uses, pinned, collapsed, colour and icon. `crex show`,
// `crex now`, `crex save` and `crex restore` all print it.
func showGroupHeader(group model.Group) string {
	header := groupStyle.Render(group.Name)
	if group.Pinned {
		header += " " + cyanStyle.Render("📌")
	}
	if group.Collapsed {
		header += " " + dimStyle.Render("▸ collapsed")
	}

	// The swatch is painted in the group's own colour; the hex code next to
	// it keeps the colour readable where the terminal shows no colour.
	if group.Color != "" {
		swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(group.Color)).Render("■")
		header += " " + swatch + " " + dimStyle.Render(group.Color)
	}

	// The icon is an SF Symbol name, which a terminal cannot draw, so it is
	// spelled out.
	if group.Icon != "" {
		header += " " + dimStyle.Render("icon "+group.Icon)
	}

	return header
}
