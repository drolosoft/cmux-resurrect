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
