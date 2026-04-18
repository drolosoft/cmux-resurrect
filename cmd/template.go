package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:               "template [template-name] [path]",
	Short:             "Browse and use the built-in template gallery",
	Long:              "Discover, preview, and use pre-built workspace templates for common developer workflows.\n\nShortcut: \"crex template <name>\" is equivalent to \"crex template use <name>\".",
	Aliases:           []string{"tpl"},
	Args:              cobra.ArbitraryArgs,
	RunE:              runTemplateDefault,
	ValidArgsFunction: completeTemplateDefaultArgs,
}

func init() {
	rootCmd.AddCommand(templateCmd)
}

// runTemplateDefault handles "crex template" with no args (shows help) or
// "crex template <name>" as a shortcut for "crex template use <name>".
func runTemplateDefault(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return runTemplateHelp(cmd)
	}
	// If the first arg is a known template name, delegate to "use".
	if _, ok := gallery.Get(args[0]); ok {
		return runTemplateUse(cmd, args)
	}
	return fmt.Errorf("unknown subcommand or template %q\n\nRun 'crex template --help' for usage", args[0])
}

// completeTemplateDefaultArgs offers template names for the shortcut form.
func completeTemplateDefaultArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// First arg: template names (for the shortcut) — subcommands are
		// handled by Cobra automatically before this function is called.
		return completeTemplateNames(cmd, args, toComplete)
	case 1:
		// Second arg: directory path (same as "template use <name> [path]").
		return nil, cobra.ShellCompDirectiveFilterDirs
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func runTemplateHelp(cmd *cobra.Command) error {
	o := newWF(cmd.OutOrStderr())

	// Title.
	o.ln()
	o.ln(categoryStyle.Render("  Template Gallery"))
	o.ln(dimStyle.Render("  Discover, preview, and use pre-built workspace templates."))
	o.ln()

	// Commands.
	o.ln(categoryStyle.Render("  Commands"))
	o.ln()
	tplHelpCmd(o, "list", "", "List available templates (--layout, --workflow, --tag)")
	tplHelpCmd(o, "show", "<name>", "Preview a template with ASCII diagram (--all for gallery)")
	tplHelpCmd(o, "use", "<template> [path]", "Create a workspace from a template (--dry-run, --pin)")
	o.ln()
	o.ln(dimStyle.Render("  Shortcut: crex template <name> is the same as crex template use <name>"))
	tplHelpCmd(o, "customize", "<name>", "Fork a template into your Blueprint for editing")
	o.ln()

	// Quick gallery preview.
	layouts := gallery.ListByCategory("layout")
	workflows := gallery.ListByCategory("workflow")

	o.ln(categoryStyle.Render("  Layouts"))
	o.ln()
	for _, tmpl := range layouts {
		renderTemplateLine(o, tmpl)
	}

	o.ln(categoryStyle.Render("  Workflows"))
	o.ln()
	for _, tmpl := range workflows {
		renderTemplateLine(o, tmpl)
	}

	// Examples.
	o.ln()
	o.ln(dimStyle.Render("  Examples:"))
	o.ln()
	tplHelpExample(o, "crex template show --all", "preview every template")
	tplHelpExample(o, "crex template show ide", "preview the IDE layout")
	tplHelpExample(o, "crex template claude .", "spin up a Claude workspace here (shortcut)")
	tplHelpExample(o, "crex tpl ls --layout", "list only layout templates")
	o.ln()

	total := len(layouts) + len(workflows)
	o.ln(dimStyle.Render(fmt.Sprintf("  %d templates (%d layouts, %d workflows) — crex template <command> --help for details", total, len(layouts), len(workflows))))
	o.ln()

	return nil
}

func tplHelpCmd(o wf, name, args, desc string) {
	nameRendered := greenStyle.Render(fmt.Sprintf("  %-14s", name))
	argsRendered := dimStyle.Render(fmt.Sprintf("%-20s", args))
	o.f("%s %s %s\n", nameRendered, argsRendered, desc)
}

func tplHelpExample(o wf, cmd, desc string) {
	o.f("    %s  %s\n", cyanStyle.Render(cmd), dimStyle.Render(desc))
}
