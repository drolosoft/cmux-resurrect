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
	if err := exec.Command("pgrep", "-x", "Ghostty").Run(); err == nil {
		return BackendGhostty
	}
	return BackendUnknown
}
