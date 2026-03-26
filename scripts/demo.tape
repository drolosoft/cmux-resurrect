# cmux-resurrect demo -- recorded with github.com/charmbracelet/vhs
#
# This tape file produces the demo.gif shown in the README.
# Edit timings and commands below, then run: vhs scripts/demo.tape
#
# Prerequisites:
#   brew install vhs       # charmbracelet/vhs -- terminal GIF recorder
#   make build && make install

# Configuration
Output assets/demo.gif
Set FontSize 14
Set Width 920
Set Height 540
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false

# --- Scene 1: Banner & Help ---

Type "crex"
Enter
Sleep 3s

# --- Scene 2: Save ---

Type "# Save your current cmux layout"
Enter
Sleep 400ms

Type "crex save work"
Enter
Sleep 1.5s

# --- Scene 3: List ---

Type "# See what you've saved"
Enter
Sleep 400ms

Type "crex list"
Enter
Sleep 1.5s

# --- Scene 4: Show ---

Type "# Inspect a layout"
Enter
Sleep 400ms

Type "crex show work"
Enter
Sleep 2s

# --- Scene 5: Dry-run ---

Type "# Preview restore (nothing is executed)"
Enter
Sleep 400ms

Type "crex restore work --dry-run"
Enter
Sleep 3s

# --- Scene 6: Restore ---

Type "# Restore! All workspaces come back"
Enter
Sleep 400ms

Type "crex restore work"
Enter
Sleep 2s

# --- Scene 7: Workspace management ---

Type "# Add a workspace entry to your Workspace Blueprint"
Enter
Sleep 400ms

Type 'crex workspace add api ~/projects/api -t dev --icon "gear"'
Enter
Sleep 1.5s

Type "crex workspace list"
Enter
Sleep 2s

# --- Scene 8: Version ---

Type "crex version"
Enter
Sleep 2s

# --- End ---

Sleep 1s
