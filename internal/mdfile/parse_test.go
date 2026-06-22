package mdfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

const testMD = `## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp        | dev      | yes | ~/projects/webapp                     |
- [x] | ⚙️ | api-server     | dev      | yes | ~/Git/go/44-api-server                 |
- [x] | 📊 | dashboard        | go       | yes | ~/projects/dashboard                       |
- [ ] | 🗿 | Obsidian       | single   | yes | ~/Library/Mobile Documents/iCloud~md~obsidian/Documents |

## Templates

### dev
- [x] main terminal (focused)
- [x] split right: ` + "`claude`" + `
- [x] split right: ` + "`lazygit`" + `

### go
- [x] main terminal (focused)
- [x] split right: ` + "`go test ./...`" + `

### single
- [x] main terminal (focused)
`

func writeTempMD(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "workspaces.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp MD: %v", err)
	}
	return path
}

func TestParse_Projects(t *testing.T) {
	path := writeTempMD(t, testMD)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(wf.Projects) != 4 {
		t.Fatalf("projects = %d, want 4", len(wf.Projects))
	}

	p0 := wf.Projects[0]
	if !p0.Enabled {
		t.Error("p0 should be enabled")
	}
	if p0.Icon != "🌐" {
		t.Errorf("p0.Icon = %q", p0.Icon)
	}
	if p0.Name != "webapp" {
		t.Errorf("p0.Name = %q", p0.Name)
	}
	if p0.Template != "dev" {
		t.Errorf("p0.Template = %q", p0.Template)
	}
	if !p0.Pin {
		t.Error("p0 should be pinned")
	}
	if p0.Path != "~/projects/webapp" {
		t.Errorf("p0.Path = %q", p0.Path)
	}

	// Disabled project.
	p3 := wf.Projects[3]
	if p3.Enabled {
		t.Error("p3 should be disabled")
	}
	if p3.Name != "Obsidian" {
		t.Errorf("p3.Name = %q", p3.Name)
	}
}

func TestParse_Templates(t *testing.T) {
	path := writeTempMD(t, testMD)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(wf.Templates) != 3 {
		t.Fatalf("templates = %d, want 3", len(wf.Templates))
	}

	dev, ok := wf.Templates["dev"]
	if !ok {
		t.Fatal("missing 'dev' template")
	}
	if len(dev.Panes) != 3 {
		t.Fatalf("dev panes = %d, want 3", len(dev.Panes))
	}

	// Main pane.
	if !dev.Panes[0].IsMain {
		t.Error("pane 0 should be main")
	}
	if !dev.Panes[0].Focus {
		t.Error("pane 0 should be focused")
	}

	// Claude split.
	if dev.Panes[1].Split != "right" {
		t.Errorf("pane 1 split = %q", dev.Panes[1].Split)
	}
	if dev.Panes[1].Command != "claude" {
		t.Errorf("pane 1 command = %q", dev.Panes[1].Command)
	}

	// Lazygit split.
	if dev.Panes[2].Command != "lazygit" {
		t.Errorf("pane 2 command = %q", dev.Panes[2].Command)
	}
}

func TestParse_EnabledProjects(t *testing.T) {
	path := writeTempMD(t, testMD)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	enabled := wf.EnabledProjects()
	if len(enabled) != 3 {
		t.Errorf("enabled = %d, want 3", len(enabled))
	}
}

func TestParse_ResolveTemplate(t *testing.T) {
	path := writeTempMD(t, testMD)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	panes := wf.ResolveTemplate("dev")
	if len(panes) != 3 {
		t.Fatalf("dev panes = %d, want 3", len(panes))
	}
	if panes[0].Type != "terminal" {
		t.Errorf("pane 0 type = %q", panes[0].Type)
	}
	if panes[1].Split != "right" {
		t.Errorf("pane 1 split = %q", panes[1].Split)
	}
	if panes[1].Command != "claude" {
		t.Errorf("pane 1 command = %q", panes[1].Command)
	}

	// Unknown template fallback.
	fallback := wf.ResolveTemplate("nonexistent")
	if len(fallback) != 1 {
		t.Errorf("fallback panes = %d, want 1", len(fallback))
	}
}

func TestParseTemplatePaneLine_FocusTargetDefault(t *testing.T) {
	wf, err := Parse("../../testdata/workspaces/minimal.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for name, tmpl := range wf.Templates {
		for i, tp := range tmpl.Panes {
			if tp.FocusTarget != -1 {
				t.Errorf("template %q pane %d: FocusTarget = %d, want -1", name, i, tp.FocusTarget)
			}
		}
	}
}

func TestParseTemplatePaneLine_BrowserSplit(t *testing.T) {
	md := `## Projects
**Icon | Name | Template | Pin | Path**

## Templates

### fullstack
- [x] main terminal: ` + "`make dev`" + ` (focused)
- [x] split right browser: ` + "`http://localhost:3000`" + `
- [x] split down: ` + "`lazygit`" + `
`
	path := writeTempMD(t, md)
	wf, err := Parse(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	tmpl, ok := wf.Templates["fullstack"]
	if !ok {
		t.Fatal("missing 'fullstack' template")
	}
	if len(tmpl.Panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(tmpl.Panes))
	}

	// Pane 1: split right browser with URL.
	p1 := tmpl.Panes[1]
	if p1.Split != "right" {
		t.Errorf("pane 1 split = %q, want 'right'", p1.Split)
	}
	if p1.Type != "browser" {
		t.Errorf("pane 1 type = %q, want 'browser'", p1.Type)
	}
	if p1.Command != "http://localhost:3000" {
		t.Errorf("pane 1 command = %q, want 'http://localhost:3000'", p1.Command)
	}

	// Pane 2: regular terminal split.
	p2 := tmpl.Panes[2]
	if p2.Type != "terminal" {
		t.Errorf("pane 2 type = %q, want 'terminal'", p2.Type)
	}
}

func TestParse_NonexistentFile(t *testing.T) {
	_, err := Parse("/tmp/does-not-exist-cmx.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseTemplatePaneLine_Tab(t *testing.T) {
	lines := []string{
		"- [x] main terminal: `plan` (focused)",
		"- [x] tab 2: `feature`",
		"- [x] split right: `diff`",
		"- [x] tab 2: `yazi`",
	}

	var pans []model.TemplatePan
	for _, line := range lines {
		tp, ok := parseTemplatePaneLine(line)
		if ok {
			pans = append(pans, tp)
		}
	}

	if len(pans) != 4 {
		t.Fatalf("got %d pans, want 4", len(pans))
	}
	if !pans[0].IsMain {
		t.Errorf("pans[0].IsMain = false, want true")
	}
	if pans[0].Command != "plan" {
		t.Errorf("pans[0].Command = %q, want plan", pans[0].Command)
	}
	if !pans[1].IsTab {
		t.Errorf("pans[1].IsTab = false, want true")
	}
	if pans[1].Command != "feature" {
		t.Errorf("pans[1].Command = %q, want feature", pans[1].Command)
	}
	if pans[2].Split != "right" {
		t.Errorf("pans[2].Split = %q, want right", pans[2].Split)
	}
	if !pans[3].IsTab {
		t.Errorf("pans[3].IsTab = false, want true")
	}
	if pans[3].Command != "yazi" {
		t.Errorf("pans[3].Command = %q, want yazi", pans[3].Command)
	}
}

func TestParseTemplatePaneLine_Names(t *testing.T) {
	cases := []struct {
		line      string
		wantName  string
		wantMain  bool
		wantTab   bool
		wantSplit string
		wantCmd   string
		wantFocus bool
	}{
		{`- [x] main terminal "Plan": ` + "`claude`" + ` (focused)`, "Plan", true, false, "", "claude", true},
		{`- [x] tab 2 "Feature": ` + "`claude`", "Feature", false, true, "", "claude", false},
		{`- [x] split right "Diff": ` + "`git diff`", "Diff", false, false, "right", "git diff", false},
		{`- [x] split down "Logs":`, "Logs", false, false, "down", "", false},
		// Backward compatible: no name.
		{`- [x] split right: ` + "`git diff`", "", false, false, "right", "git diff", false},
		{`- [x] main terminal: ` + "`claude`", "", true, false, "", "claude", false},
	}
	for _, c := range cases {
		tp, ok := parseTemplatePaneLine(c.line)
		if !ok {
			t.Fatalf("parse failed: %q", c.line)
		}
		if tp.Name != c.wantName {
			t.Errorf("%q: Name = %q, want %q", c.line, tp.Name, c.wantName)
		}
		if tp.IsMain != c.wantMain {
			t.Errorf("%q: IsMain = %v, want %v", c.line, tp.IsMain, c.wantMain)
		}
		if tp.IsTab != c.wantTab {
			t.Errorf("%q: IsTab = %v, want %v", c.line, tp.IsTab, c.wantTab)
		}
		if tp.Split != c.wantSplit {
			t.Errorf("%q: Split = %q, want %q", c.line, tp.Split, c.wantSplit)
		}
		if tp.Command != c.wantCmd {
			t.Errorf("%q: Command = %q, want %q", c.line, tp.Command, c.wantCmd)
		}
		if tp.Focus != c.wantFocus {
			t.Errorf("%q: Focus = %v, want %v", c.line, tp.Focus, c.wantFocus)
		}
	}
}
