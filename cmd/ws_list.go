package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var wsListAll bool

var wsListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List workspace entries from the Workspace Blueprint",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runWorkspaceList,
}

func init() {
	wsListCmd.Flags().BoolVarP(&wsListAll, "all", "a", false, "show disabled workspaces too")
	workspaceCmd.AddCommand(wsListCmd)
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("read Workspace Blueprint: %w", err)
	}

	fmt.Fprintln(os.Stderr, headingStyle.Render("📝 Workspace Blueprint"))
	fmt.Fprintln(os.Stderr)

	enabled := 0
	disabled := 0
	shown := 0

	for _, p := range wf.Projects {
		if !wsListAll && !p.Enabled {
			disabled++
			continue
		}

		check := greenStyle.Render("✅")
		if !p.Enabled {
			check = dimStyle.Render("⬜")
			disabled++
		} else {
			enabled++
		}

		name := greenStyle.Render(fmt.Sprintf("%-14s", p.Name))
		tmpl := cyanStyle.Render(fmt.Sprintf("%-10s", p.Template))
		path := dimStyle.Render(p.Path)

		pin := ""
		if p.Pin {
			pin = " 📌"
		}

		icon := p.Icon
		if strings.Contains(icon, "\uFE0F") {
			icon += " "
		}
		fmt.Fprintf(os.Stderr, "  %s %s %s %s %s%s\n", check, icon, name, tmpl, path, pin)
		shown++
	}

	if shown == 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No workspace entries found."))
	}

	fmt.Fprintln(os.Stderr)
	total := enabled + disabled
	if wsListAll && disabled > 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries (%d enabled, %d disabled)", total, enabled, disabled)))
	} else {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries", shown)))
	}
	return nil
}
