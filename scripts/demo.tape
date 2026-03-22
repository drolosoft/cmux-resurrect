# 🎬 cmux-resurrect demo — recorded with github.com/charmbracelet/vhs
#
# This tape file produces the demo.gif shown in the README.
# Edit timings and commands below, then run: vhs scripts/demo.tape

# Configuration
Output assets/demo.gif
Set FontSize 14
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 50ms

# --- Scene 1: Save ---

Type "# 💾 Save your current cmux layout"
Enter
Sleep 500ms

Type "cmres save work"
Enter
Sleep 1.5s

# --- Scene 2: List ---

Type "# 📋 See what you've saved"
Enter
Sleep 500ms

Type "cmres list"
Enter
Sleep 1.5s

# --- Scene 3: Show ---

Type "# 🔍 Inspect a layout"
Enter
Sleep 500ms

Type "cmres show work"
Enter
Sleep 2s

# --- Scene 4: Dry-run ---

Type "# 👁️ Preview restore (nothing is executed)"
Enter
Sleep 500ms

Type "cmres restore work --dry-run"
Enter
Sleep 3s

# --- Scene 5: Restore ---

Type "# 🔄 Restore! All workspaces come back"
Enter
Sleep 500ms

Type "cmres restore work"
Enter
Sleep 2s

# --- Scene 6: Project management ---

Type "# ➕ Add a project to your workspace file"
Enter
Sleep 500ms

Type 'cmres project add api ~/projects/api -t dev --icon "⚙️"'
Enter
Sleep 1.5s

Type "cmres project list"
Enter
Sleep 2s

# --- End ---

Sleep 1s
