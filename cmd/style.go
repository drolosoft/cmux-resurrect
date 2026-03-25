package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// -- Color palette (simple: green, bold, dim) --------------------------------

var (
	colorGreen = lipgloss.Color("#5FFF87")
	colorDim   = lipgloss.Color("#6C6C6C")
	colorWhite = lipgloss.Color("#FFFFFF")
)

// -- Reusable styles ---------------------------------------------------------

var (
	headingStyle = lipgloss.NewStyle().
			Bold(true).
			MarginTop(1)

	cmdNameStyle = lipgloss.NewStyle().
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(colorDim)
)

// -- ASCII banner ------------------------------------------------------------

func banner() string {
	art := []string{
		` ▄████▄   ███▄ ▄███▓ █    ██ ▒██   ██▒ ██▀███  ▓█████   ██████  █    ██  ██▀███   ██▀███  ▓█████  ▄████▄  ▄▄▄█████▓`,
		`▒██▀ ▀█  ▓██▒▀█▀ ██▒ ██  ▓██▒▒▒ █ █ ▒░▓██ ▒ ██▒▓█   ▀ ▒██    ▒  ██  ▓██▒▓██ ▒ ██▒▓██ ▒ ██▒▓█   ▀ ▒██▀ ▀█  ▓  ██▒ ▓▒`,
		`▒▓█    ▄ ▓██    ▓██░▓██  ▒██░░░  █   ░▓██ ░▄█ ▒▒███   ░ ▓██▄   ▓██  ▒██░▓██ ░▄█ ▒▓██ ░▄█ ▒▒███   ▒▓█    ▄ ▒ ▓██░ ▒░`,
		`▒▓▓▄ ▄██▒▒██    ▒██ ▓▓█  ░██░ ░ █ █ ▒ ▒██▀▀█▄  ▒▓█  ▄   ▒   ██▒▓▓█  ░██░▒██▀▀█▄  ▒██▀▀█▄  ▒▓█  ▄ ▒▓▓▄ ▄██▒░ ▓██▓ ░`,
		`▒ ▓███▀ ░▒██▒   ░██▒▒▒█████▓ ▒██▒ ▒██▒░██▓ ▒██▒░▒████▒▒██████▒▒▒▒█████▓ ░██▓ ▒██▒░██▓ ▒██▒░▒████▒▒ ▓███▀ ░  ▒██▒ ░`,
		`░ ░▒ ▒  ░░ ▒░   ░  ░░▒▓▒ ▒ ▒ ▒▒ ░ ░▓ ░░ ▒▓ ░▒▓░░░ ▒░ ░▒ ▒▓▒ ▒ ░░▒▓▒ ▒ ▒ ░ ▒▓ ░▒▓░░ ▒▓ ░▒▓░░░ ▒░ ░░ ░▒ ▒  ░  ▒ ░░`,
		`  ░  ▒   ░  ░      ░░░▒░ ░ ░ ░░   ░▒ ░  ░▒ ░ ▒░ ░ ░  ░░ ░▒  ░ ░░░▒░ ░ ░   ░▒ ░ ▒░  ░▒ ░ ▒░ ░ ░  ░  ░  ▒       ░`,
		`░        ░      ░    ░░░ ░ ░  ░    ░    ░░   ░    ░   ░  ░  ░   ░░░ ░ ░   ░░   ░   ░░   ░    ░   ░          ░`,
		`░ ░             ░      ░      ░    ░     ░        ░  ░      ░     ░        ░        ░        ░  ░░ ░`,
		`░                                                                                                ░`,
	}

	bannerStyle := lipgloss.NewStyle().
		Foreground(colorGreen).
		Bold(true)

	tagStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		Italic(true)

	verStyle := lipgloss.NewStyle().
		Foreground(colorDim)

	var b strings.Builder
	for _, line := range art {
		b.WriteString(bannerStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString(tagStyle.Render("  Session persistence for cmux — your terminal workspaces, resurrected."))
	b.WriteString("\n")
	b.WriteString(verStyle.Render(fmt.Sprintf("  v%s (%s) built %s", Version, Commit, Date)))
	b.WriteString("\n")

	return b.String()
}

// -- Help template -----------------------------------------------------------

func styledHelp() string {
	var b strings.Builder

	b.WriteString(banner())
	b.WriteString("\n")

	// Usage
	b.WriteString(headingStyle.Render("USAGE"))
	b.WriteString("\n")
	b.WriteString("  " + cmdNameStyle.Render("crex") + " " + dimStyle.Render("<command> [flags]"))
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
	writeFlag(&b, "--config", "string", "Config file (default ~/.config/crex/config.toml)")
	writeFlag(&b, "--layouts-dir", "string", "Layouts directory (default ~/.config/crex/layouts)")
	writeFlag(&b, "-h, --help", "", "Help for crex")

	// Examples
	b.WriteString(headingStyle.Render("EXAMPLES"))
	b.WriteString("\n")
	writeExample(&b, "crex save work", "Save current layout as 'work'")
	writeExample(&b, "crex restore work --dry-run", "Preview restore without executing")
	writeExample(&b, "crex project add api ~/code/api -t dev", "Add project with dev template")
	writeExample(&b, "crex watch autosave -i 2m", "Auto-save every 2 minutes")
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
