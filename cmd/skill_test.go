package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/agentskill"
)

func TestSkillCmd_IsRegistered(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if strings.HasPrefix(c.Use, "skill") {
			return
		}
	}
	t.Error("skillCmd is not registered as a child of rootCmd")
}

func TestSkillShow_PrintsEmbeddedSkill(t *testing.T) {
	out, err := executeCmd(t, "skill", "show")
	if err != nil {
		t.Fatalf("skill show: %v", err)
	}
	if !strings.Contains(out, "name: crex") {
		t.Errorf("skill show output missing frontmatter, got: %.80s", out)
	}
}

func TestSkillInstall_CustomDir(t *testing.T) {
	dir := t.TempDir()
	out, err := executeCmd(t, "skill", "install", "--dir", dir)
	if err != nil {
		t.Fatalf("skill install: %v", err)
	}
	path := filepath.Join(dir, "crex", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("installed skill not found at %s: %v", path, err)
	}
	if string(data) != agentskill.Content() {
		t.Error("installed content differs from embedded")
	}
	if !strings.Contains(out, path) {
		t.Errorf("output should report the installed path, got: %s", out)
	}
}

func TestSetup_InstallsDemoLayout(t *testing.T) {
	// The wizard must ship the portable example layout on first run.
	home := t.TempDir()
	t.Setenv("HOME", home)
	out, err := executeCmd(t, "setup", "--defaults")
	if err != nil {
		t.Fatalf("setup --defaults: %v\n%s", err, out)
	}
	demoPath := filepath.Join(home, ".config", "crex", "layouts", "demo.toml")
	if _, err := os.Stat(demoPath); err != nil {
		t.Errorf("demo layout not installed at %s: %v", demoPath, err)
	}
}
