package orchestrate

import (
	"github.com/drolosoft/cmux-resurrect/internal/detect"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

// foregroundCommand reads the command running on a tty from the process
// table. A variable so save tests can fake it without real processes.
var foregroundCommand = detect.ForegroundCommand

// savedForegroundCommand is the command save records for a terminal on tty.
// The pane `crex save` runs in has crex itself in the foreground; storing it
// would make a restore type `crex save now` (overwriting the layout halfway)
// or `crex restore now` (restoring recursively), so that pane gets none.
func savedForegroundCommand(tty string) string {
	return withoutCrexCommand(foregroundCommand(tty))
}

// dropCrexCommands clears every pane and tab command that would run crex
// itself. Save calls it after merging with the layout on disk, because the
// merge keeps the saved command of each pane and an older save may have
// recorded `crex save now` there.
func dropCrexCommands(layout *model.Layout) {
	for i := range layout.Workspaces {
		for j := range layout.Workspaces[i].Panes {
			pane := &layout.Workspaces[i].Panes[j]
			pane.Command = withoutCrexCommand(pane.Command)
			for k := range pane.Surfaces {
				pane.Surfaces[k].Command = withoutCrexCommand(pane.Surfaces[k].Command)
			}
		}
	}
}

// withoutCrexCommand drops a command that would run crex itself. Save uses it
// on live panes; restore uses it on layouts saved before that, which still
// hold such commands and would otherwise have them typed back.
func withoutCrexCommand(command string) string {
	if detect.IsCrexCommand(command, detect.SelfExecutable()) {
		return ""
	}
	return command
}
