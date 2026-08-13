package orchestrate

import (
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// crex replays a saved command by TYPING it into a shell, so the text has to be
// a complete, parseable command line. Layouts written by older versions can hold
// a command that was truncated mid-argument — cmux launches Claude through a
// wrapper carrying a JSON `--settings` blob, and the truncated remains blow up
// the pane with a JSON parse error (GitHub #8 field report).
//
// Rather than guess where the cut happened, an incomplete command is reduced to
// its binary name, which is always safe to type: at worst the tool starts fresh,
// exactly as a user would launch it by hand. AI detection then upgrades a bare
// tool name to a proper resume command on the next save.
//
// Detection is structural, not heuristic: a command is incomplete when its
// quotes or brackets don't balance. Hand-written commands (`git commit -m "wip"`,
// `jq '{a:1}' f.json`, `go test ./...`) balance and pass through untouched.

// unbalanced reports whether s has an unterminated quote or bracket.
func unbalanced(s string) bool {
	var quote rune // active quote character, 0 when outside quotes
	depth := map[rune]int{'{': 0, '[': 0, '(': 0}
	closers := map[rune]rune{'}': '{', ']': '[', ')': '('}

	for _, r := range s {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue // brackets inside quotes are literal text
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		case '{', '[', '(':
			depth[r]++
		case '}', ']', ')':
			depth[closers[r]]--
		}
	}
	if quote != 0 {
		return true
	}
	for _, n := range depth {
		if n != 0 {
			return true
		}
	}
	return false
}

// typeSafeCommand returns a command that can be typed into a shell verbatim,
// reducing an incomplete one to its binary name.
func typeSafeCommand(cmd string) string {
	if cmd == "" || !unbalanced(cmd) {
		return cmd
	}
	if bin, _, found := strings.Cut(cmd, " "); found {
		return bin
	}
	return cmd // single token: nothing safer to fall back to
}

// sanitizeLayoutCommands repairs every command in a layout before restore, so a
// layout saved by an older version can't break the panes it recreates. Applied
// to panes and to their extra tabs alike.
func sanitizeLayoutCommands(layout *model.Layout) {
	if layout == nil {
		return
	}
	for i := range layout.Workspaces {
		for j := range layout.Workspaces[i].Panes {
			p := &layout.Workspaces[i].Panes[j]
			p.Command = typeSafeCommand(p.Command)
			for k := range p.Surfaces {
				p.Surfaces[k].Command = typeSafeCommand(p.Surfaces[k].Command)
			}
		}
	}
}
