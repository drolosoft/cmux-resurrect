package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/juanatsap/cmux-resurrect/internal/orchestrate"
)

var watchInterval string

var watchCmd = &cobra.Command{
	Use:   "watch [name]",
	Short: "Auto-save layout periodically",
	Long:  "Watches the cmux state and saves it periodically. Deduplicates via content hash.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInterval, "interval", "i", "", "save interval (default from config, e.g. 5m)")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
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

	fmt.Fprintf(os.Stderr, "Watching cmux state, saving as %q every %s\n", name, interval)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop\n")

	return watcher.Run()
}
