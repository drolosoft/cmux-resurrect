package tui

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/agentskill"
)

// skillsBaseDir resolves the skills base directory for the given flavor.
// Overridable in tests.
var skillsBaseDir = func(codex bool) string {
	if codex {
		return agentskill.CodexDir()
	}
	return agentskill.ClaudeDir()
}

// execSkillInstall installs the agent skill for Claude Code (or Codex).
func (m *ShellModel) execSkillInstall(codex bool) {
	base := skillsBaseDir(codex)
	if base == "" {
		m.writeError("could not resolve the skills directory (no home dir?)")
		return
	}
	path, err := agentskill.Install(base)
	if err != nil {
		m.writeError("install skill: " + err.Error())
		return
	}
	fmt.Fprintf(m.output, "  %s Agent skill installed: %s\n\n",
		shellSuccessStyle.Render("✅"), path)
}
