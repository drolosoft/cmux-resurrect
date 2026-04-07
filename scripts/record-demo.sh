#!/usr/bin/env bash
# 🎬 Record a demo GIF of cmux-resurrect in action
#
# This script records the demo GIF shown in the project README.
# It creates an isolated environment so nothing from the user's real config
# leaks into the recording. All workspaces created during recording are
# automatically cleaned up afterward.
#
# WORKFLOW (always run all three steps together):
#   1. make build && make install    # ensure crex binary is current
#   2. ./scripts/record-demo.sh      # record the GIF
#   3. ./scripts/validate-demo.sh    # verify narrative consistency
#
# FILES:
#   scripts/demo.tape           — VHS recording script (scenes, timing, commands)
#   scripts/record-demo.sh      — this file (setup, record, cleanup)
#   scripts/validate-demo.sh    — post-recording validation (56 assertions)
#   testdata/layouts/my-day.toml — pre-made layout used in list/restore scenes
#   assets/demo.gif             — output GIF
#
# DEMO NARRATIVE (scenes must be consistent):
#   1. Help          — show commands + quick start examples
#   2. Blueprint     — bat workspaces.md (webapp + api)
#   3. Import        — import-from-md creates webapp + api
#   4. Save          — crex save my-day (visible, then hidden swap to clean my-day.toml)
#   5. List          — my-day layout has 3 workspaces (webapp + api + docs)
#   6. Restore       — hidden close of import workspaces, then OK all 3 (fresh)
#   7. Workspace     — add notes, list shows webapp + api + notes
#
# IMPORTANT: Between list and restore, a hidden step closes workspaces
# created during import so restore shows all 3 being created fresh.
#
# Prerequisites:
#   brew install vhs    # charmbracelet/vhs — terminal GIF recorder
#   cmux must be running
#
# Usage:
#   ./scripts/record-demo.sh
#
# Output:
#   assets/demo.gif

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TAPE_FILE="$SCRIPT_DIR/demo.tape"
OUTPUT="$PROJECT_DIR/assets/demo.gif"
DEMO_DIR="/tmp/crex-demo"

mkdir -p "$PROJECT_DIR/assets"

# Check for vhs
if ! command -v vhs &>/dev/null; then
    echo "❌ vhs not found. Install it:"
    echo "   brew install vhs"
    exit 1
fi

# Check for crex
if ! command -v crex &>/dev/null; then
    echo "❌ crex not found. Build it first:"
    echo "   make build && make install"
    exit 1
fi

# Check cmux is running (needed for real commands)
if ! cmux ping &>/dev/null; then
    echo "❌ cmux not running. Start cmux first."
    exit 1
fi

# Snapshot existing workspace refs for cleanup.
echo "📸 Snapshotting current workspaces..."
cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > /tmp/crex-demo-before.txt || true

# Set up isolated demo environment.
echo "📦 Setting up demo environment..."
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR/layouts"

# Copy the demo layout.
cp "$PROJECT_DIR/testdata/layouts/my-day.toml" "$DEMO_DIR/layouts/" 2>/dev/null || \
    cp "$HOME/.config/crex/layouts/my-day.toml" "$DEMO_DIR/layouts/" 2>/dev/null || \
    { echo "❌ my-day.toml not found"; exit 1; }

# Create a simple Workspace Blueprint for the demo.
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

# Keep a clean copy of my-day.toml — the VHS script runs 'crex save my-day' which
# overwrites it with ALL real workspaces. After save runs visibly, a hidden step
# swaps the clean version back in so list/restore show only demo workspaces.
cp "$DEMO_DIR/layouts/my-day.toml" "$DEMO_DIR/my-day-clean.toml"

# Create config pointing to demo files.
cat > "$DEMO_DIR/config.toml" << TOMLEOF
workspace_file = "$DEMO_DIR/workspaces.md"
layouts_dir = "$DEMO_DIR/layouts"
TOMLEOF

echo "🎬 Recording demo (real cmux commands)..."
vhs "$TAPE_FILE" -o "$OUTPUT"

# Cleanup: close workspaces created during recording.
echo "🧹 Cleaning up created workspaces..."
cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > /tmp/crex-demo-after.txt || true

# Find refs that are in "after" but not in "before" — those were created during recording.
NEW_REFS=$(comm -13 /tmp/crex-demo-before.txt /tmp/crex-demo-after.txt)
for ref in $NEW_REFS; do
    echo "  Closing $ref"
    cmux close-workspace --workspace "$ref" 2>/dev/null || true
    sleep 0.2
done

rm -f /tmp/crex-demo-before.txt /tmp/crex-demo-after.txt
rm -rf "$DEMO_DIR"

echo "✅ Demo saved to $OUTPUT"
echo "📏 Size: $(du -h "$OUTPUT" | cut -f1)"
