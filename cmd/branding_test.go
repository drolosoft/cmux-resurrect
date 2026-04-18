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
	if got != "Terminal workspace manager for Ghostty and cmux \u2014 your sessions, resurrected." {
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

func TestBanner_Ghostty_NoCmuxResurrect(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	out := banner()
	// The ASCII art should not spell out "cmux-resurrect", but the tagline
	// correctly mentions both backends ("Ghostty and cmux").
	if strings.Contains(out, "cmux-resurrect") {
		t.Error("Ghostty banner should not contain 'cmux-resurrect'")
	}
	if strings.Contains(out, "resurrect") && !strings.Contains(out, "resurrected") {
		t.Error("Ghostty banner ASCII art should not spell 'resurrect' (tagline 'resurrected.' is OK)")
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

// -- Theme and banner mode tests ---------------------------------------------

func TestParseBannerMode(t *testing.T) {
	tests := []struct {
		input string
		want  bannerMode
	}{
		{"classic", bannerClassic},
		{"Classic", bannerClassic},
		{"CLASSIC", bannerClassic},
		{"plain", bannerPlain},
		{"Plain", bannerPlain},
		{"flame", bannerFlame},
		{"Flame", bannerFlame},
		{"", bannerFlame},
		{"unknown", bannerFlame},
		{"  classic  ", bannerClassic},
	}
	for _, tt := range tests {
		got := parseBannerMode(tt.input)
		if got != tt.want {
			t.Errorf("parseBannerMode(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestResolveBannerMode_EnvOverridesConfig(t *testing.T) {
	t.Setenv("CREX_BANNER", "plain")
	got := resolveBannerMode()
	if got != bannerPlain {
		t.Errorf("CREX_BANNER=plain → %d, want bannerPlain", got)
	}
}

func TestResolveBannerMode_DefaultIsFlame(t *testing.T) {
	t.Setenv("CREX_BANNER", "")
	got := resolveBannerMode()
	if got != bannerFlame {
		t.Errorf("empty env → %d, want bannerFlame", got)
	}
}

func TestIsDark_EnvOverride(t *testing.T) {
	t.Setenv("CREX_THEME", "light")
	if isDark() {
		t.Error("CREX_THEME=light should return false")
	}

	t.Setenv("CREX_THEME", "dark")
	if !isDark() {
		t.Error("CREX_THEME=dark should return true")
	}
}

func TestDetectTheme_DarkVsLight(t *testing.T) {
	t.Setenv("CREX_THEME", "dark")
	dark := detectTheme()
	if !dark.dark {
		t.Error("CREX_THEME=dark should produce dark theme")
	}
	if len(dark.flame) != 9 {
		t.Errorf("dark flame gradient has %d steps, want 9", len(dark.flame))
	}

	t.Setenv("CREX_THEME", "light")
	light := detectTheme()
	if light.dark {
		t.Error("CREX_THEME=light should produce light theme")
	}
	if len(light.flame) != 9 {
		t.Errorf("light flame gradient has %d steps, want 9", len(light.flame))
	}
}

func TestGradientText_EmptyString(t *testing.T) {
	got := gradientText("", flameDark, false, false)
	if got != "" {
		t.Errorf("gradientText empty = %q, want empty", got)
	}
}

func TestGradientText_AllSpaces(t *testing.T) {
	got := gradientText("     ", flameDark, false, false)
	if got != "     " {
		t.Errorf("gradientText all-spaces = %q, want 5 spaces", got)
	}
}

func TestGradientText_ProducesOutput(t *testing.T) {
	got := gradientText("·──·", flameDark, false, false)
	if len(got) == 0 {
		t.Error("gradientText should produce non-empty output")
	}
	// In non-TTY (CI) lipgloss strips ANSI, so just verify content is preserved.
	if !strings.Contains(got, "·") {
		t.Error("gradientText should preserve visible characters")
	}
}

func TestGradientText_Reversed(t *testing.T) {
	fwd := gradientText("abc", flameDark, false, false)
	rev := gradientText("abc", flameDark, true, false)
	// Both should contain the same characters; in a TTY the colors differ.
	if !strings.Contains(fwd, "a") || !strings.Contains(rev, "a") {
		t.Error("both directions should preserve characters")
	}
}

func TestRenderTagline_ContainsResurrected(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	t.Setenv("CREX_THEME", "dark")
	th := detectTheme()

	for _, mode := range []bannerMode{bannerFlame, bannerClassic, bannerPlain} {
		out := renderTagline(th, mode)
		if !strings.Contains(out, "resurrected") {
			t.Errorf("mode %d tagline should contain 'resurrected'", mode)
		}
	}
}

func TestBanner_AllModes_ProduceOutput(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()
	t.Setenv("CREX_THEME", "dark")

	for _, mode := range []string{"flame", "classic", "plain"} {
		t.Setenv("CREX_BANNER", mode)
		out := banner()
		if len(out) == 0 {
			t.Errorf("banner mode %q produced empty output", mode)
		}
		// All modes include the tagline with "resurrected."
		if !strings.Contains(out, "resurrected") {
			t.Errorf("banner mode %q should contain 'resurrected' in tagline", mode)
		}
	}
}

func TestBanner_CmuxAllModes(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()
	t.Setenv("CREX_THEME", "dark")

	for _, mode := range []string{"flame", "classic", "plain"} {
		t.Setenv("CREX_BANNER", mode)
		out := banner()
		if !strings.Contains(out, "cmux") {
			t.Errorf("cmux banner mode %q should contain 'cmux'", mode)
		}
	}
}

// -- unitName / unitNameCap tests --------------------------------------------

func TestUnitName_Ghostty_Singular(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitName(1); got != "tab" {
		t.Errorf("unitName(1) with Ghostty = %q, want %q", got, "tab")
	}
}

func TestUnitName_Ghostty_Plural(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitName(3); got != "tabs" {
		t.Errorf("unitName(3) with Ghostty = %q, want %q", got, "tabs")
	}
}

func TestUnitName_Cmux_Singular(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = orig }()

	if got := unitName(1); got != "workspace" {
		t.Errorf("unitName(1) with cmux = %q, want %q", got, "workspace")
	}
}

func TestUnitName_Cmux_Plural(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = orig }()

	if got := unitName(3); got != "workspaces" {
		t.Errorf("unitName(3) with cmux = %q, want %q", got, "workspaces")
	}
}

func TestUnitName_Unknown_DefaultsToCmux(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendUnknown
	defer func() { cachedBackend = orig }()

	if got := unitName(2); got != "workspaces" {
		t.Errorf("unitName(2) with unknown = %q, want %q", got, "workspaces")
	}
}

func TestUnitNameCapitalized(t *testing.T) {
	orig := cachedBackend
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = orig }()

	if got := unitNameCap(1); got != "Tab" {
		t.Errorf("unitNameCap(1) with Ghostty = %q, want %q", got, "Tab")
	}
	if got := unitNameCap(3); got != "Tabs" {
		t.Errorf("unitNameCap(3) with Ghostty = %q, want %q", got, "Tabs")
	}
}
