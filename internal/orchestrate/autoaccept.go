package orchestrate

import (
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/detect"
)

// autoAcceptCache is populated once from the detect registry.
var autoAcceptCache = detect.AutoAcceptFlags()

// InjectAutoAccept checks if a command starts with a known AI tool name
// (after any leading VAR=value assignments)
// and if that tool is enabled in the autoAccept config list. If so, it
// injects the tool's auto-accept flag after the tool name.
//
// Returns the (possibly modified) command and the tool name if a flag
// was injected (empty string if not).
func InjectAutoAccept(command string, autoAccept []string) (string, string) {
	if command == "" || len(autoAccept) == 0 {
		return command, ""
	}

	// Leading VAR=value assignments (e.g. CLAUDE_CONFIG_DIR=…) are kept in
	// front; the tool name is the first word after them.
	envPrefix, cmdline := detect.SplitEnvPrefix(command)

	// Extract the first word (tool name).
	toolName := cmdline
	rest := ""
	if idx := strings.IndexByte(cmdline, ' '); idx >= 0 {
		toolName = cmdline[:idx]
		rest = cmdline[idx:]
	}

	flag, known := autoAcceptCache[toolName]
	if !known || flag == "" {
		return command, ""
	}

	// Check if tool is in the allowed list.
	allowed := false
	for _, a := range autoAccept {
		if a == "all" || a == toolName {
			allowed = true
			break
		}
	}
	if !allowed {
		return command, ""
	}

	// Don't inject if the flag is already present.
	if strings.Contains(command, flag) {
		return command, ""
	}

	// Inject flag after the tool name.
	return envPrefix + toolName + " " + flag + rest, toolName
}
