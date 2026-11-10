package orchestrate

import (
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/detect"
)

// autoAcceptCache is populated once from the detect registry.
var autoAcceptCache = detect.AutoAcceptFlags()

// InjectAutoAccept checks if a command starts with a known AI tool's launch
// command (after any leading VAR=value assignments) and if that tool is
// enabled in the autoAccept config list. If so, it injects the tool's
// auto-accept flag right after the launch command. The config lists tools by
// registry name, so `droid` is enabled by "factory".
//
// Returns the (possibly modified) command and the tool name if a flag
// was injected (empty string if not).
func InjectAutoAccept(command string, autoAccept []string) (string, string) {
	if command == "" || len(autoAccept) == 0 {
		return command, ""
	}

	// Leading VAR=value assignments (e.g. CLAUDE_CONFIG_DIR=…) are kept in
	// front; the launch command follows them.
	envPrefix, cmdline := detect.SplitEnvPrefix(command)

	// The launch command is one word for most tools and several for some
	// ("acli rovodev run"); the flag goes after all of them.
	toolName, launch, found := detect.ToolLaunch(cmdline)
	if !found {
		return command, ""
	}
	rest := strings.TrimPrefix(cmdline, launch)

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

	// Inject flag after the launch command.
	return envPrefix + launch + " " + flag + rest, toolName
}
