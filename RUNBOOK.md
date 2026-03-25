# cmres Runbook — Manual Integration Tests

Test every command end-to-end against a running cmux instance.

## Prerequisites

```sh
# Build and install
make build && make install

# Create test directories
mkdir -p /tmp/cmres-test/{alpha,beta,dev,monitor,disabled,one,two,three,four}

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
cmres sync --config /dev/null --workspace-file testdata/workspaces/minimal.md --dry-run

# Expected: CREATE 2 workspaces (test-alpha pinned, test-beta unpinned)
```

```sh
# Real sync
cmres sync

# Verify
cmux list-workspaces
# Expected: test-alpha and test-beta appear with correct titles
```

**Check:**
- [ ] 2 workspaces created
- [ ] Titles show icons: `🧪 test-alpha`, `🔬 test-beta`
- [ ] test-alpha is pinned
- [ ] test-beta is NOT pinned
- [ ] CWDs are `/tmp/cmres-test/alpha` and `/tmp/cmres-test/beta`

**Cleanup:**
```sh
# Close test workspaces (note the refs from list-workspaces)
cmux close-workspace --workspace workspace:XX
cmux close-workspace --workspace workspace:YY
```

---

## Test 2: sync with splits

```sh
cmres sync --config /dev/null --workspace-file testdata/workspaces/splits.md --dry-run

# Expected: CREATE 2 workspaces (test-disabled is [ ] so skipped)
#   test-dev: 2 panes (main + split right)
#   test-monitor: 2 panes (main + split right)
```

```sh
cmres sync
cmux list-workspaces
```

**Check:**
- [ ] 2 workspaces created (NOT 3 — disabled is skipped)
- [ ] test-dev has 2 panes with split
- [ ] test-monitor has 2 panes with split
- [ ] Commands were sent (`echo "hello from split"`, etc.)

**Cleanup:** close test workspaces.

---

## Test 3: sync with full (multi-split + numbering)

```sh
cmres sync --config /dev/null --workspace-file testdata/workspaces/full.md --dry-run
```

**Check:**
- [ ] 3 workspaces created (project-four disabled)
- [ ] project-one: 3 panes (main + right + down)
- [ ] project-two: 2 panes (main + right)
- [ ] project-three: 1 pane
- [ ] Titles include numbers: `1 🟢 project-one`, etc.

---

## Test 4: save

```sh
# First create some workspaces via sync
cmres sync --config /dev/null --workspace-file testdata/workspaces/splits.md

# Save current layout
cmres save runbook-test

# Verify
cmres list
# Expected: runbook-test appears with workspace count and timestamp
```

**Check:**
- [ ] Layout saved without errors
- [ ] `cmres list` shows it
- [ ] File exists in layouts dir

---

## Test 5: show

```sh
cmres show runbook-test
# Expected: formatted display of workspaces, panes, CWDs

cmres show runbook-test --raw
# Expected: raw TOML content
```

**Check:**
- [ ] Shows workspace names, CWDs, pane counts
- [ ] `--raw` shows valid TOML

---

## Test 6: edit

```sh
EDITOR=cat cmres edit runbook-test
# Expected: prints TOML content (cat acts as editor)
```

**Check:**
- [ ] Opens the TOML file with $EDITOR

---

## Test 7: restore

```sh
# Close all test workspaces first
# Then restore
cmres restore runbook-test --dry-run
# Expected: lists cmux commands to recreate layout

cmres restore runbook-test
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
cmres export --workspace-file /tmp/cmres-test/exported.md

cat /tmp/cmres-test/exported.md
# Expected: valid workspace MD with current cmux state
```

**Check:**
- [ ] Markdown file created
- [ ] Projects section lists current workspaces
- [ ] Paths are correct

---

## Test 9: delete

```sh
cmres list
cmres delete runbook-test
cmres list
# Expected: runbook-test no longer appears
```

**Check:**
- [ ] Layout removed from list
- [ ] File deleted from disk

---

## Test 10: project management

```sh
WF=/tmp/cmres-test/project-test.md

# Start fresh
echo '## Projects
**Icon | Name | Template | Pin | Path**

## Templates

### single
- [x] main (focused)' > $WF

# Add projects
cmres project add "web" ~/projects/web -i "🌐" -t dev --workspace-file $WF
cmres project add "api" ~/projects/api -i "⚙️" -t go --workspace-file $WF
cmres project add "docs" ~/docs -i "📖" -t single --disabled --workspace-file $WF

# List
cmres project list --workspace-file $WF
# Expected: web, api (enabled)

cmres project list --all --workspace-file $WF
# Expected: web, api, docs (docs disabled)

# Toggle
cmres project toggle "docs" --workspace-file $WF
cmres project list --workspace-file $WF
# Expected: web, api, docs (all enabled)

# Remove
cmres project remove "api" --workspace-file $WF
cmres project list --all --workspace-file $WF
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
cmres watch runbook-watch --interval 10s &
WATCH_PID=$!

# Wait 15 seconds
sleep 15

# Check autosave
cmres list
# Expected: runbook-watch appears

kill $WATCH_PID
```

**Check:**
- [ ] Autosave file created
- [ ] Content-hash deduplication (no duplicate if layout unchanged)

---

## Test 12: version

```sh
cmres version
# Expected: version string with commit and build date
```

---

## Test 13: re-sync idempotency

```sh
# Sync once
cmres sync

# Sync again — should skip existing
cmres sync
# Expected: all SKIP (already exists)
```

**Check:**
- [ ] Second sync shows SKIP for all workspaces
- [ ] No duplicates created

---

## Cleanup

```sh
rm -rf /tmp/cmres-test
cmres delete runbook-test 2>/dev/null
cmres delete runbook-watch 2>/dev/null
```
