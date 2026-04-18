package tui

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

type helpEntry struct {
	icon  string
	cmd   string
	args  string
	desc  func(client.DetectedBackend) string
	group string
}

var helpEntries = []helpEntry{
	{"🖥", "now", "", func(b client.DetectedBackend) string { return "Show current " + unitLabel(b, 2) }, "Live"},
	{"⏱", "watch", "start|stop|status", func(b client.DetectedBackend) string { return "Auto-save daemon" }, "Live"},
	{"📋", "ls", "", func(b client.DetectedBackend) string { return "List saved layouts" }, "Layouts"},
	{"🔄", "restore", "<name|#>", func(b client.DetectedBackend) string { return "Restore a saved layout" }, "Layouts"},
	{"💾", "save", "[name]", func(b client.DetectedBackend) string { return "Save current layout" }, "Layouts"},
	{"🗑", "delete", "<name|#>", func(b client.DetectedBackend) string { return "Delete a saved layout" }, "Layouts"},
	{"📦", "templates", "", func(b client.DetectedBackend) string { return "Browse template gallery" }, "Templates"},
	{"🚀", "use", "<template|#>", func(b client.DetectedBackend) string { return "Create " + unitLabel(b, 1) + " from template" }, "Templates"},
	{"📐", "bp add", "<name> <path>", func(b client.DetectedBackend) string { return "Add Blueprint entry" }, "Blueprint"},
	{"📐", "bp list", "", func(b client.DetectedBackend) string { return "List Blueprint entries" }, "Blueprint"},
	{"📐", "bp remove", "<name|#>", func(b client.DetectedBackend) string { return "Remove Blueprint entry" }, "Blueprint"},
	{"📐", "bp toggle", "<name|#>", func(b client.DetectedBackend) string { return "Enable/disable entry" }, "Blueprint"},
	{"❓", "help", "", func(b client.DetectedBackend) string { return "Show this help" }, "Shell"},
	{"👋", "exit", "", func(b client.DetectedBackend) string { return "Exit the shell" }, "Shell"},
}

// unitLabel returns "tab(s)" for Ghostty, "workspace(s)" for cmux.
// This is the shell-internal version (doesn't depend on cmd.cachedBackend).
func unitLabel(b client.DetectedBackend, count int) string {
	if b == client.BackendGhostty {
		if count == 1 {
			return "tab"
		}
		return "tabs"
	}
	if count == 1 {
		return "workspace"
	}
	return "workspaces"
}

// renderHelp builds the full help text with icons, grouped by section.
func renderHelp(backend client.DetectedBackend) string {
	var b strings.Builder

	groupOrder := []string{"Live", "Layouts", "Templates", "Blueprint", "Shell"}

	for _, group := range groupOrder {
		b.WriteString("  ")
		b.WriteString(shellHeadingStyle.Render(group))
		b.WriteString("\n")

		for _, e := range helpEntries {
			if e.group != group {
				continue
			}
			args := ""
			if e.args != "" {
				args = " " + shellDimStyle.Render(e.args)
			}
			desc := shellDimStyle.Render(e.desc(backend))
			cmd := shellSuccessStyle.Render(e.cmd)
			b.WriteString(fmt.Sprintf("  %s  %-28s %s\n", e.icon, cmd+args, desc))
		}
		b.WriteString("\n")
	}

	b.WriteString(shellDimStyle.Render("  Tip: Use # from the last listing, or ↑/↓ to navigate results."))
	b.WriteString("\n")

	return b.String()
}
