# crex Runbook -- Manual Integration Tests

> **Atuin Desktop compatible** -- Open this file in [Atuin Desktop](https://atuin.sh/)
> to run code blocks step-by-step with the play button,
> or use it as a manual reference.

Test every command end-to-end against a running cmux instance.

## Prerequisites

```sh
# Build and install
make build && make install
```

```sh
# Create test directories
mkdir -p /tmp/crex-test/{alpha,beta,dev,monitor,disabled,one,two,three,four}
```

```sh
# Verify cmux is running
cmux ping
```

---

## Fixture Files

Three workspace files in `testdata/workspaces/`:

| File | Projects | Templates | Tests |
|------|----------|-----------|-------|
| `minimal.md` | 2 single-pane | single | Basic sync, pin |
| `splits.md` | 2 enabled + 1 disabled | dev, monitor | Splits, disabled skip |
| `full.md` | 3 enabled + 1 disabled | dev (3 panes), go, single | Multi-split, numbering |

---

## Test 1: sync (minimal)

```sh
# Dry-run first
crex sync --config /dev/null --workspace-file testdata/workspaces/minimal.md --dry-run

# Expected: CREATE 2 workspaces (test-alpha pinned, test-beta unpinned)
```

```sh
# Real sync
crex sync

# Verify
cmux list-workspaces
# Expected: test-alpha and test-beta appear with correct titles
```

**Check:**
- [ ] 2 workspaces created
- [ ] Titles show icons: `test-alpha`, `test-beta`
- [ ] test-alpha is pinned
- [ ] test-beta is NOT pinned
- [ ] CWDs are `/tmp/crex-test/alpha` and `/tmp/crex-test/beta`

**Cleanup:**
```sh
# Close test workspaces (note the refs from list-workspaces)
cmux close-workspace --workspace workspace:XX
cmux close-workspace --workspace workspace:YY
```

---

## Test 2: sync with splits

```sh
crex sync --config /dev/null --workspace-file testdata/workspaces/splits.md --dry-run

# Expected: CREATE 2 workspaces (test-disabled is [ ] so skipped)
#   test-dev: 2 panes (main + split right)
#   test-monitor: 2 panes (main + split right)
```

```sh
crex sync
cmux list-workspaces
```

**Check:**
- [ ] 2 workspaces created (NOT 3 -- disabled is skipped)
- [ ] test-dev has 2 panes with split
- [ ] test-monitor has 2 panes with split
- [ ] Commands were sent (`echo "hello from split"`, etc.)

**Cleanup:** close test workspaces.

---

## Test 3: sync with full (multi-split + numbering)

```sh
crex sync --config /dev/null --workspace-file testdata/workspaces/full.md --dry-run
```

**Check:**
- [ ] 3 workspaces created (project-four disabled)
- [ ] project-one: 3 panes (main + right + down)
- [ ] project-two: 2 panes (main + right)
- [ ] project-three: 1 pane
- [ ] Titles include numbers: `1 project-one`, etc.

---

## Test 4: save

```sh
# First create some workspaces via sync
crex sync --config /dev/null --workspace-file testdata/workspaces/splits.md
```

```sh
# Save current layout
crex save runbook-test
```

```sh
# Verify
crex list
# Expected: runbook-test appears with workspace count and timestamp
```

**Check:**
- [ ] Layout saved without errors
- [ ] `crex list` shows it
- [ ] File exists in layouts dir

---

## Test 5: show

```sh
crex show runbook-test
# Expected: formatted display of workspaces, panes, CWDs
```

```sh
crex show runbook-test --raw
# Expected: raw TOML content
```

**Check:**
- [ ] Shows workspace names, CWDs, pane counts
- [ ] `--raw` shows valid TOML

---

## Test 6: edit

```sh
EDITOR=cat crex edit runbook-test
# Expected: prints TOML content (cat acts as editor)
```

**Check:**
- [ ] Opens the TOML file with $EDITOR

---

## Test 7: restore

```sh
# Dry-run first
crex restore runbook-test --dry-run
# Expected: lists cmux commands to recreate layout
```

```sh
# Close all test workspaces, then restore
crex restore runbook-test
cmux list-workspaces
# Expected: workspaces recreated with correct titles, splits, pins
```

**Check:**
- [ ] Dry-run shows expected commands
- [ ] Real restore creates correct workspaces
- [ ] Titles match
- [ ] Splits match
- [ ] Pinned state preserved

---

## Test 8: export

```sh
# With workspaces running, export to a temp file
crex export --workspace-file /tmp/crex-test/exported.md

cat /tmp/crex-test/exported.md
# Expected: valid workspace MD with current cmux state
```

**Check:**
- [ ] Markdown file created
- [ ] Projects section lists current workspaces
- [ ] Paths are correct

---

## Test 9: delete

```sh
crex list
```

```sh
crex delete runbook-test -f
```

```sh
crex list
# Expected: runbook-test no longer appears
```

**Check:**
- [ ] Layout removed from list
- [ ] File deleted from disk

---

## Test 10: project management

```sh
WF=/tmp/crex-test/project-test.md

# Start fresh
echo '## Projects
**Icon | Name | Template | Pin | Path**

## Templates

### single
- [x] main (focused)' > $WF

# Add projects
crex project add "web" ~/projects/web -i "W" -t dev --workspace-file $WF
crex project add "api" ~/projects/api -i "A" -t go --workspace-file $WF
crex project add "docs" ~/docs -i "D" -t single --disabled --workspace-file $WF
```

```sh
WF=/tmp/crex-test/project-test.md

# List enabled
crex project list --workspace-file $WF
# Expected: web, api (enabled)
```

```sh
WF=/tmp/crex-test/project-test.md

# List all
crex project list --all --workspace-file $WF
# Expected: web, api, docs (docs disabled)
```

```sh
WF=/tmp/crex-test/project-test.md

# Toggle
crex project toggle "docs" --workspace-file $WF
crex project list --workspace-file $WF
# Expected: web, api, docs (all enabled)
```

```sh
WF=/tmp/crex-test/project-test.md

# Remove
crex project remove "api" --workspace-file $WF
crex project list --all --workspace-file $WF
# Expected: web, docs
```

**Check:**
- [ ] `add` creates entries in MD file
- [ ] `list` shows only enabled
- [ ] `list --all` shows all
- [ ] `toggle` flips enabled state
- [ ] `remove` deletes entry

---

## Test 11: watch

```sh
crex watch runbook-watch --interval 10s &
WATCH_PID=$!

# Wait 15 seconds
sleep 15

# Check autosave
crex list
# Expected: runbook-watch appears

kill $WATCH_PID
```

**Check:**
- [ ] Autosave file created
- [ ] Content-hash deduplication (no duplicate if layout unchanged)

---

## Test 12: version

```sh
crex version
# Expected: ASCII banner with version, commit, and build date
```

---

## Test 13: re-sync idempotency

```sh
# Sync once
crex sync
```

```sh
# Sync again -- should skip existing
crex sync
# Expected: all SKIP (already exists)
```

**Check:**
- [ ] Second sync shows SKIP for all workspaces
- [ ] No duplicates created

---

## Cleanup

```sh
rm -rf /tmp/crex-test
crex delete runbook-test 2>/dev/null
crex delete runbook-watch 2>/dev/null
```
