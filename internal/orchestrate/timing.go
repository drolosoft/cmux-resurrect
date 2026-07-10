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

	// CWDVerifyPoll is the interval between cwd checks / cd re-sends.
	CWDVerifyPoll = 150 * time.Millisecond
)

// CWDVerifyTimeout bounds the per-pane `cd` verify/retry window (GitHub #8)
// once the target shell is READY: the live surface cwd is polled and the bare
// cd re-sent until it sticks or this deadline passes. Var (not const) so tests
// can shrink it.
var CWDVerifyTimeout = 2 * time.Second
