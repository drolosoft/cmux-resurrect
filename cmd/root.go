package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/persist"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var (
	cfgFile       string
	layoutsDir    string
	workspaceFile string
	cfg           *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "crex",
	Short: "Save, restore, and template your terminal workspaces",
	Long:  "crex saves, restores, and templates your terminal workspaces.", // updated by updateRootLong()
	RunE: func(cmd *cobra.Command, args []string) error {
		if !configExists() {
			fmt.Print(banner())
			fmt.Println()
			fmt.Println(dimStyle.Render("  First time? Run ") + greenStyle.Render("crex setup") + dimStyle.Render(" to get started."))
			fmt.Println()
			return nil
		}
		fmt.Print(banner())
		fmt.Print(styledHelp())
		return nil
	},
}

func Execute() error {
	configureColorOutput()
	rootCmd.SetArgs(normalizeSingleDashVersion(os.Args[1:]))
	return rootCmd.Execute()
}

// normalizeSingleDashVersion rewrites the Go-style `-version` to `--version`.
// POSIX flag parsing would otherwise read it as a cluster of shorthands and
// fail on 'e'. Only the exact bare token is rewritten.
func normalizeSingleDashVersion(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if a == "-version" {
			a = "--version"
		}
		out[i] = a
	}
	return out
}

// configureColorOutput disables ANSI styling when output is not an interactive
// terminal (e.g. piped or redirected) or when NO_COLOR is set, so that
// `crex list | grep` and `crex show x > file` produce clean, escape-free text.
// Honors the NO_COLOR convention (https://no-color.org).
func configureColorOutput() {
	if os.Getenv("NO_COLOR") != "" || !term.IsTerminal(os.Stdout.Fd()) {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Enable the conventional `crex --version` / `-v` flags. `crex version`
	// keeps the full banner; the flag prints a grep-friendly one-liner.
	// Defining the flag before cobra's InitDefaultVersionFlag keeps the -v
	// shorthand.
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(fmt.Sprintf("crex {{.Version}} (%s) built %s\n", Commit, Date))
	rootCmd.Flags().BoolP("version", "v", false, "version for crex")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/crex/config.toml)")
	rootCmd.PersistentFlags().StringVar(&layoutsDir, "layouts-dir", "", "layouts directory (default ~/.config/crex/layouts)")
	rootCmd.PersistentFlags().StringVar(&workspaceFile, "workspace-file", "", "Workspace Blueprint path (default ~/.config/crex/workspaces.md)")

	// Shell completion hints for persistent flags.
	_ = rootCmd.MarkPersistentFlagFilename("config", "toml")
	_ = rootCmd.MarkPersistentFlagDirname("layouts-dir")
	_ = rootCmd.MarkPersistentFlagFilename("workspace-file", "md")

	updateRootLong()

	// Override the default help command to use our styled output for the root.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.Name() == rootCmd.Name() && cmd.Parent() == nil {
			fmt.Print(banner())
			fmt.Print(styledHelp())
		} else {
			// For subcommands, use cobra's default help.
			defaultHelp(cmd, args)
		}
	})
}

func updateRootLong() {
	if isCmuxBranding() {
		rootCmd.Long = "crex (cmux-resurrect) saves, restores, and templates your terminal workspaces.\nWorks with cmux and Ghostty. Inspired by tmux-resurrect."
	} else {
		rootCmd.Long = "crex saves, restores, and templates your terminal workspaces.\nWorks with Ghostty and cmux. Inspired by tmux-resurrect."
	}
}

func initConfig() {
	path := cfgFile
	if path == "" {
		path = config.DefaultConfigPath()
	}
	var err error
	cfg, err = config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: config load: %v\n", err)
		cfg = config.DefaultConfig()
	}
	if layoutsDir != "" {
		cfg.LayoutsDir = layoutsDir
	}
	if workspaceFile != "" {
		cfg.WorkspaceFile = config.ExpandHome(workspaceFile)
	}
}

func newClient() client.Backend {
	// Auto-detection runs a (potentially slow) osascript probe, so only reach
	// for it when CREX_BACKEND isn't already forcing a choice.
	return clientFor(nil)
}

// clientFor resolves the backend, honoring CREX_BACKEND first and only falling
// back to detection when no valid override is set. detect is the detection
// function to use; pass nil to use client.Detect (callers that already know the
// detected backend, like setup, pass a closure returning it to avoid re-probing).
//
// This is the single client-selection entry point for every command: CREX_BACKEND
// is parsed once here (needed for Alfred/external apps where the cmux socket is
// restricted and osascript detection may fail).
func clientFor(detect func() client.DetectedBackend) client.Backend {
	configBackend := ""
	if cfg != nil {
		configBackend = cfg.Backend
	}
	return resolveBackendChoice(os.Getenv("CREX_BACKEND"), configBackend, callerBackend(), detect, client.BackendAppRunning)
}

// callerBackend reports which terminal crex is running INSIDE, or
// BackendUnknown when it can't tell (Alfred, cron, a plain SSH shell).
//
// Being literally inside a session is a strong context signal, stronger than
// any configured default: whichever terminal the user typed the command in is
// the one they mean. CMUX_WORKSPACE_ID / CMUX_SURFACE_ID are set by cmux for
// its own shells but NOT by Alfred's socket discovery. TERM_PROGRAM only
// decides afterwards, because cmux is built on Ghostty and sets
// TERM_PROGRAM=ghostty for its own shells too — the CMUX_* vars disambiguate.
func callerBackend() client.DetectedBackend {
	if os.Getenv("CMUX_WORKSPACE_ID") != "" || os.Getenv("CMUX_SURFACE_ID") != "" {
		return client.BackendCmux
	}
	if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "ghostty") {
		return client.BackendGhostty
	}
	return client.BackendUnknown
}

// resolveBackendChoice picks the backend by precedence:
//  1. CREX_BACKEND env override (explicit, needed for external callers)
//  2. the terminal crex was invoked from — a command typed inside cmux acts on
//     cmux, one typed inside Ghostty acts on Ghostty, so the config default
//     never hijacks the session the user is actually looking at. This holds even
//     when that terminal is unreachable but still RUNNING: the command fails
//     naming the right backend instead of quietly rebuilding the user's
//     workspaces in a different app
//  3. the config's default backend (applies to external/ambiguous contexts
//     like Alfred, where cmux and Ghostty may both be running)
//  4. liveness-aware auto-detection
//
// Unrecognized override/config values warn and fall through. detect is the
// detection function; nil uses client.Detect. appRunning probes whether a
// backend's app is running; nil uses client.BackendAppRunning.
func resolveBackendChoice(envOverride, configBackend string, caller client.DetectedBackend, detect func() client.DetectedBackend, appRunning func(client.DetectedBackend) bool) client.Backend {
	if envOverride != "" {
		if cl, ok := client.NewForOverride(envOverride); ok {
			return cl
		}
		fmt.Fprintf(os.Stderr, "warning: unknown CREX_BACKEND=%q, ignoring\n", envOverride)
	}
	if detect == nil {
		detect = client.Detect
	}
	if appRunning == nil {
		appRunning = client.BackendAppRunning
	}
	// The caller's own terminal is authoritative. Reachable is the normal case;
	// unreachable-but-running still wins, so a cmux tab whose control socket is
	// closed reports "cmux not reachable" instead of restoring into Ghostty.
	// Only a genuinely absent app (leaked env from a closed cmux) falls through.
	if caller != client.BackendUnknown {
		if detect() == caller || appRunning(caller) {
			return client.NewForDetected(caller)
		}
	}
	if configBackend != "" {
		if cl, ok := client.NewForOverride(configBackend); ok {
			return cl
		}
		fmt.Fprintf(os.Stderr, "warning: unknown backend=%q in config, ignoring\n", configBackend)
	}
	return client.NewForDetected(detect())
}

func newStore() (persist.Store, error) {
	return persist.NewFileStore(cfg.LayoutsDir)
}

func configExists() bool {
	path := config.DefaultConfigPath()
	_, err := os.Stat(path)
	return err == nil
}
