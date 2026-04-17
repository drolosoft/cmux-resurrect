package cmd

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:     "template",
	Short:   "Browse and use the built-in template gallery",
	Long:    "Discover, preview, and use pre-built workspace templates for common developer workflows.",
	Aliases: []string{"tpl"},
	RunE:    runTemplateHelp,
}

func init() {
	rootCmd.AddCommand(templateCmd)
}

func runTemplateHelp(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStderr()

	// Title.
	fmt.Fprintln(w)
	fmt.Fprintln(w, categoryStyle.Render("  Template Gallery"))
	fmt.Fprintln(w, dimStyle.Render("  Discover, preview, and use pre-built workspace templates."))
	fmt.Fprintln(w)

	// Commands.
	fmt.Fprintln(w, categoryStyle.Render("  Commands"))
	fmt.Fprintln(w)
	tplHelpCmd(w, "list", "", "List available templates (--layout, --workflow, --tag)")
	tplHelpCmd(w, "show", "<name>", "Preview a template with ASCII diagram (--all for gallery)")
	tplHelpCmd(w, "use", "<template> [path]", "Create a workspace from a template (--dry-run, --pin)")
	tplHelpCmd(w, "customize", "<name>", "Fork a template into your Blueprint for editing")
	fmt.Fprintln(w)

	// Quick gallery preview.
	layouts := gallery.ListByCategory("layout")
	workflows := gallery.ListByCategory("workflow")

	fmt.Fprintln(w, categoryStyle.Render("  Layouts"))
	fmt.Fprintln(w)
	for _, tmpl := range layouts {
		renderTemplateLine(w, tmpl)
	}

	fmt.Fprintln(w, categoryStyle.Render("  Workflows"))
	fmt.Fprintln(w)
	for _, tmpl := range workflows {
		renderTemplateLine(w, tmpl)
	}

	// Examples.
	fmt.Fprintln(w)
	fmt.Fprintln(w, dimStyle.Render("  Examples:"))
	fmt.Fprintln(w)
	tplHelpExample(w, "crex template show --all", "preview every template")
	tplHelpExample(w, "crex template show ide", "preview the IDE layout")
	tplHelpExample(w, "crex template use claude .", "spin up a Claude workspace here")
	tplHelpExample(w, "crex tpl ls --layout", "list only layout templates")
	fmt.Fprintln(w)

	total := len(layouts) + len(workflows)
	fmt.Fprintln(w, dimStyle.Render(fmt.Sprintf("  %d templates (%d layouts, %d workflows) — crex template <command> --help for details", total, len(layouts), len(workflows))))
	fmt.Fprintln(w)

	return nil
}

func tplHelpCmd(w interface{ Write([]byte) (int, error) }, name, args, desc string) {
	nameRendered := greenStyle.Render(fmt.Sprintf("  %-14s", name))
	argsRendered := dimStyle.Render(fmt.Sprintf("%-20s", args))
	fmt.Fprintf(w, "%s %s %s\n", nameRendered, argsRendered, desc)
}

func tplHelpExample(w interface{ Write([]byte) (int, error) }, cmd, desc string) {
	var b strings.Builder
	fmt.Fprintf(&b, "    %s  %s", cyanStyle.Render(cmd), dimStyle.Render(desc))
	fmt.Fprintln(w, b.String())
}
