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
	{"🔎", "show", "<name|#>", func(b client.DetectedBackend) string { return "Show layout details" }, "Layouts"},
	{"📝", "edit", "<name|#>", func(b client.DetectedBackend) string { return "Edit layout in $EDITOR" }, "Layouts"},
	{"✏️", "rename", "<old|#> <new>", func(b client.DetectedBackend) string { return "Rename a saved layout" }, "Layouts"},
	{"🗑", "delete", "<name|#>", func(b client.DetectedBackend) string { return "Delete a saved layout" }, "Layouts"},
	{"📦", "templates", "", func(b client.DetectedBackend) string { return "Browse template gallery" }, "Templates"},
	{"🚀", "use", "<template|#>", func(b client.DetectedBackend) string { return "Create " + unitLabel(b, 1) + " from template" }, "Templates"},
	{"🔎", "template show", "<name|#>", func(b client.DetectedBackend) string { return "Show template details" }, "Templates"},
	{"📝", "template customize", "<name|#>", func(b client.DetectedBackend) string { return "Copy template to Blueprint" }, "Templates"},
	{"📐", "bp add", "<name> <path>", func(b client.DetectedBackend) string { return "Add Blueprint entry" }, "Blueprint"},
	{"📐", "bp list", "", func(b client.DetectedBackend) string { return "List Blueprint entries" }, "Blueprint"},
	{"📐", "bp remove", "<name|#>", func(b client.DetectedBackend) string { return "Remove Blueprint entry" }, "Blueprint"},
	{"📐", "bp toggle", "<name|#>", func(b client.DetectedBackend) string { return "Enable/disable entry" }, "Blueprint"},
	{"📥", "import", "", func(b client.DetectedBackend) string { return "Create " + unitLabel(b, 2) + " from Blueprint" }, "Blueprint"},
	{"📤", "export", "", func(b client.DetectedBackend) string { return "Export live state to Blueprint" }, "Blueprint"},
	{"🎨", "settings banner set", "<flame|classic|plain>", func(b client.DetectedBackend) string { return "Set banner style" }, "Settings"},
	{"🔍", "settings banner get", "", func(b client.DetectedBackend) string { return "Show current style" }, "Settings"},
	{"📋", "settings banner list", "", func(b client.DetectedBackend) string { return "List available styles" }, "Settings"},
	{"🔧", "settings restore-mode set", "<ask|replace|add>", func(b client.DetectedBackend) string { return "Set restore mode" }, "Settings"},
	{"🔍", "settings restore-mode get", "", func(b client.DetectedBackend) string { return "Show current mode" }, "Settings"},
	{"📋", "settings restore-mode list", "", func(b client.DetectedBackend) string { return "List available modes" }, "Settings"},
	{"🤖", "skill install", "[codex]", func(b client.DetectedBackend) string { return "Install the agent skill (Claude Code / Codex)" }, "Shell"},
	{"⬆️", "update", "", func(b client.DetectedBackend) string { return "Update crex to latest version" }, "Shell"},
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
	b.WriteString("\n")

	groupOrder := []string{"Live", "Layouts", "Templates", "Blueprint", "Settings", "Shell"}

	for _, group := range groupOrder {
		b.WriteString("  ")
		b.WriteString(shellHeadingStyle.Render(group))
		b.WriteString("\n\n")

		for _, e := range helpEntries {
			if e.group != group {
				continue
			}
			// Compute visual padding from unstyled text (ANSI codes
			// break fmt's %-Ns padding).
			raw := e.cmd
			if e.args != "" {
				raw += " " + e.args
			}
			const colWidth = 24
			pad := colWidth - len(raw)
			if pad < 1 {
				pad = 1
			}

			cmd := shellSuccessStyle.Render(e.cmd)
			args := ""
			if e.args != "" {
				args = " " + shellDimStyle.Render(e.args)
			}
			desc := shellDimStyle.Render(e.desc(backend))
			fmt.Fprintf(&b, "    %s  %s%s%s\n", padIcon(e.icon), cmd+args, strings.Repeat(" ", pad), desc)
		}
		b.WriteString("\n")
	}

	b.WriteString(shellDimStyle.Render("  Tip: Use # from the last listing, or ↑/↓ to navigate results."))
	b.WriteString("\n")

	return b.String()
}
