package detect

// detectProcessAware returns a Detect function for tools where we recognize
// the process but don't know the session file format. The tool name is
// captured as the command.
func detectProcessAware(tool string) func(cwd, pid string) *Session {
	return func(cwd, _ string) *Session {
		return &Session{Tool: tool, CWD: cwd, Command: tool}
	}
}
