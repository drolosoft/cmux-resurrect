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
	RunE:  runProjectAdd,
}

func init() {
	wsAddCmd.Flags().StringVarP(&addIcon, "icon", "i", "📁", "workspace icon emoji")
	wsAddCmd.Flags().StringVarP(&addTemplate, "template", "t", "dev", "template name (dev, go, single, monitor)")
	wsAddCmd.Flags().BoolVar(&addPin, "pin", true, "pin workspace in sidebar")
	wsAddCmd.Flags().BoolVar(&addDisabled, "disabled", false, "add as disabled (unchecked)")
	workspaceCmd.AddCommand(wsAddCmd)
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
