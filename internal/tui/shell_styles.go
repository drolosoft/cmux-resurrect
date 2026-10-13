package tui

import "github.com/charmbracelet/lipgloss"

var (
	shellPromptStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
	shellFlameStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFB454", Light: "#D4820A"})
	shellHeadingStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}).Bold(true)
	shellDimStyle        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	shellErrorStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#CC3333"})
	shellSuccessStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	shellCursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
	shellCompletionStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#6C6C6C", Light: "#9C9C9C"})

	// shellGroupStyle is the workspace group header, the same bold magenta
	// the CLI uses so a group looks alike in both.
	shellGroupStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#D787FF", Light: "#8B008B"}).Bold(true)
)
