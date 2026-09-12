package tui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/configgraph"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

// Mode is the interaction state of the editor.
type Mode uint8

const (
	ModeBrowse Mode = iota
	ModeEdit
	ModePreview
	ModeConfirmQuit
)

type choiceMode uint8

const (
	choiceNone choiceMode = iota
	choiceTheme
	choiceColor
)

// ColorChoice is a named Ghostty/X11 color and its canonical RGB value.
// Providers may populate this from `ghostty +list-colors`; the TUI also has a
// small built-in fallback palette for machines without Ghostty installed.
type ColorChoice struct {
	Name  string
	Value string
}

// Model is a fixture-safe config editor. It keeps original and draft
// documents separate and never writes the selected file.
type Model struct {
	options    []schema.Option
	selected   int
	width      int
	height     int
	configPath string
	readOnly   bool
	graph      *configgraph.Graph
	visible    []int
	search     textinput.Model
	searching  bool

	themeChoices []string
	colorChoices []ColorChoice
	choiceQuery  textinput.Model
	choiceMode   choiceMode
	choiceMatch  []int
	choiceIndex  int
	numberMode   bool

	original *configdoc.Document
	draft    *configdoc.Document
	desired  map[string]string
	changes  map[string]configdoc.Change

	mode       Mode
	returnMode Mode
	input      textinput.Model
	preview    viewport.Model
	editError  error
	status     string
	help       bool
}

func NewModel(options []schema.Option, configPath string, original configdoc.Document) Model {
	originalCopy := original.Clone()
	input := textinput.New()
	input.CharLimit = 1024
	preview := viewport.New(viewport.WithHeight(12), viewport.WithWidth(72))
	search := textinput.New()
	search.Prompt = "/ "
	search.CharLimit = 256
	choiceQuery := textinput.New()
	choiceQuery.Prompt = "filter > "
	choiceQuery.CharLimit = 256
	visible := make([]int, len(options))
	for index := range options {
		visible[index] = index
	}
	return Model{
		options:      append([]schema.Option(nil), options...),
		configPath:   configPath,
		visible:      visible,
		original:     originalCopy,
		draft:        originalCopy.Clone(),
		desired:      make(map[string]string),
		changes:      make(map[string]configdoc.Change),
		mode:         ModeBrowse,
		input:        input,
		preview:      preview,
		search:       search,
		colorChoices: defaultColorChoices(),
		choiceQuery:  choiceQuery,
		choiceIndex:  -1,
	}
}

// NewGraphModel creates the read-only full-catalog view. The graph is kept
// separate from the selected source document so the UI can explain effective
// values without flattening or rewriting files.
func NewGraphModel(options []schema.Option, configPath string, original configdoc.Document, graph configgraph.Graph) Model {
	model := NewModel(options, configPath, original)
	model.graph = &graph
	model.readOnly = true
	return model
}

// SetReadOnly controls whether the current model exposes editing actions.
// Full graph mode uses this until graph-aware draft targets are implemented.
func (m *Model) SetReadOnly(readOnly bool) {
	m.readOnly = readOnly
}

// SetStatus sets startup or interaction feedback shown in the details panel.
func (m *Model) SetStatus(status string) {
	m.status = status
}

// SetThemeChoices installs an authoritative theme inventory. An empty list
// intentionally leaves the chooser unavailable so custom/raw theme values can
// still be entered through the explicit raw fallback.
func (m *Model) SetThemeChoices(choices []string) {
	m.themeChoices = append([]string(nil), choices...)
}

// SetColorChoices installs an authoritative named-color inventory. The
// built-in palette remains available if a provider cannot be queried.
func (m *Model) SetColorChoices(choices []ColorChoice) {
	if len(choices) == 0 {
		return
	}
	m.colorChoices = append([]ColorChoice(nil), choices...)
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
		m.resize()
		return m, nil
	}

	switch m.mode {
	case ModeBrowse:
		return m.updateBrowse(msg)
	case ModeEdit:
		return m.updateEdit(msg)
	case ModePreview:
		return m.updatePreview(msg)
	case ModeConfirmQuit:
		return m.updateConfirmQuit(msg)
	default:
		return m, nil
	}
}

func (m *Model) resize() {
	if m.width > 0 {
		m.input.SetWidth(maxInt(24, m.width-12))
		m.search.SetWidth(maxInt(24, m.width-12))
		m.choiceQuery.SetWidth(maxInt(24, m.width-12))
		m.preview.SetWidth(maxInt(40, m.width-6))
	}
	if m.height > 0 {
		m.preview.SetHeight(maxInt(6, m.height-10))
	}
}

func (m Model) updateBrowse(msg tea.Msg) (Model, tea.Cmd) {
	if m.searching {
		return m.updateSearch(msg)
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, defaultKeyMap.Quit) {
		return m, m.requestQuit()
	}
	if keyMsg.String() == "/" {
		m.searching = true
		m.search.CursorEnd()
		return m, m.search.Focus()
	}

	switch {
	case key.Matches(keyMsg, defaultKeyMap.Up):
		if m.selected > 0 {
			m.selected--
		}
	case key.Matches(keyMsg, defaultKeyMap.Down):
		if m.selected < len(m.visibleOptionIndexes())-1 {
			m.selected++
		}
	case key.Matches(keyMsg, defaultKeyMap.Enter), key.Matches(keyMsg, defaultKeyMap.Edit):
		return m, m.beginEdit()
	case key.Matches(keyMsg, defaultKeyMap.Reset):
		m.resetSelected()
	case key.Matches(keyMsg, defaultKeyMap.Undo):
		m.revertSelected()
	case key.Matches(keyMsg, defaultKeyMap.Preview):
		m.openPreview()
	case key.Matches(keyMsg, defaultKeyMap.Help):
		m.help = !m.help
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.Msg) (Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.search.Blur()
			m.searching = false
			return m, nil
		case "enter":
			m.search.Blur()
			m.searching = false
			m.applySearch(m.search.Value())
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	m.applySearch(m.search.Value())
	return m, cmd
}

func (m *Model) applySearch(query string) {
	previousKey := m.SelectedKey()
	query = strings.TrimSpace(query)
	m.visible = m.visible[:0]
	for index, option := range m.options {
		if query == "" || containsFold(option.Key, query) || containsFold(option.Category, query) || containsFold(option.Description, query) {
			m.visible = append(m.visible, index)
		}
	}
	m.selected = 0
	for position, index := range m.visible {
		if m.options[index].Key == previousKey {
			m.selected = position
			break
		}
	}
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

func (m Model) updateEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, keyPressed := msg.(tea.KeyPressMsg)
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Escape) {
		return m, m.cancelEdit()
	}
	if m.choiceMode != choiceNone {
		return m.updateChoiceEdit(msg)
	}
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Enter) {
		return m, m.commitEdit()
	}
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Quit) && keyMsg.String() == "ctrl+c" {
		return m, m.requestQuit()
	}
	if keyPressed && keyMsg.String() == "ctrl+r" && m.numberMode {
		m.numberMode = false
		m.status = "Raw numeric input enabled"
		return m, m.input.Focus()
	}

	if keyPressed {
		if _, ok := m.selectedOption(); ok && m.numberMode && (keyMsg.String() == "left" || keyMsg.String() == "right") {
			delta := -1
			if keyMsg.String() == "right" {
				delta = 1
			}
			m.adjustNumber(delta)
			return m, nil
		}
		if option, ok := m.selectedOption(); ok && (len(option.Values) > 0 || option.Kind == schema.KindBoolean) {
			switch {
			case key.Matches(keyMsg, defaultKeyMap.Left):
				m.cycleInput(option, -1)
				return m, nil
			case key.Matches(keyMsg, defaultKeyMap.Right):
				m.cycleInput(option, 1)
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.editError = m.input.Err
	return m, cmd
}

func (m Model) updatePreview(msg tea.Msg) (Model, tea.Cmd) {
	switch msgString(msg) {
	case "esc":
		m.mode = ModeBrowse
		return m, nil
	case "q", "ctrl+c":
		return m, m.requestQuit()
	}

	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmQuit(msg tea.Msg) (Model, tea.Cmd) {
	switch msgString(msg) {
	case "y", "enter":
		return m, tea.Quit
	case "n", "esc":
		m.mode = m.returnMode
	}
	return m, nil
}

func (m *Model) requestQuit() tea.Cmd {
	if m.HasChanges() {
		m.returnMode = m.mode
		m.mode = ModeConfirmQuit
		return nil
	}
	return tea.Quit
}

func (m Model) selectedOption() (schema.Option, bool) {
	visible := m.visibleOptionIndexes()
	if m.selected < 0 || m.selected >= len(visible) {
		return schema.Option{}, false
	}
	return m.options[visible[m.selected]], true
}

func (m Model) selectedOptionIndex() (int, bool) {
	visible := m.visibleOptionIndexes()
	if m.selected < 0 || m.selected >= len(visible) {
		return 0, false
	}
	return visible[m.selected], true
}

func (m Model) visibleOptionIndexes() []int {
	if m.visible != nil {
		return m.visible
	}
	visible := make([]int, len(m.options))
	for index := range m.options {
		visible[index] = index
	}
	return visible
}

func (m *Model) beginEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
		return nil
	}
	if m.readOnly {
		m.status = "Full catalog view is read-only until an edit target is selected"
		return nil
	}
	if !option.Editable() {
		m.status = fmt.Sprintf("%s is read-only in the MVP", option.Key)
		return nil
	}
	if assignments := m.draft.Assignments(option.Key); len(assignments) > 1 {
		m.status = fmt.Sprintf("%s has duplicate assignments and is read-only", option.Key)
		return nil
	}

	value, _ := m.draft.Lookup(option.Key)
	m.input = textinput.New()
	m.input.Prompt = option.Key + " = "
	m.input.Placeholder = valueHint(option)
	m.input.CharLimit = 1024
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.resize()
	m.editError = nil
	m.numberMode = option.Key == "font-size" && option.Kind == schema.KindNumber
	m.choiceMode = choiceNone
	m.choiceMatch = nil
	m.choiceIndex = -1
	m.mode = ModeEdit
	if option.Key == "theme" && len(m.themeChoices) > 0 {
		return m.beginChoiceEdit(choiceTheme, value)
	}
	if (option.Key == "background" || option.Key == "foreground") && len(m.colorChoices) > 0 {
		return m.beginChoiceEdit(choiceColor, value)
	}
	return m.input.Focus()
}

func (m *Model) commitEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
		return nil
	}
	value := m.input.Value()
	if err := option.Validate(value); err != nil {
		m.editError = err
		m.input.Err = err
		return nil
	}
	if err := m.stageValue(option.Key, value); err != nil {
		m.editError = err
		m.input.Err = err
		return nil
	}
	m.input.Blur()
	m.clearChoiceEdit()
	m.mode = ModeBrowse
	m.editError = nil
	if _, staged := m.desired[option.Key]; staged {
		m.status = "Staged " + option.Key + " (dry run)"
	} else {
		m.status = "No change for " + option.Key
	}
	return nil
}

func (m *Model) cancelEdit() tea.Cmd {
	m.input.Blur()
	m.choiceQuery.Blur()
	m.clearChoiceEdit()
	m.numberMode = false
	m.editError = nil
	m.mode = ModeBrowse
	m.status = "Edit cancelled"
	return nil
}

func (m *Model) clearChoiceEdit() {
	m.choiceMode = choiceNone
	m.choiceMatch = nil
	m.choiceIndex = -1
}

func (m *Model) beginChoiceEdit(kind choiceMode, current string) tea.Cmd {
	m.choiceMode = kind
	m.choiceQuery.SetValue("")
	m.choiceQuery.CursorStart()
	m.choiceIndex = -1
	m.applyChoiceFilter(current)
	m.input.Blur()
	return m.choiceQuery.Focus()
}

func (m Model) updateChoiceEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c":
		return m, m.requestQuit()
	case "enter":
		if m.choiceIndex < 0 || m.choiceIndex >= len(m.choiceMatch) {
			m.status = "Choose a value or press ctrl+r for raw input"
			return m, nil
		}
		choice := m.choiceMatch[m.choiceIndex]
		if !m.choiceValueMatches(choice, m.input.Value()) {
			m.input.SetValue(m.choiceValue(choice))
		}
		m.choiceQuery.Blur()
		m.clearChoiceEdit()
		return m, m.commitEdit()
	case "up":
		m.moveChoice(-1)
		return m, nil
	case "down":
		m.moveChoice(1)
		return m, nil
	case "left":
		m.moveChoice(-1)
		return m, nil
	case "right":
		m.moveChoice(1)
		return m, nil
	case "ctrl+r":
		m.choiceQuery.Blur()
		m.clearChoiceEdit()
		m.status = "Raw value input enabled"
		return m, m.input.Focus()
	}

	var cmd tea.Cmd
	current := m.input.Value()
	previous := m.choiceSelectionValue()
	m.choiceQuery, cmd = m.choiceQuery.Update(msg)
	m.applyChoiceFilter(current)
	if previous != "" && m.choiceIndex >= 0 {
		for position, index := range m.choiceMatch {
			if m.choiceValue(index) == previous {
				m.choiceIndex = position
				break
			}
		}
	}
	return m, cmd
}

func (m *Model) applyChoiceFilter(current string) {
	query := strings.TrimSpace(m.choiceQuery.Value())
	previous := m.choiceSelectionValue()
	m.choiceMatch = m.choiceMatch[:0]
	for index := range m.choiceCount() {
		name, value := m.choiceAt(index)
		if query == "" || containsFold(name, query) || containsFold(value, query) {
			m.choiceMatch = append(m.choiceMatch, index)
		}
	}
	m.choiceIndex = -1
	for position, index := range m.choiceMatch {
		if previous != "" && m.choiceValue(index) == previous || current != "" && m.choiceValueMatches(index, current) {
			m.choiceIndex = position
			break
		}
	}
	if m.choiceIndex < 0 && query != "" && len(m.choiceMatch) > 0 {
		m.choiceIndex = 0
	}
}

func (m *Model) moveChoice(delta int) {
	if len(m.choiceMatch) == 0 {
		return
	}
	if m.choiceIndex < 0 {
		m.choiceIndex = 0
	} else {
		m.choiceIndex = (m.choiceIndex + delta + len(m.choiceMatch)) % len(m.choiceMatch)
	}
	m.input.SetValue(m.choiceValue(m.choiceMatch[m.choiceIndex]))
	m.input.Err = nil
	m.editError = nil
}

func (m Model) choiceSelectionValue() string {
	if m.choiceIndex < 0 || m.choiceIndex >= len(m.choiceMatch) {
		return ""
	}
	return m.choiceValue(m.choiceMatch[m.choiceIndex])
}

func (m Model) choiceCount() int {
	if m.choiceMode == choiceTheme {
		return len(m.themeChoices)
	}
	return len(m.colorChoices)
}

func (m Model) choiceAt(index int) (name, value string) {
	if m.choiceMode == choiceTheme {
		return m.themeChoices[index], m.themeChoices[index]
	}
	choice := m.colorChoices[index]
	return choice.Name, choice.Value
}

func (m Model) choiceValue(index int) string {
	_, value := m.choiceAt(index)
	return value
}

func (m Model) choiceValueMatches(index int, current string) bool {
	value := m.choiceValue(index)
	if m.choiceMode != choiceColor {
		return value == current
	}
	return strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(value), "#"), strings.TrimPrefix(strings.TrimSpace(current), "#"))
}

func (m *Model) adjustNumber(delta int) {
	option, ok := m.selectedOption()
	if !ok {
		return
	}
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		value = strings.TrimSpace(option.Default)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		m.editError = fmt.Errorf("type a number or press ctrl+r for raw input")
		m.input.Err = m.editError
		return
	}
	number += float64(delta) * numberStep(option)
	m.input.SetValue(strconv.FormatFloat(number, 'f', -1, 64))
	m.input.Err = nil
	m.editError = nil
}

func (m *Model) stageValue(key, value string) error {
	if strings.ContainsAny(value, "\r\n\x00") {
		return configdoc.ErrUnsafeValue
	}
	if assignments := m.original.Assignments(key); len(assignments) == 1 && assignments[0].Value == value {
		delete(m.desired, key)
		return m.rebuildDraft()
	}
	m.desired[key] = value
	return m.rebuildDraft()
}

func (m *Model) rebuildDraft() error {
	m.draft = m.original.Clone()
	m.changes = make(map[string]configdoc.Change)
	for _, option := range m.options {
		value, ok := m.desired[option.Key]
		if !ok {
			continue
		}
		change, err := m.draft.SetScalar(option.Key, value)
		if err != nil {
			return err
		}
		m.changes[option.Key] = change
	}
	return nil
}

func (m *Model) resetSelected() {
	option, ok := m.selectedOption()
	if !ok || !option.Editable() {
		return
	}
	if m.readOnly {
		m.status = "Full catalog view is read-only until an edit target is selected"
		return
	}
	if len(m.draft.Assignments(option.Key)) > 1 {
		m.status = fmt.Sprintf("%s has duplicate assignments and is read-only", option.Key)
		return
	}
	if err := m.stageValue(option.Key, ""); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Staged explicit default reset for " + option.Key + " (dry run)"
}

func (m *Model) revertSelected() {
	option, ok := m.selectedOption()
	if !ok {
		return
	}
	if _, changed := m.desired[option.Key]; !changed {
		m.status = "No staged change for " + option.Key
		return
	}
	delete(m.desired, option.Key)
	if err := m.rebuildDraft(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Reverted " + option.Key
}

func (m *Model) cycleInput(option schema.Option, delta int) {
	values := option.Values
	if option.Kind == schema.KindBoolean && len(values) == 0 {
		values = []string{"false", "true"}
	}
	if len(values) == 0 {
		return
	}
	current := m.input.Value()
	if current == "" && option.Default != "" {
		current = option.Default
	}
	index := 0
	for candidateIndex, value := range values {
		if value == current {
			index = candidateIndex
			break
		}
	}
	index = (index + delta + len(values)) % len(values)
	m.input.SetValue(values[index])
	m.input.Err = nil
	m.editError = nil
}

func (m *Model) openPreview() {
	m.preview.SetContent(m.previewContent())
	m.preview.GotoTop()
	m.mode = ModePreview
}

func (m Model) HasChanges() bool {
	return len(m.desired) > 0
}

func (m Model) Mode() Mode {
	return m.mode
}

func (m Model) DraftBytes() []byte {
	if m.draft == nil {
		return nil
	}
	return m.draft.Bytes()
}

func (m Model) OriginalBytes() []byte {
	if m.original == nil {
		return nil
	}
	return m.original.Bytes()
}

func (m Model) Status() string {
	return m.status
}

// ReadOnly reports whether this model can stage edits. It is useful to hosts
// that need to explain why a discovered multi-file graph is browse-only.
func (m Model) ReadOnly() bool {
	return m.readOnly
}

// CanEdit reports whether a catalog option is exposed by the current editor.
func (m Model) CanEdit(key string) bool {
	if m.readOnly {
		return false
	}
	for _, option := range m.options {
		if option.Key == key {
			return option.Editable()
		}
	}
	return false
}

func (m Model) EditError() error {
	return m.editError
}

func (m Model) SelectedKey() string {
	option, ok := m.selectedOption()
	if !ok {
		return ""
	}
	return option.Key
}

func msgString(msg tea.Msg) string {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		return keyMsg.String()
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
