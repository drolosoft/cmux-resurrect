package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/mdfile"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var templateCustomizeCmd = &cobra.Command{
	Use:   "customize <name>",
	Short: "Copy a gallery template into your Workspace Blueprint",
	Long:  "Copies a built-in gallery template into your Workspace Blueprint for customization.\nYour copy takes priority over the built-in version.",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateCustomize,
}

func init() {
	templateCustomizeCmd.ValidArgsFunction = completeTemplateNames
	templateCmd.AddCommand(templateCustomizeCmd)
}

func runTemplateCustomize(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Look up the gallery template.
	tmpl, ok := gallery.Get(name)
	if !ok {
		return fmt.Errorf("template %q not found in gallery", name)
	}

	// Parse existing workspace file (create empty if not exists).
	wsFile := cfg.WorkspaceFile
	wf, err := mdfile.Parse(wsFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			wf = &model.WorkspaceFile{
				Templates: make(map[string]*model.Template),
			}
		} else {
			return err
		}
	}

	// Check if user already has a template with this name.
	if wf.Templates == nil {
		wf.Templates = make(map[string]*model.Template)
	}
	if _, exists := wf.Templates[name]; exists {
		return fmt.Errorf("template %q already exists in your Workspace Blueprint; edit with: crex edit", name)
	}

	// Copy the gallery template: keep Name + Panes, strip gallery-only metadata.
	userTmpl := &model.Template{
		Name: tmpl.Name,
	}
	for _, tp := range tmpl.Panes {
		pane := tp
		// Strip FocusTarget since user Blueprint syntax doesn't support @focus=N.
		pane.FocusTarget = -1
		userTmpl.Panes = append(userTmpl.Panes, pane)
	}

	wf.Templates[name] = userTmpl

	// Write back.
	if err := mdfile.Write(wsFile, wf); err != nil {
		return err
	}

	o := newWF(cmd.OutOrStderr())
	o.ln()
	o.f("  %s Copied %s to your Workspace Blueprint.\n",
		greenStyle.Render("✅"),
		greenStyle.Render("'"+name+"'"))
	o.f("  Your copy now takes priority over the built-in.\n")
	o.f("  Edit with: %s\n", cyanStyle.Render("crex edit"))
	o.ln()

	return nil
}
