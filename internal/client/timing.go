package client

import "time"

// Timing constants for CLI polling loops.
// After creating workspaces or splits, the client polls cmux to
// discover the newly-created ref. These values control how long
// and how often it polls.

const (
	// PollInterval is the pause between successive list/tree polls.
	PollInterval = 100 * time.Millisecond

	// NewWorkspaceDeadline is how long to poll for a new workspace ref
	// after cmux new-workspace succeeds.
	NewWorkspaceDeadline = 5 * time.Second

	// NewSplitDeadline is how long to poll for a new surface ref
	// after cmux new-split succeeds.
	NewSplitDeadline = 3 * time.Second
)
