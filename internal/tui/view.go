package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m Model) render() string {
	title := titleStyle.Render("Ghostty Config TUI")
	subtitle := mutedStyle.Render("A guided editor for Ghostty's text configuration")
	header := title + "\n" + subtitle

	left := m.renderOptions()
	right := m.renderDetails()
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	config := m.configPath
	if config == "" {
		config = "No config path discovered"
	}
	ghostty := m.ghostty
	if ghostty == "" {
		ghostty = "Ghostty binary not found (schema-only mode)"
	} else {
		ghostty = "Ghostty: " + ghostty
	}

	footer := mutedStyle.Render("↑/k ↓/j navigate   q quit")
	return strings.Join([]string{
		header,
		"",
		panels,
		"",
		accentStyle.Render("Config: ") + config,
		mutedStyle.Render(ghostty),
		"",
		footer,
	}, "\n")
}

func (m Model) renderOptions() string {
	rows := []string{accentStyle.Render("Options")}
	for index, option := range m.options {
		marker := "  "
		if index == m.selected {
			marker = selectedStyle.Render("▸ ")
		}
		rows = append(rows, marker+fmt.Sprintf("%-20s %s", option.Key, mutedStyle.Render(string(option.Kind))))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderDetails() string {
	if len(m.options) == 0 {
		return panelStyle.Render("No schema options loaded")
	}

	option := m.options[m.selected]
	rows := []string{
		accentStyle.Render(option.Category),
		selectedStyle.Render(option.Key),
		"",
		option.Description,
		"",
		mutedStyle.Render("Editor: " + string(option.Kind)),
		mutedStyle.Render("Status: bootstrap metadata"),
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}
