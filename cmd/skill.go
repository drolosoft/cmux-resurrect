package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/agentskill"
	"github.com/spf13/cobra"
)

var (
	skillInstallDir   string
	skillInstallCodex bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Agent skill — teach AI coding agents to drive crex",
	Long: `The crex agent skill is a reference document that teaches AI coding
agents (Claude Code, Codex, and compatible tools) how to drive crex:
save/restore semantics, safe scripting flags, AI-session resume, and
programmatic layout queries.

Install it once and agents on this machine pick it up automatically.`,
}

var skillShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the agent skill to stdout",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), agentskill.Content())
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the agent skill for Claude Code (or Codex with --codex)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := skillInstallDir
		if dir == "" {
			if skillInstallCodex {
				dir = agentskill.CodexDir()
			} else {
				dir = agentskill.ClaudeDir()
			}
		}
		if dir == "" {
			return fmt.Errorf("could not resolve the skills directory (no home dir?); use --dir")
		}
		path, err := agentskill.Install(dir)
		if err != nil {
			return fmt.Errorf("install skill: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Agent skill installed: %s\n", path)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "   Agents pick it up on their next session. Re-run after crex upgrades to refresh it.")
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().StringVar(&skillInstallDir, "dir", "", "skills base directory (default: ~/.claude/skills, or ~/.agents/skills with --codex)")
	skillInstallCmd.Flags().BoolVar(&skillInstallCodex, "codex", false, "install for Codex-compatible agents (~/.agents/skills)")
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillInstallCmd)
	rootCmd.AddCommand(skillCmd)
}
