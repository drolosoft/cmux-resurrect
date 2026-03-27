#!/usr/bin/env bash
# Validate demo GIF content — ensures narrative consistency across all scenes.
#
# This script runs the same commands as demo.tape against an isolated demo
# environment and validates that each scene produces the expected output.
# It can also extract and validate GIF frames if ffmpeg is available.
#
# Prerequisites:
#   cmux must be running
#   crex must be built and installed
#   ffmpeg (optional, for frame extraction)
#
# Usage:
#   ./scripts/validate-demo.sh              # validate commands only
#   ./scripts/validate-demo.sh --with-gif   # also validate GIF frames
#
# Exit codes:
#   0 — all validations passed
#   1 — one or more validations failed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
DEMO_DIR="/tmp/crex-demo-validate"
GIF_FILE="$PROJECT_DIR/assets/demo.gif"
SCREENSHOTS_DIR="/tmp/crex-demo-validate-frames"
WITH_GIF=false

if [[ "${1:-}" == "--with-gif" ]]; then
    WITH_GIF=true
fi

PASS=0
FAIL=0
CLEANUP_REFS=()

pass() {
    PASS=$((PASS + 1))
    echo "  ✅ $1"
}

fail() {
    FAIL=$((FAIL + 1))
    echo "  ❌ $1"
}

assert_contains() {
    local output="$1"
    local expected="$2"
    local label="$3"
    if echo "$output" | grep -q "$expected"; then
        pass "$label"
    else
        fail "$label — expected '$expected'"
        echo "     Got: $(echo "$output" | head -5)"
    fi
}

assert_not_contains() {
    local output="$1"
    local unexpected="$2"
    local label="$3"
    if echo "$output" | grep -q "$unexpected"; then
        fail "$label — found unexpected '$unexpected'"
    else
        pass "$label"
    fi
}

cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    for ref in "${CLEANUP_REFS[@]}"; do
        cmux close-workspace --workspace "$ref" 2>/dev/null || true
        sleep 0.2
    done
    rm -rf "$DEMO_DIR" "$SCREENSHOTS_DIR"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Results: $PASS passed, $FAIL failed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if [[ $FAIL -gt 0 ]]; then
        exit 1
    fi
}
trap cleanup EXIT

# ── Preflight checks ────────────────────────────────────────

echo "🔍 Preflight checks..."

if ! command -v crex &>/dev/null; then
    fail "crex not found — run: make build && make install"
    exit 1
fi
pass "crex binary found"

if ! cmux ping &>/dev/null; then
    fail "cmux not running"
    exit 1
fi
pass "cmux is reachable"

# ── Setup isolated demo environment ─────────────────────────

echo ""
echo "📦 Setting up demo environment..."

rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR/layouts"

# Copy demo layout.
cp "$PROJECT_DIR/testdata/layouts/demo.toml" "$DEMO_DIR/layouts/" 2>/dev/null || \
    cp "$HOME/.config/crex/layouts/demo.toml" "$DEMO_DIR/layouts/" 2>/dev/null || \
    { fail "demo.toml not found"; exit 1; }

# Create blueprint.
cat > "$DEMO_DIR/workspaces.md" << 'MDEOF'
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp     | dev    | yes | ~/projects/webapp
- [x] | ⚙️ | api        | go     | yes | ~/projects/api

## Templates

### dev
- [x] main terminal (focused)
- [x] split right: `npm run dev`

### go
- [x] main terminal (focused)
- [x] split right: `go test ./...`

### single
- [x] main terminal (focused)
MDEOF

# Config pointing to demo files.
cat > "$DEMO_DIR/config.toml" << TOMLEOF
workspace_file = "$DEMO_DIR/workspaces.md"
layouts_dir = "$DEMO_DIR/layouts"
TOMLEOF

CREX="crex --config $DEMO_DIR/config.toml"

# Snapshot existing workspaces for cleanup.
cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > "$DEMO_DIR/before.txt" || true

echo ""

# ── Scene 1: Help ───────────────────────────────────────────

echo "📋 Scene 1: Help"
HELP_OUTPUT=$($CREX 2>&1 || true)

assert_contains "$HELP_OUTPUT" "resurrected" "banner visible"
assert_contains "$HELP_OUTPUT" "save" "save command listed"
assert_contains "$HELP_OUTPUT" "restore" "restore command listed"
assert_contains "$HELP_OUTPUT" "import-from-md" "import-from-md command listed"
assert_contains "$HELP_OUTPUT" "workspace" "workspace command listed"
assert_contains "$HELP_OUTPUT" "Quick start" "quick start section present"
assert_contains "$HELP_OUTPUT" "crex import-from-md" "quick start: import-from-md example"
assert_contains "$HELP_OUTPUT" "crex save work" "quick start: save example"
assert_contains "$HELP_OUTPUT" "crex list" "quick start: list example"
assert_contains "$HELP_OUTPUT" "crex restore work --mode add" "quick start: restore example"
assert_contains "$HELP_OUTPUT" "crex workspace add" "quick start: workspace add example"

HELP_LINES=$(echo "$HELP_OUTPUT" | wc -l)
if [[ $HELP_LINES -le 30 ]]; then
    pass "help fits on screen ($HELP_LINES lines <= 30)"
else
    fail "help too long ($HELP_LINES lines > 30)"
fi

echo ""

# ── Scene 2: Blueprint file ────────────────────────────────

echo "📋 Scene 2: Workspace Blueprint"
BP_OUTPUT=$(cat "$DEMO_DIR/workspaces.md")

assert_contains "$BP_OUTPUT" "webapp" "blueprint contains webapp"
assert_contains "$BP_OUTPUT" "api" "blueprint contains api"
assert_contains "$BP_OUTPUT" "dev" "blueprint contains dev template"
assert_contains "$BP_OUTPUT" "go" "blueprint contains go template"
assert_contains "$BP_OUTPUT" "npm run dev" "dev template has npm command"
assert_contains "$BP_OUTPUT" "go test" "go template has go test command"

echo ""

# ── Scene 3: Import from Blueprint ──────────────────────────

echo "📋 Scene 3: Import from Blueprint"
IMPORT_OUTPUT=$($CREX import-from-md --workspace-file "$DEMO_DIR/workspaces.md" 2>&1)

assert_contains "$IMPORT_OUTPUT" "Importing from" "importing header shown"
assert_contains "$IMPORT_OUTPUT" "webapp" "webapp created"
assert_contains "$IMPORT_OUTPUT" "api" "api created"
assert_contains "$IMPORT_OUTPUT" "2 panes" "pane count shown"
assert_contains "$IMPORT_OUTPUT" "2 created" "summary: 2 created"
assert_contains "$IMPORT_OUTPUT" "0 skipped" "summary: 0 skipped"

# Track new workspaces for cleanup.
cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > "$DEMO_DIR/after_import.txt" || true
while IFS= read -r ref; do
    CLEANUP_REFS+=("$ref")
done < <(comm -13 "$DEMO_DIR/before.txt" "$DEMO_DIR/after_import.txt")

echo ""

# ── Scene 4: Save layout ────────────────────────────────────

echo "📋 Scene 4: Save layout"
SAVE_OUTPUT=$($CREX save demo 2>&1)

assert_contains "$SAVE_OUTPUT" "Saving layout" "saving header shown"
assert_contains "$SAVE_OUTPUT" "demo" "layout name in output"
assert_contains "$SAVE_OUTPUT" "Saved" "save confirmation"

# Restore the clean demo.toml (save captures all real workspaces).
cp "$PROJECT_DIR/testdata/layouts/demo.toml" "$DEMO_DIR/layouts/"

echo ""

# ── Scene 5: List saved layouts ─────────────────────────────

echo "📋 Scene 5: List saved layouts"
LIST_OUTPUT=$($CREX list 2>&1)

assert_contains "$LIST_OUTPUT" "Saved Layouts" "heading present"
assert_contains "$LIST_OUTPUT" "demo" "demo layout listed"
assert_contains "$LIST_OUTPUT" "3 workspaces" "workspace count matches demo.toml"

echo ""

# ── Scene 6: Restore layout ────────────────────────────────

# Close workspaces created by import so restore creates them fresh (mirrors demo.tape).
echo "📋 Scene 6: Close import workspaces + Restore layout"
for ref in "${CLEANUP_REFS[@]}"; do
    cmux close-workspace --workspace "$ref" 2>/dev/null || true
    sleep 0.3
done
CLEANUP_REFS=()
sleep 1

RESTORE_OUTPUT=$($CREX restore demo --mode add 2>&1)

assert_contains "$RESTORE_OUTPUT" "Adding from" "adding header shown"
assert_contains "$RESTORE_OUTPUT" "demo" "layout name shown"
assert_contains "$RESTORE_OUTPUT" "webapp" "webapp created"
assert_contains "$RESTORE_OUTPUT" "api" "api created"
assert_contains "$RESTORE_OUTPUT" "docs" "docs created"
assert_not_contains "$RESTORE_OUTPUT" "SKIP" "no SKIPs — all created fresh"
assert_contains "$RESTORE_OUTPUT" "Restored" "restore summary shown"

# Track new workspaces for cleanup.
cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > "$DEMO_DIR/after_restore.txt" || true
while IFS= read -r ref; do
    CLEANUP_REFS+=("$ref")
done < <(comm -13 "$DEMO_DIR/before.txt" "$DEMO_DIR/after_restore.txt")

echo ""

# ── Scene 7: Workspace add + list ──────────────────────────

echo "📋 Scene 7: Workspace add + list"
ADD_OUTPUT=$($CREX workspace add notes ~/docs -t single --icon "📓" 2>&1)

assert_contains "$ADD_OUTPUT" "notes" "workspace name shown"
assert_contains "$ADD_OUTPUT" "single" "template shown"
assert_contains "$ADD_OUTPUT" "Added" "success message"

WS_LIST_OUTPUT=$($CREX workspace list 2>&1)

assert_contains "$WS_LIST_OUTPUT" "Workspace Blueprint" "heading present"
assert_contains "$WS_LIST_OUTPUT" "webapp" "webapp in blueprint"
assert_contains "$WS_LIST_OUTPUT" "api" "api in blueprint"
assert_contains "$WS_LIST_OUTPUT" "notes" "notes added to blueprint"
assert_contains "$WS_LIST_OUTPUT" "3 entries" "3 total entries"

echo ""

# ── Narrative consistency checks ────────────────────────────

echo "📋 Narrative consistency"

# The demo.toml must contain the same workspace names as the blueprint.
DEMO_TOML=$(cat "$DEMO_DIR/layouts/demo.toml")
assert_contains "$DEMO_TOML" "webapp" "demo.toml contains webapp (matches blueprint)"
assert_contains "$DEMO_TOML" "api" "demo.toml contains api (matches blueprint)"
assert_contains "$DEMO_TOML" "docs" "demo.toml contains docs (extra workspace for restore)"

# demo.toml workspace count must match what list reports.
TOML_WS_COUNT=$(grep -c '^\[\[workspace\]\]' "$DEMO_DIR/layouts/demo.toml")
if [[ $TOML_WS_COUNT -eq 3 ]]; then
    pass "demo.toml has 3 workspaces (matches list output)"
else
    fail "demo.toml has $TOML_WS_COUNT workspaces, expected 3"
fi

# Quick start examples must match the demo flow.
assert_contains "$HELP_OUTPUT" "import-from-md" "help quick start mentions import (scene 3)"
assert_contains "$HELP_OUTPUT" "crex save" "help quick start mentions save (scene 4)"
assert_contains "$HELP_OUTPUT" "crex list" "help quick start mentions list (scene 5)"
assert_contains "$HELP_OUTPUT" "restore.*--mode add" "help quick start mentions restore --mode add (scene 6)"
assert_contains "$HELP_OUTPUT" "workspace add" "help quick start mentions workspace add (scene 7)"

echo ""

# ── GIF frame validation (optional) ─────────────────────────

if [[ "$WITH_GIF" == true ]]; then
    echo "📋 GIF frame validation"

    if [[ ! -f "$GIF_FILE" ]]; then
        fail "GIF file not found at $GIF_FILE"
    else
        pass "GIF file exists"

        GIF_SIZE=$(stat -f%z "$GIF_FILE" 2>/dev/null || stat -c%s "$GIF_FILE" 2>/dev/null)
        if [[ $GIF_SIZE -gt 100000 && $GIF_SIZE -lt 2000000 ]]; then
            pass "GIF size reasonable ($(( GIF_SIZE / 1024 ))KB)"
        else
            fail "GIF size unexpected: $(( GIF_SIZE / 1024 ))KB"
        fi

        if command -v ffmpeg &>/dev/null; then
            mkdir -p "$SCREENSHOTS_DIR"
            ffmpeg -i "$GIF_FILE" -vf "fps=1" "$SCREENSHOTS_DIR/frame_%03d.png" 2>/dev/null

            FRAME_COUNT=$(ls "$SCREENSHOTS_DIR"/frame_*.png 2>/dev/null | wc -l | tr -d ' ')
            if [[ $FRAME_COUNT -ge 45 && $FRAME_COUNT -le 70 ]]; then
                pass "GIF duration reasonable ($FRAME_COUNT seconds)"
            else
                fail "GIF duration unexpected: $FRAME_COUNT seconds (expected 45-70)"
            fi
        else
            echo "  ⏭️  ffmpeg not found, skipping frame extraction"
        fi
    fi

    echo ""
fi
