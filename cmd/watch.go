package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var watchInterval string

var watchCmd = &cobra.Command{
	Use:   "watch [name]",
	Short: "Auto-save layout periodically",
	Long:  "Watches terminal state and saves it periodically. Deduplicates via content hash.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().StringVarP(&watchInterval, "interval", "i", "", "save interval (default from config, e.g. 5m)")
	watchCmd.ValidArgsFunction = completeLayoutNames
	_ = watchCmd.RegisterFlagCompletionFunc("interval", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"1m", "5m", "10m", "30m"}, cobra.ShellCompDirectiveNoFileComp
	})
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

	fmt.Fprintf(os.Stderr, "Watching terminal state, saving as %q every %s\n", name, interval)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop\n")

	return watcher.Run()
}
