package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) render() string {
	header := titleStyle.Render("Ghostty Config TUI") + "\n" + mutedStyle.Render("Guided editor · dry run only")

	var body string
	switch m.mode {
	case ModeEdit:
		body = m.renderEdit()
	case ModePreview:
		body = m.renderPreview()
	case ModeConfirmQuit:
		body = m.renderConfirmQuit()
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderOptions(), m.renderDetails())
	}

	config := m.configPath
	if config == "" {
		config = "(unknown)"
	}
	return strings.Join([]string{
		header,
		"",
		body,
		"",
		accentStyle.Render("Config: ") + config,
		mutedStyle.Render("No files will be written in this MVP"),
		m.footer(),
	}, "\n")
}

func (m Model) renderOptions() string {
	rows := []string{accentStyle.Render("Options")}
	for index, option := range m.options {
		marker := "  "
		if index == m.selected {
			marker = selectedStyle.Render("▸ ")
		}
		state := m.optionStatus(option)
		rows = append(rows, marker+fmt.Sprintf("%-20s %s", option.Key, mutedStyle.Render(state)))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderDetails() string {
	option, ok := m.selectedOption()
	if !ok {
		return panelStyle.Render("No schema options loaded")
	}

	rows := []string{
		accentStyle.Render(option.Category),
		selectedStyle.Render(option.Key),
		"",
		option.Description,
		"",
		"Config value: " + m.optionStatus(option),
		mutedStyle.Render("Editor: " + string(option.Kind)),
	}
	if !option.Editable() {
		rows = append(rows, mutedStyle.Render("Read-only in MVP; repeatable values need a dedicated editor."))
	}
	if len(m.draft.Assignments(option.Key)) > 1 {
		rows = append(rows, mutedStyle.Render("Duplicate assignments detected; edit is disabled."))
	}
	if len(m.draft.Assignments("config-file")) > 0 {
		rows = append(rows, mutedStyle.Render("Included files are not resolved; this is only the selected file."))
	}
	if m.status != "" {
		rows = append(rows, "", accentStyle.Render(m.status))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderEdit() string {
	option, _ := m.selectedOption()
	rows := []string{
		accentStyle.Render("Edit value"),
		selectedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		m.input.View(),
	}
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	rows = append(rows, "", mutedStyle.Render("enter stage · esc cancel"))
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderPreview() string {
	header := strings.Join([]string{
		accentStyle.Render("Preview"),
		selectedStyle.Render("DRY RUN — no file will be written"),
	}, "\n")
	return panelStyle.Render(header + "\n\n" + m.preview.View())
}

func (m Model) renderConfirmQuit() string {
	message := "Discard staged changes and quit?\n\n" + selectedStyle.Render("y") + " confirm    " + mutedStyle.Render("n / esc cancel")
	return panelStyle.Render(message)
}

func (m Model) footer() string {
	if m.help {
		return mutedStyle.Render("↑/k ↓/j move · enter/e edit · r reset · u revert · p preview · ? close help · q quit")
	}
	switch m.mode {
	case ModeEdit:
		return mutedStyle.Render("enter stage · esc cancel")
	case ModePreview:
		return mutedStyle.Render("↑/k ↓/j scroll · pgup/pgdn page · esc back · q quit")
	case ModeConfirmQuit:
		return mutedStyle.Render("y confirm · n/esc cancel")
	default:
		return mutedStyle.Render("↑/k ↓/j move · enter/e edit · r reset · u revert · p preview · ? help · q quit")
	}
}
