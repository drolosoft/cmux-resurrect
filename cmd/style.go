package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// -- Color palette -------------------------------------------------------

var (
	colorCyan    = lipgloss.Color("#00D7FF")
	colorMagenta = lipgloss.Color("#FF5FAF")
	colorGreen   = lipgloss.Color("#5FFF87")
	colorYellow  = lipgloss.Color("#FFD75F")
	colorDim     = lipgloss.Color("#6C6C6C")
	colorWhite   = lipgloss.Color("#FFFFFF")
	colorOrange  = lipgloss.Color("#FF8700")
)

// -- Reusable styles -----------------------------------------------------

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	headingStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMagenta).
			MarginTop(1)

	cmdNameStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	cmdDescStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	flagStyle = lipgloss.NewStyle().
			Foreground(colorYellow)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorOrange).
			Bold(true)
)

// -- ASCII banner --------------------------------------------------------

// banner returns the styled ASCII art banner for cmux-resurrect.
// Minimal 5-line design that is tasteful and not overwhelming.
func banner() string {
	art := []string{
		`                                  `,
		`   ___ _ __ ___  _ __ ___  ___    `,
		`  / __| '_ ` + "`" + ` _ \| '__/ _ \/ __|   `,
		`  \__ \ | | | | | | |  __/\__ \   `,
		`  |___/_| |_| |_|_|  \___||___/   `,
		`                                  `,
	}

	bannerStyle := lipgloss.NewStyle().
		Foreground(colorCyan).
		Bold(true)

	taglineStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		Italic(true).
		Align(lipgloss.Center)

	versionLine := lipgloss.NewStyle().
		Foreground(colorMagenta).
		Align(lipgloss.Center)

	var b strings.Builder
	for _, line := range art {
		b.WriteString(bannerStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString(taglineStyle.Render("  Session persistence for cmux"))
	b.WriteString("\n")
	b.WriteString(versionLine.Render(fmt.Sprintf("  v%s (%s) built %s", Version, Commit, Date)))
	b.WriteString("\n")

	return b.String()
}

// -- Help template -------------------------------------------------------

// styledHelp generates a beautiful help output for the root command.
func styledHelp() string {
	var b strings.Builder

	b.WriteString(banner())
	b.WriteString("\n")

	// Usage
	b.WriteString(headingStyle.Render("USAGE"))
	b.WriteString("\n")
	b.WriteString("  " + cmdNameStyle.Render("cmres") + " " + dimStyle.Render("<command>") + " " + dimStyle.Render("[flags]"))
	b.WriteString("\n")

	// Core commands
	b.WriteString(headingStyle.Render("CORE COMMANDS"))
	b.WriteString("\n")
	writeCmd(&b, "save", "[name]", "Capture current cmux layout to TOML")
	writeCmd(&b, "restore", "<name>", "Recreate workspaces, splits, and commands")
	writeCmd(&b, "list", "", "List saved layouts with workspace count")
	writeCmd(&b, "show", "<name>", "Display layout details (--raw for TOML)")
	writeCmd(&b, "edit", "<name>", "Open layout in $EDITOR")
	writeCmd(&b, "delete", "<name>", "Delete a saved layout")

	// Workspace commands
	b.WriteString(headingStyle.Render("WORKSPACE COMMANDS"))
	b.WriteString("\n")
	writeCmd(&b, "sync", "", "Reconcile Markdown workspace file with cmux")
	writeCmd(&b, "export", "", "Export live cmux state to workspace file")
	writeCmd(&b, "watch", "[name]", "Auto-save layout periodically")

	// Project commands
	b.WriteString(headingStyle.Render("PROJECT COMMANDS"))
	b.WriteString("\n")
	writeCmd(&b, "project add", "<name> <path>", "Add project to workspace file")
	writeCmd(&b, "project remove", "<name>", "Remove project from workspace file")
	writeCmd(&b, "project list", "", "List projects in workspace file")
	writeCmd(&b, "project toggle", "<name>", "Enable/disable a project")

	// Other
	b.WriteString(headingStyle.Render("OTHER"))
	b.WriteString("\n")
	writeCmd(&b, "version", "", "Print version, commit, build date")
	writeCmd(&b, "help", "[command]", "Help about any command")

	// Global flags
	b.WriteString(headingStyle.Render("GLOBAL FLAGS"))
	b.WriteString("\n")
	writeFlag(&b, "--config", "string", "Config file (default ~/.config/cmres/config.toml)")
	writeFlag(&b, "--layouts-dir", "string", "Layouts directory (default ~/.config/cmres/layouts)")
	writeFlag(&b, "-h, --help", "", "Help for cmres")

	// Examples
	b.WriteString(headingStyle.Render("EXAMPLES"))
	b.WriteString("\n")
	writeExample(&b, "cmres save work", "Save current layout as 'work'")
	writeExample(&b, "cmres restore work --dry-run", "Preview restore without executing")
	writeExample(&b, "cmres project add api ~/code/api -t dev", "Add project with dev template")
	writeExample(&b, "cmres watch autosave -i 2m", "Auto-save every 2 minutes")

	// Footer
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Forged by "))
	b.WriteString(accentStyle.Render("Drolosoft"))
	b.WriteString(dimStyle.Render(" -- Tools we wish existed"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  https://drolosoft.com"))
	b.WriteString("\n")

	return b.String()
}

func writeCmd(b *strings.Builder, name, args, desc string) {
	nameRendered := cmdNameStyle.Render(fmt.Sprintf("  %-18s", name))
	argsRendered := ""
	if args != "" {
		argsRendered = dimStyle.Render(fmt.Sprintf("%-16s", args))
	} else {
		argsRendered = dimStyle.Render(fmt.Sprintf("%-16s", ""))
	}
	descRendered := cmdDescStyle.Render(desc)
	b.WriteString(fmt.Sprintf("%s %s %s\n", nameRendered, argsRendered, descRendered))
}

func writeFlag(b *strings.Builder, flag, typeName, desc string) {
	flagRendered := flagStyle.Render(fmt.Sprintf("  %-18s", flag))
	typeRendered := ""
	if typeName != "" {
		typeRendered = dimStyle.Render(fmt.Sprintf("%-8s", typeName))
	} else {
		typeRendered = dimStyle.Render(fmt.Sprintf("%-8s", ""))
	}
	descRendered := cmdDescStyle.Render(desc)
	b.WriteString(fmt.Sprintf("%s %s %s\n", flagRendered, typeRendered, descRendered))
}

func writeExample(b *strings.Builder, example, desc string) {
	b.WriteString("  " + accentStyle.Render("$") + " " + cmdNameStyle.Render(example))
	b.WriteString("\n")
	b.WriteString("    " + dimStyle.Render(desc))
	b.WriteString("\n")
}
