package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// -- Minimal palette: green banner, bold headings, dim descriptions ----------

var (
	colorGreen = lipgloss.Color("#5FFF87")
	colorDim   = lipgloss.Color("#6C6C6C")
)

var (
	headingStyle = lipgloss.NewStyle().Bold(true).MarginTop(1)
	cmdNameStyle = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)
)

// -- ASCII banner (only shown on `crex version`) ----------------------------

func banner() string {
	art := []string{
		`                                                                        _   `,
		`  ___ _ __ ___  _   ___  __     _ __ ___  ___ _   _ _ __ _ __ ___  ___| |_ `,
		` / __| '_ ` + "`" + ` _ \| | | \ \/ /____| '__/ _ \/ __| | | | '__| '__/ _ \/ __| __|`,
		`| (__| | | | | | |_| |>  <_____| | |  __/\__ \ |_| | |  | | |  __/ (__| |_ `,
		` \___|_| |_| |_|\__,_/_/\_\    |_|  \___||___/\__,_|_|  |_|  \___|\___|\__|`,
	}

	bannerStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	tagStyle := lipgloss.NewStyle().Foreground(colorDim).Italic(true)
	verStyle := lipgloss.NewStyle().Foreground(colorDim)

	var b strings.Builder
	for _, line := range art {
		b.WriteString(bannerStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString(tagStyle.Render("  Session persistence for cmux — your terminal workspaces, resurrected."))
	b.WriteString("\n")
	b.WriteString(verStyle.Render(fmt.Sprintf("  %s (%s) built %s", Version, Commit, Date)))
	b.WriteString("\n")
	return b.String()
}

// -- Help (shown on `crex` with no args, or `crex --help`) -------------------

func styledHelp() string {
	var b strings.Builder

	b.WriteString(headingStyle.Render("cmux-resurrect (crex)"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Session persistence for cmux — save, restore, and manage your workspaces."))
	b.WriteString("\n")

	b.WriteString(headingStyle.Render("USAGE"))
	b.WriteString("\n")
	b.WriteString("  " + cmdNameStyle.Render("crex") + " " + dimStyle.Render("<command> [flags]"))
	b.WriteString("\n")

	b.WriteString(headingStyle.Render("LAYOUT COMMANDS"))
	b.WriteString("\n")
	writeCmd(&b, "save", "[name]", "Snapshot live cmux state to a TOML layout file")
	writeCmd(&b, "restore", "<name>", "Recreate workspaces, panes, and commands from a layout")
	writeCmd(&b, "list", "", "List saved layouts")
	writeCmd(&b, "show", "<name>", "Display layout details (--raw for TOML)")
	writeCmd(&b, "edit", "<name>", "Open layout in $EDITOR")
	writeCmd(&b, "delete", "<name>", "Delete a saved layout")
	writeCmd(&b, "watch", "[name]", "Auto-save layout on a timer")

	b.WriteString(headingStyle.Render("WORKSPACE COMMANDS"))
	b.WriteString("\n")
	writeCmd(&b, "import-from-md", "", "Create workspaces in cmux from a Markdown file")
	writeCmd(&b, "export-to-md", "", "Capture live cmux state into a Markdown file")
	writeCmd(&b, "workspace add", "<name> <path>", "Add a workspace entry to the workspace file")
	writeCmd(&b, "workspace remove", "<name>", "Remove a workspace entry")
	writeCmd(&b, "workspace list", "", "List workspace entries")
	writeCmd(&b, "workspace toggle", "<name>", "Enable/disable a workspace entry")

	b.WriteString(headingStyle.Render("OTHER"))
	b.WriteString("\n")
	writeCmd(&b, "version", "", "Print version, commit, build date")
	writeCmd(&b, "help", "[command]", "Help about any command")

	b.WriteString(headingStyle.Render("GLOBAL FLAGS"))
	b.WriteString("\n")
	writeFlag(&b, "--config", "string", "Config file (default ~/.config/crex/config.toml)")
	writeFlag(&b, "--layouts-dir", "string", "Layouts directory (default ~/.config/crex/layouts)")
	writeFlag(&b, "-h, --help", "", "Help for crex")

	b.WriteString(headingStyle.Render("EXAMPLES"))
	b.WriteString("\n")
	writeExample(&b, "crex restore demo", "Try the included demo layout")
	writeExample(&b, "crex save work", "Snapshot current layout as 'work'")
	writeExample(&b, "crex restore work --dry-run", "Preview restore without executing")
	writeExample(&b, "crex import-from-md", "Create workspaces from the Markdown workspace file")
	writeExample(&b, "crex workspace add api ~/projects/api -t dev", "Add workspace entry with dev template")
	b.WriteString("\n")

	return b.String()
}

func writeCmd(b *strings.Builder, name, args, desc string) {
	nameRendered := cmdNameStyle.Render(fmt.Sprintf("  %-18s", name))
	argsRendered := dimStyle.Render(fmt.Sprintf("%-16s", args))
	descRendered := dimStyle.Render(desc)
	b.WriteString(fmt.Sprintf("%s %s %s\n", nameRendered, argsRendered, descRendered))
}

func writeFlag(b *strings.Builder, flag, typeName, desc string) {
	flagRendered := cmdNameStyle.Render(fmt.Sprintf("  %-18s", flag))
	typeRendered := dimStyle.Render(fmt.Sprintf("%-8s", typeName))
	descRendered := dimStyle.Render(desc)
	b.WriteString(fmt.Sprintf("%s %s %s\n", flagRendered, typeRendered, descRendered))
}

func writeExample(b *strings.Builder, example, desc string) {
	b.WriteString("  " + cmdNameStyle.Render("$ "+example))
	b.WriteString("\n")
	b.WriteString("    " + dimStyle.Render(desc))
	b.WriteString("\n")
}
