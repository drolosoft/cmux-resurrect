[Home](../README.md) > TUI Testing

# TUI Shell Testing via ttyd + Playwright

Internal guide for systematically testing the interactive shell (`crex tui`) using ttyd and Playwright MCP. This is the only reliable way to test the TUI end-to-end — unit tests verify logic, but rendering bugs (like the tea.Println fix in v1.5.0) only surface in a real terminal.

## Why This Exists

Bubble Tea's inline renderer can silently corrupt output when `View()` grows between renders. Unit tests pass while the shell is visually broken. ttyd gives us a real terminal we can drive programmatically.

## Prerequisites

```sh
brew install ttyd          # terminal sharing over HTTP
# Playwright MCP server must be configured
```

## Setup

### 1. Build the binary

```sh
go build -o /tmp/crex-test ./cmd/crex
```

### 2. Start ttyd

```sh
ttyd -W -p 7681 /tmp/crex-test tui
```

- `-W` enables write access (required for sending input)
- `-p 7681` sets the port
- The shell launches automatically inside ttyd

### 3. Navigate Playwright to ttyd

```js
// Via mcp__playwright__browser_navigate
"http://localhost:7681"
```

### 4. Wait for terminal ready

```js
// Via mcp__playwright__browser_evaluate
// Wait for xterm.js to initialize
new Promise(r => setTimeout(r, 2000)).then(() => 'ready')
```

## Sending Input

### Basic command

```js
window.term.input('help\r')    // \r = Enter (NOT \n)
```

**Critical:** Bubble Tea in raw mode expects `\r` (0x0D) for Enter. `\n` (0x0A) does NOT trigger `tea.KeyEnter`.

### Known quirk: some words get swallowed

The word `delete` sent as `window.term.input('delete\r')` doesn't register in ttyd. Workaround — split the input:

```js
// Option A: split before \r
window.term.input('delet');
window.term.input('e\r');

// Option B: type chars then enter separately
window.term.input('d'); window.term.input('e'); window.term.input('l');
window.term.input('e'); window.term.input('t'); window.term.input('e');
window.term.input('\r');
```

This is a ttyd/xterm.js quirk, not a crex bug. Other affected words: unknown — test if a command doesn't respond by splitting.

### Special keys

```js
window.term.input('q')              // single key (no Enter needed for browse quit)
window.term.input('\x1b[A')         // Arrow Up (history)
window.term.input('\x1b[B')         // Arrow Down (history)
window.term.input('\x03')           // Ctrl+C
window.term.input('\x1b')           // Escape
window.term.input('/')              // Filter mode in browse
```

### Scrolling

```js
window.term.scrollToTop()           // see full output history
window.term.scrollToBottom()        // return to live view
```

## Capturing Results

```js
// Via mcp__playwright__browser_screenshot
{ name: "test-01-help" }            // descriptive name for each test step
```

Always screenshot after each command. For commands with delayed output (osascript calls), wait before capturing:

```js
new Promise(r => setTimeout(r, 3000)).then(() => 'waited')
// Then screenshot
```

## Full Test Matrix

Run these in order. Each test verifies rendering, error handling, and prompt recovery.

### Core Shell

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 1 | *(launch)* | Welcome message + `crex>` prompt | Init, tea.Println welcome |
| 2 | `help` | 5 groups with icons, colors | Multi-line output flush |
| 3 | `ls` | Numbered items in browse mode | Browse mode entry |
| 4 | `q` in browse | Return to prompt | Browse mode exit |
| 5 | `templates` | 16 templates in browse mode | Template listing |
| 6 | `q` in browse | Return to prompt | Browse exit consistency |

### Backend-Dependent (expect errors in ttyd)

| # | Command | Expected in ttyd | Validates |
|---|---------|-----------------|-----------|
| 7 | `now` | `✗ tree: osascript: ...` | Error display + recovery |
| 8 | `save test` | `✗ get tree: ...` | Save error path |
| 9 | `restore <name>` | Restoring... then error or success | Restore flow |

### Usage Errors (no args)

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 10 | `restore` | `✗ Usage: restore <name\|#>` | Arg validation |
| 11 | `delete` | `✗ Usage: delete <name\|#>` | Arg validation |
| 12 | `use` | `✗ Usage: use <template\|#>` | Arg validation |
| 13 | `bp add` | `✗ Usage: bp add <name> <path>` | Arg validation |
| 14 | `bp remove` | `✗ Usage: bp remove <name\|#>` | Arg validation |
| 15 | `bp toggle` | `✗ Usage: bp toggle <name\|#>` | Arg validation |

### Features

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 16 | `watch status` | `watch daemon is not running` | Watch status |
| 17 | `bp list` | Entries in browse mode | Blueprint listing |
| 18 | `foobar` | `✗ Unknown command: foobar` | Unknown command handling |
| 19 | `ls` then `restore 2` | Resolves name from listing | Number references |
| 20 | `exit` | Shell terminates, back to zsh | Clean exit |

### Edge Cases

| # | Action | Expected | Validates |
|---|--------|----------|-----------|
| 21 | Empty Enter | No output, stay in prompt | Empty input handling |
| 22 | Ctrl+C | Shell terminates | Interrupt handling |
| 23 | Arrow Up after commands | Recalls previous command | History navigation |
| 24 | Arrow Down | Navigates forward in history | History navigation |
| 25 | `save` (no name) | Saves as "default" | Default name |

## Adding Tests for New Features

When adding a new shell command:

1. Add the command to the test matrix above
2. Build and start ttyd with the new binary
3. Run through the full matrix (takes ~5 minutes)
4. Screenshot each step with descriptive names
5. Document any new quirks in this guide

When modifying rendering (View, output, browse):

1. Run the FULL matrix — rendering bugs are subtle
2. Pay special attention to transitions: prompt → browse → prompt
3. Test commands that produce large output (help, ls with many items)
4. Test commands that produce small output (watch status, errors)

## Troubleshooting

**Shell doesn't respond to input:** Check that ttyd was started with `-W`. Without write access, `window.term.input()` silently does nothing.

**Enter doesn't submit:** You're using `\n` instead of `\r`. Always use `\r`.

**Output appears but prompt doesn't return:** Command is blocking (e.g., osascript timeout). Wait 5-10 seconds and screenshot again.

**Browse mode won't quit:** Send `q` as a single character without `\r`. Browse intercepts single keypresses.

**Text appears garbled:** Inline rendering corruption — this is the bug that tea.Println was designed to fix. If you see it, the View() output is too large or changing size between renders.
