package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/client"
	"github.com/drolosoft/cmux-resurrect/internal/config"
	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/drolosoft/cmux-resurrect/internal/setup"
	"github.com/spf13/cobra"
)

var setupDefaults bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "First-run wizard — configure crex for your terminal",
	Long:  "Interactive guided configuration: detects your terminal backend, creates config, offers to save your current layout, and optionally sets up auto-persistence.",
	Args:  cobra.NoArgs,
	RunE:  runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupDefaults, "defaults", false, "accept all defaults without prompts (for CI/scripting)")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	o := newWF(os.Stderr)

	o.ln(headingStyle.Render("crex setup"))
	o.ln()

	// Step 1: Backend Detection
	o.ln(cyanStyle.Render("Step 1/4") + dimStyle.Render(" — Backend Detection"))
	detected := client.Detect()
	desc := setup.DescribeBackend(detected)
	if detected != client.BackendUnknown {
		o.f("  %s  Detected backend: %s\n", greenStyle.Render("✓"), desc)
	} else {
		o.f("  %s  No backend detected (%s) — some features will be limited\n", yellowStyle.Render("!"), desc)
	}
	o.ln()

	// Step 2: Configuration
	o.ln(cyanStyle.Render("Step 2/4") + dimStyle.Render(" — Configuration"))
	cfgPath := config.DefaultConfigPath()
	created, err := setup.WriteConfigIfNotExists(cfgPath, "5m", 10)
	if err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if created {
		o.f("  %s  Created config: %s\n", greenStyle.Render("✓"), dimStyle.Render(cfgPath))
	} else {
		o.f("  %s  Config already exists: %s\n", dimStyle.Render("·"), dimStyle.Render(cfgPath))
	}
	o.ln()

	// Step 3: Layouts Directory
	o.ln(cyanStyle.Render("Step 3/4") + dimStyle.Render(" — Layouts Directory"))
	layoutsDir := config.DefaultLayoutsDir()
	if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
		return fmt.Errorf("create layouts dir: %w", err)
	}
	o.f("  %s  Layouts directory ready: %s\n", greenStyle.Render("✓"), dimStyle.Render(layoutsDir))
	o.ln()

	// Step 4: First Save
	o.ln(cyanStyle.Render("Step 4/4") + dimStyle.Render(" — First Save"))
	if detected != client.BackendUnknown && setupDefaults {
		if err := doFirstSave(o, detected); err != nil {
			o.f("  %s  First save skipped: %v\n", yellowStyle.Render("!"), err)
		}
	} else if detected == client.BackendUnknown {
		o.f("  %s  No backend detected — skipping first save\n", dimStyle.Render("·"))
	} else {
		o.f("  %s  Run %s to save your current layout\n",
			dimStyle.Render("·"),
			cyanStyle.Render("crex save my-layout"))
	}
	o.ln()

	// Summary
	o.ln(headingStyle.Render("Setup complete!"))
	o.ln()
	o.ln(dimStyle.Render("  Quick-start examples:"))
	o.f("    %s  %s\n", cyanStyle.Render("crex save my-day"), dimStyle.Render("save current layout"))
	o.f("    %s  %s\n", cyanStyle.Render("crex list"), dimStyle.Render("list saved layouts"))
	o.f("    %s  %s\n", cyanStyle.Render("crex restore my-day"), dimStyle.Render("restore a saved layout"))
	o.f("    %s  %s\n", cyanStyle.Render("crex watch my-day"), dimStyle.Render("auto-save on a timer"))
	o.ln()
	o.ln(dimStyle.Render("  crex <command> --help for flags and details"))
	o.ln()

	return nil
}

func doFirstSave(o wf, detected client.DetectedBackend) error {
	var cl client.Backend
	switch detected {
	case client.BackendGhostty:
		cl = client.NewGhosttyClient()
	default:
		cl = client.NewCLIClient()
	}
	store, err := newStore()
	if err != nil {
		return err
	}
	saver := &orchestrate.Saver{Client: cl, Store: store}
	layout, err := saver.Save("initial", "created by crex setup")
	if err != nil {
		return err
	}
	o.f("  %s  %s\n", greenStyle.Render("✓"), fmt.Sprintf("Saved %d workspaces as 'initial'", len(layout.Workspaces)))
	return nil
}
