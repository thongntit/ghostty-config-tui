package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func (m Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m Model) render() string {
	session := "Guided editor · preview before save"
	if m.saveFunc != nil {
		session = "Guided editor · saves are confirmed explicitly"
	}
	header := titleStyle.Render("Ghostty Config TUI") + "\n" + mutedStyle.Render(session)

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
		mutedStyle.Render(m.fileSummary()),
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
		rows = append(rows, marker+fmt.Sprintf("%-24s %s", friendlyOptionName(option.Key), mutedStyle.Render(state)))
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
		selectedStyle.Render(friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		"",
		option.Description,
		"",
		"Config value: " + m.optionStatus(option),
		mutedStyle.Render("Editor: " + m.editorLabel(option)),
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
		draftGraph := m.draftGraph()
		rows = append(rows, mutedStyle.Render(fmt.Sprintf("Config graph: %d files · %d effective assignments", len(draftGraph.Files), len(draftGraph.Assignments))))
		if len(m.graph.Diagnostics) > 0 {
			rows = append(rows, errorStyle.Render(fmt.Sprintf("Diagnostics: %d (press p for details)", len(m.graph.Diagnostics))))
		}
	}
	if m.graph != nil {
		if len(m.optionAssignments(option.Key)) > 1 {
			rows = append(rows, mutedStyle.Render("Multiple assignments; editor targets the effective source."))
		}
	} else {
		if len(m.optionAssignments(option.Key)) > 1 {
			rows = append(rows, mutedStyle.Render("Multiple assignments; editor targets the last one."))
		}
	}
	if m.status != "" {
		rows = append(rows, "", accentStyle.Render(m.status))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderEdit() string {
	option, _ := m.selectedOption()
	if m.repeatableMode {
		return m.renderRepeatableEdit(option)
	}
	if m.multiMode {
		return m.renderMultiEdit(option)
	}
	if m.durationMode {
		return m.renderDurationEdit(option)
	}
	if m.pathMode {
		return m.renderPathEdit(option)
	}
	if m.choiceMode != choiceNone {
		return m.renderChoiceEdit(option)
	}
	rows := []string{
		accentStyle.Render("Edit value"),
		selectedStyle.Render(friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		mutedStyle.Render("Expected: " + valueHint(option)),
		"",
		m.input.View(),
	}
	if m.numberMode {
		rows = append(rows, mutedStyle.Render("←/→ adjust by "+numberStepLabel(option)+" · ctrl+r raw numeric input"))
	}
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	shortcut := "enter stage · esc cancel"
	if len(option.Values) > 0 || option.Kind == schema.KindBoolean {
		shortcut = "←/→ choose · enter stage · esc cancel"
	}
	rows = append(rows, "", mutedStyle.Render(shortcut))
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderRepeatableEdit(option schema.Option) string {
	if m.choiceMode != choiceNone {
		return m.renderChoiceEdit(option)
	}
	if m.pathMode {
		return m.renderPathEdit(option)
	}
	if m.repeatableForm != repeatableFormNone {
		return m.renderRepeatableForm(option)
	}
	rows := []string{
		accentStyle.Render("Edit " + friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
	}
	if m.repeatableEditing {
		rows = append(rows,
			selectedStyle.Render("Value"),
			m.input.View(),
			mutedStyle.Render("Enter a complete one-line Ghostty value."),
		)
		if m.editError != nil {
			rows = append(rows, errorStyle.Render(m.editError.Error()))
		}
		rows = append(rows, mutedStyle.Render("enter update · esc back to list"))
		return panelStyle.Render(strings.Join(rows, "\n"))
	}
	rows = append(rows, mutedStyle.Render(fmt.Sprintf("%d value(s)", len(m.repeatableItems))))
	if len(m.repeatableItems) == 0 {
		rows = append(rows, mutedStyle.Render("No values. Press a to add one."))
	} else {
		limit := m.repeatableRowLimit()
		start := maxInt(0, m.repeatableIndex-limit/2)
		if start+limit > len(m.repeatableItems) {
			start = maxInt(0, len(m.repeatableItems)-limit)
		}
		end := minInt(len(m.repeatableItems), start+limit)
		if start > 0 {
			rows = append(rows, mutedStyle.Render("↑ more"))
		}
		for index := start; index < end; index++ {
			item := m.repeatableItems[index]
			marker := "  "
			if index == m.repeatableIndex {
				marker = selectedStyle.Render("▸ ")
			}
			location := "new"
			if item.Existing {
				location = fmt.Sprintf("%s:%d", filepath.Base(item.Path), item.Line)
			}
			rows = append(rows, marker+fmt.Sprintf("%-30s %s", item.Value, mutedStyle.Render(location)))
		}
		if end < len(m.repeatableItems) {
			rows = append(rows, mutedStyle.Render("↓ more"))
		}
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderChoiceEdit(option schema.Option) string {
	title := "Choose " + friendlyOptionName(option.Key)
	switch m.choiceMode {
	case choiceFont:
		title = "Choose font family"
	case choiceAction:
		title = "Choose action"
	case choiceEnum:
		title = "Choose " + friendlyOptionName(option.Key)
	}
	rows := []string{
		accentStyle.Render(title),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		"Current: " + m.input.Value(),
	}
	if m.choiceMode == choiceColor {
		rows[len(rows)-1] += "  " + colorSwatch(m.input.Value())
	}
	rows = append(rows, "", m.choiceQuery.View(), "")
	if len(m.choiceMatch) == 0 {
		rows = append(rows, mutedStyle.Render("No matching values"))
	} else {
		limit := m.choiceRowLimit()
		start := maxInt(0, m.choiceIndex-limit/2)
		if start+limit > len(m.choiceMatch) {
			start = maxInt(0, len(m.choiceMatch)-limit)
		}
		end := minInt(len(m.choiceMatch), start+limit)
		if start > 0 {
			rows = append(rows, mutedStyle.Render("↑ more"))
		}
		for position := start; position < end; position++ {
			index := m.choiceMatch[position]
			marker := "  "
			if position == m.choiceIndex {
				marker = selectedStyle.Render("▸ ")
			}
			name, value := m.choiceAt(index)
			label := name
			if m.choiceMode == choiceColor {
				label = fmt.Sprintf("%-24s %s %s", name, value, colorSwatch(value))
			}
			rows = append(rows, marker+label)
		}
		if end < len(m.choiceMatch) {
			rows = append(rows, mutedStyle.Render("↓ more"))
		}
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func colorSwatch(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 6 {
		value = "#" + value
	}
	if len(value) != 7 || value[0] != '#' {
		return mutedStyle.Render("[no swatch]")
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(value)).Render("  ")
}

func (m Model) renderPreview() string {
	header := strings.Join([]string{
		accentStyle.Render("Preview"),
		selectedStyle.Render("DRY RUN — no file will be written"),
	}, "\n")
	return panelStyle.Render(header + "\n\n" + m.preview.View())
}

func (m Model) renderConfirmQuit() string {
	message := "Save staged changes before quitting?\n\n" + selectedStyle.Render("y") + " save and quit    " + selectedStyle.Render("d") + " discard and quit    " + mutedStyle.Render("esc back")
	return panelStyle.Render(message)
}

func (m Model) footer() string {
	if m.help {
		return mutedStyle.Render("↑/k ↓/j move · enter/e edit · r reset · u revert · p preview · ctrl+s save · ? close help · q quit")
	}
	switch m.mode {
	case ModeEdit:
		if m.repeatableMode {
			if m.repeatableForm != repeatableFormNone {
				return mutedStyle.Render("tab/enter next field · ctrl+r raw value · esc back to list")
			}
			if m.pathMode {
				return mutedStyle.Render("↑/↓ browse · enter open/select · s select folder · ctrl+r raw path · esc back")
			}
			if m.choiceMode != choiceNone {
				return mutedStyle.Render("↑/↓ choose · type to filter · enter select · ctrl+r raw · esc back to list")
			}
			if m.repeatableEditing {
				return mutedStyle.Render("enter update · esc back to list")
			}
			return mutedStyle.Render("↑/k ↓/j select · a add · e edit · d delete · r clear · enter stage · esc cancel")
		}
		if m.choiceMode != choiceNone {
			return mutedStyle.Render("↑/↓ choose · type to filter · enter select · ctrl+r raw · esc cancel")
		}
		if m.multiMode {
			return mutedStyle.Render("↑/↓ select item · ←/→/space cycle · ctrl+r raw list · enter stage · esc cancel")
		}
		if m.durationMode {
			return mutedStyle.Render("←/→ unit · ↑/↓ amount · ctrl+r raw duration · enter stage · esc cancel")
		}
		if m.pathMode {
			return mutedStyle.Render("↑/↓ browse · enter open/select · s select folder · ctrl+r raw path · esc cancel")
		}
		if m.numberMode {
			option, ok := m.selectedOption()
			if ok {
				return mutedStyle.Render("←/→ adjust by " + numberStepLabel(option) + " · ctrl+r raw input · enter stage · esc cancel")
			}
			return mutedStyle.Render("←/→ adjust · ctrl+r raw input · enter stage · esc cancel")
		}
		option, ok := m.selectedOption()
		if ok && (len(option.Values) > 0 || option.Kind == schema.KindBoolean) {
			return mutedStyle.Render("←/→ choose · enter stage · esc cancel")
		}
		return mutedStyle.Render("enter stage · esc cancel")
	case ModePreview:
		return mutedStyle.Render("↑/k ↓/j scroll · pgup/pgdn page · esc back · q quit")
	case ModeConfirmQuit:
		return mutedStyle.Render("y save and quit · d discard and quit · esc back")
	default:
		return mutedStyle.Render("↑/k ↓/j move · / search · enter/e edit · r reset · u revert · p preview · ctrl+s save · ? help · q quit")
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
	return maxInt(5, m.height-14)
}

func (m Model) choiceRowLimit() int {
	if m.height <= 0 {
		return 6
	}
	return maxInt(3, minInt(8, m.height-21))
}

func (m Model) repeatableRowLimit() int {
	if m.height <= 0 {
		return 8
	}
	return maxInt(3, minInt(10, m.height-13))
}

func (m Model) fileSummary() string {
	if m.saveFunc == nil {
		return "Preview only; changes stay in memory until this host provides saving"
	}
	if m.HasChanges() {
		return fmt.Sprintf("%d file(s) changed in memory; ctrl+s or y confirms the write", len(m.changedFileSnapshots()))
	}
	return "No staged changes"
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
