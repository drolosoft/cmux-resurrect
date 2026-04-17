# Post-Testing Fixes Prompt

**Give this to the session that built the Ghostty backend, on branch `feat/multi-backend`.**

---

## Context

The Ghostty backend was live-tested inside Ghostty 1.3.1 on macOS. **17/17 feature tests passed**, but two bugs were found and fixed by the tester. You need to review these fixes, understand why they happened, and apply any follow-on improvements.

Pull latest first — the fixes are already committed:
```sh
git pull origin feat/multi-backend
```

---

## Bug 1: `Detect()` fails to find Ghostty (commit `c214775`)

**File:** `internal/client/detect.go`

**Problem:** `pgrep -x "Ghostty"` returns exit code 1 even though Ghostty is running. On macOS, the binary name is lowercase `ghostty` (from `Ghostty.app/Contents/MacOS/ghostty`), and `pgrep -x` uses a process name field that doesn't match. Even `pgrep -x ghostty` fails — `pgrep` on macOS cannot reliably find GUI app processes.

**Fix applied:** Replaced `pgrep` with osascript System Events check:
```go
out, err := exec.Command("osascript", "-e",
    `tell application "System Events" to (name of processes) contains "Ghostty"`).Output()
if err == nil && len(out) > 0 && out[0] == 't' { // "true\n"
    return BackendGhostty
}
```

**Action needed:**
1. Review the fix in `detect.go` — it's already committed
2. Update `TestDetect_NoCmuxEnv` if it assumes `pgrep` behavior
3. Consider: should `Detect()` cache its result? It runs osascript on every `crex` invocation

---

## Bug 2: Dry-run tests assume cmux backend (commit `7490785`)

**File:** `cmd/template_test.go`

**Problem:** Four tests checked for cmux-specific strings in dry-run output:
```go
if !strings.Contains(output, "new-workspace") { // fails when Ghostty backend is active
```

When running inside Ghostty, auto-detection picks `BackendGhostty`, so the `GhosttyDryRun` formatter outputs `osascript: new tab...` instead of `cmux new-workspace...`.

**Fix applied:** Tests now accept either backend's format:
```go
if !strings.Contains(output, "new-workspace") && !strings.Contains(output, "new tab") {
    t.Error("dry-run output missing workspace creation command")
}
```

**Action needed:**
1. Review the fix in `template_test.go` — it's already committed
2. Consider: should dry-run tests force a specific backend instead? You could set `CMUX_SOCKET_PATH` in the test to force cmux detection, or inject the backend directly

---

## Test Results Summary

All features work. Here's the full report:

| Test | Result | Notes |
|------|--------|-------|
| Auto-detection | PASS | After detect fix |
| Ping | PASS | |
| Save (simple) | PASS | 3 workspaces, correct titles + CWDs |
| Save (with splits) | PASS | 3 panes captured |
| Restore (add mode) | PASS | Tabs + splits recreated, CWDs correct, caller tab skipped |
| Template use (`code`) | PASS | 3 terminals, correct CWD, title set with icon |
| Export to markdown | PASS | |
| Watch mode | PASS | Saves on interval, handles SIGTERM |
| PinWorkspace | PASS | Silent no-op |
| `go test ./...` | PASS | All packages green |
| `go vet ./...` | PASS | Clean |
| Integration tests | PASS | 16/16 |

**No permission prompts from macOS.** No Accessibility grants needed.

---

## Optional Follow-ups (not blocking)

These are observations from testing, not bugs:

1. **Tab index drift during multi-tab operations** — When restore creates multiple tabs, tab indices shift as tabs are added. If other tabs exist, the index-based ref (`tab:N`) can point to the wrong tab mid-operation. This didn't cause failures in testing but could in edge cases. Consider using tab IDs (returned by `new tab`) instead of positional indices.

2. **`initial input` on surface configuration** — The implementation uses `waitForShellReady` + `Send()` for startup commands. An alternative is passing `initial input` in the surface config at creation time, which lets Ghostty handle the timing internally. This was validated as working during AppleScript testing (see `ghostty-validation-tests.md`, Additional Discovery #8). Could simplify the `NewWorkspace` + command flow.

3. **Split direction not preserved in save** — `buildWorkspace()` in `save.go` defaults all non-first panes to `split: "right"`. Ghostty's API doesn't expose which direction a split was created with, so this is unavoidable. The merge logic preserves user-edited split directions from the TOML, which is the right approach.
