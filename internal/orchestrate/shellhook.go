package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"
)

// DetectShell reads $SHELL and returns the basename (e.g. "zsh", "bash", "fish").
// Returns "" if $SHELL is unset.
func DetectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

// ShellHook returns a shell snippet that auto-starts `crex watch --daemon` on
// shell startup if the daemon is not already running. Returns "" for unsupported
// shells. Supported: "zsh", "bash", "fish".
func ShellHook(shell string) string {
	pidPath := DefaultPIDPath()

	switch shell {
	case "zsh", "bash":
		return fmt.Sprintf(`# crex auto-persistence — add to .zshrc or .bashrc
# Starts crex watch in daemon mode if not already running.
if [ -z "$CREX_NO_WATCH" ]; then
  if ! kill -0 "$(cat "%s" 2>/dev/null)" 2>/dev/null; then
    crex watch --daemon &>/dev/null
  fi
fi
`, pidPath)

	case "fish":
		return fmt.Sprintf(`# crex auto-persistence — add to ~/.config/fish/config.fish
# Starts crex watch in daemon mode if not already running.
if not set -q CREX_NO_WATCH
  if not kill -0 (cat "%s" 2>/dev/null) 2>/dev/null
    crex watch --daemon &>/dev/null
  end
end
`, pidPath)

	default:
		return ""
	}
}
