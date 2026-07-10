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
// Priority: LIVE cmux (env vars set AND the socket answers) > Ghostty (app
// running) > cmux env alone > unknown. Presence of CMUX_* env is not enough:
// Ghostty sessions launched from within cmux inherit those vars, and when
// cmux is closed the socket is dead — picking it would break every command
// with "backend not reachable" even though a live Ghostty is right there.
func Detect() DetectedBackend {
	cmuxEnv := os.Getenv("CMUX_SOCKET_PATH") != "" || os.Getenv("CMUX_WORKSPACE_ID") != ""
	return detectWith(cmuxEnv, cmuxAlive, ghosttyRunning)
}

// detectWith is the pure decision core, with liveness probes injected for
// testability.
func detectWith(cmuxEnv bool, cmuxAlive, ghosttyRunning func() bool) DetectedBackend {
	if cmuxEnv {
		if cmuxAlive() {
			return BackendCmux
		}
		if ghosttyRunning() {
			return BackendGhostty
		}
		// Env says cmux and nothing else is available — report against
		// cmux so the error message names the right backend.
		return BackendCmux
	}
	if ghosttyRunning() {
		return BackendGhostty
	}
	return BackendUnknown
}

// cmuxAlive reports whether the cmux socket actually answers.
func cmuxAlive() bool {
	c := &CLIClient{Binary: "cmux", Timeout: 3 * time.Second}
	return c.Ping() == nil
}

// ghosttyRunning reports whether the Ghostty app is running.
// pgrep -x "Ghostty" fails on macOS because the binary name is lowercase
// "ghostty" while the app bundle is "Ghostty.app". Use osascript to check
// via System Events, which matches the app name reliably.
func ghosttyRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e",
		`tell application "System Events" to (name of processes) contains "Ghostty"`).Output()
	return err == nil && len(out) > 0 && out[0] == 't' // "true\n"
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
