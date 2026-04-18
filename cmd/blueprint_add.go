package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	bpAddIcon     string
	bpAddTemplate string
	bpAddPin      bool
	bpAddDisabled bool
)

var blueprintAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add an entry to the Blueprint",
	Args:  cobra.ExactArgs(2),
	RunE:  runBlueprintAdd,
}

// bpAddValidArgsFunc is the shared ValidArgsFunction for blueprint/workspace add.
func bpAddValidArgsFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// First arg = name (freeform), no file completion
		return nil, cobra.ShellCompDirectiveNoFileComp
	case 1:
		// Second arg = path, use directory completion
		return nil, cobra.ShellCompDirectiveFilterDirs
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func init() {
	blueprintAddCmd.Flags().StringVarP(&bpAddIcon, "icon", "i", "📁", "entry icon emoji")
	blueprintAddCmd.Flags().StringVarP(&bpAddTemplate, "template", "t", "dev", "template name (run 'crex template list' for options)")
	blueprintAddCmd.Flags().BoolVar(&bpAddPin, "pin", true, "pin entry in sidebar")
	blueprintAddCmd.Flags().BoolVar(&bpAddDisabled, "disabled", false, "add as disabled (unchecked)")
	_ = blueprintAddCmd.RegisterFlagCompletionFunc("template", completeTemplateNames)
	blueprintAddCmd.ValidArgsFunction = bpAddValidArgsFunc
	blueprintCmd.AddCommand(blueprintAddCmd)

	// Legacy subcommand under workspaceLegacyCmd for backward compatibility.
	// workspaceLegacyCmd is Hidden, so this won't appear in normal --help.
	legacyAdd := &cobra.Command{
		Use:  "add <name> <path>",
		Args: cobra.ExactArgs(2),
		RunE: runBlueprintAdd,
	}
	legacyAdd.Flags().StringVarP(&bpAddIcon, "icon", "i", "📁", "entry icon emoji")
	legacyAdd.Flags().StringVarP(&bpAddTemplate, "template", "t", "dev", "template name (run 'crex template list' for options)")
	legacyAdd.Flags().BoolVar(&bpAddPin, "pin", true, "pin entry in sidebar")
	legacyAdd.Flags().BoolVar(&bpAddDisabled, "disabled", false, "add as disabled (unchecked)")
	_ = legacyAdd.RegisterFlagCompletionFunc("template", completeTemplateNames)
	legacyAdd.ValidArgsFunction = bpAddValidArgsFunc
	workspaceLegacyCmd.AddCommand(legacyAdd)
}

func runBlueprintAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := args[1]

	p := model.Project{
		Enabled:  !bpAddDisabled,
		Icon:     bpAddIcon,
		Name:     name,
		Template: bpAddTemplate,
		Pin:      bpAddPin,
		Path:     path,
	}

	wsFile := cfg.WorkspaceFile
	if err := mdfile.AddProject(wsFile, p); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	check := greenStyle.Render("✅")
	if bpAddDisabled {
		check = dimStyle.Render("⬜")
	}
	fmt.Fprintf(os.Stderr, "  %s %s %s  %s  %s\n",
		check,
		p.Icon,
		greenStyle.Render(p.Name),
		cyanStyle.Render("template="+p.Template),
		dimStyle.Render(path))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "%s\n\n",
		greenStyle.Render("✅ Added to Blueprint"))
	return nil
}
