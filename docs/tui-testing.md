[Home](../README.md) > TUI Testing

# TUI Shell Testing via ttyd + Playwright

Internal guide for systematically testing the interactive shell (`crex tui`) using ttyd and Playwright. This is the only reliable way to test the TUI end-to-end — unit tests verify logic, but rendering bugs (like the tea.Println fix in v1.5.0) only surface in a real terminal.

## Automated Testing (Recommended)

Run the bundled runner to execute the full 27-case test matrix. It drives all test cases via Playwright and captures a screenshot per step for visual review.

```sh
# Requires ttyd running on port 7682 (see Manual Setup below):
node scripts/e2e-tui-runner.js [--port 7682] [--cases 1,2,5]
```

The runner outputs screenshots to `/tmp/crex-e2e/screenshots/` and a structured report to `/tmp/crex-e2e/report.json`. Review the screenshots after each run — the runner captures state; pass/fail is a visual judgment.

## Why This Exists

Bubble Tea's inline renderer can silently corrupt output when `View()` grows between renders. Unit tests pass while the shell is visually broken. ttyd gives us a real terminal we can drive programmatically.

## Manual Setup (for ad-hoc testing)

### 1. Build the binary

```sh
go build -o /tmp/crex-test ./cmd/crex
```

### 2. Start ttyd

```sh
ttyd -W -p 7682 /tmp/crex-test tui
```

- `-W` enables write access (required for sending input)
- `-p 7682` sets the port
- The shell launches automatically inside ttyd

### 3. Navigate Playwright to ttyd

```js
await page.goto('http://localhost:7682');
```

### 4. Wait for terminal ready

```js
// Wait for xterm.js to initialize
await page.waitForTimeout(2000);
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
await page.screenshot({ path: 'test-01-help.png' });  // descriptive name per step
```

Always screenshot after each command. For commands with delayed output (osascript calls), wait before capturing:

```js
new Promise(r => setTimeout(r, 3000)).then(() => 'waited')
// Then screenshot
```

## Full Test Matrix (27 cases)

Run these in order. Each test verifies rendering, error handling, and prompt recovery.
The automated runner (`scripts/e2e-tui-runner.js`) executes cases 1-27 sequentially.

### Core Shell (1-6)

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 1 | *(launch)* | Welcome message + `crex ->` prompt, help/exit highlighted | Init, welcome rendering |
| 2 | `help` | 6 groups with icons, colored headers, aligned columns | Multi-line output, ANSI alignment |
| 3 | `ls` | Numbered items in browse mode with cursor | Browse mode entry |
| 4 | `q` in browse | Return to prompt | Browse mode exit |
| 5 | `templates` | 16 templates in browse mode | Template listing |
| 6 | `q` in browse | Return to prompt | Browse exit consistency |

### Backend-Dependent (7-8, expect errors in ttyd)

| # | Command | Expected in ttyd | Validates |
|---|---------|-----------------|-----------|
| 7 | `now` | Error message (no backend in ttyd), prompt recovery | Error display + recovery |
| 8 | `save test` | Error message, prompt recovery | Save error path |

### Usage Errors (9-14)

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 9 | `restore` | `✗ Usage: restore <name\|#>` | Arg validation |
| 10 | `delete` (split: `delet` + `e\r`) | `✗ Usage: delete <name\|#>` | Arg validation (ttyd quirk) |
| 11 | `use` | `✗ Usage: use <template\|#>` | Arg validation |
| 12 | `bp add` | `✗ Usage: bp add <name> <path>` | Arg validation |
| 13 | `bp remove` | `✗ Usage: bp remove <name\|#>` | Arg validation |
| 14 | `bp toggle` | `✗ Usage: bp toggle <name\|#>` | Arg validation |

### Features (15-18)

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 15 | `watch status` | `watch daemon is not running` | Watch status |
| 16 | `bp list` | Entries in browse mode or empty message | Blueprint listing |
| 17 | `q` in browse | Return to prompt | Browse exit after bp list |
| 18 | `foobar` | `✗ Unknown command: foobar` | Unknown command handling |

### Tab Completion (19-22)

| # | Action | Expected | Validates |
|---|--------|----------|-----------|
| 19 | Tab on empty prompt | All commands with icons and descriptions | Level 1 completion |
| 20 | Escape | Completion dismissed, back to prompt | Completion dismiss |
| 21 | `bp` + Tab | bp subcommands: add, list, ls, remove, rm, toggle | Level 2 completion |
| 22 | `settings banner` + Tab | banner subcommands: get, list, set | Level 3 completion |

### Settings Output (23-24)

| # | Command | Expected | Validates |
|---|---------|----------|-----------|
| 23 | `settings banner get` | `Current banner style: flame` (green text) | Settings get rendering |
| 24 | `settings banner list` | 3 styles with aligned descriptions | Settings list rendering |

### Edge Cases (25-27)

| # | Action | Expected | Validates |
|---|--------|----------|-----------|
| 25 | Arrow Up x2 | Recalls previous commands in prompt | History navigation |
| 26 | Empty Enter | No output, stays in prompt | Empty input handling |
| 27 | `exit` | Phoenix bye message, ttyd shows reconnect | Clean exit |

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
