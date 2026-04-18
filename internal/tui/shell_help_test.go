package tui

import (
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestRenderHelp_ContainsAllGroups(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	groups := []string{"Live", "Layouts", "Templates", "Blueprint", "Shell"}
	for _, g := range groups {
		if !strings.Contains(help, g) {
			t.Errorf("help output missing group %q", g)
		}
	}
}

func TestRenderHelp_ContainsAllCommands(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	commands := []string{"now", "watch", "ls", "restore", "save", "delete", "templates", "use", "bp add", "bp list", "bp remove", "bp toggle", "help", "exit"}
	for _, cmd := range commands {
		if !strings.Contains(help, cmd) {
			t.Errorf("help output missing command %q", cmd)
		}
	}
}

func TestRenderHelp_ContainsIcons(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	icons := []string{"🖥", "⏱", "📋", "🔄", "💾", "🗑", "📦", "🚀", "📐", "❓", "👋"}
	for _, icon := range icons {
		if !strings.Contains(help, icon) {
			t.Errorf("help output missing icon %q", icon)
		}
	}
}

func TestRenderHelp_GhosttyShowsTabs(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	if !strings.Contains(help, "tabs") {
		t.Error("Ghostty help should say 'tabs'")
	}
}

func TestRenderHelp_CmuxShowsWorkspaces(t *testing.T) {
	help := renderHelp(client.BackendCmux)
	if !strings.Contains(help, "workspaces") {
		t.Error("cmux help should say 'workspaces'")
	}
}

func TestRenderHelp_ContainsTip(t *testing.T) {
	help := renderHelp(client.BackendGhostty)
	if !strings.Contains(help, "Tip") {
		t.Error("help should contain tip line")
	}
}
