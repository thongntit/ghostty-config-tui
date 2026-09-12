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
		m.searchBar(),
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
	visible := m.visibleOptionIndexes()
	if len(visible) == 0 {
		rows = append(rows, mutedStyle.Render("No options match the search"))
		return panelStyle.Render(strings.Join(rows, "\n"))
	}

	limit := m.optionRowLimit()
	start := maxInt(0, m.selected-limit/2)
	if start+limit > len(visible) {
		start = maxInt(0, len(visible)-limit)
	}
	end := minInt(len(visible), start+limit)
	if start > 0 {
		rows = append(rows, mutedStyle.Render("↑ more"))
	}
	for position := start; position < end; position++ {
		index := visible[position]
		option := m.options[index]
		marker := "  "
		if position == m.selected {
			marker = selectedStyle.Render("▸ ")
		}
		state := m.optionStatus(option)
		rows = append(rows, marker+fmt.Sprintf("%-20s %s", option.Key, mutedStyle.Render(state)))
	}
	if end < len(visible) {
		rows = append(rows, mutedStyle.Render("↓ more"))
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
	if option.Default != "" {
		rows = append(rows, mutedStyle.Render("Ghostty default: "+option.Default))
	}
	if len(option.Availability) > 0 {
		rows = append(rows, mutedStyle.Render("Available on: "+strings.Join(option.Availability, ", ")))
	}
	if option.Docs != "" {
		rows = append(rows, mutedStyle.Render("Docs: "+option.Docs))
	}
	if source := m.optionSource(option); source != "" {
		rows = append(rows, mutedStyle.Render(source))
	}
	if m.graph != nil {
		rows = append(rows, mutedStyle.Render(fmt.Sprintf("Config graph: %d files · %d effective assignments", len(m.graph.Files), len(m.graph.Assignments))))
		if len(m.graph.Diagnostics) > 0 {
			rows = append(rows, errorStyle.Render(fmt.Sprintf("Diagnostics: %d (press p for details)", len(m.graph.Diagnostics))))
		}
	}
	if m.readOnly {
		rows = append(rows, mutedStyle.Render("Read-only full catalog; typed editors arrive in a later phase."))
	} else if !option.Editable() {
		rows = append(rows, mutedStyle.Render("Read-only in MVP; repeatable values need a dedicated editor."))
	}
	if m.graph != nil {
		if len(m.graph.AssignmentsFor(option.Key)) > 1 {
			rows = append(rows, mutedStyle.Render("Multiple effective assignments; later source wins."))
		}
	} else {
		if len(m.draft.Assignments(option.Key)) > 1 {
			rows = append(rows, mutedStyle.Render("Duplicate assignments detected; edit is disabled."))
		}
		if len(m.draft.Assignments("config-file")) > 0 {
			rows = append(rows, mutedStyle.Render("Included files are not resolved; this is only the selected file."))
		}
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
		return mutedStyle.Render("↑/k ↓/j move · / search · enter/e edit · r reset · u revert · p preview · ? help · q quit")
	}
}

func (m Model) searchBar() string {
	if m.mode != ModeBrowse {
		return ""
	}
	if m.searching {
		return m.search.View()
	}
	return mutedStyle.Render("/ search options")
}

func (m Model) optionRowLimit() int {
	if m.height <= 0 {
		return 12
	}
	return maxInt(5, m.height-12)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
