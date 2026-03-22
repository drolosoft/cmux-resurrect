#!/usr/bin/env bash
# 🎬 Record a demo GIF of cmux-resurrect in action
#
# Prerequisites:
#   brew install vhs    # charmbracelet/vhs — terminal GIF recorder
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

mkdir -p "$PROJECT_DIR/assets"

# Check for vhs
if ! command -v vhs &>/dev/null; then
    echo "❌ vhs not found. Install it:"
    echo "   brew install vhs"
    exit 1
fi

# Check for cmres
if ! command -v cmres &>/dev/null; then
    echo "❌ cmres not found. Build it first:"
    echo "   make build && make install"
    exit 1
fi

echo "🎬 Recording demo..."
vhs "$TAPE_FILE" -o "$OUTPUT"
echo "✅ Demo saved to $OUTPUT"
echo "📏 Size: $(du -h "$OUTPUT" | cut -f1)"
