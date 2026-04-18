package cmd

import (
	"fmt"
	"strings"

	"github.com/drolosoft/cmux-resurrect/internal/gallery"
	"github.com/drolosoft/cmux-resurrect/internal/model"
	"github.com/spf13/cobra"
)

var tplShowAll bool

var templateShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a gallery template",
	Long:  "Preview a gallery template with its ASCII diagram and metadata.\nUse --all to display the full gallery at a glance.",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runTemplateShow,
}

func init() {
	templateShowCmd.Flags().BoolVar(&tplShowAll, "all", false, "show all templates at once")
	templateShowCmd.ValidArgsFunction = completeTemplateNames
	templateCmd.AddCommand(templateShowCmd)
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	if tplShowAll {
		return runTemplateShowAll(cmd)
	}

	if len(args) == 0 {
		return fmt.Errorf("provide a template name, or use --all to show the full gallery")
	}

	return showSingleTemplate(cmd, args[0])
}

func showSingleTemplate(cmd *cobra.Command, name string) error {
	tmpl, ok := gallery.Get(name)
	if !ok {
		return fmt.Errorf("template %q not found in gallery", name)
	}

	renderTemplateCard(newWF(cmd.OutOrStderr()), tmpl)
	return nil
}

func runTemplateShowAll(cmd *cobra.Command) error {
	o := newWF(cmd.OutOrStderr())

	layouts := gallery.ListByCategory("layout")
	workflows := gallery.ListByCategory("workflow")

	o.ln()
	o.ln(categoryStyle.Render("  Layouts"))

	for _, tmpl := range layouts {
		renderTemplateCard(o, tmpl)
	}

	o.ln(categoryStyle.Render("  Workflows"))

	for _, tmpl := range workflows {
		renderTemplateCard(o, tmpl)
	}

	total := len(layouts) + len(workflows)
	o.ln(dimStyle.Render(fmt.Sprintf("  %d templates (%d layouts, %d workflows)", total, len(layouts), len(workflows))))
	o.ln()
	return nil
}

// renderTemplateCard renders a full template preview (header + diagram + metadata).
func renderTemplateCard(o wf, tmpl *model.Template) {
	// Header: icon + name + description.
	o.f("\n  %s %s — %s\n\n",
		tmpl.Icon,
		greenStyle.Render(tmpl.Name),
		dimStyle.Render(tmpl.Description),
	)

	// ASCII diagram.
	diagram := renderDiagram(tmpl)
	for _, line := range strings.Split(diagram, "\n") {
		o.f("  %s\n", line)
	}
	o.ln()

	// Metadata.
	o.f("  %s  %s\n", dimStyle.Render("Category:"), cyanStyle.Render(tmpl.Category))
	o.f("  %s     %s\n", dimStyle.Render("Panes:"), cyanStyle.Render(fmt.Sprintf("%d", len(tmpl.Panes))))

	// Split sequence.
	splits := buildSplitSequence(tmpl)
	o.f("  %s    %s\n", dimStyle.Render("Splits:"), cyanStyle.Render(splits))

	// Tags.
	if len(tmpl.Tags) > 0 {
		o.f("  %s      %s\n", dimStyle.Render("Tags:"), cyanStyle.Render(strings.Join(tmpl.Tags, ", ")))
	}

	o.ln()
}

// buildSplitSequence returns a human-readable split sequence like "main → right → down".
func buildSplitSequence(tmpl *model.Template) string {
	parts := []string{"main"}
	for _, p := range tmpl.Panes {
		if p.Split != "" {
			parts = append(parts, p.Split)
		}
	}
	return strings.Join(parts, " → ")
}

// ---------------------------------------------------------------------------
// Pane label helpers
// ---------------------------------------------------------------------------

// paneLabel builds a display label for a pane.
// Priority: command > name > "main" (if IsMain) > "shell".
func paneLabel(p model.TemplatePan) string {
	switch {
	case p.Command != "":
		return p.Command
	case p.Name != "":
		return p.Name
	case p.IsMain:
		return "main"
	default:
		return "shell"
	}
}

// paneFocused returns true if the pane has focus.
func paneFocused(p model.TemplatePan) bool {
	return p.Focus
}

// truncLabel truncates a label to fit a given width.
func truncLabel(label string, width int) string {
	if len(label) <= width {
		return label
	}
	if width <= 3 {
		return label[:width]
	}
	return label[:width-3] + "..."
}

// padLabel pads a label to exactly width characters.
func padLabel(label string, width int) string {
	if len(label) >= width {
		return label[:width]
	}
	return label + strings.Repeat(" ", width-len(label))
}

// fmtLabel truncates then pads a label for box rendering.
// If focused is true, appends " *" after truncation (reserving space for it).
func fmtLabel(label string, width int) string {
	return padLabel(truncLabel(label, width), width)
}

// fmtLabelFocused truncates then pads a label, appending " *" for focused panes.
func fmtLabelFocused(label string, focused bool, width int) string {
	if focused {
		maxLabel := width - 2 // reserve space for " *"
		if maxLabel < 1 {
			maxLabel = 1
		}
		l := truncLabel(label, maxLabel)
		l += " *"
		return padLabel(l, width)
	}
	return fmtLabel(label, width)
}

// ---------------------------------------------------------------------------
// Diagram rendering
// ---------------------------------------------------------------------------

// paneInfo holds label and focus state for diagram rendering.
type paneInfo struct {
	label   string
	focused bool
}

// renderDiagram dispatches to the correct hardcoded diagram variant based on template name.
func renderDiagram(tmpl *model.Template) string {
	panes := make([]paneInfo, len(tmpl.Panes))
	for i, p := range tmpl.Panes {
		panes[i] = paneInfo{label: paneLabel(p), focused: paneFocused(p)}
	}

	switch tmpl.Name {
	case "single":
		return singleDiagram(panes)
	case "cols", "sidebar", "explore", "system", "network":
		return twoPaneHorizontalDiagram(panes)
	case "rows":
		return twoPaneVerticalDiagram(panes)
	case "shelf":
		return shelfDiagram(panes)
	case "aside", "claude", "code", "logs":
		return asideDiagram(panes)
	case "triple":
		return tripleDiagram(panes)
	case "quad":
		return quadDiagram(panes)
	case "dashboard":
		return dashboardDiagram(panes)
	case "ide":
		return ideDiagram(panes)
	default:
		return fallbackDiagram(panes)
	}
}

// safePane returns the paneInfo at index i, or a default "shell" pane if out of range.
func safePane(panes []paneInfo, i int) paneInfo {
	if i < len(panes) {
		return panes[i]
	}
	return paneInfo{label: "shell", focused: false}
}

// fl is a shorthand for fmtLabelFocused using a paneInfo.
func fl(p paneInfo, width int) string {
	return fmtLabelFocused(p.label, p.focused, width)
}

// singleDiagram: 1 pane.
//
//	┌──────────────────────────────────┐
//	│                                  │
//	│            label0                │
//	│                                  │
//	└──────────────────────────────────┘
func singleDiagram(panes []paneInfo) string {
	w := 34
	inner := w - 2
	l0 := fl(safePane(panes, 0), inner)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", inner) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", inner) + "│\n")
	b.WriteString("│" + l0 + "│\n")
	b.WriteString("│" + strings.Repeat(" ", inner) + "│\n")
	b.WriteString("└" + strings.Repeat("─", inner) + "┘")
	return b.String()
}

// twoPaneHorizontalDiagram: 2 side-by-side.
//
//	┌──────────┬───────────────────────┐
//	│          │                       │
//	│  label0  │  label1               │
//	│          │                       │
//	└──────────┴───────────────────────┘
func twoPaneHorizontalDiagram(panes []paneInfo) string {
	lw := 10 // left inner width
	rw := 23 // right inner width
	l0 := fl(safePane(panes, 0), lw)
	l1 := fl(safePane(panes, 1), rw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", lw) + "┬" + strings.Repeat("─", rw) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + strings.Repeat(" ", rw) + "│\n")
	b.WriteString("│" + l0 + "│" + l1 + "│\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + strings.Repeat(" ", rw) + "│\n")
	b.WriteString("└" + strings.Repeat("─", lw) + "┴" + strings.Repeat("─", rw) + "┘")
	return b.String()
}

// twoPaneVerticalDiagram: 2 stacked.
//
//	┌──────────────────────────────────┐
//	│           label0                 │
//	├──────────────────────────────────┤
//	│           label1                 │
//	└──────────────────────────────────┘
func twoPaneVerticalDiagram(panes []paneInfo) string {
	w := 34
	inner := w - 2
	l0 := fl(safePane(panes, 0), inner)
	l1 := fl(safePane(panes, 1), inner)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", inner) + "┐\n")
	b.WriteString("│" + l0 + "│\n")
	b.WriteString("├" + strings.Repeat("─", inner) + "┤\n")
	b.WriteString("│" + l1 + "│\n")
	b.WriteString("└" + strings.Repeat("─", inner) + "┘")
	return b.String()
}

// shelfDiagram: big top + 2 bottom.
//
//	┌──────────────────────────────────┐
//	│            label0                │
//	│                                  │
//	├────────────────┬─────────────────┤
//	│    label1      │    label2       │
//	└────────────────┴─────────────────┘
func shelfDiagram(panes []paneInfo) string {
	w := 34
	inner := w - 2
	blw := 16 // bottom-left inner width
	brw := inner - blw - 1

	l0 := fl(safePane(panes, 0), inner)
	l1 := fl(safePane(panes, 1), blw)
	l2 := fl(safePane(panes, 2), brw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", inner) + "┐\n")
	b.WriteString("│" + l0 + "│\n")
	b.WriteString("│" + strings.Repeat(" ", inner) + "│\n")
	b.WriteString("├" + strings.Repeat("─", blw) + "┬" + strings.Repeat("─", brw) + "┤\n")
	b.WriteString("│" + l1 + "│" + l2 + "│\n")
	b.WriteString("└" + strings.Repeat("─", blw) + "┴" + strings.Repeat("─", brw) + "┘")
	return b.String()
}

// asideDiagram: big left + 2 stacked right.
//
//	┌──────────┬───────────────────────┐
//	│          │  label1               │
//	│  label0  │                       │
//	│          ├───────────────────────┤
//	│          │  label2               │
//	└──────────┴───────────────────────┘
func asideDiagram(panes []paneInfo) string {
	lw := 10 // left inner width
	rw := 23 // right inner width

	l0 := fl(safePane(panes, 0), lw)
	l1 := fl(safePane(panes, 1), rw)
	l2 := fl(safePane(panes, 2), rw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", lw) + "┬" + strings.Repeat("─", rw) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + l1 + "│\n")
	b.WriteString("│" + l0 + "│" + strings.Repeat(" ", rw) + "│\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "├" + strings.Repeat("─", rw) + "┤\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + l2 + "│\n")
	b.WriteString("└" + strings.Repeat("─", lw) + "┴" + strings.Repeat("─", rw) + "┘")
	return b.String()
}

// tripleDiagram: 3 columns.
//
//	┌──────────┬──────────┬──────────┐
//	│          │          │          │
//	│  label0  │  label1  │  label2  │
//	│          │          │          │
//	└──────────┴──────────┴──────────┘
func tripleDiagram(panes []paneInfo) string {
	cw := 10 // column inner width

	l0 := fl(safePane(panes, 0), cw)
	l1 := fl(safePane(panes, 1), cw)
	l2 := fl(safePane(panes, 2), cw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", cw) + "┬" + strings.Repeat("─", cw) + "┬" + strings.Repeat("─", cw) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", cw) + "│" + strings.Repeat(" ", cw) + "│" + strings.Repeat(" ", cw) + "│\n")
	b.WriteString("│" + l0 + "│" + l1 + "│" + l2 + "│\n")
	b.WriteString("│" + strings.Repeat(" ", cw) + "│" + strings.Repeat(" ", cw) + "│" + strings.Repeat(" ", cw) + "│\n")
	b.WriteString("└" + strings.Repeat("─", cw) + "┴" + strings.Repeat("─", cw) + "┴" + strings.Repeat("─", cw) + "┘")
	return b.String()
}

// quadDiagram: 2x2 grid.
//
//	┌────────────────┬─────────────────┐
//	│    label0      │    label1       │
//	├────────────────┼─────────────────┤
//	│    label2      │    label3       │
//	└────────────────┴─────────────────┘
func quadDiagram(panes []paneInfo) string {
	w := 34
	inner := w - 2
	lw := 16 // left inner width
	rw := inner - lw - 1

	l0 := fl(safePane(panes, 0), lw)
	l1 := fl(safePane(panes, 1), rw)
	l2 := fl(safePane(panes, 2), lw)
	l3 := fl(safePane(panes, 3), rw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", lw) + "┬" + strings.Repeat("─", rw) + "┐\n")
	b.WriteString("│" + l0 + "│" + l1 + "│\n")
	b.WriteString("├" + strings.Repeat("─", lw) + "┼" + strings.Repeat("─", rw) + "┤\n")
	b.WriteString("│" + l2 + "│" + l3 + "│\n")
	b.WriteString("└" + strings.Repeat("─", lw) + "┴" + strings.Repeat("─", rw) + "┘")
	return b.String()
}

// dashboardDiagram: big top + 3 bottom.
//
//	┌──────────────────────────────────┐
//	│            label0                │
//	│                                  │
//	├──────────┬──────────┬──────────┤
//	│  label1  │  label2  │  label3  │
//	└──────────┴──────────┴──────────┘
func dashboardDiagram(panes []paneInfo) string {
	w := 34
	inner := w - 2
	cw := 10 // bottom column inner width

	l0 := fl(safePane(panes, 0), inner)
	l1 := fl(safePane(panes, 1), cw)
	l2 := fl(safePane(panes, 2), cw)
	l3 := fl(safePane(panes, 3), cw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", inner) + "┐\n")
	b.WriteString("│" + l0 + "│\n")
	b.WriteString("│" + strings.Repeat(" ", inner) + "│\n")
	b.WriteString("├" + strings.Repeat("─", cw) + "┬" + strings.Repeat("─", cw) + "┬" + strings.Repeat("─", cw) + "┤\n")
	b.WriteString("│" + l1 + "│" + l2 + "│" + l3 + "│\n")
	b.WriteString("└" + strings.Repeat("─", cw) + "┴" + strings.Repeat("─", cw) + "┴" + strings.Repeat("─", cw) + "┘")
	return b.String()
}

// ideDiagram: file tree + editor + console + tools.
//
//	┌────────┬─────────────────────────┐
//	│        │  label1                 │
//	│ label0 │                         │
//	│        ├────────────┬────────────┤
//	│        │  label2    │  label3    │
//	└────────┴────────────┴────────────┘
func ideDiagram(panes []paneInfo) string {
	lw := 8  // left (file tree) inner width
	rw := 25 // right total inner width
	rlw := 12
	rrw := rw - rlw - 1

	l0 := fl(safePane(panes, 0), lw)
	l1 := fl(safePane(panes, 1), rw)
	l2 := fl(safePane(panes, 2), rlw)
	l3 := fl(safePane(panes, 3), rrw)

	var b strings.Builder
	b.WriteString("┌" + strings.Repeat("─", lw) + "┬" + strings.Repeat("─", rw) + "┐\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + l1 + "│\n")
	b.WriteString("│" + l0 + "│" + strings.Repeat(" ", rw) + "│\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "├" + strings.Repeat("─", rlw) + "┬" + strings.Repeat("─", rrw) + "┤\n")
	b.WriteString("│" + strings.Repeat(" ", lw) + "│" + l2 + "│" + l3 + "│\n")
	b.WriteString("└" + strings.Repeat("─", lw) + "┴" + strings.Repeat("─", rlw) + "┴" + strings.Repeat("─", rrw) + "┘")
	return b.String()
}

// fallbackDiagram renders a simple single-pane diagram for unknown templates.
func fallbackDiagram(panes []paneInfo) string {
	return singleDiagram(panes)
}
