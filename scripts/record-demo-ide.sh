#!/usr/bin/env bash
# Record the IDE template demo GIF
#
# Shows the IDE template creating a full development workspace:
#   nvim (70% top, focused) + lazygit (bottom-left) + terminal (bottom-right)
#
# WORKFLOW:
#   1. make build && make install    # ensure crex binary is current
#   2. ./scripts/record-demo-ide.sh  # record the GIF
#
# OUTPUT: assets/demo-ide.gif
#
# NOTE: The VHS tape captures CLI output. To record the full cmux layout
# with nvim + lazygit visible, screen-record Ghostty while running:
#   crex template use ide

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TAPE_FILE="$SCRIPT_DIR/demo-ide.tape"
OUTPUT="$PROJECT_DIR/assets/demo-ide.gif"

mkdir -p "$PROJECT_DIR/assets"

# Check for vhs
if ! command -v vhs &>/dev/null; then
    echo "vhs not found. Install it:"
    echo "   brew install vhs"
    exit 1
fi

# Check for crex
if ! command -v crex &>/dev/null; then
    echo "crex not found. Build it first:"
    echo "   make build && make install"
    exit 1
fi

# Detect backend
BACKEND=""
if command -v cmux &>/dev/null && cmux ping &>/dev/null 2>&1; then
    BACKEND="cmux"
elif osascript -e 'tell application "System Events" to (name of processes) contains "Ghostty"' 2>/dev/null | grep -q true; then
    BACKEND="ghostty"
fi

if [ -z "$BACKEND" ]; then
    echo "No supported backend detected."
    echo "   Start cmux or Ghostty before recording."
    exit 1
fi

echo "Detected backend: $BACKEND"

# Snapshot existing workspace refs for cleanup.
echo "Snapshotting current workspaces..."
if [ "$BACKEND" = "cmux" ]; then
    cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > /tmp/crex-ide-before.txt || true
fi

echo "Recording IDE demo ($BACKEND backend)..."
vhs "$TAPE_FILE" -o "$OUTPUT"

# Cleanup: close workspaces created during recording.
echo "Cleaning up created workspaces..."
if [ "$BACKEND" = "cmux" ]; then
    cmux list-workspaces 2>/dev/null | grep -o 'workspace:[0-9]*' | sort > /tmp/crex-ide-after.txt || true
    NEW_REFS=$(comm -13 /tmp/crex-ide-before.txt /tmp/crex-ide-after.txt)
    for ref in $NEW_REFS; do
        echo "  Closing $ref"
        cmux close-workspace --workspace "$ref" 2>/dev/null || true
        sleep 0.2
    done
    rm -f /tmp/crex-ide-before.txt /tmp/crex-ide-after.txt
fi

echo "Demo saved to $OUTPUT"
echo "Size: $(du -h "$OUTPUT" | cut -f1)"
echo ""
echo "TIP: For a full-layout recording (nvim + lazygit visible),"
echo "     screen-record Ghostty while running: crex template use ide"
