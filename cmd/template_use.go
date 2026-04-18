package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var (
	tplUseName   string
	tplUseIcon   string
	tplUseDryRun bool
	tplUsePin    bool
)

var templateUseCmd = &cobra.Command{
	Use:   "use <template> [path]",
	Short: "Create a workspace from a gallery template",
	Long:  "Creates a new workspace using a gallery template's layout and commands.\n\nThe first argument is the template name (e.g., cols, claude, ide).\nThe optional second argument is the working directory (defaults to \".\").",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runTemplateUse,
}

func init() {
	templateUseCmd.Flags().StringVar(&tplUseName, "name", "", "workspace title (default: directory name)")
	templateUseCmd.Flags().StringVar(&tplUseIcon, "icon", "", "workspace icon (default: template icon for workflows)")
	templateUseCmd.Flags().BoolVar(&tplUseDryRun, "dry-run", false, "show commands without executing")
	templateUseCmd.Flags().BoolVar(&tplUsePin, "pin", false, "pin the workspace after creation")
	templateUseCmd.ValidArgsFunction = completeTemplateUseArgs
	templateCmd.AddCommand(templateUseCmd)
}

// completeTemplateUseArgs provides completion for template use arguments.
func completeTemplateUseArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// First arg: template names.
		return completeTemplateNames(cmd, args, toComplete)
	case 1:
		// Second arg: directory path.
		return nil, cobra.ShellCompDirectiveFilterDirs
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTemplateUse(cmd *cobra.Command, args []string) error {
	templateName := args[0]

	// Resolve template.
	tmpl, ok := gallery.Get(templateName)
	if !ok {
		return fmt.Errorf("template %q not found in gallery", templateName)
	}

	panes := gallery.BuildPanes(tmpl)

	// Resolve working directory.
	cwd := "."
	if len(args) >= 2 {
		cwd = args[1]
	}
	cwd = config.ExpandHome(cwd)
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Default icon: use template icon for workflows, empty for layouts.
	icon := tplUseIcon
	if icon == "" && tmpl.Category == "workflow" {
		icon = tmpl.Icon
	}

	// Check cmux connectivity (skip in dry-run mode).
	cl := newClient()
	if !tplUseDryRun {
		if err := cl.Ping(); err != nil {
			return fmt.Errorf("backend not reachable: %w", err)
		}
	}

	user := &orchestrate.TemplateUser{
		Client: cl,
		OnProgress: func(msg string) {
			fmt.Fprintf(os.Stderr, "  %s  %s\n", dimStyle.Render("INFO"), msg)
		},
	}

	result, err := user.Use(panes, orchestrate.TemplateUseOpts{
		Title: tplUseName,
		Icon:  icon,
		CWD:   absCWD,
		Pin:   tplUsePin,
	}, tplUseDryRun)
	if err != nil {
		return err
	}

	o := newWF(cmd.OutOrStderr())

	if result.DryRun {
		o.ln()
		o.f("%s %s %s %s\n",
			yellowStyle.Render("Dry-run"),
			greenStyle.Render(tmpl.Icon+" "+tmpl.Name),
			dimStyle.Render("in"),
			cyanStyle.Render(absCWD))
		o.ln()

		for _, c := range result.Commands {
			parts := strings.SplitN(c, " ", 3)
			if len(parts) >= 2 {
				o.f("  %s %s", dimStyle.Render(parts[0]), cyanStyle.Render(parts[1]))
				if len(parts) == 3 {
					o.f(" %s", dimStyle.Render(parts[2]))
				}
				o.ln()
			} else {
				o.f("  %s\n", c)
			}
		}

		o.ln()
		o.f("%s\n\n",
			greenStyle.Render(fmt.Sprintf("  %d commands for %q (%d panes)", len(result.Commands), result.Title, result.Panes)))
		return nil
	}

	o.ln()
	o.f("%s  %s %s (%d panes)\n\n",
		greenStyle.Render("OK"),
		greenStyle.Render(padTitle(result.Title)),
		dimStyle.Render("from "+tmpl.Name),
		result.Panes)

	return nil
}
