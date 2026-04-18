package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/spf13/cobra"
)

var bpListAll bool

var blueprintListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List entries in the Blueprint",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runBlueprintList,
}

func init() {
	blueprintListCmd.Flags().BoolVarP(&bpListAll, "all", "a", false, "show disabled entries too")
	blueprintCmd.AddCommand(blueprintListCmd)

	// Legacy subcommand under workspaceLegacyCmd for backward compatibility.
	legacyList := &cobra.Command{
		Use:  "list",
		Args: cobra.NoArgs,
		RunE: runBlueprintList,
	}
	legacyList.Flags().BoolVarP(&bpListAll, "all", "a", false, "show disabled entries too")
	workspaceLegacyCmd.AddCommand(legacyList)
}

func runBlueprintList(cmd *cobra.Command, args []string) error {
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		return fmt.Errorf("read Blueprint: %w", err)
	}

	fmt.Fprintln(os.Stderr, headingStyle.Render("📝 Blueprint"))
	fmt.Fprintln(os.Stderr)

	enabled := 0
	disabled := 0
	shown := 0

	for _, p := range wf.Projects {
		if !bpListAll && !p.Enabled {
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
		fmt.Fprintln(os.Stderr, dimStyle.Render("  No entries found."))
	}

	fmt.Fprintln(os.Stderr)
	total := enabled + disabled
	if bpListAll && disabled > 0 {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries (%d enabled, %d disabled)", total, enabled, disabled)))
	} else {
		fmt.Fprintln(os.Stderr, dimStyle.Render(fmt.Sprintf("  %d entries", shown)))
	}
	return nil
}
