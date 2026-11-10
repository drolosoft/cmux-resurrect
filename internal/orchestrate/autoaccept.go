package orchestrate

import (
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/detect"
)

// autoAcceptCache is populated once from the detect registry.
var autoAcceptCache = detect.AutoAcceptFlags()

// flagOnFreshOnly lists the tools that reject their flag on a resumed session.
var flagOnFreshOnly = detect.FlagOnFreshOnlyTools()

// flagSubcommands names the subcommand each tool's flag has to follow, for
// the tools whose top level rejects the flag.
var flagSubcommands = detect.FlagSubcommands()

// resumeArguments are the arguments that make a command continue an earlier
// session instead of starting a new one.
var resumeArguments = map[string]bool{
	"--resume":   true,
	"-r":         true,
	"--continue": true,
	"-c":         true,
}

// resumesSession reports whether the arguments after the launch command ask
// the tool to continue an earlier session.
func resumesSession(arguments string) bool {
	for _, argument := range strings.Fields(arguments) {
		if resumeArguments[argument] {
			return true
		}
	}

	return false
}

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

	// A session keeps the approval mode it was created with, and some tools
	// exit with an error when a resume asks for another one.
	if flagOnFreshOnly[toolName] && resumesSession(rest) {
		return command, ""
	}

	// Don't inject if the flag is already present.
	if strings.Contains(command, flag) {
		return command, ""
	}

	// A tool whose top level rejects the flag takes it after its subcommand.
	if subcommand := flagSubcommands[toolName]; subcommand != "" {
		var placed bool
		launch, rest, placed = withSubcommand(launch, rest, subcommand)
		if !placed {
			return command, ""
		}
	}

	// Inject flag after the launch command.
	return envPrefix + launch + " " + flag + rest, toolName
}

// withSubcommand moves subcommand from the start of rest onto the end of
// launch, so the flag lands after it: `kiro-cli chat --resume` becomes
// `kiro-cli chat` and ` --resume`. A command that starts with options or with
// nothing gets the subcommand, because the tool runs it by default (bare
// kiro-cli opens a chat). A command that runs another subcommand has no place
// for the flag, and placed is false.
func withSubcommand(launch, rest, subcommand string) (newLaunch, newRest string, placed bool) {
	arguments := strings.Fields(rest)
	if len(arguments) > 0 && arguments[0] == subcommand {
		return launch + " " + subcommand, strings.TrimPrefix(strings.TrimLeft(rest, " "), subcommand), true
	}

	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return launch + " " + subcommand, rest, true
	}

	return launch, rest, false
}
