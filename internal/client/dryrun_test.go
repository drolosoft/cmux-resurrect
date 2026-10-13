package client

import "testing"

// The dry-run lines are what a user copies into a terminal to replay a
// restore by hand, so names with spaces and emoji must come out quoted.
func TestCmuxDryRun_GroupCommands(t *testing.T) {
	formatter := CmuxDryRun{}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "create",
			got:  formatter.FmtCreateGroup("🌋 Dos grupo", "workspace:74"),
			want: `cmux workspace-group create --name "🌋 Dos grupo" --from workspace:74`,
		},
		{
			name: "set-anchor",
			got:  formatter.FmtSetGroupAnchor("workspace_group:16", "workspace:74"),
			want: "cmux workspace-group set-anchor --group workspace_group:16 --workspace workspace:74",
		},
		{
			name: "new-workspace in group",
			got:  formatter.FmtNewWorkspaceInGroup("/Users/me/my project", "workspace_group:16"),
			want: `cmux new-workspace --cwd "/Users/me/my project" --group workspace_group:16 --group-placement end`,
		},
		{
			name: "collapse",
			got:  formatter.FmtCollapseGroup("workspace_group:16"),
			want: "cmux workspace-group collapse workspace_group:16",
		},
		{
			name: "pin",
			got:  formatter.FmtPinGroup("workspace_group:16"),
			want: "cmux workspace-group pin workspace_group:16",
		},
		{
			name: "set-color",
			got:  formatter.FmtSetGroupColor("workspace_group:16", "#FF8800"),
			want: `cmux workspace-group set-color workspace_group:16 --hex "#FF8800"`,
		},
		{
			name: "set-icon",
			got:  formatter.FmtSetGroupIcon("workspace_group:16", "flame"),
			want: `cmux workspace-group set-icon workspace_group:16 --symbol "flame"`,
		},
	}

	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s:\n got %s\nwant %s", testCase.name, testCase.got, testCase.want)
		}
	}
}

// Only the cmux formatter knows group commands; the Ghostty one must not
// pretend to, or a dry run would print commands Ghostty cannot run.
func TestGroupDryRunFormatter_OnlyCmux(t *testing.T) {
	var cmuxFormatter DryRunFormatter = CmuxDryRun{}
	if _, ok := cmuxFormatter.(GroupDryRunFormatter); !ok {
		t.Error("CmuxDryRun must implement GroupDryRunFormatter")
	}

	var ghosttyFormatter DryRunFormatter = GhosttyDryRun{}
	if _, ok := ghosttyFormatter.(GroupDryRunFormatter); ok {
		t.Error("GhosttyDryRun must not implement GroupDryRunFormatter")
	}
}
