package cmd

import (
	"fmt"
	"os"
	"strings"

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
	o.ln(cyanStyle.Render("Step 1/6") + dimStyle.Render(" — Backend Detection"))
	detected := client.Detect()
	desc := setup.DescribeBackend(detected)
	if detected != client.BackendUnknown {
		o.f("  %s  Detected backend: %s\n", greenStyle.Render("✓"), desc)
	} else {
		o.f("  %s  No backend detected (%s) — some features will be limited\n", yellowStyle.Render("!"), desc)
	}
	o.ln()

	// Step 2: Configuration
	o.ln(cyanStyle.Render("Step 2/6") + dimStyle.Render(" — Configuration"))
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
	o.ln(cyanStyle.Render("Step 3/6") + dimStyle.Render(" — Layouts Directory"))
	layoutsDir := config.DefaultLayoutsDir()
	if err := os.MkdirAll(layoutsDir, 0o755); err != nil {
		return fmt.Errorf("create layouts dir: %w", err)
	}
	o.f("  %s  Layouts directory ready: %s\n", greenStyle.Render("✓"), dimStyle.Render(layoutsDir))
	o.ln()

	// Step 4: First Save
	o.ln(cyanStyle.Render("Step 4/6") + dimStyle.Render(" — First Save"))
	switch {
	case detected != client.BackendUnknown && setupDefaults:
		if err := doFirstSave(o, detected); err != nil {
			o.f("  %s  First save skipped: %v\n", yellowStyle.Render("!"), err)
		}
	case detected == client.BackendUnknown:
		o.f("  %s  No backend detected — skipping first save\n", dimStyle.Render("·"))
	default:
		o.f("  %s  Run %s to save your current layout\n",
			dimStyle.Render("·"),
			cyanStyle.Render("crex save my-layout"))
	}
	o.ln()

	// Step 5: Quick Launch Hook
	o.ln(cyanStyle.Render("Step 5/6") + dimStyle.Render(" — Quick Launch (Ctrl+G)"))
	shell, rcFile, key := setup.OfferPopHook()
	if rcFile != "" && key != "" {
		if setupDefaults {
			if err := setup.InstallHookToFile(rcFile, shell, key); err == nil {
				o.f("  %s  Ctrl+G → crex pop installed to %s\n", greenStyle.Render("✓"), dimStyle.Render(rcFile))
			} else {
				o.f("  %s  Hook install skipped: %v\n", yellowStyle.Render("!"), err)
			}
		} else {
			o.f("  Add Ctrl+G shortcut to open %s instantly?\n", cyanStyle.Render("crex pop"))
			o.f("  This adds one line to %s\n\n", dimStyle.Render(rcFile))
			o.f("  %s ", dimStyle.Render("[y/N]"))
			var answer string
			fmt.Scanln(&answer)
			if strings.ToLower(strings.TrimSpace(answer)) == "y" {
				if err := setup.InstallHookToFile(rcFile, shell, key); err != nil {
					o.f("  %s  %v\n", yellowStyle.Render("!"), err)
				} else {
					o.f("  %s  Added to %s — restart your shell or: %s\n",
						greenStyle.Render("✓"),
						dimStyle.Render(rcFile),
						cyanStyle.Render("source "+rcFile))
				}
			} else {
				o.f("  %s  Skipped — you can always run: %s\n", dimStyle.Render("·"), cyanStyle.Render("crex pop"))
			}
		}
	} else {
		o.f("  %s  Shell not detected — you can run %s manually\n", dimStyle.Render("·"), cyanStyle.Render("crex pop"))
	}
	o.ln()

	// Step 6: Auto-Accept for AI Agents
	o.ln(cyanStyle.Render("Step 6/6") + dimStyle.Render(" — Auto-Accept for AI Agents"))
	if setupDefaults {
		o.f("  %s  Auto-accept disabled (safe default)\n", dimStyle.Render("·"))
	} else {
		o.ln(dimStyle.Render("  When restoring workspaces, crex can skip permission prompts for"))
		o.ln(dimStyle.Render("  AI coding agents (Claude Code, Codex, OpenCode, etc)."))
		o.ln()
		o.f("  %s  This is dangerous — agents will execute without asking.\n", yellowStyle.Render("⚠"))
		o.ln()
		o.f("  Enable auto-accept? %s ", dimStyle.Render("[y/N]"))
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(strings.TrimSpace(answer)) == "y" {
			o.ln()
			o.f("  Which agents? (comma-separated, or %s)\n", cyanStyle.Render("all"))
			o.f("  Suggestions: %s\n", cyanStyle.Render("claude, codex, opencode"))
			o.f("  > ")
			var agents string
			fmt.Scanln(&agents)
			agentList := parseAgentList(agents)
			if len(agentList) > 0 {
				currentCfg, loadErr := config.Load(cfgPath)
				if loadErr != nil {
					currentCfg = config.DefaultConfig()
				}
				currentCfg.AutoAccept = agentList
				if saveErr := config.Save(cfgPath, currentCfg); saveErr != nil {
					o.f("  %s  Failed to save: %v\n", yellowStyle.Render("!"), saveErr)
				} else {
					o.f("  %s  Auto-accept enabled for: %s\n",
						greenStyle.Render("✓"),
						cyanStyle.Render(strings.Join(agentList, ", ")))
				}
			} else {
				o.f("  %s  No agents specified — auto-accept disabled\n", dimStyle.Render("·"))
			}
		} else {
			o.f("  %s  Auto-accept disabled\n", dimStyle.Render("·"))
		}
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
	o.f("    %s  %s\n", cyanStyle.Render("crex pop"), dimStyle.Render("quick workspace picker (Ctrl+G)"))
	o.ln()
	o.ln(dimStyle.Render("  crex <command> --help for flags and details"))
	o.ln()

	return nil
}

// parseAgentList splits a comma/space-separated string into trimmed, non-empty agent names.
func parseAgentList(input string) []string {
	input = strings.ReplaceAll(input, ",", " ")
	parts := strings.Fields(input)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
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
