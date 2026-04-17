package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// -- Color palette -----------------------------------------------------------

var (
	colorGreen   = lipgloss.Color("#5FFF87")
	colorDim     = lipgloss.Color("#6C6C6C")
	colorCyan    = lipgloss.Color("#87D7FF")
	colorYellow  = lipgloss.Color("#FFD787")
	colorMagenta = lipgloss.Color("#D787FF")
)

// -- Shared styles -----------------------------------------------------------

var (
	headingStyle = lipgloss.NewStyle().Bold(true).MarginTop(1)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)
	cyanStyle    = lipgloss.NewStyle().Foreground(colorCyan)
	yellowStyle  = lipgloss.NewStyle().Foreground(colorYellow)
	greenStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	magentaStyle = lipgloss.NewStyle().Foreground(colorMagenta)
)

// -- Template styles ---------------------------------------------------------

var (
	templateIconStyle = lipgloss.NewStyle().Width(3)
	templateNameStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Width(14)
	templatePaneStyle = lipgloss.NewStyle().Foreground(colorCyan).Width(5)
	templateDescStyle = lipgloss.NewStyle().Foreground(colorDim)
	categoryStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorYellow).MarginTop(1)
)

// -- ASCII banner ------------------------------------------------------------

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
	b.WriteString(tagStyle.Render("  Terminal workspace manager for cmux and Ghostty — your sessions, resurrected."))
	b.WriteString("\n")
	b.WriteString(verStyle.Render(fmt.Sprintf("  %s (%s) built %s", Version, Commit, Date)))
	b.WriteString("\n")
	return b.String()
}

// -- Compact Help (fits on one screen with the banner) -----------------------

func styledHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	helpCmd(&b, "save", "[name]", "Snapshot current layout")
	helpCmd(&b, "restore", "[name]", "Recreate workspaces from layout")
	helpCmd(&b, "list", "", "List saved layouts")
	helpCmd(&b, "show", "<name>", "Display layout details")
	helpCmd(&b, "edit", "<name>", "Open in $EDITOR")
	helpCmd(&b, "delete", "<name>", "Delete a layout")
	helpCmd(&b, "watch", "[name]", "Auto-save on a timer")
	helpCmd(&b, "import-from-md", "", "Import from Workspace Blueprint")
	helpCmd(&b, "export-to-md", "", "Export to Workspace Blueprint")
	helpCmd(&b, "workspace", "<cmd>", "Manage Blueprint (add|remove|list|toggle)")
	helpCmd(&b, "template", "<cmd>", "Template gallery (list|show|use|customize)")
	helpCmd(&b, "version", "", "Version info")
	helpCmd(&b, "completion", "<shell>", "Shell completions (bash|zsh|fish)")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Quick start:"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "    %s%s%s%s%s\n",
		dimStyle.Render("("),
		greenStyle.Render("crex"),
		dimStyle.Render(" is the short name for "),
		greenStyle.Render("cmux-resurrect"),
		dimStyle.Render(")"))
	b.WriteString("\n")
	helpExample(&b, "crex import-from-md", "create workspaces from Blueprint")
	helpExample(&b, "crex save my-day", "save current layout")
	helpExample(&b, "crex list", "list saved layouts")
	helpExample(&b, "crex restore my-day --mode add", "restore a saved layout")
	helpExample(&b, "crex workspace add notes ~/docs", "add workspace to Blueprint")
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  crex <command> --help for flags and details"))
	b.WriteString("\n")

	return b.String()
}

func helpCmd(b *strings.Builder, name, args, desc string) {
	nameRendered := greenStyle.Render(fmt.Sprintf("  %-18s", name))
	argsRendered := dimStyle.Render(fmt.Sprintf("%-12s", args))
	fmt.Fprintf(b, "%s %s %s\n", nameRendered, argsRendered, desc)
}

func helpExample(b *strings.Builder, cmd, desc string) {
	fmt.Fprintf(b, "    %s  %s\n", cyanStyle.Render(cmd), dimStyle.Render(desc))
}

// padTitle inserts an extra space after any variation selector (U+FE0F) in a
// title. Some terminals miscalculate emoji width when VS16 is present (e.g.,
// ⚙️ renders narrower than expected), causing the next character to overlap.
// By replacing "VS16 " with "VS16  " the icon-to-name gap stays visible.
func padTitle(title string) string {
	return strings.ReplaceAll(title, "\uFE0F ", "\uFE0F  ")
}
