package detect

// processAware completes the registry entry of a tool crex recognises by its
// process but cannot resume: its Detect relaunches the tool fresh with the
// entry's launch command, so the command lives in one place.
func processAware(entry detector) detector {
	entry.Detect = detectProcessAware(entry.Name, entry.launchCommand())
	return entry
}

// detectProcessAware returns a Detect function for tools where we recognize
// the process but don't know the session file format. The session carries
// the tool name and the command that starts the tool again, which is not
// always the name (Factory's binary is droid).
func detectProcessAware(tool, command string) func(cwd, pid string) *Session {
	return func(cwd, _ string) *Session {
		return &Session{Tool: tool, CWD: cwd, Command: command}
	}
}
