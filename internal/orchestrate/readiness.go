package orchestrate

import (
	"fmt"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/client"
)

var (
	// ShellReadyTimeout is the maximum time to wait for a shell to become interactive.
	ShellReadyTimeout = 10 * time.Second

	// ShellReadyPoll is the interval between readiness checks (phase 1: CWD detection).
	ShellReadyPoll = 150 * time.Millisecond

	// StableCheckInterval is the poll rate during the stability phase.
	StableCheckInterval = 200 * time.Millisecond

	// StableRequiredCount is the number of consecutive identical reads to declare stable.
	StableRequiredCount = 3

	// StableMaxWait is the maximum time to spend in the stability phase.
	StableMaxWait = 3 * time.Second
)

// waitForShellReady polls the backend until the shell in the target pane
// has initialized and stabilized.
//
// Phase 1: Poll SidebarState until CWD is non-empty (shell process exists).
// Phase 2: Poll until N consecutive reads return the same CWD (shell stable).
//
// This approach sends NO text to the terminal — no probe commands, no
// temp files, no shell history pollution, no visible artifacts.
func waitForShellReady(c client.Backend, workspaceRef, surfaceRef string) error {
	deadline := time.Now().Add(ShellReadyTimeout)

	// Preferred path: deterministic per-surface readiness. The workspace-level
	// heuristic below can't see a brand-new split's shell (sidebar-state is
	// workspace-scoped and reflects pane 0), so it returns "ready" too early and
	// the follow-up cd/command is typed into a PTY that isn't draining input yet
	// (GitHub #8). When the backend can report the specific surface's readiness
	// (cmux via debug.terminals), gate on that instead.
	if ss, ok := c.(client.SurfaceStater); ok && surfaceRef != "" {
		for time.Now().Before(deadline) {
			if st, err := ss.SurfaceState(surfaceRef); err == nil && st != nil && st.Ready {
				return nil
			}
			time.Sleep(ShellReadyPoll)
		}
		// Timed out waiting for the surface; fall through is pointless (the
		// heuristic watches the wrong surface). Best-effort return — the
		// caller's cwd verify/retry is the real safety net.
		return nil
	}

	// Fallback (Ghostty, or no surface ref): workspace-level CWD heuristic.
	// Phase 1: Wait for CWD to appear.
	var lastCWD string
	for time.Now().Before(deadline) {
		sidebar, err := c.SidebarState(workspaceRef)
		if err == nil && sidebar != nil && sidebar.CWD != "" {
			lastCWD = sidebar.CWD
			break
		}
		time.Sleep(ShellReadyPoll)
	}

	if lastCWD == "" {
		return fmt.Errorf("shell not ready after %v", ShellReadyTimeout)
	}

	// Phase 2: Wait for stability — N consecutive identical reads.
	stableDeadline := time.Now().Add(StableMaxWait)
	consecutiveStable := 1

	for time.Now().Before(stableDeadline) && time.Now().Before(deadline) {
		time.Sleep(StableCheckInterval)

		sidebar, err := c.SidebarState(workspaceRef)
		if err != nil || sidebar == nil {
			consecutiveStable = 0
			continue
		}

		if sidebar.CWD == lastCWD {
			consecutiveStable++
		} else {
			lastCWD = sidebar.CWD
			consecutiveStable = 1
		}

		if consecutiveStable >= StableRequiredCount {
			return nil
		}
	}

	// Stability phase timed out — shell is ready (CWD appeared) but may
	// still be changing. This is acceptable; we don't error on instability.
	return nil
}

// noHistoryCmd prefixes a command with a space so shells with
// HIST_IGNORE_SPACE (zsh default, bash with HISTCONTROL=ignorespace)
// don't record it in history. The trailing \\n triggers Enter.
func noHistoryCmd(cmd string) string {
	return " " + cmd + "\\n"
}
