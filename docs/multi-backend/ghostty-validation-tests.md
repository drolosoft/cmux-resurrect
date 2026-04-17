# Ghostty AppleScript Validation Tests

**Purpose:** Run these tests manually inside Ghostty to verify the AppleScript API works as documented before building the full backend.
**Requires:** Ghostty 1.3+ on macOS, running with at least one window open.

---

## Before You Start

1. Open Ghostty
2. Open a terminal tab inside Ghostty
3. Run these commands **from that terminal** (you're controlling Ghostty from within itself)
4. Grant Accessibility permissions if macOS prompts you (System Settings > Privacy & Security > Accessibility > Terminal/Ghostty)

---

## Test 1: Ping — Is Ghostty Running?

```sh
osascript -e 'tell application "System Events" to (name of processes) contains "Ghostty"'
```

**Expected:** `true`

---

## Test 2: Count Windows

```sh
osascript -e 'tell application "Ghostty" to count of windows'
```

**Expected:** A number (1 or more).

---

## Test 3: Count Tabs in Front Window

```sh
osascript -e 'tell application "Ghostty" to count of tabs of window 1'
```

**Expected:** A number matching your open tabs.

---

## Test 4: Read Tab Name

```sh
osascript -e 'tell application "Ghostty" to name of tab 1 of window 1'
```

**Expected:** The title shown on your first tab (usually the shell prompt or CWD).

---

## Test 5: Read Selected Tab

```sh
osascript -e 'tell application "Ghostty" to selected of tab 1 of window 1'
```

**Expected:** `true` if tab 1 is active, `false` otherwise.

---

## Test 6: Count Terminals in a Tab

```sh
osascript -e 'tell application "Ghostty" to count of terminals of tab 1 of window 1'
```

**Expected:** Number of split panes in that tab (1 if no splits).

---

## Test 7: Read Working Directory (CRITICAL)

```sh
osascript -e 'tell application "Ghostty" to working directory of terminal 1 of tab 1 of window 1'
```

**Expected:** The CWD of the first terminal (e.g., `/Users/txeo`).

### Test 7b: Does CWD Update After `cd`?

1. In the Ghostty terminal, run: `cd /tmp`
2. Wait 2 seconds
3. Run the same command again:

```sh
osascript -e 'tell application "Ghostty" to working directory of terminal 1 of tab 1 of window 1'
```

**Expected:** `/private/tmp` (or `/tmp`). If it still shows the old CWD, **this is a critical problem for `save`**.

---

## Test 8: Create a New Tab

```sh
osascript -e 'tell application "Ghostty" to new tab'
```

**Expected:** A new tab appears. Verify visually.

### Test 8b: New Tab with Working Directory

```sh
osascript -e 'tell application "Ghostty" to new tab with config "initial-working-directory=/tmp"'
```

**Expected:** A new tab opens with CWD at `/tmp`. Verify:

```sh
osascript -e 'tell application "Ghostty" to working directory of terminal 1 of tab (count of tabs of window 1) of window 1'
```

---

## Test 9: Select a Tab

First note which tab is active, then switch:

```sh
osascript -e 'tell application "Ghostty" to select tab 1 of window 1'
```

**Expected:** Tab 1 becomes active. Verify visually.

---

## Test 10: Rename a Tab (set_tab_title)

```sh
osascript -e 'tell application "Ghostty" to perform action "set_tab_title:crex-test"'
```

**Expected:** The **currently active** tab's title changes to "crex-test".

**Note:** This only renames the active tab. If you need to rename a specific tab, select it first:

```sh
osascript -e 'tell application "Ghostty"
    select tab 2 of window 1
    perform action "set_tab_title:my-workspace"
end tell'
```

### Test 10b: Does the Shell Overwrite the Title?

After renaming, press Enter in the terminal a few times. Does the title revert to the shell prompt?

**Expected behavior:** If the title reverts, we need the same delay-before-rename strategy crex already uses for cmux. Note the behavior here.

---

## Test 11: Split a Terminal

```sh
osascript -e 'tell application "Ghostty" to split right'
```

**Expected:** The current terminal splits horizontally. A new terminal appears to the right.

### Test 11b: Split Directions

```sh
osascript -e 'tell application "Ghostty" to split down'
```

**Expected:** A new terminal appears below.

### Test 11c: Count Terminals After Split

```sh
osascript -e 'tell application "Ghostty" to count of terminals of tab 1 of window 1'
```

**Expected:** One more than before the split.

---

## Test 12: Focus a Specific Terminal

After splitting (so you have 2+ terminals):

```sh
osascript -e 'tell application "Ghostty" to focus terminal 1 of tab 1 of window 1'
```

**Expected:** Terminal 1 (the original) gets focus. Verify by seeing the cursor move.

```sh
osascript -e 'tell application "Ghostty" to focus terminal 2 of tab 1 of window 1'
```

**Expected:** Terminal 2 (the split) gets focus.

---

## Test 13: Send Text to a Terminal

```sh
osascript -e 'tell application "Ghostty" to input text "echo hello-from-crex" of terminal 1 of tab 1 of window 1'
```

**Expected:** The text `echo hello-from-crex` appears in terminal 1 **but is NOT executed** (no newline sent).

### Test 13b: Send Text with Return

```sh
osascript -e 'tell application "Ghostty" to input text "echo hello-from-crex
" of terminal 1 of tab 1 of window 1'
```

**Expected:** The command executes and prints `hello-from-crex`. If the literal newline in the script doesn't work, try:

```sh
osascript -e 'tell application "Ghostty" to input text ("echo hello-from-crex" & return) of terminal 1 of tab 1 of window 1'
```

**Record which approach works.** This determines how `Send()` handles the `\n` suffix.

---

## Test 14: Close a Tab

Create a throwaway tab first, then close it:

```sh
osascript -e 'tell application "Ghostty" to new tab'
```

```sh
osascript -e 'tell application "Ghostty" to close tab (count of tabs of window 1) of window 1'
```

**Expected:** The last tab closes. If Ghostty shows a confirmation dialog, note that — crex may need to handle it.

---

## Test 15: Read Terminal ID

```sh
osascript -e 'tell application "Ghostty" to id of terminal 1 of tab 1 of window 1'
```

**Expected:** A unique identifier. Note the format (integer, UUID, string?). This is what we'll use as refs.

### Test 15b: Read Tab ID

```sh
osascript -e 'tell application "Ghostty" to id of tab 1 of window 1'
```

**Expected:** A unique identifier for the tab.

---

## Test 16: Enumerate All Tabs (Full Tree)

```sh
osascript -e 'tell application "Ghostty"
    set tabCount to count of tabs of window 1
    set output to ""
    repeat with t from 1 to tabCount
        set tabName to name of tab t of window 1
        set isSel to selected of tab t of window 1
        set termCount to count of terminals of tab t of window 1
        set output to output & "tab:" & t & "|" & tabName & "|selected:" & isSel & "|terminals:" & termCount & linefeed
    end repeat
    return output
end tell'
```

**Expected:** A list of all tabs with their names, selection state, and terminal count. This is the `Tree()` equivalent.

### Test 16b: Enumerate Terminals Within a Tab

```sh
osascript -e 'tell application "Ghostty"
    set termCount to count of terminals of tab 1 of window 1
    set output to ""
    repeat with term from 1 to termCount
        set termCWD to working directory of terminal term of tab 1 of window 1
        set output to output & "terminal:" & term & "|cwd:" & termCWD & linefeed
    end repeat
    return output
end tell'
```

**Expected:** A list of terminals with their CWDs. This is the `SidebarState()` equivalent per terminal.

---

## Test 17: Full Workflow — Create Workspace from Scratch

This simulates what `crex template use dev /tmp/myproject` would do:

```sh
osascript -e 'tell application "Ghostty"
    -- 1. Create tab
    new tab with config "initial-working-directory=/tmp"
    delay 0.5
    
    -- 2. Split right
    split right
    delay 0.3
    
    -- 3. Split the new terminal down
    split down
    delay 0.3
    
    -- 4. Focus terminal 1 and send command
    focus terminal 1 of tab (count of tabs of window 1) of window 1
    delay 0.1
    input text ("echo pane-1" & return) of terminal 1 of tab (count of tabs of window 1) of window 1
    
    -- 5. Send command to terminal 2
    input text ("echo pane-2" & return) of terminal 2 of tab (count of tabs of window 1) of window 1
    
    -- 6. Send command to terminal 3
    input text ("echo pane-3" & return) of terminal 3 of tab (count of tabs of window 1) of window 1
    
    -- 7. Rename tab
    select tab (count of tabs of window 1) of window 1
    delay 0.3
    perform action "set_tab_title:dev-workspace"
end tell'
```

**Expected:** A new tab named "dev-workspace" with 3 terminals arranged as:
```
┌──────────┬──────────┐
│          │  pane-2  │
│  pane-1  ├──────────┤
│          │  pane-3  │
└──────────┴──────────┘
```

Each terminal should have run its `echo` command.

---

## Results Template

Fill this in after running all tests:

| Test | Result | Notes |
|------|--------|-------|
| 1. Ping | PASS / FAIL | |
| 2. Count windows | PASS / FAIL | Value: |
| 3. Count tabs | PASS / FAIL | Value: |
| 4. Tab name | PASS / FAIL | Value: |
| 5. Selected tab | PASS / FAIL | |
| 6. Terminal count | PASS / FAIL | Value: |
| 7. Working directory | PASS / FAIL | Value: |
| 7b. CWD after cd | PASS / FAIL | **Critical** — does it update? |
| 8. New tab | PASS / FAIL | |
| 8b. New tab with CWD | PASS / FAIL | |
| 9. Select tab | PASS / FAIL | |
| 10. Rename tab | PASS / FAIL | |
| 10b. Shell overwrites title? | YES / NO | |
| 11. Split right | PASS / FAIL | |
| 11b. Split down | PASS / FAIL | |
| 11c. Terminal count after split | PASS / FAIL | Value: |
| 12. Focus terminal | PASS / FAIL | |
| 13. Send text (no return) | PASS / FAIL | |
| 13b. Send text (with return) | PASS / FAIL | Which syntax worked? |
| 14. Close tab | PASS / FAIL | Confirmation dialog? |
| 15. Terminal ID | PASS / FAIL | Format: |
| 15b. Tab ID | PASS / FAIL | Format: |
| 16. Enumerate tabs | PASS / FAIL | |
| 16b. Enumerate terminals | PASS / FAIL | |
| 17. Full workflow | PASS / FAIL | Layout correct? |

### Critical Unknowns to Resolve

- [ ] Does `working directory` update after `cd`? (Test 7b)
- [ ] Which `input text` + newline syntax works? (Test 13b)
- [ ] Does `perform action "set_tab_title"` get overwritten by shell? (Test 10b)
- [ ] Does `close tab` show a confirmation dialog? (Test 14)
- [ ] What format are terminal/tab IDs? (Test 15)
- [ ] Does Accessibility permission need to be granted? To which app?
