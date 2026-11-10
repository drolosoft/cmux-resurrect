package detect

import (
	"os"
	"path/filepath"
	"testing"
)

// testMatcher builds a matcher over a subset of the real registry: plain
// names, a dashed name, a tool with a generic process name guarded by its
// install path, and one started through a launcher binary.
func testMatcher() processMatcher {
	return newProcessMatcher([]detector{
		{Name: "claude", ProcessNames: []string{"claude"}},
		{Name: "codex", ProcessNames: []string{"codex"}},
		{Name: "opencode", ProcessNames: []string{"opencode"}},
		{Name: "gemini", ProcessNames: []string{"gemini"}},
		{Name: "pi", ProcessNames: []string{"pi"}},
		{Name: "factory", ProcessNames: []string{"droid"}},
		{Name: "grok", ProcessNames: []string{"grok", "agent"}, PathMarker: "grok"},
		{Name: "cursor", ProcessNames: []string{"agent", "cursor-agent"}, PathMarker: "cursor-agent"},
		{Name: "rovo", ProcessNames: []string{"atlassian_cli_rovodev"}, LaunchPrefixes: []string{"acli rovodev"}},
	})
}

func TestMatchArgs(t *testing.T) {
	matcher := testMatcher()

	tests := []struct {
		name     string
		args     string
		wantTool string
		wantOK   bool
	}{
		// argv[0] basename is the tool — alias expansion, nvm/brew bin shim, wrapper named after the tool.
		{"nvm bin shim", "/Users/dev/.nvm/versions/node/v23.11.0/bin/claude --dangerously-skip-permissions", "claude", true},
		{"plain binary", "claude -c", "claude", true},
		{"alias expanded", "claude --resume abc123", "claude", true},
		{"dashed process name", "/opt/homebrew/bin/cursor-agent chat", "cursor", true},
		{"binary not named after the tool", "/Users/dev/.local/bin/droid", "factory", true},

		// Interpreter launching a tool script — comm would be "node", argv0 misses, path arg matches.
		{"node + script basename", "node /home/u/.local/share/codex/cli.js", "codex", true},
		{"node + path segment", "node /opt/lib/node_modules/opencode/dist/index.js serve", "opencode", true},
		{"bun + script", "bun /Users/x/.bun/install/global/node_modules/gemini/bin.js", "gemini", true},
		{"npx wrapper", "npx /tmp/cache/claude/cli.js", "claude", true},
		{"node in the cursor install dir", "node /Users/x/.local/share/cursor-agent/versions/2026.01/index.js", "cursor", true},

		// A launcher binary followed by the words that start the tool.
		{"launcher subcommand", "acli rovodev run", "rovo", true},
		{"launcher by full path", "/opt/homebrew/bin/acli rovodev run --yolo", "rovo", true},
		{"launcher without run", "acli rovodev", "rovo", true},
		{"tool binary of a launcher", "/Users/x/.rovodev/atlassian_cli_rovodev serve", "rovo", true},

		// False positives that MUST NOT match.
		{"tool as file arg to cat", "cat claude", "", false},
		{"tool as file arg to vim", "vim /Users/x/claude/notes.md", "", false},
		{"unrelated node app", "node /srv/app/server.js --port 3000", "", false},
		{"interpreter flag only", "node --inspect", "", false},
		{"substring not segment", "node /srv/apidocs/index.js", "", false}, // "pi" must not match inside "apidocs"
		{"generic name outside its install dir", "/usr/local/bin/agent --serve", "", false},
		{"generic name bare", "agent", "", false},
		{"generic name as script dir", "node /srv/agent/server.js", "", false},
		{"launcher with another subcommand", "acli jira workitem view", "", false},
		{"launcher alone", "acli", "", false},
		{"empty", "", "", false},
		{"shell only", "-zsh", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchArgs(tt.args)
			if ok != tt.wantOK {
				t.Fatalf("matchArgs(%q) ok = %v, want %v (tool=%q)", tt.args, ok, tt.wantOK, tool)
			}
			if ok && tool != tt.wantTool {
				t.Fatalf("matchArgs(%q) tool = %q, want %q", tt.args, tool, tt.wantTool)
			}
		})
	}
}

func TestMatchComm(t *testing.T) {
	matcher := testMatcher()

	tests := []struct {
		name     string
		comm     string
		wantTool string
		wantOK   bool
	}{
		{"full path", "/Users/dev/.local/bin/claude", "claude", true},
		{"bare name", "codex", "codex", true},
		{"binary not named after the tool", "/Users/dev/.local/bin/droid", "factory", true},
		{"second process name", "cursor-agent", "cursor", true},
		{"generic name inside its install dir", "/Users/dev/.local/share/cursor-agent/versions/2026.01/agent", "cursor", true},
		{"generic name outside its install dir", "/usr/local/bin/agent", "", false},
		{"generic name bare", "agent", "", false},
		{"unknown", "/usr/bin/htop", "", false},

		// Grok's installer links agent to the same binary as grok, in
		// ~/.grok/bin. The Cursor marker is longer, so it is tried first.
		{"grok's agent alias by path", "/Users/dev/.grok/bin/agent", "grok", true},
		{"cursor install under a home named after grok", "/Users/grokfan/.local/share/cursor-agent/versions/2026.11.05-86047df/agent", "cursor", true},

		// ps cuts a column that is not the last to 16 characters. A cut
		// path is no tool's, even when it ends in a name's first letters.
		{"comm cut to 16 characters", "/Users/dev/claud", "", false},
		{"node path cut to 16 characters", "/Users/dev1/.nvm", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchComm(tt.comm)
			if ok != tt.wantOK {
				t.Fatalf("matchComm(%q) ok = %v, want %v (tool=%q)", tt.comm, ok, tt.wantOK, tool)
			}
			if ok && tool != tt.wantTool {
				t.Fatalf("matchComm(%q) tool = %q, want %q", tt.comm, tool, tt.wantTool)
			}
		})
	}
}

// TestMatchArgs_InterpreterNames: the lab ran each tool from a real install
// and these are the command lines ps printed, with the lab folder renamed.
// Homebrew's and python.org's framework Pythons re-exec themselves as
// …/Python.app/Contents/MacOS/Python, a capital P that the interpreter list
// did not hold, so Aider was never recognised there. uv's Pythons are links
// to a versioned name such as python3.12.
func TestMatchArgs_InterpreterNames(t *testing.T) {
	matcher := newProcessMatcher(registry)

	tests := []struct {
		name     string
		args     string
		wantTool string
		wantOK   bool
	}{
		{"homebrew framework python", "/opt/homebrew/Cellar/python@3.12/3.12.11/Frameworks/Python.framework/Versions/3.12/Resources/Python.app/Contents/MacOS/Python /Users/dev/.local/bin/aider", "aider", true},
		{"python.org framework python", "/Library/Frameworks/Python.framework/Versions/3.13/Resources/Python.app/Contents/MacOS/Python /Users/dev/.local/bin/aider --yes-always", "aider", true},
		{"uv venv python", "/Users/dev/.local/share/uv/tools/aider-chat/bin/python /Users/dev/.local/bin/aider", "aider", true},
		{"versioned python", "/opt/homebrew/opt/python@3.12/bin/python3.12 /Users/dev/.local/bin/aider", "aider", true},
		{"python 3.14", "python3.14 /usr/local/bin/aider --restore-chat-history", "aider", true},
		{"bun by full path", "/Users/dev/.bun/bin/bun /Users/dev/.bun/bin/qwen", "qwen", true},

		// Gemini's npm launcher starts itself again as a child with a heap
		// option in front of the script, from the absolute node path.
		{"gemini launcher", "node /Users/dev/.npm-global/bin/gemini", "gemini", true},
		{"gemini child with a heap option", "/Users/dev/.nvm/versions/node/v23.11.0/bin/node --max-old-space-size=18432 /Users/dev/.npm-global/bin/gemini", "gemini", true},

		{"framework python running something else", "/Library/Frameworks/Python.framework/Versions/3.13/Resources/Python.app/Contents/MacOS/Python /srv/app/manage.py runserver", "", false},
		{"a program whose name only starts like python", "/usr/local/bin/pythonista /Users/dev/.local/bin/aider", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchArgs(tt.args)
			if ok != tt.wantOK || tool != tt.wantTool {
				t.Errorf("matchArgs(%q) = (%q, %v), want (%q, %v)", tt.args, tool, ok, tt.wantTool, tt.wantOK)
			}
		})
	}
}

// TestInterpreterFamily: an interpreter is known by its family whatever its
// case or version suffix.
func TestInterpreterFamily(t *testing.T) {
	tests := map[string]string{
		"node":       "node",
		"Python":     "python",
		"python3":    "python",
		"python3.12": "python",
		"python3.14": "python",
		"bun":        "bun",
		"ruby3.3":    "ruby",
		"perl5.34":   "perl",
		"ts-node":    "ts-node",
		"pythonista": "pythonista",
		"zsh":        "zsh",
	}
	for name, want := range tests {
		if got := interpreterFamily(name); got != want {
			t.Errorf("interpreterFamily(%q) = %q, want %q", name, got, want)
		}
	}
}

// hermesLauncherScript is the start and the end of the 7.8 KB program that
// the hermes launcher of install.sh hands its own python3 with -c, as ps
// prints it: newlines show as \012. The middle, which repairs a checkout an
// interrupted `hermes update` left behind, is cut out.
const hermesLauncherScript = `import os, re, sys\012import contextlib, hashlib, io, pathlib, shutil, subprocess\012os.environ.pop('PYTHONHOME', None)\012os.environ.pop('PYTHONPATH', None)\012sys.path.insert(0, '/Users/dev/.hermes/hermes-agent')\012def _hermes_closure_repair():\012    import contextlib, hashlib, io, shutil, subprocess\012    from pathlib import Path\012    root = Path('/Users/dev/.hermes/hermes-agent')\012    git = root / '.git'\012` +
	`sys._hermes_pin_default_home = True\012try:\012    import hermes_bootstrap\012except Exception:\012    _hermes_closure_repair()\012    raise\012if not os.environ.get('HERMES_HOME'):\012    from hermes_constants import get_default_hermes_root\012    os.environ['HERMES_HOME'] = str(get_default_hermes_root())\012from hermes_cli.main import main\012sys.argv[0] = re.sub(r'(-script\.pyw|\.exe)?$', '', sys.argv[0])\012sys.exit(main())\012`

// hermesPython is the interpreter install.sh puts under ~/.hermes/tools.
// No part of its path, nor of the script, is called hermes.
const hermesPython = "/Users/dev/.hermes/tools/python-3.14.9+20261021-darwin-arm64/bin/python3"

// TestMatchArgs_Hermes: Hermes Agent runs as its own python3 with the
// program above as -c, its --tui adds a Node child, and the client starts a
// gateway daemon with `-m hermes_cli.main … gateway run`. The client and the
// TUI are a session; the gateway runs for every session at once, detached
// from any terminal, and is never one.
func TestMatchArgs_Hermes(t *testing.T) {
	matcher := newProcessMatcher(registry)

	tests := []struct {
		name     string
		args     string
		wantTool string
		wantOK   bool
	}{
		{"classic client", hermesPython + " -I -c " + hermesLauncherScript, "hermes", true},
		{"client resuming a session", hermesPython + " -I -c " + hermesLauncherScript + " --resume local-63000000000000000000000000000000000000000000000000000000000028d3", "hermes", true},
		{"client with --tui", hermesPython + " -I -c " + hermesLauncherScript + " --tui", "hermes", true},
		{"tui child", "/Users/dev/.hermes/tools/node-26.12.1-darwin-arm64/bin/node --expose-gc /Users/dev/.hermes/hermes-agent/ui-tui/dist/entry.js", "hermes", true},
		{"pip install", "/Users/dev/.local/share/uv/tools/hermes-agent/bin/python /Users/dev/.local/bin/hermes", "hermes", true},

		{"gateway daemon", hermesPython + " -m hermes_cli.main --profile default gateway run --quiet --idle-exit", "", false},
		{"gateway started by hand", hermesPython + " -I -c " + hermesLauncherScript + " gateway run", "", false},
		{"pip gateway", "/Users/dev/.local/share/uv/tools/hermes-agent/bin/python /Users/dev/.local/bin/hermes gateway run", "", false},
		{"editor on the source", "vim /Users/dev/src/hermes-agent/hermes_cli/main.py", "", false},
		{"another node app with the same flags", "node --expose-gc /srv/app/dist/entry.js", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, ok := matcher.matchArgs(tt.args)
			if ok != tt.wantOK || tool != tt.wantTool {
				t.Errorf("matchArgs(%.120q…) = (%q, %v), want (%q, %v)", tt.args, tool, ok, tt.wantTool, tt.wantOK)
			}
		})
	}
}

// The Cursor installer puts both `agent` and `cursor-agent` in ~/.local/bin
// as links into ~/.local/share/cursor-agent/versions/<version>/. The link
// path says nothing about Cursor; the file it points to does.
func TestMatchArgs_GenericNameThroughSymlink(t *testing.T) {
	home := t.TempDir()
	versionDir := filepath.Join(home, ".local", "share", "cursor-agent", "versions", "2026.01")
	binDir := filepath.Join(home, ".local", "bin")
	for _, dir := range []string{versionDir, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	launcher := filepath.Join(versionDir, "cursor-agent")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(binDir, "agent")
	if err := os.Symlink(launcher, link); err != nil {
		t.Fatal(err)
	}

	matcher := testMatcher()

	tool, ok := matcher.matchArgs(link + " --model auto")
	if !ok || tool != "cursor" {
		t.Errorf("matchArgs through the agent link = (%q, %v), want (cursor, true)", tool, ok)
	}

	tool, ok = matcher.matchComm(link)
	if !ok || tool != "cursor" {
		t.Errorf("matchComm through the agent link = (%q, %v), want (cursor, true)", tool, ok)
	}

	// A link named agent that points anywhere else stays unmatched.
	otherTarget := filepath.Join(home, "other-agent")
	if err := os.WriteFile(otherTarget, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	otherLink := filepath.Join(home, "bin-other", "agent")
	if err := os.MkdirAll(filepath.Dir(otherLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherTarget, otherLink); err != nil {
		t.Fatal(err)
	}
	if tool, ok := matcher.matchArgs(otherLink); ok {
		t.Errorf("matchArgs(%q) = %q, want no match", otherLink, tool)
	}
}

// TestMatchArgs_CursorAndGrokAgent: the command lines ps printed in the lab
// for Cursor's launcher, which re-execs its bundled node with argv[0] set to
// the link it was started through, and for Grok's agent alias by path. Both
// go through real links, as the installers make them.
func TestMatchArgs_CursorAndGrokAgent(t *testing.T) {
	home := t.TempDir()
	versionDir := filepath.Join(home, ".local", "share", "cursor-agent", "versions", "2026.11.05-86047df")
	grokDownloads := filepath.Join(home, ".grok", "downloads")
	for _, dir := range []string{versionDir, filepath.Join(home, ".local", "bin"), grokDownloads, filepath.Join(home, ".grok", "bin")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cursorLauncher := filepath.Join(versionDir, "cursor-agent")
	grokBinary := filepath.Join(grokDownloads, "grok-macos-aarch64")
	for _, file := range []string{cursorLauncher, grokBinary} {
		if err := os.WriteFile(file, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cursorLink := filepath.Join(home, ".local", "bin", "agent")
	grokLink := filepath.Join(home, ".grok", "bin", "agent")
	if err := os.Symlink(cursorLauncher, cursorLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../downloads/grok-macos-aarch64", grokLink); err != nil {
		t.Fatal(err)
	}

	matcher := newProcessMatcher(registry)

	cursorArgs := cursorLink + " --use-system-ca " + filepath.Join(versionDir, "index.js") + " --force"
	if tool, ok := matcher.matchArgs(cursorArgs); !ok || tool != "cursor" {
		t.Errorf("matchArgs(cursor launcher) = (%q, %v), want (cursor, true)", tool, ok)
	}
	if tool, ok := matcher.matchComm(cursorLink); !ok || tool != "cursor" {
		t.Errorf("matchComm(cursor link) = (%q, %v), want (cursor, true)", tool, ok)
	}

	if tool, ok := matcher.matchArgs(grokLink + " --always-approve"); !ok || tool != "grok" {
		t.Errorf("matchArgs(grok's agent link) = (%q, %v), want (grok, true)", tool, ok)
	}
}
