package model

import "testing"

func TestIsDefaultGroupTitle(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"Group 2", true},
		{"Group 17", true},
		{"Group", false},
		{"Group 2b", false},
		{"My Group 2", false},
		{"group 2", false},
		{"🎮 Cmux", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := IsDefaultGroupTitle(tc.title); got != tc.want {
			t.Errorf("IsDefaultGroupTitle(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestAnchorTitle(t *testing.T) {
	cases := []struct {
		name      string
		title     string
		groupName string
		want      string
	}{
		{"default placeholder", "Group 2", "Backend", "Backend"},
		{"empty title", "", "Backend", "Backend"},
		{"title the user chose", "API gateway", "Backend", "API gateway"},
		{"no group to name it after", "Group 2", "", "Group 2"},
	}

	for _, tc := range cases {
		if got := AnchorTitle(tc.title, tc.groupName); got != tc.want {
			t.Errorf("%s: AnchorTitle(%q, %q) = %q, want %q", tc.name, tc.title, tc.groupName, got, tc.want)
		}
	}
}

func TestWorkspaceEffectiveTitle(t *testing.T) {
	cases := []struct {
		name      string
		workspace Workspace
		want      string
	}{
		{"anchor with the placeholder", Workspace{Title: "Group 2", Group: "Backend", GroupAnchor: true}, "Backend"},
		{"anchor without a title", Workspace{Title: "", Group: "Backend", GroupAnchor: true}, "Backend"},
		{"anchor the user renamed", Workspace{Title: "API gateway", Group: "Backend", GroupAnchor: true}, "API gateway"},
		{"member titled like a placeholder", Workspace{Title: "Group 2", Group: "Backend"}, "Group 2"},
		{"loose workspace", Workspace{Title: "notes"}, "notes"},
	}

	for _, tc := range cases {
		if got := tc.workspace.EffectiveTitle(); got != tc.want {
			t.Errorf("%s: EffectiveTitle() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNameAnchorsAfterGroups(t *testing.T) {
	layout := &Layout{
		Workspaces: []Workspace{
			{Title: "Group 2", Group: "Backend", GroupAnchor: true},
			{Title: "Group 7", Group: "Backend"},
			{Title: "Mine", Group: "Frontend", GroupAnchor: true},
		},
	}

	layout.NameAnchorsAfterGroups()

	want := []string{"Backend", "Group 7", "Mine"}
	for index, title := range want {
		if layout.Workspaces[index].Title != title {
			t.Errorf("workspace %d title = %q, want %q", index, layout.Workspaces[index].Title, title)
		}
	}
}

func TestAnchorShownAsGroup(t *testing.T) {
	cases := []struct {
		name      string
		title     string
		groupName string
		want      bool
	}{
		{"named after its group", "🎮 Cmux", "🎮 Cmux", true},
		{"cmux placeholder", "Group 7", "🎮 Cmux", true},
		{"no title", "", "🎮 Cmux", true},
		{"title chosen by the user", "manual", "Docs", false},
		{"no group to name it", "Group 7", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnchorShownAsGroup(tc.title, tc.groupName); got != tc.want {
				t.Errorf("AnchorShownAsGroup(%q, %q) = %v, want %v", tc.title, tc.groupName, got, tc.want)
			}
		})
	}
}
