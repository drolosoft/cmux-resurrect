package detect

import (
	"os"
	"path/filepath"
	"strings"
)

// detectCopilot finds the active session for a GitHub Copilot CLI instance.
// Sessions are stored in ~/.copilot/session-state/<UUID>/workspace.yaml.
// Uses `copilot --continue` which auto-selects by CWD.
func detectCopilot(cwd, _ string) *Session {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	stateDir := filepath.Join(home, ".copilot", "session-state")
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		yamlPath := filepath.Join(stateDir, e.Name(), "workspace.yaml")
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "cwd:") {
				sessionCWD := strings.TrimSpace(strings.TrimPrefix(line, "cwd:"))
				if sessionCWD == cwd {
					return &Session{
						Tool:    "copilot",
						CWD:     cwd,
						Command: "copilot --continue",
					}
				}
			}
		}
	}
	return nil
}
