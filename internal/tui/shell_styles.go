package tui

import "github.com/charmbracelet/lipgloss"

var (
	shellPromptStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
	shellHeadingStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FFD787", Light: "#B8860B"}).Bold(true)
	shellDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#8C8C8C", Light: "#6C6C6C"})
	shellErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#CC3333"})
	shellSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"})
	shellCursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#5FFF87", Light: "#1A8A3E"}).Bold(true)
)
