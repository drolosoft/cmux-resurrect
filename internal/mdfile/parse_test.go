package mdfile

import (
	"os"
	"path/filepath"
	"testing"
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

func TestParse_NonexistentFile(t *testing.T) {
	_, err := Parse("/tmp/does-not-exist-cmx.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
