package cmd

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

func TestAppTitle_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex (cmux-resurrect)" {
		t.Errorf("appTitle() = %q, want %q", got, "crex (cmux-resurrect)")
	}
}

func TestAppTitle_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex" {
		t.Errorf("appTitle() = %q, want %q", got, "crex")
	}
}

func TestAppTitle_Unknown(t *testing.T) {
	cachedBackend = client.BackendUnknown
	defer func() { cachedBackend = client.Detect() }()

	got := appTitle()
	if got != "crex" {
		t.Errorf("appTitle() = %q, want %q", got, "crex")
	}
}

func TestAppTagline_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	got := appTagline()
	if got != "Terminal workspace manager for cmux and Ghostty \u2014 your sessions, resurrected." {
		t.Errorf("appTagline() = %q", got)
	}
}

func TestAppTagline_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	got := appTagline()
	if got != "Terminal workspace manager for Ghostty \u2014 your sessions, resurrected." {
		t.Errorf("appTagline() = %q", got)
	}
}

func TestIsCmuxBranding(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	if !isCmuxBranding() {
		t.Error("isCmuxBranding() should be true for cmux")
	}

	cachedBackend = client.BackendGhostty
	if isCmuxBranding() {
		t.Error("isCmuxBranding() should be false for Ghostty")
	}
}
