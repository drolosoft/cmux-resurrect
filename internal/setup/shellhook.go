package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const hookMarker = "# crex-pop-hook"

// DetectShell returns the shell name from $SHELL (e.g. "zsh", "bash", "fish").
func DetectShell() string {
	shell := os.Getenv("SHELL")
	return filepath.Base(shell)
}

// HookLine returns the shell-specific keybinding line for crex pop.
func HookLine(shell, key string) string {
	switch shell {
	case "zsh":
		return fmt.Sprintf(`bindkey -s '%s' 'crex pop\n' %s`, key, hookMarker)
	case "bash":
		return fmt.Sprintf(`bind '"%s": "crex pop\n"' %s`, key, hookMarker)
	case "fish":
		return fmt.Sprintf(`bind %s 'crex pop; commandline -f repaint' %s`, key, hookMarker)
	default:
		return ""
	}
}

// DefaultKey returns the default key binding string for the given shell.
func DefaultKey(shell string) string {
	switch shell {
	case "zsh":
		return "^G"
	case "bash":
		return "\\C-g"
	case "fish":
		return "\\cg"
	default:
		return ""
	}
}

// RCFilePath returns the default rc file path for the given shell.
func RCFilePath(shell string) string {
	home, _ := os.UserHomeDir()
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	default:
		return ""
	}
}

// InstallHookToFile appends the hook line to an rc file if not already present.
func InstallHookToFile(path, shell, key string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if strings.Contains(string(content), hookMarker) {
		return nil // already installed
	}
	line := HookLine(shell, key)
	if line == "" {
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if _, err = fmt.Fprintf(f, "\n%s\n", line); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// UninstallHookFromFile removes the hook line from an rc file.
func UninstallHookFromFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var lines []string
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.Contains(line, hookMarker) {
			lines = append(lines, line)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
