package cmd

import (
	"fmt"
	"io"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var (
	tplListLayout   bool
	tplListWorkflow bool
	tplListTag      string
)

var templateListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List available templates from the gallery",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runTemplateList,
}

func init() {
	templateListCmd.Flags().BoolVar(&tplListLayout, "layout", false, "show only layout templates")
	templateListCmd.Flags().BoolVar(&tplListWorkflow, "workflow", false, "show only workflow templates")
	templateListCmd.Flags().StringVar(&tplListTag, "tag", "", "filter templates by tag")
	_ = templateListCmd.RegisterFlagCompletionFunc("tag", completeTemplateTags)
	templateCmd.AddCommand(templateListCmd)
}

func runTemplateList(cmd *cobra.Command, _ []string) error {
	w := cmd.OutOrStderr()

	showLayouts := !tplListWorkflow  // show layouts unless --workflow only
	showWorkflows := !tplListLayout  // show workflows unless --layout only

	var totalShown int

	if showLayouts {
		layouts := filterByTag(gallery.ListByCategory("layout"), tplListTag)
		if len(layouts) > 0 {
			fmt.Fprintln(w, categoryStyle.Render("  LAYOUTS"))
			fmt.Fprintln(w)
			for _, tmpl := range layouts {
				renderTemplateLine(w, tmpl)
				totalShown++
			}
		}
	}

	if showWorkflows {
		workflows := filterByTag(gallery.ListByCategory("workflow"), tplListTag)
		if len(workflows) > 0 {
			fmt.Fprintln(w, categoryStyle.Render("  WORKFLOWS"))
			fmt.Fprintln(w)
			for _, tmpl := range workflows {
				renderTemplateLine(w, tmpl)
				totalShown++
			}
		}
	}

	// Summary line.
	layoutCount := len(gallery.ListByCategory("layout"))
	workflowCount := len(gallery.ListByCategory("workflow"))
	total := layoutCount + workflowCount

	fmt.Fprintln(w)
	if tplListTag != "" || tplListLayout || tplListWorkflow {
		fmt.Fprintln(w, dimStyle.Render(fmt.Sprintf("  %d templates shown (of %d total)", totalShown, total)))
	} else {
		fmt.Fprintln(w, dimStyle.Render(fmt.Sprintf("  %d templates (%d layouts, %d workflows)", total, layoutCount, workflowCount)))
	}

	return nil
}

// renderTemplateLine renders a single template entry.
func renderTemplateLine(w io.Writer, tmpl *model.Template) {
	icon := templateIconStyle.Render(tmpl.Icon)
	name := templateNameStyle.Render(tmpl.Name)
	panes := templatePaneStyle.Render(fmt.Sprintf("[%d]", len(tmpl.Panes)))
	desc := templateDescStyle.Render(tmpl.Description)
	fmt.Fprintf(w, "  %s %s %s %s\n", icon, name, panes, desc)
}

// filterByTag returns templates that have the given tag. If tag is empty, returns all.
func filterByTag(templates []*model.Template, tag string) []*model.Template {
	if tag == "" {
		return templates
	}
	var out []*model.Template
	for _, tmpl := range templates {
		for _, t := range tmpl.Tags {
			if t == tag {
				out = append(out, tmpl)
				break
			}
		}
	}
	return out
}
