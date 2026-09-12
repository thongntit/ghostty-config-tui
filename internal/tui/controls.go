package tui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

const (
	multiDefault uint8 = iota
	multiEnabled
	multiDisabled
)

type pathEntry struct {
	Name  string
	Path  string
	IsDir bool
}

func (m *Model) clearControlState() {
	m.clearChoiceEdit()
	m.durationMode = false
	m.durationOriginal = ""
	m.durationUnit = 0
	m.multiMode = false
	m.multiChoices = nil
	m.multiStates = nil
	m.multiIndex = -1
	m.pathMode = false
	m.pathRepeatable = false
	m.pathDirectory = false
	m.pathOriginal = ""
	m.pathDir = ""
	m.pathEntries = nil
	m.pathIndex = -1
	m.repeatableChoice = false
	m.repeatableForm = repeatableFormNone
	m.formField = 0
	m.formLeft = ""
	m.formRight = ""
	m.formDescription = ""
	m.formAction = ""
	m.formTrigger = ""
	m.formArgs = ""
	m.pairColorChoice = false
	m.formActionChoice = false
}

func defaultActionChoices() []string {
	return []string{
		"ignore", "unbind", "copy_to_clipboard", "paste_from_clipboard", "paste_from_selection",
		"new_window", "new_tab", "close_surface", "close_window", "quit", "reload_config",
		"toggle_fullscreen", "toggle_maximize", "toggle_window_decorations", "toggle_mouse_reporting",
		"toggle_split_zoom", "new_split", "goto_split", "goto_window", "move_tab", "close_tab",
		"increase_font_size", "decrease_font_size", "reset_font_size", "reset", "undo", "redo",
		"scroll_to_bottom", "scroll_page_up", "scroll_page_down", "start_search", "end_search",
		"select_all", "open_config", "prompt_tab_title", "prompt_surface_title", "prompt_window_title",
	}
}

// FallbackActionChoices is used when the host cannot invoke Ghostty's action
// inventory. It is intentionally smaller than the provider result and still
// leaves ctrl+r available for version-specific actions.
func FallbackActionChoices() []string {
	return append([]string(nil), defaultActionChoices()...)
}

func durationUnits() []string {
	return []string{"ns", "us", "ms", "s", "m", "h", "d", "w", "y"}
}

func (m *Model) beginMultiEdit(option schema.Option, current string) tea.Cmd {
	m.multiMode = true
	m.multiChoices = append([]string(nil), option.Values...)
	m.multiStates = make([]uint8, len(m.multiChoices))
	m.multiIndex = 0
	if len(m.multiChoices) == 0 {
		m.status = "This list has no reviewed choices; use ctrl+r for raw input"
		return m.input.Focus()
	}
	trimmed := strings.TrimSpace(current)
	if trimmed == "true" {
		for index := range m.multiStates {
			m.multiStates[index] = multiEnabled
		}
	} else if trimmed == "false" {
		for index := range m.multiStates {
			m.multiStates[index] = multiDisabled
		}
	} else {
		for _, raw := range strings.Split(trimmed, ",") {
			candidate := strings.TrimSpace(raw)
			if candidate == "" {
				continue
			}
			state := multiEnabled
			base := candidate
			if strings.HasPrefix(candidate, "no-") {
				state = multiDisabled
				base = strings.TrimPrefix(candidate, "no-")
			}
			for index, choice := range m.multiChoices {
				if choice == base {
					m.multiStates[index] = state
					break
				}
			}
		}
	}
	m.input.Blur()
	m.editError = nil
	return nil
}

func (m Model) serializeMulti() string {
	values := make([]string, 0, len(m.multiChoices))
	for index, choice := range m.multiChoices {
		if index >= len(m.multiStates) {
			continue
		}
		switch m.multiStates[index] {
		case multiEnabled:
			values = append(values, choice)
		case multiDisabled:
			values = append(values, "no-"+choice)
		}
	}
	return strings.Join(values, ",")
}

func (m Model) multiStateLabel(state uint8) string {
	switch state {
	case multiEnabled:
		return "enabled"
	case multiDisabled:
		return "disabled"
	default:
		return "default"
	}
}

func (m *Model) updateMultiEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c":
		return *m, m.requestQuit()
	case "enter":
		return *m, m.commitMultiEdit()
	case "up", "k":
		m.moveMulti(-1)
	case "down", "j":
		m.moveMulti(1)
	case "left", "h":
		m.cycleMulti(-1)
	case "right", "l", " ":
		m.cycleMulti(1)
	case "ctrl+r":
		m.multiMode = false
		m.input.SetValue(m.currentMultiRaw())
		m.status = "Raw list input enabled"
		return *m, m.input.Focus()
	}
	return *m, nil
}

func (m *Model) moveMulti(delta int) {
	if len(m.multiChoices) == 0 {
		return
	}
	if m.multiIndex < 0 {
		m.multiIndex = 0
	} else {
		m.multiIndex = (m.multiIndex + delta + len(m.multiChoices)) % len(m.multiChoices)
	}
}

func (m *Model) cycleMulti(delta int) {
	if m.multiIndex < 0 || m.multiIndex >= len(m.multiStates) {
		return
	}
	state := int(m.multiStates[m.multiIndex]) + delta
	if state < int(multiDefault) {
		state = int(multiDisabled)
	}
	if state > int(multiDisabled) {
		state = int(multiDefault)
	}
	m.multiStates[m.multiIndex] = uint8(state)
	m.editError = nil
}

func (m Model) currentMultiRaw() string {
	option, ok := m.selectedOption()
	if !ok {
		return ""
	}
	value, _ := m.currentValue(option.Key)
	return value
}

func (m *Model) commitMultiEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
		return nil
	}
	value := m.serializeMulti()
	if strings.TrimSpace(value) == strings.TrimSpace(m.currentMultiRaw()) {
		m.completeScalarEdit(option)
		return nil
	}
	if value == "" {
		if err := m.stageScalar(option.Key, ""); err != nil {
			m.editError = err
			return nil
		}
		m.completeScalarEdit(option)
		return nil
	}
	m.input.SetValue(value)
	return m.commitEdit()
}

func (m *Model) completeScalarEdit(option schema.Option) {
	m.input.Blur()
	m.clearControlState()
	m.numberMode = false
	m.mode = ModeBrowse
	m.editError = nil
	if m.HasChanges() {
		m.status = "Staged " + friendlyOptionName(option.Key)
	} else {
		m.status = "No change for " + option.Key
	}
}

func (m *Model) beginDurationEdit(option schema.Option, current string) tea.Cmd {
	if len(m.durationUnits) == 0 {
		m.durationUnits = durationUnits()
	}
	m.durationMode = true
	m.durationOriginal = current
	amount, unit := parseDurationPickerValue(current, option.Default)
	m.durationUnit = unit
	m.input.Prompt = "amount > "
	m.input.Placeholder = "amount"
	m.input.SetValue(amount)
	m.input.CursorEnd()
	m.input.Blur()
	m.editError = nil
	return m.input.Focus()
}

func parseDurationPickerValue(current, fallback string) (string, int) {
	value := strings.TrimSpace(current)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return "0", durationUnitIndex("s")
	}
	value = strings.ReplaceAll(value, " ", "")
	amountEnd := 0
	for amountEnd < len(value) && (value[amountEnd] >= '0' && value[amountEnd] <= '9' || value[amountEnd] == '.') {
		amountEnd++
	}
	if amountEnd == 0 {
		return "0", durationUnitIndex("s")
	}
	amount := value[:amountEnd]
	unit := value[amountEnd:]
	if unit == "" {
		unit = "s"
	}
	if unit == "µs" {
		unit = "us"
	}
	return amount, durationUnitIndex(unit)
}

func durationUnitIndex(unit string) int {
	for index, candidate := range durationUnits() {
		if candidate == unit {
			return index
		}
	}
	return durationUnitIndex("s")
}

func (m *Model) updateDurationEdit(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.durationUnits) == 0 {
		m.durationUnits = durationUnits()
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c":
		return *m, m.requestQuit()
	case "enter":
		return *m, m.commitDurationEdit()
	case "left", "h":
		m.durationUnit = (m.durationUnit - 1 + len(m.durationUnits)) % len(m.durationUnits)
	case "right", "l":
		m.durationUnit = (m.durationUnit + 1) % len(m.durationUnits)
	case "up", "k", "down", "j":
		delta := 1
		if keyMsg.String() == "down" || keyMsg.String() == "j" {
			delta = -1
		}
		m.adjustDurationAmount(delta)
	case "ctrl+r":
		m.durationMode = false
		m.input.SetValue(m.durationOriginal)
		m.status = "Raw duration input enabled"
		return *m, m.input.Focus()
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.editError = m.input.Err
		return *m, cmd
	}
	return *m, nil
}

func (m *Model) adjustDurationAmount(delta int) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(m.input.Value()), 64)
	if err != nil {
		m.editError = fmt.Errorf("type a non-negative number for the duration amount")
		m.input.Err = m.editError
		return
	}
	amount += float64(delta)
	if amount < 0 {
		amount = 0
	}
	m.input.SetValue(strconv.FormatFloat(amount, 'f', -1, 64))
	m.editError = nil
	m.input.Err = nil
}

func (m *Model) commitDurationEdit() tea.Cmd {
	amount, err := strconv.ParseFloat(strings.TrimSpace(m.input.Value()), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		m.editError = fmt.Errorf("duration amount must be a finite non-negative number")
		m.input.Err = m.editError
		return nil
	}
	value := strconv.FormatFloat(amount, 'f', -1, 64) + m.durationUnits[m.durationUnit]
	m.input.SetValue(value)
	return m.commitEdit()
}

func (m *Model) beginPathEdit(option schema.Option, current string, repeatable bool) tea.Cmd {
	m.pathMode = true
	m.pathRepeatable = repeatable
	m.pathDirectory = option.Key == "working-directory"
	m.pathOriginal = current
	m.pathDir = pathPickerDirectory(m.configPath, current)
	m.pathIndex = 0
	m.loadPathEntries()
	m.input.Blur()
	m.editError = nil
	return nil
}

func pathPickerDirectory(configPath, current string) string {
	base := configPath
	if base == "" {
		base = "."
	}
	base, err := filepath.Abs(base)
	if err != nil {
		base = "."
	}
	base = filepath.Dir(base)
	candidate := expandUserPath(strings.TrimSpace(current))
	if candidate == "" {
		return base
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return base
	}
	if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
		return candidate
	}
	return filepath.Dir(candidate)
}

func expandUserPath(value string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if value == "~" {
			return home
		}
		if strings.HasPrefix(value, "~/") {
			return filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	return os.ExpandEnv(value)
}

func (m *Model) loadPathEntries() {
	directory := m.pathDir
	entries := make([]pathEntry, 0)
	parent := filepath.Dir(directory)
	if parent != directory {
		entries = append(entries, pathEntry{Name: "..", Path: parent, IsDir: true})
	}
	read, err := os.ReadDir(directory)
	if err != nil {
		m.pathEntries = entries
		m.pathIndex = 0
		m.editError = fmt.Errorf("read %s: %w", directory, err)
		return
	}
	directories := make([]pathEntry, 0)
	files := make([]pathEntry, 0)
	for _, entry := range read {
		path := filepath.Join(directory, entry.Name())
		item := pathEntry{Name: entry.Name(), Path: path, IsDir: entry.IsDir()}
		if item.IsDir {
			directories = append(directories, item)
		} else {
			files = append(files, item)
		}
	}
	sort.Slice(directories, func(left, right int) bool {
		return strings.ToLower(directories[left].Name) < strings.ToLower(directories[right].Name)
	})
	sort.Slice(files, func(left, right int) bool {
		return strings.ToLower(files[left].Name) < strings.ToLower(files[right].Name)
	})
	entries = append(entries, directories...)
	if !m.pathDirectory {
		entries = append(entries, files...)
	}
	m.pathEntries = entries
	if m.pathIndex < 0 || m.pathIndex >= len(entries) {
		m.pathIndex = 0
	}
	m.editError = nil
}

func (m *Model) updatePathEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch keyMsg.String() {
	case "esc":
		if m.pathRepeatable {
			m.leavePathToRepeatable()
			return *m, nil
		}
		return *m, m.cancelEdit()
	case "ctrl+c":
		return *m, m.requestQuit()
	case "up", "k":
		m.movePath(-1)
	case "down", "j":
		m.movePath(1)
	case "backspace", "left", "h":
		m.pathDir = filepath.Dir(m.pathDir)
		m.loadPathEntries()
	case "enter":
		return *m, m.selectPathEntry()
	case "s", ".":
		if m.pathDirectory {
			return *m, m.selectPath(m.pathDir)
		}
	case "ctrl+r":
		m.pathMode = false
		m.input.SetValue(m.pathOriginal)
		m.status = "Raw path input enabled"
		if m.pathRepeatable {
			m.repeatableEditing = true
		}
		return *m, m.input.Focus()
	}
	return *m, nil
}

func (m *Model) movePath(delta int) {
	if len(m.pathEntries) == 0 {
		return
	}
	m.pathIndex = (m.pathIndex + delta + len(m.pathEntries)) % len(m.pathEntries)
}

func (m *Model) selectPathEntry() tea.Cmd {
	if m.pathIndex < 0 || m.pathIndex >= len(m.pathEntries) {
		m.status = "No path selected; press ctrl+r for raw input"
		return nil
	}
	entry := m.pathEntries[m.pathIndex]
	if entry.IsDir {
		m.pathDir = entry.Path
		m.loadPathEntries()
		return nil
	}
	return m.selectPath(entry.Path)
}

func (m *Model) selectPath(path string) tea.Cmd {
	value := shortenHomePath(path)
	m.input.SetValue(value)
	m.pathMode = false
	m.pathEntries = nil
	m.pathIndex = -1
	if m.pathRepeatable {
		m.repeatableEditing = false
		return m.commitRepeatableItem()
	}
	return m.commitEdit()
}

func (m *Model) leavePathToRepeatable() {
	m.pathMode = false
	m.pathEntries = nil
	m.pathIndex = -1
	m.repeatableEditing = false
	m.input.Blur()
	m.editError = nil
}

func shortenHomePath(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		cleanHome, homeErr := filepath.Abs(home)
		cleanPath, pathErr := filepath.Abs(path)
		if homeErr == nil && pathErr == nil {
			if cleanPath == cleanHome {
				return "~"
			}
			prefix := cleanHome + string(filepath.Separator)
			if strings.HasPrefix(cleanPath, prefix) {
				return "~/" + strings.TrimPrefix(cleanPath, prefix)
			}
		}
	}
	return path
}

func isFontOption(key string) bool {
	switch key {
	case "font-family", "font-family-bold", "font-family-italic", "font-family-bold-italic", "window-title-font-family":
		return true
	default:
		return false
	}
}

func repeatablePathKey(key string) bool {
	switch key {
	case "config-file", "custom-shader", "gtk-custom-css":
		return true
	default:
		return false
	}
}

func repeatablePairKey(key string) bool {
	switch key {
	case "env", "font-variation", "font-codepoint-map", "clipboard-codepoint-map", "key-remap", "palette":
		return true
	default:
		return false
	}
}

func pairFieldLabels(key string) (string, string) {
	switch key {
	case "env":
		return "Variable", "Value"
	case "font-variation":
		return "Axis (4 chars)", "Value"
	case "font-codepoint-map":
		return "Codepoint / range", "Font family"
	case "clipboard-codepoint-map":
		return "Codepoint / range", "Replacement"
	case "key-remap":
		return "From", "To"
	case "palette":
		return "Index (0–255)", "Color"
	default:
		return "Key", "Value"
	}
}

func (m *Model) beginPairForm(optionKey, current string) tea.Cmd {
	left, right := pairFieldLabels(optionKey)
	m.repeatableForm = repeatableFormPair
	m.formField = 0
	m.formLeft, m.formRight = splitPairValue(current)
	m.formDescription = left + " → " + right
	m.input.SetValue(m.formLeft)
	m.input.Prompt = left + " > "
	m.input.Placeholder = left
	m.input.CursorEnd()
	m.input.Err = nil
	m.editError = nil
	return m.input.Focus()
}

func splitPairValue(value string) (string, string) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func (m Model) serializePair() string {
	return strings.TrimSpace(m.formLeft) + "=" + strings.TrimSpace(m.formRight)
}

func (m *Model) beginKeybindForm(current string) tea.Cmd {
	m.repeatableForm = repeatableFormKeybind
	m.formField = 0
	m.formTrigger, m.formAction, m.formArgs = splitKeybindValue(current)
	m.input.SetValue(m.formTrigger)
	m.input.Prompt = "Trigger > "
	m.input.Placeholder = "e.g. super+shift+p"
	m.input.CursorEnd()
	m.input.Err = nil
	m.editError = nil
	return m.input.Focus()
}

func splitKeybindValue(value string) (trigger, action, args string) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) == 0 {
		return "", "", ""
	}
	trigger = strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return trigger, "", ""
	}
	rhs := strings.TrimSpace(parts[1])
	actionParts := strings.SplitN(rhs, ":", 2)
	action = strings.TrimSpace(actionParts[0])
	if len(actionParts) == 2 {
		args = strings.TrimSpace(actionParts[1])
	}
	return trigger, action, args
}

func (m Model) serializeKeybind() string {
	value := strings.TrimSpace(m.formTrigger) + "=" + strings.TrimSpace(m.formAction)
	if strings.TrimSpace(m.formArgs) != "" {
		value += ":" + strings.TrimSpace(m.formArgs)
	}
	return value
}

func (m *Model) beginCommandPaletteForm(current string) tea.Cmd {
	if strings.TrimSpace(current) == "clear" {
		m.repeatableForm = repeatableFormNone
		m.repeatableEditing = true
		m.input.SetValue(current)
		m.input.Prompt = "command-palette-entry = "
		m.input.Placeholder = "clear or title/action fields"
		return m.input.Focus()
	}
	fields := parseCommandPalette(current)
	m.repeatableForm = repeatableFormCommandPalette
	m.formField = 0
	m.formLeft = fields["title"]
	m.formDescription = fields["description"]
	m.formAction = fields["action"]
	m.activateFormField()
	m.editError = nil
	return m.input.Focus()
}

func splitConfigFields(value string) []string {
	fields := make([]string, 0)
	start := 0
	quoted := false
	escaped := false
	for index, character := range value {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && quoted {
			escaped = true
			continue
		}
		if character == '"' {
			quoted = !quoted
			continue
		}
		if character == ',' && !quoted {
			fields = append(fields, strings.TrimSpace(value[start:index]))
			start = index + 1
		}
	}
	fields = append(fields, strings.TrimSpace(value[start:]))
	return fields
}

func parseCommandPalette(value string) map[string]string {
	fields := map[string]string{}
	for _, field := range splitConfigFields(value) {
		parts := strings.SplitN(field, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		item := strings.TrimSpace(parts[1])
		if len(item) >= 2 && item[0] == '"' && item[len(item)-1] == '"' {
			if unquoted, err := strconv.Unquote(item); err == nil {
				item = unquoted
			}
		}
		fields[key] = item
	}
	return fields
}

func (m Model) serializeCommandPalette() string {
	parts := []string{"title:" + strconv.Quote(m.formLeft)}
	if strings.TrimSpace(m.formDescription) != "" {
		parts = append(parts, "description:"+strconv.Quote(m.formDescription))
	}
	parts = append(parts, "action:"+strconv.Quote(m.formAction))
	return strings.Join(parts, ",")
}

func (m *Model) activeFormValue() string {
	switch m.repeatableForm {
	case repeatableFormPair:
		if m.formField == 0 {
			return m.formLeft
		}
		return m.formRight
	case repeatableFormKeybind:
		switch m.formField {
		case 0:
			return m.formTrigger
		case 1:
			return m.formAction
		default:
			return m.formArgs
		}
	case repeatableFormCommandPalette:
		switch m.formField {
		case 0:
			return m.formLeft
		case 1:
			return m.formDescription
		default:
			return m.formAction
		}
	default:
		return ""
	}
}

func (m *Model) activeFormLabel() string {
	switch m.repeatableForm {
	case repeatableFormPair:
		left, right := pairFieldLabels(m.currentRepeatableKey())
		if m.formField == 0 {
			return left
		}
		return right
	case repeatableFormKeybind:
		switch m.formField {
		case 0:
			return "Trigger"
		case 1:
			return "Action"
		default:
			return "Arguments"
		}
	case repeatableFormCommandPalette:
		switch m.formField {
		case 0:
			return "Title"
		case 1:
			return "Description"
		default:
			return "Action"
		}
	default:
		return "Value"
	}
}

func (m Model) currentRepeatableKey() string {
	option, ok := m.selectedOption()
	if !ok {
		return ""
	}
	return option.Key
}

func (m *Model) syncActiveFormField() {
	value := m.input.Value()
	switch m.repeatableForm {
	case repeatableFormPair:
		if m.formField == 0 {
			m.formLeft = value
		} else {
			m.formRight = value
		}
	case repeatableFormKeybind:
		switch m.formField {
		case 0:
			m.formTrigger = value
		case 1:
			m.formAction = value
		default:
			m.formArgs = value
		}
	case repeatableFormCommandPalette:
		switch m.formField {
		case 0:
			m.formLeft = value
		case 1:
			m.formDescription = value
		default:
			m.formAction = value
		}
	}
}

func (m *Model) activateFormField() tea.Cmd {
	m.input.SetValue(m.activeFormValue())
	m.input.Prompt = m.activeFormLabel() + " > "
	m.input.Placeholder = m.activeFormLabel()
	m.input.CursorEnd()
	m.input.Err = nil
	return m.input.Focus()
}

func (m *Model) updateRepeatableForm(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return *m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.repeatableForm = repeatableFormNone
		m.input.Blur()
		m.editError = nil
		return *m, nil
	case "ctrl+r":
		m.syncActiveFormField()
		rawValue := m.serializeActiveRepeatable()
		m.repeatableForm = repeatableFormNone
		m.repeatableEditing = true
		m.input.SetValue(rawValue)
		m.input.Prompt = m.currentRepeatableKey() + " = "
		m.input.Placeholder = "raw Ghostty value"
		m.status = "Raw value input enabled"
		return *m, m.input.Focus()
	case "tab":
		return *m, m.advanceFormField()
	case "shift+tab":
		m.syncActiveFormField()
		m.formField--
		if m.formField < 0 {
			m.formField = m.formFieldCount() - 1
		}
		return *m, m.activateFormField()
	case "enter":
		return *m, m.advanceFormField()
	case "c":
		if m.repeatableForm == repeatableFormPair && m.formField == 1 && m.currentRepeatableKey() == "palette" {
			m.syncActiveFormField()
			m.pairColorChoice = true
			return *m, m.beginChoiceEdit(choiceColor, m.formRight)
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.syncActiveFormField()
	m.editError = m.input.Err
	return *m, cmd
}

func (m Model) formFieldCount() int {
	if m.repeatableForm == repeatableFormPair {
		return 2
	}
	if m.repeatableForm == repeatableFormKeybind || m.repeatableForm == repeatableFormCommandPalette {
		return 3
	}
	return 1
}

func (m Model) serializeActiveRepeatable() string {
	switch m.repeatableForm {
	case repeatableFormPair:
		return m.serializePair()
	case repeatableFormKeybind:
		return m.serializeKeybind()
	case repeatableFormCommandPalette:
		return m.serializeCommandPalette()
	default:
		return m.input.Value()
	}
}

func (m *Model) advanceFormField() tea.Cmd {
	m.syncActiveFormField()
	switch m.repeatableForm {
	case repeatableFormPair:
		if m.formField == 0 {
			m.formField = 1
			return m.activateFormField()
		}
		m.input.SetValue(m.serializePair())
		return m.commitRepeatableItem()
	case repeatableFormKeybind:
		if m.formField == 0 {
			m.formField = 1
			return m.beginFormActionChoice()
		}
		if m.formField == 2 {
			m.input.SetValue(m.serializeKeybind())
			return m.commitRepeatableItem()
		}
		if m.formField == 1 {
			m.formField = 2
			return m.activateFormField()
		}
	case repeatableFormCommandPalette:
		if m.formField == 0 {
			m.formField = 1
			return m.activateFormField()
		}
		if m.formField == 1 {
			m.formField = 2
			return m.beginFormActionChoice()
		}
		m.input.SetValue(m.serializeCommandPalette())
		return m.commitRepeatableItem()
	}
	return nil
}

func (m *Model) beginFormActionChoice() tea.Cmd {
	m.formActionChoice = true
	m.input.SetValue(m.formAction)
	m.choiceValues = nil
	return m.beginChoiceEdit(choiceAction, m.formAction)
}

func (m *Model) setActiveFormAction(value string) tea.Cmd {
	m.formAction = value
	m.formActionChoice = false
	m.clearChoiceEdit()
	switch m.repeatableForm {
	case repeatableFormKeybind:
		m.formField = 2
		return m.activateFormField()
	case repeatableFormCommandPalette:
		m.formField = 2
		m.input.SetValue(m.serializeCommandPalette())
		return m.commitRepeatableItem()
	default:
		return nil
	}
}

func (m *Model) leaveActionChoiceRaw() tea.Cmd {
	m.formActionChoice = false
	m.clearChoiceEdit()
	if m.repeatableForm == repeatableFormKeybind {
		m.formField = 1
	} else {
		m.formField = 2
	}
	return m.activateFormField()
}

func (m *Model) validateRepeatableValue(option schema.Option, value string) error {
	if err := option.Validate(value); err != nil {
		return err
	}
	if option.Key == "keybind" {
		trigger, action, _ := splitKeybindValue(value)
		if trigger == "" || action == "" {
			return fmt.Errorf("keybind needs a trigger and an action")
		}
	}
	if repeatablePairKey(option.Key) {
		left, right := splitPairValue(value)
		if left == "" || right == "" {
			return fmt.Errorf("%s needs both sides of the pair", friendlyOptionName(option.Key))
		}
		if option.Key == "palette" {
			index, err := strconv.ParseInt(left, 0, 64)
			if err != nil || index < 0 || index > 255 {
				return fmt.Errorf("palette index must be an integer from 0 to 255")
			}
			if err := (schema.Option{Kind: schema.KindColor}).Validate(right); err != nil {
				return fmt.Errorf("palette color: %w", err)
			}
		}
	}
	if option.Key == "command-palette-entry" && value != "clear" {
		fields := parseCommandPalette(value)
		if strings.TrimSpace(fields["title"]) == "" || strings.TrimSpace(fields["action"]) == "" {
			return fmt.Errorf("command palette entry needs a title and an action")
		}
	}
	return nil
}

func (m Model) renderMultiEdit(option schema.Option) string {
	rows := []string{
		accentStyle.Render("Choose " + friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		"Cycle: default → enabled → disabled",
	}
	limit := m.choiceRowLimit() + 2
	start := maxInt(0, m.multiIndex-limit/2)
	if start+limit > len(m.multiChoices) {
		start = maxInt(0, len(m.multiChoices)-limit)
	}
	end := minInt(len(m.multiChoices), start+limit)
	for index := start; index < end; index++ {
		marker := "  "
		if index == m.multiIndex {
			marker = selectedStyle.Render("▸ ")
		}
		rows = append(rows, marker+fmt.Sprintf("%-22s %s", m.multiChoices[index], mutedStyle.Render(m.multiStateLabel(m.multiStates[index]))))
	}
	if len(m.multiChoices) == 0 {
		rows = append(rows, mutedStyle.Render("No reviewed choices; press ctrl+r for raw input"))
	}
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderDurationEdit(option schema.Option) string {
	unit := "s"
	if m.durationUnit >= 0 && m.durationUnit < len(m.durationUnits) {
		unit = m.durationUnits[m.durationUnit]
	}
	rows := []string{
		accentStyle.Render("Choose " + friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		selectedStyle.Render("Amount: ") + m.input.View(),
		"Unit: " + selectedStyle.Render(unit),
		mutedStyle.Render("←/→ change unit · ↑/↓ adjust amount"),
	}
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderPathEdit(option schema.Option) string {
	rows := []string{
		accentStyle.Render("Choose " + friendlyOptionName(option.Key)),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		"Folder: " + shortenHomePath(m.pathDir),
		"Current: " + m.input.Value(),
	}
	limit := m.choiceRowLimit() + 1
	start := maxInt(0, m.pathIndex-limit/2)
	if start+limit > len(m.pathEntries) {
		start = maxInt(0, len(m.pathEntries)-limit)
	}
	end := minInt(len(m.pathEntries), start+limit)
	for index := start; index < end; index++ {
		entry := m.pathEntries[index]
		marker := "  "
		if index == m.pathIndex {
			marker = selectedStyle.Render("▸ ")
		}
		name := entry.Name
		if entry.IsDir {
			name += "/"
		}
		rows = append(rows, marker+name)
	}
	if len(m.pathEntries) == 0 {
		rows = append(rows, mutedStyle.Render("Directory cannot be read; press ctrl+r for raw input"))
	}
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func (m Model) renderRepeatableForm(option schema.Option) string {
	title := "Edit " + friendlyOptionName(option.Key)
	rows := []string{
		accentStyle.Render(title),
		mutedStyle.Render(option.Key),
		mutedStyle.Render(option.Description),
		"",
		mutedStyle.Render("Structured form · raw fallback: ctrl+r"),
	}
	switch m.repeatableForm {
	case repeatableFormPair:
		left, right := pairFieldLabels(option.Key)
		rows = append(rows, formRow("1 "+left, m.formLeft, m.formField == 0), formRow("2 "+right, m.formRight, m.formField == 1))
	case repeatableFormKeybind:
		rows = append(rows,
			formRow("1 Trigger", m.formTrigger, m.formField == 0),
			formRow("2 Action", m.formAction, m.formField == 1),
			formRow("3 Arguments", m.formArgs, m.formField == 2),
		)
	case repeatableFormCommandPalette:
		rows = append(rows,
			formRow("1 Title", m.formLeft, m.formField == 0),
			formRow("2 Description", m.formDescription, m.formField == 1),
			formRow("3 Action", m.formAction, m.formField == 2),
		)
	}
	rows = append(rows, "", m.input.View())
	if m.editError != nil {
		rows = append(rows, "", errorStyle.Render(m.editError.Error()))
	}
	return panelStyle.Render(strings.Join(rows, "\n"))
}

func formRow(label, value string, active bool) string {
	marker := "  "
	if active {
		marker = selectedStyle.Render("▸ ")
	}
	if value == "" {
		value = mutedStyle.Render("(empty)")
	}
	return marker + fmt.Sprintf("%-18s %s", label+":", value)
}

func (m *Model) finishRepeatableFormCancel() {
	m.repeatableForm = repeatableFormNone
	m.formActionChoice = false
	m.pairColorChoice = false
	m.input.Blur()
	m.editError = nil
}
