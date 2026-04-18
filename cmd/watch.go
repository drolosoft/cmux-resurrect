package cmd

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var (
	watchInterval  string
	watchDaemon    bool
	watchStop      bool
	watchStatus    bool
	watchShellHook bool
)

var watchCmd = &cobra.Command{
	Use:   "watch [name]",
	Short: "Auto-save layout periodically",
	Long:  "Watches terminal state and saves it periodically. Deduplicates via content hash.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInterval, "interval", "i", "", "save interval (default from config, e.g. 5m)")
	watchCmd.Flags().BoolVar(&watchDaemon, "daemon", false, "register as daemon (write PID file, log to file)")
	watchCmd.Flags().BoolVar(&watchStop, "stop", false, "kill running daemon")
	watchCmd.Flags().BoolVar(&watchStatus, "status", false, "check if daemon is running")
	watchCmd.Flags().BoolVar(&watchShellHook, "shell-hook", false, "print shell hook snippet")
	watchCmd.ValidArgsFunction = completeLayoutNames
	_ = watchCmd.RegisterFlagCompletionFunc("interval", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"1m", "5m", "10m", "30m"}, cobra.ShellCompDirectiveNoFileComp
	})
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	pidPath := orchestrate.DefaultPIDPath()
	logPath := orchestrate.DefaultLogPath()

	// --shell-hook: detect shell, print hook, exit
	if watchShellHook {
		shell := orchestrate.DetectShell()
		hook := orchestrate.ShellHook(shell)
		if hook == "" {
			fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render("No hook available for shell: "+shell))
			return nil
		}
		fmt.Print(hook)
		return nil
	}

	// --status: check if daemon is running, print status, exit
	if watchStatus {
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if running {
			fmt.Fprintf(os.Stderr, "%s  pid %d\n", greenStyle.Render("crex watch daemon running"), pid)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render("crex watch daemon not running"))
		}
		return nil
	}

	// --stop: find process, send SIGINT, remove PID file, exit
	if watchStop {
		running, pid := orchestrate.IsDaemonRunning(pidPath)
		if !running {
			fmt.Fprintf(os.Stderr, "%s\n", dimStyle.Render("crex watch daemon not running"))
			return nil
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("find process %d: %w", pid, err)
		}
		if err := proc.Signal(syscall.SIGINT); err != nil {
			return fmt.Errorf("signal process %d: %w", pid, err)
		}
		orchestrate.RemovePIDFile(pidPath)
		fmt.Fprintf(os.Stderr, "%s  pid %d\n", greenStyle.Render("stopped crex watch daemon"), pid)
		return nil
	}

	// Normal / daemon mode — build watcher
	name := "autosave"
	if len(args) > 0 {
		name = args[0]
	}

	interval := cfg.WatchInterval
	if watchInterval != "" {
		d, err := time.ParseDuration(watchInterval)
		if err != nil {
			return fmt.Errorf("invalid interval: %w", err)
		}
		interval = d
	}

	cl := newClient()
	store, err := newStore()
	if err != nil {
		return err
	}

	watcher := &orchestrate.Watcher{
		Client:        cl,
		Store:         store,
		Name:          name,
		Interval:      interval,
		WorkspaceFile: cfg.WorkspaceFile,
	}

	if watchDaemon {
		// Check if already running
		if running, pid := orchestrate.IsDaemonRunning(pidPath); running {
			fmt.Fprintf(os.Stderr, "%s  pid %d\n", dimStyle.Render("crex watch daemon already running"), pid)
			return nil
		}

		// Open log with 1MB rotation
		logFile, err := orchestrate.OpenLogWriter(logPath, 1024*1024)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}

		// Write PID file
		if err := orchestrate.WritePIDFile(pidPath, os.Getpid()); err != nil {
			logFile.Close()
			return fmt.Errorf("write pid: %w", err)
		}

		watcher.LogWriter = logFile

		fmt.Fprintf(os.Stderr, "%s  pid %d  log %s\n",
			greenStyle.Render("crex watch daemon started"), os.Getpid(), dimStyle.Render(logPath))

		defer func() {
			orchestrate.RemovePIDFile(pidPath)
			logFile.Close()
		}()

		return watcher.Run()
	}

	// Foreground mode
	fmt.Fprintf(os.Stderr, "Watching terminal state, saving as %q every %s\n", name, interval)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop\n")

	return watcher.Run()
}
