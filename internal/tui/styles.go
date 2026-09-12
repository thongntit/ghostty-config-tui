package tui

import "charm.land/lipgloss/v2"

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f5c2e7"))
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6adc8"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#cba6f7"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
)
