package cmd

import (
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

// cachedBackend stores the detected backend, evaluated once at package init.
var cachedBackend = client.Detect()

// appTitle returns the application title appropriate for the active backend.
func appTitle() string {
	if cachedBackend == client.BackendCmux {
		return "crex (cmux-resurrect)"
	}
	return "crex"
}

// appTagline returns the tagline appropriate for the active backend.
// Both paths mention both backends — crex always supports cmux and Ghostty.
// The active backend is listed first for relevance.
func appTagline() string {
	if cachedBackend == client.BackendCmux {
		return "Terminal workspace manager for cmux and Ghostty \u2014 your sessions, resurrected."
	}
	return "Terminal workspace manager for Ghostty and cmux \u2014 your sessions, resurrected."
}

// isCmuxBranding returns true when cmux-specific branding should be shown.
func isCmuxBranding() bool {
	return cachedBackend == client.BackendCmux
}

// unitName returns the backend-adaptive label for a terminal tab/workspace.
// Ghostty users see "tab(s)", cmux users see "workspace(s)".
func unitName(count int) string {
	if cachedBackend == client.BackendGhostty {
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

// unitNameCap returns unitName with the first letter capitalized.
func unitNameCap(count int) string {
	s := unitName(count)
	return strings.ToUpper(s[:1]) + s[1:]
}
