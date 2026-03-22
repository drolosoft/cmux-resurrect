package orchestrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/txeo/cmux-persist/internal/client"
	"github.com/txeo/cmux-persist/internal/persist"
)

// Watcher periodically saves the cmux layout, deduplicating via content hash.
type Watcher struct {
	Client        client.CmuxClient
	Store         persist.Store
	Name          string
	Interval      time.Duration
	WorkspaceFile string // MD file path; if set, also updates the MD on each tick

	lastHash string
}

// Run starts the watch loop, blocking until interrupted.
func (w *Watcher) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Save immediately on start.
	w.saveOnce()

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\nStopping watcher. Final save...\n")
			w.saveOnce()
			return nil
		case <-ticker.C:
			w.saveOnce()
		}
	}
}

func (w *Watcher) saveOnce() {
	saver := &Saver{Client: w.Client, Store: w.Store}
	layout, err := saver.Save(w.Name, "autosave")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  watch save error: %v\n", err)
		return
	}

	// Compute hash to detect changes.
	data, _ := os.ReadFile(w.Store.Path(w.Name))
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	if hash == w.lastHash {
		return // no change, skip logging and MD update
	}
	w.lastHash = hash

	// Also update the MD file if configured.
	if w.WorkspaceFile != "" {
		exporter := &Exporter{Client: w.Client}
		if err := exporter.ExportToMD(w.WorkspaceFile); err != nil {
			fmt.Fprintf(os.Stderr, "  watch md update error: %v\n", err)
		}
	}

	fmt.Fprintf(os.Stderr, "  saved %d workspaces at %s\n",
		len(layout.Workspaces),
		time.Now().Format("15:04:05"))
}
