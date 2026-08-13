package orchestrate

import (
	"testing"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

func TestTypeSafeCommand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			// The GitHub #8 field report: a layout saved by an older crex holds
			// a command truncated mid-JSON. Typed back it breaks the pane, so
			// only the binary survives — the tool starts fresh instead.
			name: "truncated json blob from an old layout",
			in:   `claude --settings {"preferredNotifChannel":"notifications_disabled","hooks":{...`,
			want: "claude",
		},
		{
			name: "unterminated quote",
			in:   `git commit -m "work in progres`,
			want: "git",
		},
		// Legitimate hand-written commands must survive untouched: users author
		// these in Blueprints and templates.
		{name: "balanced quotes kept", in: `git commit -m "wip"`, want: `git commit -m "wip"`},
		{name: "balanced braces kept", in: `jq '{a:1}' file.json`, want: `jq '{a:1}' file.json`},
		{name: "go ellipsis kept", in: "go test ./... -v", want: "go test ./... -v"},
		{name: "plain resume kept", in: "claude --resume abc-123", want: "claude --resume abc-123"},
		{name: "empty stays empty", in: "", want: ""},
		{name: "bare binary kept", in: "htop", want: "htop"},
		{
			// Nothing to fall back to but the whole string; leave it alone
			// rather than emit an empty command.
			name: "single unbalanced token",
			in:   `{oops`,
			want: `{oops`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := typeSafeCommand(c.in); got != c.want {
				t.Errorf("typeSafeCommand(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSanitizeLayoutCommands_CoversPanesAndTabs(t *testing.T) {
	broken := `claude --settings {"preferredNotifChannel":"notifications_disabled","hooks":{...`
	layout := &model.Layout{
		Workspaces: []model.Workspace{{
			Title: "proj",
			Panes: []model.Pane{{
				Type: "terminal", Command: broken,
				Surfaces: []model.Surface{
					{Type: "terminal", Command: broken},
					{Type: "terminal", Command: "npm run dev"},
				},
			}},
		}},
	}

	sanitizeLayoutCommands(layout)

	pane := layout.Workspaces[0].Panes[0]
	if pane.Command != "claude" {
		t.Errorf("pane command = %q, want claude", pane.Command)
	}
	if pane.Surfaces[0].Command != "claude" {
		t.Errorf("tab command = %q, want claude — old layouts break tabs too", pane.Surfaces[0].Command)
	}
	if pane.Surfaces[1].Command != "npm run dev" {
		t.Errorf("safe tab command was altered: %q", pane.Surfaces[1].Command)
	}
}
