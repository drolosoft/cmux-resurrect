package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// -- Adaptive color palette --------------------------------------------------
// Colors resolve automatically based on terminal background (dark vs light).

var (
	colorGreen = lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}
	colorDim   = lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"}
	colorCyan  = lipgloss.AdaptiveColor{Dark: "#87D7FF", Light: "#0277BD"}
	colorYellow = lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}
	colorMagenta = lipgloss.AdaptiveColor{Dark: "#D787FF", Light: "#8B008B"}
)

// -- Shared styles -----------------------------------------------------------

var (
	headingStyle = lipgloss.NewStyle().Bold(true).MarginTop(1)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)
	cyanStyle    = lipgloss.NewStyle().Foreground(colorCyan)
	yellowStyle  = lipgloss.NewStyle().Foreground(colorYellow)
	greenStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	magentaStyle = lipgloss.NewStyle().Foreground(colorMagenta)
)

// -- Template styles ---------------------------------------------------------

var (
	templateIconStyle = lipgloss.NewStyle().Width(3)
	templateNameStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Width(14)
	templatePaneStyle = lipgloss.NewStyle().Foreground(colorCyan).Width(5)
	templateDescStyle = lipgloss.NewStyle().Foreground(colorDim)
	categoryStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorYellow).MarginTop(1)
)

// -- Flame wing rendering ----------------------------------------------------

// flameWing colors each visible character in a wing string using the given
// gradient. For left wings the gradient flows left→right (ember→gold); for
// right wings it flows right→left (gold→ember).
func flameWing(s string, reverse bool, gradient []lipgloss.Color) string {
	runes := []rune(s)

	var visible []int
	for i, ch := range runes {
		if ch != ' ' {
			visible = append(visible, i)
		}
	}
	if len(visible) == 0 {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) * 20)

	visMap := make(map[int]int, len(visible))
	for rank, idx := range visible {
		visMap[idx] = rank
	}

	for i, ch := range runes {
		if ch == ' ' {
			result.WriteRune(' ')
			continue
		}
		rank := visMap[i]
		if reverse {
			rank = len(visible) - 1 - rank
		}
		colorIdx := rank * (len(gradient) - 1) / max(len(visible)-1, 1)
		style := lipgloss.NewStyle().Foreground(gradient[colorIdx])
		result.WriteString(style.Render(string(ch)))
	}
	return result.String()
}

// flameLine applies the flame→green gradient across a full line of text.
// Visible characters are colored left-to-right: ember → gold → green.
// Used for the cmux-resurrect banner where there are no separate wings.
func flameLine(s string, th theme) string {
	runes := []rune(s)

	// Build combined gradient: flame colors + the banner green at the end.
	grad := make([]lipgloss.Color, len(th.flame)+1)
	copy(grad, th.flame)
	grad[len(grad)-1] = th.green

	var visible []int
	for i, ch := range runes {
		if ch != ' ' {
			visible = append(visible, i)
		}
	}
	if len(visible) == 0 {
		return s
	}

	var result strings.Builder
	result.Grow(len(s) * 20)

	visMap := make(map[int]int, len(visible))
	for rank, idx := range visible {
		visMap[idx] = rank
	}

	for i, ch := range runes {
		if ch == ' ' {
			result.WriteRune(' ')
			continue
		}
		rank := visMap[i]
		colorIdx := rank * (len(grad) - 1) / max(len(visible)-1, 1)
		style := lipgloss.NewStyle().Foreground(grad[colorIdx]).Bold(true)
		result.WriteString(style.Render(string(ch)))
	}
	return result.String()
}

// -- ASCII banner ------------------------------------------------------------

func banner() string {
	th := detectTheme()
	mode := resolveBannerMode()
	greenStyle := lipgloss.NewStyle().Foreground(th.green).Bold(true)
	plainStyle := lipgloss.NewStyle().Foreground(th.dim)

	var b strings.Builder

	if isCmuxBranding() {
		art := []string{
			`                                                                        _   `,
			`  ___ _ __ ___  _   ___  __     _ __ ___  ___ _   _ _ __ _ __ ___  ___| |_ `,
			` / __| '_ ` + "`" + ` _ \| | | \ \/ /____| '__/ _ \/ __| | | | '__| '__/ _ \/ __| __|`,
			`| (__| | | | | | |_| |>  <_____| | |  __/\__ \ |_| | |  | | |  __/ (__| |_ `,
			` \___|_| |_| |_|\__,_/_/\_\    |_|  \___||___/\__,_|_|  |_|  \___|\___|\__|`,
		}
		for _, line := range art {
			switch mode {
			case bannerPlain:
				b.WriteString(plainStyle.Render(line))
			case bannerClassic:
				b.WriteString(greenStyle.Render(line))
			default:
				b.WriteString(flameLine(line, th))
			}
			b.WriteString("\n")
		}
	} else {
		type wingedLine struct {
			left, text, right string
		}
		lines := []wingedLine{
			{`·  · · ────────  `, `  ___ _ __ _____  __`, `  ──────── · ·  ·`},
			{`   · · ────────  `, ` / __| '__/ _ \ \/ /`, `  ──────── · ·   `},
			{`     · · ──────  `, `| (__| | |  __/>  < `, `  ────── · ·     `},
			{`       · · ────  `, ` \___|_|  \___/_/\_\`, `  ──── · ·       `},
		}
		for _, l := range lines {
			switch mode {
			case bannerPlain:
				b.WriteString(plainStyle.Render(l.left))
				b.WriteString(plainStyle.Render(l.text))
				b.WriteString(plainStyle.Render(l.right))
			case bannerClassic:
				b.WriteString(greenStyle.Render(l.left))
				b.WriteString(greenStyle.Render(l.text))
				b.WriteString(greenStyle.Render(l.right))
			default:
				b.WriteString(flameWing(l.left, false, th.flame))
				b.WriteString(greenStyle.Render(l.text))
				b.WriteString(flameWing(l.right, true, th.flame))
			}
			b.WriteString("\n")
		}
	}

	// Tagline with accented "resurrected."
	b.WriteString(renderTagline(th, mode))
	b.WriteString("\n")

	// Version line — dimmer than tagline.
	ver := fmt.Sprintf("  %s (%s) built %s", Version, Commit, Date)
	b.WriteString(th.version.Render(ver))
	b.WriteString("\n")
	return b.String()
}

// renderTagline splits the tagline to accent "resurrected." in flame color.
// In classic mode the accent is green; in plain mode there is no accent.
func renderTagline(th theme, mode bannerMode) string {
	tag := appTagline()
	const keyword = "resurrected."
	idx := strings.LastIndex(tag, keyword)
	if idx == -1 || mode == bannerPlain {
		return th.tagline.Render("  " + tag)
	}
	var b strings.Builder
	b.WriteString(th.tagline.Render("  " + tag[:idx]))
	switch mode {
	case bannerClassic:
		accent := lipgloss.NewStyle().Foreground(th.green).Italic(true).Bold(true)
		b.WriteString(accent.Render(keyword))
	default:
		b.WriteString(th.accent.Render(keyword))
	}
	return b.String()
}

// -- Compact Help (fits on one screen with the banner) -----------------------

func styledHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	helpCmd(&b, "save", "[name]", "Snapshot current layout")
	helpCmd(&b, "restore", "[name]", "Recreate workspaces from layout")
	helpCmd(&b, "list", "", "List saved layouts")
	helpCmd(&b, "show", "<name>", "Display layout details")
	helpCmd(&b, "edit", "<name>", "Open in $EDITOR")
	helpCmd(&b, "delete", "<name>", "Delete a layout")
	helpCmd(&b, "watch", "[name]", "Auto-save on a timer")
	helpCmd(&b, "import-from-md", "", "Import from Workspace Blueprint")
	helpCmd(&b, "export-to-md", "", "Export to Workspace Blueprint")
	helpCmd(&b, "workspace", "<cmd>", "Manage Blueprint (add|remove|list|toggle)")
	helpCmd(&b, "template", "<cmd>", "Template gallery (list|show|use|customize)")
	helpCmd(&b, "version", "", "Version info")
	helpCmd(&b, "completion", "<shell>", "Shell completions (bash|zsh|fish)")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Quick start:"))
	b.WriteString("\n")
	if isCmuxBranding() {
		fmt.Fprintf(&b, "    %s%s%s%s%s\n",
			dimStyle.Render("("),
			greenStyle.Render("crex"),
			dimStyle.Render(" is the short name for "),
			greenStyle.Render("cmux-resurrect"),
			dimStyle.Render(")"))
		b.WriteString("\n")
	}
	helpExample(&b, "crex import-from-md", "create workspaces from Blueprint")
	helpExample(&b, "crex save my-day", "save current layout")
	helpExample(&b, "crex list", "list saved layouts")
	helpExample(&b, "crex restore my-day --mode add", "restore a saved layout")
	helpExample(&b, "crex workspace add notes ~/docs", "add workspace to Blueprint")
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  crex <command> --help for flags and details"))
	b.WriteString("\n")

	return b.String()
}

func helpCmd(b *strings.Builder, name, args, desc string) {
	nameRendered := greenStyle.Render(fmt.Sprintf("  %-18s", name))
	argsRendered := dimStyle.Render(fmt.Sprintf("%-12s", args))
	fmt.Fprintf(b, "%s %s %s\n", nameRendered, argsRendered, desc)
}

func helpExample(b *strings.Builder, cmd, desc string) {
	fmt.Fprintf(b, "    %s  %s\n", cyanStyle.Render(cmd), dimStyle.Render(desc))
}

// padTitle inserts an extra space after any variation selector (U+FE0F) in a
// title. Some terminals miscalculate emoji width when VS16 is present (e.g.,
// ⚙️ renders narrower than expected), causing the next character to overlap.
// By replacing "VS16 " with "VS16  " the icon-to-name gap stays visible.
func padTitle(title string) string {
	return strings.ReplaceAll(title, "\uFE0F ", "\uFE0F  ")
}
