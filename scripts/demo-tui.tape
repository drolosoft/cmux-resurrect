# crex v1.5.0 interactive shell demo — recorded with github.com/charmbracelet/vhs
#
# Run via: ./scripts/record-demo-tui.sh (needs Ghostty or cmux running)

Output assets/demo-tui.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup: alias crex to use demo config
Hide
Type "alias crex='/tmp/crex-demo-tui-bin --config /tmp/crex-demo-tui-env/config.toml'"
Enter
Sleep 500ms
Type "clear"
Enter
Sleep 1s
Show

# ── Scene 1: Launch the interactive shell ─────────────────

Type "# Interactive shell"
Enter
Sleep 500ms
Type "crex tui"
Enter
Sleep 2s

# ── Scene 2: Help ─────────────────────────────────────────

Type "help"
Enter
Sleep 3s

# ── Scene 3: List layouts with browse ─────────────────────

Type "ls"
Enter
Sleep 2s
# Browse with arrows
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 1s
# Quit browse
Type "q"
Sleep 1s

# ── Scene 4: Restore by number ────────────────────────────

Type "restore 2"
Enter
Sleep 4s

# ── Scene 5: Show live state ──────────────────────────────

Type "now"
Enter
Sleep 3s

# ── Scene 6: Browse template gallery ─────────────────────

Type "templates"
Enter
Sleep 2s
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 1s
Type "q"
Sleep 1s

# ── Scene 7: Blueprint management ────────────────────────

Type "bp list"
Enter
Sleep 2s
Type "q"
Sleep 1s

# ── Scene 8: Watch status ────────────────────────────────

Type "watch status"
Enter
Sleep 2s

# ── Scene 9: Exit ────────────────────────────────────────

Type "exit"
Enter
Sleep 2s
