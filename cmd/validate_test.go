package cmd

// validate_test.go — comprehensive validation suite for v1.3.x features.
//
// Covers four feature areas across both backends:
//   1. Template shortcut (crex template <name> → crex template use <name>)
//   2. Adaptive theme system (flame/classic/plain, dark/light detection)
//   3. Workspace descriptions (persist across saves, show in crex show)
//   4. Branding — both backends
//
// Run:  go test ./cmd/ -run TestValidate -v -count=1
//       make validate

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
	"github.com/spf13/cobra"
)

// ═══════════════════════════════════════════════════════════════════════════
// 1. TEMPLATE SHORTCUT
// ═══════════════════════════════════════════════════════════════════════════

func TestValidate_Shortcut_AllTemplates(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/fake.sock")

	// Every template in the gallery should work via "template use <name> --dry-run".
	for _, tmpl := range gallery.List() {
		t.Run(tmpl.Name, func(t *testing.T) {
			output := executeTemplateCmd(t, "template", "use", tmpl.Name, "/tmp", "--dry-run")
			if !strings.Contains(output, "new-workspace") {
				t.Errorf("template %q missing 'new-workspace' in dry-run", tmpl.Name)
			}
		})
	}
}

func TestValidate_Shortcut_WithPathArg(t *testing.T) {
	t.Setenv("CMUX_SOCKET_PATH", "/tmp/fake.sock")

	explicit := executeTemplateCmd(t, "template", "use", "cols", "/tmp/testdir", "--dry-run")
	if !strings.Contains(explicit, "/tmp/testdir") {
		t.Error("use should contain the path argument")
	}
}

func TestValidate_Shortcut_ErrorForNonExistent(t *testing.T) {
	_, err := executeTemplateCmdErr(t, "template", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent template, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "does-not-exist") {
		t.Errorf("error should mention the bad name, got: %s", errMsg)
	}
}

func TestValidate_Shortcut_DoesNotBreakSubcommands(t *testing.T) {
	// list
	out := executeTemplateCmd(t, "template", "list")
	if !strings.Contains(out, "LAYOUTS") {
		t.Error("'template list' broken after shortcut addition")
	}

	// show
	out = executeTemplateCmd(t, "template", "show", "ide")
	if !strings.Contains(out, "ide") {
		t.Error("'template show ide' broken after shortcut addition")
	}

	// show --all
	out = executeTemplateCmd(t, "template", "show", "--all")
	if !strings.Contains(out, "16 templates") {
		t.Error("'template show --all' broken after shortcut addition")
	}

	// bare (help)
	out = executeTemplateCmd(t, "template")
	if !strings.Contains(out, "Template Gallery") {
		t.Error("bare 'template' help broken after shortcut addition")
	}
}

func TestValidate_Shortcut_TplAlias(t *testing.T) {
	out := executeTemplateCmd(t, "tpl", "list")
	if !strings.Contains(out, "LAYOUTS") {
		t.Error("'tpl list' broken")
	}

	out = executeTemplateCmd(t, "tpl")
	if !strings.Contains(out, "Template Gallery") {
		t.Error("bare 'tpl' help broken")
	}

	_, err := executeTemplateCmdErr(t, "tpl", "nope-nope")
	if err == nil {
		t.Error("'tpl nope-nope' should error")
	}
}

func TestValidate_Shortcut_HelpMentionsShortcut(t *testing.T) {
	out := executeTemplateCmd(t, "template")
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "shortcut") {
		t.Error("template help should mention the shortcut")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 2. ADAPTIVE THEME SYSTEM
// ═══════════════════════════════════════════════════════════════════════════

func TestValidate_Theme_DarkPalette(t *testing.T) {
	t.Setenv("CREX_THEME", "dark")
	th := detectTheme()

	if !th.dark {
		t.Fatal("CREX_THEME=dark should produce dark theme")
	}
	if th.green != lipgloss.Color("#5FFF87") {
		t.Errorf("dark green = %q, want #5FFF87", th.green)
	}
	if len(th.flame) != 9 {
		t.Errorf("dark flame has %d steps, want 9", len(th.flame))
	}
}

func TestValidate_Theme_LightPalette(t *testing.T) {
	t.Setenv("CREX_THEME", "light")
	th := detectTheme()

	if th.dark {
		t.Fatal("CREX_THEME=light should produce light theme")
	}
	if th.green != lipgloss.Color("#1A8A3E") {
		t.Errorf("light green = %q, want #1A8A3E", th.green)
	}
	if len(th.flame) != 9 {
		t.Errorf("light flame has %d steps, want 9", len(th.flame))
	}
}

func TestValidate_Theme_FlameGradientNoDuplicates(t *testing.T) {
	for name, grad := range map[string][]lipgloss.Color{
		"dark":  flameDark,
		"light": flameLight,
	} {
		seen := make(map[lipgloss.Color]bool)
		for _, c := range grad {
			if seen[c] {
				t.Errorf("%s gradient has duplicate color %s", name, c)
			}
			seen[c] = true
		}
	}
}

func TestValidate_Theme_BannerModeResolution(t *testing.T) {
	tests := []struct {
		env  string
		want bannerMode
	}{
		{"flame", bannerFlame},
		{"classic", bannerClassic},
		{"plain", bannerPlain},
		{"CLASSIC", bannerClassic},
		{"  plain  ", bannerPlain},
		{"", bannerFlame},
		{"invalid", bannerFlame},
	}
	for _, tt := range tests {
		t.Run("env="+tt.env, func(t *testing.T) {
			t.Setenv("CREX_BANNER", tt.env)
			got := resolveBannerMode()
			if got != tt.want {
				t.Errorf("CREX_BANNER=%q → %d, want %d", tt.env, got, tt.want)
			}
		})
	}
}

func TestValidate_Theme_BannerAllCombinations(t *testing.T) {
	// 2 backends × 3 modes × 2 themes = 12 combinations — all must produce output.
	type backendEntry struct {
		name    string
		backend client.DetectedBackend
	}
	backends := []backendEntry{
		{"cmux", client.BackendCmux},
		{"ghostty", client.BackendGhostty},
	}
	modes := []string{"flame", "classic", "plain"}
	themes := []string{"dark", "light"}

	for _, be := range backends {
		for _, mode := range modes {
			for _, th := range themes {
				name := be.name + "/" + mode + "/" + th
				t.Run(name, func(t *testing.T) {
					cachedBackend = be.backend
					defer func() { cachedBackend = client.Detect() }()
					t.Setenv("CREX_BANNER", mode)
					t.Setenv("CREX_THEME", th)

					out := banner()
					if len(out) == 0 {
						t.Fatal("banner produced empty output")
					}
					if !strings.Contains(out, "resurrected") {
						t.Error("banner missing 'resurrected' in tagline")
					}
				})
			}
		}
	}
}

func TestValidate_Theme_TaglineAccent(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()
	t.Setenv("CREX_THEME", "dark")

	th := detectTheme()

	for _, mode := range []bannerMode{bannerFlame, bannerClassic, bannerPlain} {
		out := renderTagline(th, mode)
		if !strings.Contains(out, "resurrected") {
			t.Errorf("mode %d tagline missing 'resurrected'", mode)
		}
	}
}

func TestValidate_Theme_GradientEdgeCases(t *testing.T) {
	grad := flameDark

	tests := []struct {
		name    string
		input   string
		reverse bool
	}{
		{"empty", "", false},
		{"single char", "x", false},
		{"all spaces", "     ", false},
		{"one visible", "  x  ", false},
		{"reversed", "abc", true},
		{"unicode", "·──────·", false},
		{"mixed", "  ·  ─  ·  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradientText(tt.input, grad, tt.reverse, false)
			for _, ch := range tt.input {
				if !strings.ContainsRune(got, ch) {
					t.Errorf("output missing rune %q from input", string(ch))
				}
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 3. WORKSPACE DESCRIPTIONS
// ═══════════════════════════════════════════════════════════════════════════

func TestValidate_Description_InTOML(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "toml-desc",
		Workspaces: []model.Workspace{
			{
				Title:       "api",
				Description: "backend service — postgres + redis",
				CWD:         "/home/user/api",
				Index:       1,
			},
		},
	}

	if err := store.Save("toml-desc", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Read raw TOML and verify description is serialized.
	raw, _ := os.ReadFile(store.Path("toml-desc"))
	if !strings.Contains(string(raw), "backend service") {
		t.Error("TOML file should contain the description text")
	}

	// Reload and verify roundtrip.
	loaded, _ := store.Load("toml-desc")
	if got := loaded.Workspaces[0].Description; got != "backend service — postgres + redis" {
		t.Errorf("loaded description = %q, want original", got)
	}
}

func TestValidate_Description_OmitEmptyInTOML(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "no-desc",
		Workspaces: []model.Workspace{
			{Title: "clean", CWD: "/tmp", Index: 1},
		},
	}

	if err := store.Save("no-desc", layout); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, _ := os.ReadFile(store.Path("no-desc"))
	if strings.Contains(string(raw), "description") {
		t.Error("TOML should omit empty description (omitempty)")
	}
}

func TestValidate_Description_MultipleWorkspaces(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	layout := &model.Layout{
		Name: "multi-desc",
		Workspaces: []model.Workspace{
			{Title: "api", Description: "backend", CWD: "/api", Index: 1},
			{Title: "web", Description: "", CWD: "/web", Index: 2},
			{Title: "db", Description: "postgres + redis", CWD: "/db", Index: 3},
		},
	}

	if err := store.Save("multi-desc", layout); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, _ := store.Load("multi-desc")

	if loaded.Workspaces[0].Description != "backend" {
		t.Error("ws[0] description lost")
	}
	if loaded.Workspaces[1].Description != "" {
		t.Error("ws[1] should have empty description")
	}
	if loaded.Workspaces[2].Description != "postgres + redis" {
		t.Error("ws[2] description lost")
	}
}

func TestValidate_Description_SpecialChars(t *testing.T) {
	dir := t.TempDir()
	store, _ := persist.NewFileStore(dir)

	special := `API "server" — handles <requests> & 'responses' (v2.0)`
	layout := &model.Layout{
		Name: "special-desc",
		Workspaces: []model.Workspace{
			{Title: "api", Description: special, CWD: "/api", Index: 1},
		},
	}

	if err := store.Save("special-desc", layout); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, _ := store.Load("special-desc")

	if loaded.Workspaces[0].Description != special {
		t.Errorf("special chars mangled: got %q", loaded.Workspaces[0].Description)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// 4. BRANDING — BOTH BACKENDS
// ═══════════════════════════════════════════════════════════════════════════

func TestValidate_Branding_CmuxIdentity(t *testing.T) {
	cachedBackend = client.BackendCmux
	defer func() { cachedBackend = client.Detect() }()

	if appTitle() != "crex (cmux-resurrect)" {
		t.Errorf("cmux title = %q", appTitle())
	}

	tag := appTagline()
	if !strings.HasPrefix(tag, "Terminal workspace manager for cmux") {
		t.Errorf("cmux tagline should lead with 'cmux': %q", tag)
	}

	if !isCmuxBranding() {
		t.Error("isCmuxBranding should be true for cmux")
	}

	b := banner()
	if !strings.Contains(b, "cmux") {
		t.Error("cmux banner should contain 'cmux'")
	}
	if !strings.Contains(b, "resurrect") {
		t.Error("cmux banner should contain 'resurrect'")
	}

	h := styledHelp()
	if !strings.Contains(h, "cmux-resurrect") {
		t.Error("cmux help should contain legacy name hint")
	}

	updateRootLong()
	if !strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("cmux root Long should mention cmux-resurrect")
	}
}

func TestValidate_Branding_GhosttyIdentity(t *testing.T) {
	cachedBackend = client.BackendGhostty
	defer func() { cachedBackend = client.Detect() }()

	if appTitle() != "crex" {
		t.Errorf("ghostty title = %q", appTitle())
	}

	tag := appTagline()
	if !strings.HasPrefix(tag, "Terminal workspace manager for Ghostty") {
		t.Errorf("ghostty tagline should lead with 'Ghostty': %q", tag)
	}

	if isCmuxBranding() {
		t.Error("isCmuxBranding should be false for Ghostty")
	}

	b := banner()
	if strings.Contains(b, "cmux-resurrect") {
		t.Error("ghostty banner should not contain 'cmux-resurrect'")
	}

	h := styledHelp()
	if strings.Contains(h, "cmux-resurrect") {
		t.Error("ghostty help should not contain legacy name hint")
	}

	updateRootLong()
	if strings.Contains(rootCmd.Long, "cmux-resurrect") {
		t.Error("ghostty root Long should not mention cmux-resurrect")
	}
}

func TestValidate_Branding_UnknownDefaultsToCrex(t *testing.T) {
	cachedBackend = client.BackendUnknown
	defer func() { cachedBackend = client.Detect() }()

	if appTitle() != "crex" {
		t.Errorf("unknown title = %q, want 'crex'", appTitle())
	}
	if isCmuxBranding() {
		t.Error("unknown backend should not use cmux branding")
	}
}

func TestValidate_Branding_SubcommandsNeutral(t *testing.T) {
	cmds := []*cobra.Command{
		saveCmd, restoreCmd, watchCmd,
		exportToMDCmd, importFromMDCmd, templateUseCmd,
	}
	for _, c := range cmds {
		lower := strings.ToLower(c.Short + " " + c.Long)
		if strings.Contains(lower, "cmux") {
			t.Errorf("%s mentions 'cmux' in description", c.Name())
		}
	}
}
