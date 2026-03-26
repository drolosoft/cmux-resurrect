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
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 35ms
Set CursorBlink false

# --- Scene 1: Banner & Help ---

Type "crex"
Enter
Sleep 3s

# --- Scene 2: List saved layouts ---

Type "crex list"
Enter
Sleep 1.5s

# --- Scene 3: Dry-run restore (works without running cmux) ---

Type "# Preview restore -- see every command before executing"
Enter
Sleep 400ms

Type "crex restore demo --dry-run"
Enter
Sleep 3s

# --- Scene 4: Import from Workspace Blueprint (dry-run) ---

Type "# Import workspaces from a Workspace Blueprint"
Enter
Sleep 400ms

Type "crex import-from-md --dry-run"
Enter
Sleep 2s

# --- Scene 5: Workspace Blueprint management ---

Type "# Manage workspace entries in your Blueprint"
Enter
Sleep 400ms

Type 'crex workspace add api ~/projects/api -t dev --icon "⚙️"'
Enter
Sleep 1.5s

Type "crex workspace list"
Enter
Sleep 2s

# --- Scene 6: Show a layout ---

Type "crex show demo"
Enter
Sleep 2.5s

# --- Scene 7: Version ---

Type "crex version"
Enter
Sleep 2s

# --- End ---

Sleep 1s
