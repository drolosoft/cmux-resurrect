package client

import (
	"os"
	"os/exec"
)

// DetectedBackend identifies which terminal backend is available.
type DetectedBackend string

const (
	BackendCmux    DetectedBackend = "cmux"
	BackendGhostty DetectedBackend = "ghostty"
	BackendUnknown DetectedBackend = "unknown"
)

// Detect returns which terminal backend is available.
// Priority: cmux (if env vars are set) > Ghostty (if app is running) > unknown.
func Detect() DetectedBackend {
	if os.Getenv("CMUX_SOCKET_PATH") != "" || os.Getenv("CMUX_WORKSPACE_ID") != "" {
		return BackendCmux
	}
	// pgrep -x "Ghostty" fails on macOS because the binary name is lowercase
	// "ghostty" while the app bundle is "Ghostty.app". Use osascript to check
	// via System Events, which matches the app name reliably.
	out, err := exec.Command("osascript", "-e",
		`tell application "System Events" to (name of processes) contains "Ghostty"`).Output()
	if err == nil && len(out) > 0 && out[0] == 't' { // "true\n"
		return BackendGhostty
	}
	return BackendUnknown
}
