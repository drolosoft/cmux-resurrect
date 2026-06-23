package orchestrate

import "time"

// Timing constants for orchestration delays.
// These sleeps allow cmux to settle between operations so that
// subsequent commands target the correct workspace/surface.

const (
	// DelayAfterClose is the pause after closing each workspace.
	DelayAfterClose = 100 * time.Millisecond

	// DelayAfterCloseAll is the extra pause after a batch of workspace closes.
	DelayAfterCloseAll = 300 * time.Millisecond

	// DelayAfterCreate is the pause after cmux new-workspace.
	DelayAfterCreate = 300 * time.Millisecond

	// DelayAfterSelect is the pause after cmux select-workspace.
	DelayAfterSelect = 100 * time.Millisecond

	// DelayAfterSplit is the pause after cmux new-split,
	// giving the shell in the new pane time to initialize.
	DelayAfterSplit = 500 * time.Millisecond

	// DelayBeforeRename is the pause before cmux rename-workspace.
	// Shell prompt sets the terminal title on startup; renaming too
	// early gets overwritten.
	DelayBeforeRename = 500 * time.Millisecond

	// CWDVerifyTimeout bounds the per-pane `cd` verify/retry loop (GitHub #8):
	// after sending a cd, the live surface cwd is polled and the bare cd
	// re-sent until it sticks or this deadline passes.
	CWDVerifyTimeout = 2 * time.Second

	// CWDVerifyPoll is the interval between cwd checks / cd re-sends.
	CWDVerifyPoll = 150 * time.Millisecond
)
