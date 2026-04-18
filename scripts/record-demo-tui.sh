#!/usr/bin/env bash
# 🎬 Record a TUI interactive shell demo GIF
#
# Records the demo GIF for the v1.5.0 interactive shell feature.
# Uses an isolated demo environment — nothing from real config leaks in.
#
# WORKFLOW:
#   1. make build && make install    # ensure crex binary is current
#   2. ./scripts/record-demo-tui.sh  # record the GIF
#
# FILES:
#   scripts/demo-tui.tape           — VHS recording script
#   scripts/record-demo-tui.sh      — this file (setup, record, cleanup)
#   assets/demo-tui.gif             — output GIF
#
# DEMO NARRATIVE:
#   1. Launch       — crex tui, welcome message + prompt
#   2. Help         — show all 18 commands grouped by category
#   3. List         — browse saved layouts with arrows, quit
#   4. Restore #    — restore by number reference from listing
#   5. Now          — show live terminal state
#   6. Templates    — browse the 16-template gallery
#   7. Blueprint    — bp list shows entries
#   8. Watch        — watch status shows daemon state
#   9. Exit         — clean exit
#
# Prerequisites:
#   brew install vhs    # charmbracelet/vhs — terminal GIF recorder
#   Ghostty or cmux must be running (for now/restore scenes)
#
# Usage:
#   ./scripts/record-demo-tui.sh
#
# Output:
#   assets/demo-tui.gif

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TAPE_FILE="$SCRIPT_DIR/demo-tui.tape"
OUTPUT="$PROJECT_DIR/assets/demo-tui.gif"
DEMO_DIR="/tmp/crex-demo-tui-env"
DEMO_BIN="/tmp/crex-demo-tui-bin"

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

# Set up isolated demo environment
echo "📦 Setting up demo environment..."
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR/layouts"

# Copy existing layouts so `ls` has content to show
if [[ -d "$HOME/.config/crex/layouts" ]]; then
    cp "$HOME/.config/crex/layouts"/*.toml "$DEMO_DIR/layouts/" 2>/dev/null || true
fi

# Ensure we have at least one layout for the demo
if [[ ! -f "$DEMO_DIR/layouts/my-day.toml" ]]; then
    cp "$PROJECT_DIR/testdata/layouts/my-day.toml" "$DEMO_DIR/layouts/" 2>/dev/null || true
fi

# Create Blueprint so `bp list` has content
cat > "$DEMO_DIR/workspaces.md" << 'MDEOF'
## Projects
**Icon | Name | Template | Pin | Path**

- [x] | 🌐 | webapp     | dev    | yes | ~/projects/webapp
- [x] | ⚙️ | api        | go     | yes | ~/projects/api
- [x] | 🧪 | tests      | single | no  | ~/projects/testing

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

# Config pointing to demo files
cat > "$DEMO_DIR/config.toml" << TOMLEOF
workspace_file = "$DEMO_DIR/workspaces.md"
layouts_dir = "$DEMO_DIR/layouts"
TOMLEOF

# Build fresh binary with demo config baked in via alias
echo "🔨 Building fresh binary..."
go build -o "$DEMO_BIN" "$PROJECT_DIR/cmd/crex"

# Update tape to use demo config
export CREX_DEMO_CONFIG="$DEMO_DIR/config.toml"

echo "🎬 Recording TUI demo..."
vhs "$TAPE_FILE" -o "$OUTPUT"

# Cleanup
rm -rf "$DEMO_DIR"
rm -f "$DEMO_BIN"

echo "✅ Demo saved to $OUTPUT"
echo "📏 Size: $(du -h "$OUTPUT" | cut -f1)"
