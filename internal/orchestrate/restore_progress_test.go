package orchestrate

import (
	"slices"
	"testing"
)

// TestRestoreProgress_ReportsGroupAndAnchor: OnWorkspace gets every restored
// workspace with the group it went into, so the CLI can print it under its
// group header, and OnProgress then no longer sees those workspaces.
func TestRestoreProgress_ReportsGroupAndAnchor(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	var reported []WorkspaceProgress
	var notices []string
	restorer := &Restorer{
		Client:   fake,
		Store:    storeRestoreLayout(t, restoreOneGroupLayout()),
		SkipPing: true,
		OnWorkspace: func(progress WorkspaceProgress) {
			reported = append(reported, progress)
		},
		OnProgress: func(message string, _ int, _ error) {
			notices = append(notices, message)
		},
	}

	if _, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true); err != nil {
		t.Fatalf("restore: %v", err)
	}

	if len(reported) != 3 {
		t.Fatalf("reported %d workspaces, want 3: %+v", len(reported), reported)
	}

	anchor := reported[0]
	// The layout stores the anchor as "Group 1"; it is reported under the
	// group name it was restored with.
	if anchor.Workspace.Title != "Backend" || !anchor.Workspace.GroupAnchor {
		t.Errorf("first report = %+v, want the anchor named Backend", anchor.Workspace)
	}
	if anchor.Group == nil || anchor.Group.Name != "Backend" {
		t.Errorf("anchor group = %+v, want Backend", anchor.Group)
	}
	if anchor.Err != nil || anchor.Skipped {
		t.Errorf("anchor reported err=%v skipped=%v, want a plain success", anchor.Err, anchor.Skipped)
	}

	if reported[2].Group != nil {
		t.Errorf("loose workspace reported in group %+v", reported[2].Group)
	}

	if slices.Contains(notices, "Group 1") || slices.Contains(notices, "notes") {
		t.Errorf("OnProgress still received workspaces: %v", notices)
	}
}

// TestRestoreProgress_SkippedIsFlagged: a workspace left as it is reports
// Skipped, so callers do not have to parse the error text.
func TestRestoreProgress_SkippedIsFlagged(t *testing.T) {
	t.Parallel()

	fake := newRestoreGroupFake()
	if _, err := fake.addWorkspace("notes", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var reported []WorkspaceProgress
	restorer := &Restorer{
		Client:   fake,
		Store:    storeRestoreLayout(t, restoreOneGroupLayout()),
		SkipPing: true,
		OnWorkspace: func(progress WorkspaceProgress) {
			reported = append(reported, progress)
		},
	}

	if _, err := restorer.Restore("one-group", false, RestoreModeAdd, "", true); err != nil {
		t.Fatalf("restore: %v", err)
	}

	last := reported[len(reported)-1]
	if last.Workspace.Title != "notes" || !last.Skipped || last.Err == nil {
		t.Errorf("last report = %+v, want notes skipped", last)
	}
}

// TestRestoreProgress_DryRunNamesAnchorByGroup: the comment heading each
// workspace in a dry run names a placeholder anchor by its group instead of
// "Group 1".
func TestRestoreProgress_DryRunNamesAnchorByGroup(t *testing.T) {
	t.Parallel()

	fake := &restoreFake{}
	restorer := &Restorer{Client: fake, Store: storeRestoreLayout(t, restoreOneGroupLayout())}

	result, err := restorer.Restore("one-group", true, RestoreModeAdd, "", true)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	if !slices.Contains(result.Commands, "# Backend (anchor)") {
		t.Errorf("dry run has no anchor heading:\n%v", result.Commands)
	}
	if slices.Contains(result.Commands, "# Group 1") {
		t.Errorf("dry run still heads the anchor as Group 1")
	}
	if !slices.Contains(result.Commands, "# api") {
		t.Errorf("dry run lost the member heading:\n%v", result.Commands)
	}
}
