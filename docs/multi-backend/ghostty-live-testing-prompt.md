# Ghostty Live Testing Prompt

**Run this from a Claude Code instance INSIDE Ghostty on Kosti.**

**Branch:** `feat/multi-backend`
**Repo:** `/Users/txeo/Git/drolosoft/cmux-resurrect`

Pull latest first:
```sh
git pull origin feat/multi-backend
```

Then build:
```sh
go build -o crex ./cmd/crex
```

---

## Pre-flight Checks

### Test 1: Auto-detection works
```sh
./crex version
```
**Expected:** Prints version. No crash, no "Ghostty backend not yet implemented" error. This confirms `Detect()` found Ghostty (via `pgrep -x Ghostty`) and `newClient()` returned a `GhosttyClient`.

### Test 2: Ping
```sh
# Quick Go test to verify Ping works:
go test ./internal/client/ -tags="integration,darwin" -run TestGhosttyPing -v
```
**Expected:** PASS (not skipped).

---

## Core Feature Tests

### Test 3: Save current layout
Open 2-3 tabs in Ghostty with different working directories. Then:
```sh
./crex save ghostty-test
```
**Expected:** Saves successfully, shows workspace count matching your tab count.

Verify the saved layout:
```sh
./crex show ghostty-test
```
**Expected:** Shows workspaces with titles and CWDs matching your Ghostty tabs.

### Test 4: Save with splits
In one Ghostty tab, create some splits (Cmd+D for right split, Cmd+Shift+D for down split). Then:
```sh
./crex save ghostty-splits
./crex show ghostty-splits
```
**Expected:** The workspace shows multiple panes. Pane count should match your terminal count in that tab.

### Test 5: Restore
Close some tabs, then:
```sh
./crex restore ghostty-test
```
**Expected:** New tabs appear in Ghostty, matching the saved layout. Check:
- [ ] Correct number of tabs created
- [ ] Tab titles match saved names
- [ ] CWDs are correct (check with `pwd` in each tab)
- [ ] Startup commands were executed (if any were saved)

### Test 6: Template use
```sh
./crex template use dev /tmp/crex-ghostty-test
```
**Expected:** A new tab appears in Ghostty with the dev template layout:
- [ ] Tab is named "dev" (or whatever the template specifies)
- [ ] Splits match the template definition
- [ ] CWD is `/tmp/crex-ghostty-test`

### Test 7: Export and Import
```sh
./crex save roundtrip-test
./crex export-to-md roundtrip-test --output /tmp/roundtrip.md
cat /tmp/roundtrip.md
```
**Expected:** Markdown Blueprint with workspace data.

Then close the tabs and:
```sh
./crex import-from-md /tmp/roundtrip.md
```
**Expected:** Tabs recreated from the Blueprint.

---

## Edge Cases

### Test 8: PinWorkspace no-op
Save a layout that had pinned workspaces in cmux, then restore. The pin silently does nothing — no error.

### Test 9: Watch mode
```sh
./crex watch ghostty-watch
```
Open/close tabs while watch is running.
**Expected:** Layout auto-saves on changes. Ctrl+C to stop. No crashes.

### Test 10: Multiple windows
If you have multiple Ghostty windows, check that save/restore uses the front window.

---

## Integration Tests (full suite)
```sh
go test ./internal/client/ -tags="integration,darwin" -v
```
**Expected:** All 4 integration tests PASS (not skipped).

---

## Report Back

For each test, note:
- **PASS** / **FAIL** / **PARTIAL**
- Any error messages (copy the full output)
- Any unexpected behavior (wrong CWDs, missing splits, etc.)
- AppleScript permission prompts (did macOS ask for anything?)
- Timing observations (did splits feel slow? were tabs created smoothly?)

**Critical unknowns to resolve:**
1. Does `crex save` correctly capture all tabs + splits?
2. Does `crex restore` recreate the layout correctly?
3. Does `crex template use dev` create the right splits in Ghostty?
4. Does the `\n` handling in Send work (commands actually execute)?
5. Any AppleScript permission issues?
