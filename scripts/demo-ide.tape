# crex IDE template demo — recorded with github.com/charmbracelet/vhs
#
# Shows the IDE template creating a full development workspace:
#   nvim (70% top) + lazygit (bottom-left) + terminal (bottom-right)
#
# Run via: ./scripts/record-demo-ide.sh (needs cmux running)
#
# AFTER RECORDING: The user pastes a screenshot of the resulting cmux
# layout (nvim + lazygit + terminal) into the GIF. VHS only captures
# the CLI output — the actual split layout is in cmux.

Output assets/demo-ide.gif
Set FontSize 14
Set Width 1000
Set Height 600
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Set Shell "bash"

# Hidden setup
Hide
Type "shopt -s interactive_comments"
Enter
Sleep 200ms
Type "export CREX_BRAND=crex"
Enter
Sleep 200ms
Type "clear"
Enter
Sleep 1s
Show

# -- Scene 1: Show available templates --

Type "# Browse the template gallery"
Enter
Sleep 500ms
Type "crex template list"
Enter
Sleep 3s

# -- Scene 2: Use the IDE template --

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "# Create a full IDE workspace"
Enter
Sleep 500ms
Type "crex template use ide"
Enter
Sleep 5s

# -- Scene 3: Pause for screenshot paste --
# The user screen-captures cmux here showing the result:
#   nvim (70% top) + lazygit (bottom-left) + terminal (bottom-right)
# This screenshot is composited into the final GIF.

Sleep 3s

# -- End --

Sleep 2s
