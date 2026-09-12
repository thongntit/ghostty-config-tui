package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
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

// Model is a fixture-safe config editor. It keeps original and draft
// documents separate and never writes the selected file.
type Model struct {
	options    []schema.Option
	selected   int
	width      int
	height     int
	configPath string

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
	return Model{
		options:    append([]schema.Option(nil), options...),
		configPath: configPath,
		original:   originalCopy,
		draft:      originalCopy.Clone(),
		desired:    make(map[string]string),
		changes:    make(map[string]configdoc.Change),
		mode:       ModeBrowse,
		input:      input,
		preview:    preview,
	}
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
		m.preview.SetWidth(maxInt(40, m.width-6))
	}
	if m.height > 0 {
		m.preview.SetHeight(maxInt(6, m.height-10))
	}
}

func (m Model) updateBrowse(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if key.Matches(keyMsg, defaultKeyMap.Quit) {
		return m, m.requestQuit()
	}

	switch {
	case key.Matches(keyMsg, defaultKeyMap.Up):
		if m.selected > 0 {
			m.selected--
		}
	case key.Matches(keyMsg, defaultKeyMap.Down):
		if m.selected < len(m.options)-1 {
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

func (m Model) updateEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, keyPressed := msg.(tea.KeyPressMsg)
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Escape) {
		m.input.Blur()
		m.editError = nil
		m.mode = ModeBrowse
		m.status = "Edit cancelled"
		return m, nil
	}
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Enter) {
		return m, m.commitEdit()
	}
	if keyPressed && key.Matches(keyMsg, defaultKeyMap.Quit) && keyMsg.String() == "ctrl+c" {
		return m, m.requestQuit()
	}

	if keyPressed {
		if option, ok := m.selectedOption(); ok && len(option.Values) > 0 {
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
	if m.selected < 0 || m.selected >= len(m.options) {
		return schema.Option{}, false
	}
	return m.options[m.selected], true
}

func (m *Model) beginEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
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
	m.input.Placeholder = "enter a value"
	m.input.CharLimit = 1024
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.resize()
	m.editError = nil
	m.mode = ModeEdit
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
	m.mode = ModeBrowse
	m.editError = nil
	m.status = "Staged " + option.Key + " (dry run)"
	return nil
}

func (m *Model) stageValue(key, value string) error {
	if strings.ContainsAny(value, "\r\n\x00") {
		return configdoc.ErrUnsafeValue
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
	if len(option.Values) == 0 {
		return
	}
	current := m.input.Value()
	index := 0
	for candidateIndex, value := range option.Values {
		if value == current {
			index = candidateIndex
			break
		}
	}
	index = (index + delta + len(option.Values)) % len(option.Values)
	m.input.SetValue(option.Values[index])
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
