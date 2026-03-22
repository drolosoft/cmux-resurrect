package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
)

var (
	addIcon     string
	addTemplate string
	addPin      bool
	addDisabled bool
)

var projectAddCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Add a project to the workspace file",
	Args:  cobra.ExactArgs(2),
	RunE:  runProjectAdd,
}

func init() {
	projectAddCmd.Flags().StringVarP(&addIcon, "icon", "i", "📁", "project icon emoji")
	projectAddCmd.Flags().StringVarP(&addTemplate, "template", "t", "dev", "template name (dev, go, single, monitor)")
	projectAddCmd.Flags().BoolVar(&addPin, "pin", true, "pin workspace in sidebar")
	projectAddCmd.Flags().BoolVar(&addDisabled, "disabled", false, "add as disabled (unchecked)")
	projectCmd.AddCommand(projectAddCmd)
}

func runProjectAdd(cmd *cobra.Command, args []string) error {
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

	state := "enabled"
	if addDisabled {
		state = "disabled"
	}
	fmt.Fprintf(os.Stderr, "Added %s %s (%s, %s, template=%s) to %s\n",
		p.Icon, p.Name, state, path, p.Template, wsFile)
	return nil
}
