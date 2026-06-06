package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	listJSON   bool
	listAlfred bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List saved layouts",
	Args:    cobra.NoArgs,
	RunE:    runList,
	Aliases: []string{"ls"},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON array")
	listCmd.Flags().BoolVar(&listAlfred, "alfred", false, "output as Alfred Script Filter JSON")
	listCmd.MarkFlagsMutuallyExclusive("json", "alfred")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	store, err := newStore()
	if err != nil {
		return err
	}

	metas, err := store.List()
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()

	if listJSON {
		return renderListJSON(w, metas)
	}
	if listAlfred {
		return renderListAlfred(w, metas)
	}
	return renderListStyled(metas)
}

func renderListJSON(w io.Writer, metas []model.LayoutMeta) error {
	if metas == nil {
		metas = []model.LayoutMeta{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(metas)
}

// -- Alfred Script Filter JSON ------------------------------------------------

type alfredItem struct {
	UID          string     `json:"uid"`
	Title        string     `json:"title"`
	Subtitle     string     `json:"subtitle"`
	Arg          string     `json:"arg"`
	Autocomplete string     `json:"autocomplete"`
	Match        string     `json:"match"`
	Icon         alfredIcon `json:"icon"`
	Mods         alfredMods `json:"mods"`
}

type alfredIcon struct {
	Path string `json:"path"`
}

type alfredMod struct {
	Subtitle string `json:"subtitle"`
	Arg      string `json:"arg"`
}

type alfredMods struct {
	Cmd  alfredMod `json:"cmd"`
	Alt  alfredMod `json:"alt"`
	Ctrl alfredMod `json:"ctrl"`
}

type alfredOutput struct {
	Items []alfredItem `json:"items"`
}

func renderListAlfred(w io.Writer, metas []model.LayoutMeta) error {
	var items []alfredItem

	for _, m := range metas {
		// Layout item — restores the whole layout.
		items = append(items, alfredItem{
			UID:          "layout:" + m.Name,
			Title:        "📦 " + m.Name,
			Subtitle:     alfredLayoutSubtitle(m),
			Arg:          "restore:" + m.Name,
			Autocomplete: m.Name,
			Match:        m.Name,
			Icon:         alfredIcon{Path: "icon.png"},
			Mods: alfredMods{
				Cmd:  alfredMod{Subtitle: "Show layout details", Arg: "show:" + m.Name},
				Alt:  alfredMod{Subtitle: "Delete layout", Arg: "delete:" + m.Name},
				Ctrl: alfredMod{Subtitle: "Open TOML file in editor", Arg: "open:" + m.Name},
			},
		})

		// Individual workspace items — restore a single workspace.
		for i, wsTitle := range m.WorkspaceTitles {
			subtitle := m.Name
			if i < len(m.WorkspacePanes) {
				subtitle += fmt.Sprintf(" · %d panes", m.WorkspacePanes[i])
			}
			if i < len(m.WorkspaceSummaries) && m.WorkspaceSummaries[i] != "" {
				subtitle += " · " + m.WorkspaceSummaries[i]
			}

			items = append(items, alfredItem{
				UID:          fmt.Sprintf("ws:%s:%d", m.Name, i),
				Title:        wsTitle,
				Subtitle:     subtitle,
				Arg:          "workspace:" + m.Name + ":" + wsTitle,
				Autocomplete: wsTitle,
				Match:        wsTitle + " " + m.Name,
				Icon:         alfredIcon{Path: "icon.png"},
				Mods: alfredMods{
					Cmd:  alfredMod{Subtitle: "Restore full layout: " + m.Name, Arg: "restore:" + m.Name},
					Alt:  alfredMod{Subtitle: "Show layout details", Arg: "show:" + m.Name},
					Ctrl: alfredMod{Subtitle: "Open TOML file in editor", Arg: "open:" + m.Name},
				},
			})
		}
	}

	out := alfredOutput{Items: items}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func alfredLayoutSubtitle(m model.LayoutMeta) string {
	parts := []string{fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount))}
	if m.Description != "" {
		parts = append(parts, m.Description)
	}
	for _, t := range m.WorkspaceTitles {
		parts = append(parts, t)
	}
	parts = append(parts, m.SavedAt.Local().Format("Jan 02 15:04"))
	return strings.Join(parts, " · ")
}

// -- Styled terminal output ---------------------------------------------------

func renderListStyled(metas []model.LayoutMeta) error {
	if len(metas) == 0 {
		fmt.Println(dimStyle.Render("  No saved layouts. Use 'crex save <name>' to create one."))
		return nil
	}

	fmt.Fprintln(os.Stderr, headingStyle.Render("💾 Saved Layouts"))
	fmt.Fprintln(os.Stderr)

	for _, m := range metas {
		name := greenStyle.Render(fmt.Sprintf("%-16s", m.Name))
		ws := cyanStyle.Render(fmt.Sprintf("%d %s", m.WorkspaceCount, unitName(m.WorkspaceCount)))
		date := dimStyle.Render(m.SavedAt.Local().Format("Jan 02 15:04"))

		var parts []string
		parts = append(parts, ws, date)
		if m.Description != "" {
			desc := m.Description
			if len(desc) > 40 {
				desc = desc[:37] + "..."
			}
			parts = append(parts, desc)
		}

		fmt.Fprintf(os.Stderr, "  %s %s\n", name, strings.Join(parts, dimStyle.Render(" · ")))
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d layout(s)", len(metas))))
	fmt.Fprintln(os.Stderr)
	return nil
}
