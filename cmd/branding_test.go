package cmd

import (
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/spf13/cobra"
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

func TestBanner_Cmux_ShowsFullName(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	out := banner()
	if !strings.Contains(out, "cmux") {
		t.Error("cmux banner should contain 'cmux'")
	}
	if !strings.Contains(out, "resurrect") {
		t.Error("cmux banner should contain 'resurrect'")
	}
}

func TestBanner_Ghostty_NoCmux(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	out := banner()
	if strings.Contains(out, "cmux") {
		t.Error("Ghostty banner should not contain 'cmux'")
	}
}

func TestStyledHelp_Cmux_ShowsLegacyHint(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	out := styledHelp()
	if !strings.Contains(out, "cmux-resurrect") {
		t.Error("cmux help should show legacy name hint")
	}
}

func TestStyledHelp_Ghostty_NoLegacyHint(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	out := styledHelp()
	if strings.Contains(out, "cmux-resurrect") {
		t.Error("Ghostty help should not mention cmux-resurrect")
	}
}

func TestSubcommandDescriptions_NoCmuxMention(t *testing.T) {
	cmds := []*cobra.Command{
		saveCmd, restoreCmd, watchCmd,
		exportToMDCmd, importFromMDCmd, templateUseCmd,
	}

	for _, c := range cmds {
		if strings.Contains(strings.ToLower(c.Short), "cmux") {
			t.Errorf("%s Short contains 'cmux': %q", c.Name(), c.Short)
		}
		if strings.Contains(strings.ToLower(c.Long), "cmux") {
			t.Errorf("%s Long contains 'cmux': %q", c.Name(), c.Long)
		}
	}
}

func TestRootLongDescription_Ghostty(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	updateRootLong()

	if strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("Ghostty root Long should not mention cmux-resurrect")
	}
}

func TestRootLongDescription_Cmux(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	updateRootLong()

	if !strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("cmux root Long should mention cmux-resurrect")
	}
}
