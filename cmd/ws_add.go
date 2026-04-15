package cmd

import (
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	addIcon     string
	addTemplate string
	addPin      bool
	addDisabled bool
)

var wsAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add a workspace entry to the Workspace Blueprint",
	Args:  cobra.ExactArgs(2),
	RunE:  runWorkspaceAdd,
}

func init() {
	wsAddCmd.Flags().StringVarP(&addIcon, "icon", "i", "📁", "workspace icon emoji")
	wsAddCmd.Flags().StringVarP(&addTemplate, "template", "t", "dev", "template name (run 'crex template list' for options)")
	wsAddCmd.Flags().BoolVar(&addPin, "pin", true, "pin workspace in sidebar")
	wsAddCmd.Flags().BoolVar(&addDisabled, "disabled", false, "add as disabled (unchecked)")
	_ = wsAddCmd.RegisterFlagCompletionFunc("template", completeTemplateNames)
	wsAddCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
	workspaceCmd.AddCommand(wsAddCmd)
}

func runWorkspaceAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := args[1]

	p := model.Project{
		Enabled:  !addDisabled,
		Icon:     addIcon,
		Name:     name,
		Template: addTemplate,
		Pin:      addPin,
		Path:     path,
	}

	wsFile := cfg.WorkspaceFile
	if err := mdfile.AddProject(wsFile, p); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr)
	check := greenStyle.Render("✅")
	if addDisabled {
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
		greenStyle.Render("✅ Added to Workspace Blueprint"))
	return nil
}
