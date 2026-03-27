# cmux-resurrect demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo.sh (needs cmux running)

Output assets/demo.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 20ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup: alias crex + enable bash comments
Hide
Type "alias crex='crex --config /tmp/crex-demo/config.toml'"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 1: Help ──────────────────────────────────────────

Type "# Show available commands"
Enter
Sleep 500ms
Type "crex"
Enter
Sleep 4s

# ── Scene 2: Show the Workspace Blueprint ──────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# View the Workspace Blueprint"
Enter
Sleep 500ms
Type "bat --style=plain /tmp/crex-demo/workspaces.md"
Enter
Sleep 4s

# ── Scene 3: Import from Blueprint (real) ──────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Create workspaces from the Blueprint"
Enter
Sleep 500ms
Type "crex import-from-md --workspace-file /tmp/crex-demo/workspaces.md"
Enter
Sleep 10s

# ── Scene 4: Save current layout ─────────────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Save current workspaces as a layout"
Enter
Sleep 500ms
Type "crex save demo"
Enter
Sleep 3s

# Hidden: replace saved file with clean demo.toml (save captures ALL cmux
# workspaces including the user's real ones — we swap in the curated version).
Hide
Type "cp /tmp/crex-demo/demo-clean.toml /tmp/crex-demo/layouts/demo.toml"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 5: List saved layouts ───────────────────────────

Type "# List saved layouts"
Enter
Sleep 500ms
Type "crex list"
Enter
Sleep 4s

# ── Scene 6: Restore layout (real) ────────────────────────

# Hidden: close workspaces created by import so restore creates them fresh (no SKIPs).
Hide
Type "cmux list-workspaces 2>/dev/null | grep -E 'webapp|api' | grep -o 'workspace:[0-9]*' | while read ref; do cmux close-workspace --workspace $ref; sleep 0.3; done"
Enter
Sleep 2s
Type "clear"
Enter
Sleep 1s
Show

Type "# Restore a saved layout"
Enter
Sleep 500ms
Type "crex restore demo --mode add"
Enter
Sleep 10s
Type "# Workspaces are now live in cmux"
Enter
Sleep 4s

# ── Scene 7: Add workspace to Blueprint ───────────────────

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Add a workspace and list all"
Enter
Sleep 500ms
Type 'crex workspace add notes ~/docs -t single --icon "📓"'
Enter
Sleep 2s
Type "crex workspace list"
Enter
Sleep 4s

# ── End ───────────────────────────────────────────────────

Sleep 2s
