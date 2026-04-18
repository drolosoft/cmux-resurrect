package cmd

import "github.com/drolosoft/cmux-resurrect/internal/client"

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
