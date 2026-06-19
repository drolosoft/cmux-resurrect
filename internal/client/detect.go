package client

import (
	"context"
	"os"
	"os/exec"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e",
		`tell application "System Events" to (name of processes) contains "Ghostty"`).Output()
	if err == nil && len(out) > 0 && out[0] == 't' { // "true\n"
		return BackendGhostty
	}
	return BackendUnknown
}

// NewForOverride returns the Backend selected by an explicit override string
// (e.g. the CREX_BACKEND env var). Recognized values: "ghostty", "cmux",
// "cmux-applescript". For any unrecognized value it returns (nil, false) so the
// caller can warn and fall back to auto-detection. This is the single source of
// truth for override→backend mapping, shared by every command.
func NewForOverride(override string) (Backend, bool) {
	switch override {
	case "ghostty":
		return NewGhosttyClient(), true
	case "cmux-applescript":
		return NewGhosttyClientForApp("cmux"), true
	case "cmux":
		return NewCLIClient(), true
	default:
		return nil, false
	}
}

// NewForDetected returns the Backend for an auto-detected backend. It is the
// single source of truth for detected→backend mapping, shared by every command.
func NewForDetected(detected DetectedBackend) Backend {
	switch detected {
	case BackendGhostty:
		return NewGhosttyClient()
	default:
		return NewCLIClient()
	}
}
