package tui

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
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

// FileSnapshot is the candidate content for one source file. The app supplies
// a SaveFunc so the TUI stays independent from the persistence policy.
type FileSnapshot struct {
	Path      string
	Original  []byte
	Candidate []byte
}

// SaveFunc is called only after save confirmation. Returning an error keeps
// the editor open and reports the failure in the status line.
type SaveFunc func([]FileSnapshot) error

type scalarEdit struct {
	Path  string
	Line  int
	Value string
}

type repeatableItem struct {
	Key      string
	Path     string
	Line     int
	Value    string
	Existing bool
}

type repeatableEdit struct {
	Items []repeatableItem
}

type stagedOption struct {
	Scalar     *scalarEdit
	Repeatable *repeatableEdit
}

// Model is the full config editor. It keeps an independent draft document
// for every loaded source file and never flattens included files into one
// synthetic document.
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

	// original/draft are retained for the simple NewModel API. Graph models
	// use fileOriginals/fileDrafts and currentDraftValue instead.
	original *configdoc.Document
	draft    *configdoc.Document

	fileOriginals []configdoc.Document
	fileDrafts    []configdoc.Document
	selectedFile  int
	staged        map[string]stagedOption

	repeatableMode         bool
	repeatableItems        []repeatableItem
	repeatableOriginal     []repeatableItem
	repeatableIndex        int
	repeatableEditing      bool
	repeatableEditExisting bool

	saveFunc SaveFunc

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
	input.CharLimit = 4096
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
		options:         append([]schema.Option(nil), options...),
		configPath:      configPath,
		visible:         visible,
		original:        originalCopy,
		draft:           originalCopy.Clone(),
		staged:          make(map[string]stagedOption),
		mode:            ModeBrowse,
		input:           input,
		preview:         preview,
		search:          search,
		colorChoices:    defaultColorChoices(),
		choiceQuery:     choiceQuery,
		choiceIndex:     -1,
		repeatableIndex: -1,
	}
}

// NewGraphModel creates a graph-aware editor. Every loaded source file gets a
// separate draft, so edits can target the assignment that actually supplies
// the effective value.
func NewGraphModel(options []schema.Option, configPath string, original configdoc.Document, graph configgraph.Graph) Model {
	model := NewModel(options, configPath, original)
	model.graph = &graph
	model.readOnly = false
	model.fileOriginals = make([]configdoc.Document, len(graph.Files))
	model.fileDrafts = make([]configdoc.Document, len(graph.Files))
	for index, file := range graph.Files {
		model.fileOriginals[index] = *file.Document.Clone()
		model.fileDrafts[index] = *file.Document.Clone()
	}
	model.selectedFile = fileIndex(graph.Files, configPath)
	model.syncLegacyDocuments()
	return model
}

// SetReadOnly is retained for hosts that intentionally want a browse-only
// model. The application no longer enables this mode for discovered graphs.
func (m *Model) SetReadOnly(readOnly bool) {
	m.readOnly = readOnly
}

// SetSaveFunc installs the persistence boundary used after save confirmation.
func (m *Model) SetSaveFunc(save SaveFunc) {
	m.saveFunc = save
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
	if m.graph != nil {
		m.syncLegacyDocuments()
	}
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
		m.search.SetValue("")
		m.applySearch("")
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
	case key.Matches(keyMsg, defaultKeyMap.Save):
		m.saveNow()
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
		if query == "" || containsFold(option.Key, query) || containsFold(friendlyOptionName(option.Key), query) || containsFold(option.Category, query) || containsFold(option.Description, query) {
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
	if m.repeatableMode {
		return m.updateRepeatableEdit(msg)
	}
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

func (m Model) updateRepeatableEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	if keyMsg.String() == "ctrl+c" {
		return m, m.requestQuit()
	}
	if m.repeatableEditing {
		switch keyMsg.String() {
		case "esc":
			m.repeatableEditing = false
			m.repeatableEditExisting = false
			m.input.Blur()
			m.editError = nil
			return m, nil
		case "enter":
			return m, m.commitRepeatableItem()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.editError = m.input.Err
		return m, cmd
	}

	switch keyMsg.String() {
	case "esc":
		return m, m.cancelEdit()
	case "up", "k":
		m.moveRepeatable(-1)
	case "down", "j":
		m.moveRepeatable(1)
	case "a":
		m.beginRepeatableItem(false)
	case "e":
		if len(m.repeatableItems) == 0 {
			m.beginRepeatableItem(false)
		} else {
			m.beginRepeatableItem(true)
		}
	case "enter":
		return m, m.commitRepeatableEdit()
	case "d", "backspace", "delete":
		m.deleteRepeatableItem()
	case "r":
		m.repeatableItems = nil
		m.repeatableIndex = -1
		m.status = "All values cleared; press enter to stage"
	}
	return m, nil
}

func (m Model) updateChoiceEdit(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		return m, m.cancelEdit()
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

func (m *Model) beginEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
		return nil
	}
	if m.readOnly {
		m.status = "This editor is configured as read-only"
		return nil
	}
	if !option.Editable() {
		m.status = fmt.Sprintf("%s is not editable in the loaded catalog", option.Key)
		return nil
	}
	if option.Repeatable() {
		m.beginRepeatableEdit(option)
		return nil
	}

	value, _ := m.currentValue(option.Key)
	m.input = textinput.New()
	m.input.Prompt = option.Key + " = "
	m.input.Placeholder = valueHint(option)
	m.input.CharLimit = 4096
	m.input.SetValue(value)
	m.input.CursorEnd()
	m.resize()
	m.editError = nil
	m.numberMode = option.Kind == schema.KindNumber
	m.choiceMode = choiceNone
	m.choiceMatch = nil
	m.choiceIndex = -1
	m.mode = ModeEdit
	if option.Key == "theme" && len(m.themeChoices) > 0 {
		return m.beginChoiceEdit(choiceTheme, value)
	}
	if option.Kind == schema.KindColor && option.Key != "palette" && len(m.colorChoices) > 0 {
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
	if err := m.stageScalar(option.Key, value); err != nil {
		m.editError = err
		m.input.Err = err
		return nil
	}
	m.input.Blur()
	m.clearChoiceEdit()
	m.numberMode = false
	m.mode = ModeBrowse
	m.editError = nil
	if m.HasChanges() {
		m.status = "Staged " + friendlyOptionName(option.Key)
	} else {
		m.status = "No change for " + option.Key
	}
	return nil
}

func (m *Model) beginRepeatableEdit(option schema.Option) {
	m.repeatableMode = true
	m.repeatableItems = m.repeatableItemsFor(option.Key)
	m.repeatableOriginal = cloneRepeatableItems(m.repeatableItems)
	m.repeatableIndex = 0
	if len(m.repeatableItems) == 0 {
		m.repeatableIndex = -1
	}
	m.repeatableEditing = false
	m.repeatableEditExisting = false
	m.numberMode = false
	m.choiceMode = choiceNone
	m.editError = nil
	m.mode = ModeEdit
}

func (m *Model) beginRepeatableItem(existing bool) {
	if existing {
		if m.repeatableIndex < 0 || m.repeatableIndex >= len(m.repeatableItems) {
			return
		}
		m.repeatableEditExisting = true
		m.input.SetValue(m.repeatableItems[m.repeatableIndex].Value)
	} else {
		m.repeatableEditExisting = false
		m.input.SetValue("")
	}
	if option, ok := m.selectedOption(); ok {
		m.input.Prompt = option.Key + " = "
	}
	m.input.Placeholder = "raw Ghostty value"
	m.input.CharLimit = 4096
	m.input.CursorEnd()
	m.input.Err = nil
	m.editError = nil
	m.repeatableEditing = true
	m.input.Focus()
}

func (m *Model) commitRepeatableItem() tea.Cmd {
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
	if m.repeatableEditExisting {
		m.repeatableItems[m.repeatableIndex].Value = value
	} else {
		m.repeatableItems = append(m.repeatableItems, repeatableItem{Key: option.Key, Path: m.configPath, Value: value})
		m.repeatableIndex = len(m.repeatableItems) - 1
	}
	m.repeatableEditing = false
	m.repeatableEditExisting = false
	m.input.Blur()
	m.editError = nil
	m.status = "Value updated; press enter to stage"
	return nil
}

func (m *Model) commitRepeatableEdit() tea.Cmd {
	option, ok := m.selectedOption()
	if !ok {
		return nil
	}
	if err := m.stageRepeatable(option.Key, m.repeatableItems); err != nil {
		m.editError = err
		return nil
	}
	m.repeatableMode = false
	m.repeatableItems = nil
	m.repeatableOriginal = nil
	m.repeatableEditing = false
	m.repeatableIndex = -1
	m.mode = ModeBrowse
	if m.HasChanges() {
		m.status = "Staged " + friendlyOptionName(option.Key)
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
	m.repeatableMode = false
	m.repeatableItems = nil
	m.repeatableOriginal = nil
	m.repeatableEditing = false
	m.repeatableEditExisting = false
	m.repeatableIndex = -1
	m.mode = ModeBrowse
	m.status = "Edit cancelled"
	return nil
}

func (m *Model) clearChoiceEdit() {
	m.choiceMode = choiceNone
	m.choiceMatch = nil
	m.choiceIndex = -1
}

func (m Model) currentValue(key string) (string, bool) {
	assignments := m.optionAssignments(key)
	if len(assignments) == 0 {
		return "", false
	}
	return assignments[len(assignments)-1].Value, true
}

func (m Model) optionAssignments(key string) []configgraph.Assignment {
	if m.graph == nil {
		assignments := make([]configgraph.Assignment, 0)
		if m.draft == nil {
			return assignments
		}
		for index, node := range m.draft.Nodes {
			if node.Kind != configdoc.AssignmentNode || node.Key != key {
				continue
			}
			assignments = append(assignments, configgraph.Assignment{
				Key:   key,
				Value: node.Value,
				Empty: node.Value == "",
				Path:  m.configPath,
				Line:  index + 1,
			})
		}
		return assignments
	}

	draftGraph := m.draftGraph()
	if key != "config-file" {
		return draftGraph.AssignmentsFor(key)
	}
	assignments := make([]configgraph.Assignment, 0)
	for fileIndex, file := range draftGraph.Files {
		for lineIndex, node := range file.Document.Nodes {
			if node.Kind != configdoc.AssignmentNode || node.Key != key {
				continue
			}
			assignments = append(assignments, configgraph.Assignment{
				Key:        key,
				Value:      node.Value,
				Empty:      node.Value == "",
				Path:       file.Path,
				Line:       lineIndex + 1,
				SourceFile: fileIndex,
			})
		}
	}
	return assignments
}

func (m Model) repeatableItemsFor(key string) []repeatableItem {
	assignments := m.optionAssignments(key)
	items := make([]repeatableItem, 0, len(assignments))
	for _, assignment := range assignments {
		items = append(items, repeatableItem{
			Key:      key,
			Path:     assignment.Path,
			Line:     assignment.Line,
			Value:    assignment.Value,
			Existing: m.originalHasAssignment(key, assignment.Path, assignment.Line),
		})
	}
	return items
}

func cloneRepeatableItems(items []repeatableItem) []repeatableItem {
	return append([]repeatableItem(nil), items...)
}

func (m Model) draftGraph() *configgraph.Graph {
	if m.graph == nil {
		return nil
	}
	documents := make(map[string]configdoc.Document, len(m.fileDrafts))
	for index, file := range m.graph.Files {
		if index < len(m.fileDrafts) {
			documents[file.Path] = m.fileDrafts[index]
		}
	}
	draftGraph := m.graph.WithDocuments(documents)
	return &draftGraph
}

func (m Model) originalHasAssignment(key, path string, line int) bool {
	document := m.originalDocument(path)
	if document == nil || line <= 0 || line > len(document.Nodes) {
		return false
	}
	node := document.Nodes[line-1]
	return node.Kind == configdoc.AssignmentNode && node.Key == key
}

func (m Model) originalAssignments(key string) []configgraph.Assignment {
	if m.graph == nil {
		assignments := make([]configgraph.Assignment, 0)
		if m.original == nil {
			return assignments
		}
		for index, node := range m.original.Nodes {
			if node.Kind == configdoc.AssignmentNode && node.Key == key {
				assignments = append(assignments, configgraph.Assignment{Key: key, Value: node.Value, Empty: node.Value == "", Path: m.configPath, Line: index + 1})
			}
		}
		return assignments
	}
	if key != "config-file" {
		return m.graph.AssignmentsFor(key)
	}
	assignments := make([]configgraph.Assignment, 0)
	for fileIndex, file := range m.fileOriginals {
		for lineIndex, node := range file.Nodes {
			if node.Kind == configdoc.AssignmentNode && node.Key == key {
				assignments = append(assignments, configgraph.Assignment{Key: key, Value: node.Value, Empty: node.Value == "", Path: m.graph.Files[fileIndex].Path, Line: lineIndex + 1, SourceFile: fileIndex})
			}
		}
	}
	return assignments
}

func (m *Model) stageScalar(key, value string) error {
	edit := scalarEdit{Path: m.configPath, Value: value}
	if previous, ok := m.staged[key]; ok && previous.Scalar != nil {
		edit = *previous.Scalar
		edit.Value = value
	} else if assignments := m.optionAssignments(key); len(assignments) > 0 {
		effective := assignments[len(assignments)-1]
		edit.Path = effective.Path
		edit.Line = effective.Line
	}
	m.staged[key] = stagedOption{Scalar: &edit}
	return m.rebuildDraft()
}

func (m *Model) stageRepeatable(key string, items []repeatableItem) error {
	m.staged[key] = stagedOption{Repeatable: &repeatableEdit{Items: cloneRepeatableItems(items)}}
	return m.rebuildDraft()
}

// stageValue is kept as a small compatibility seam for callers of the first
// scalar editor.
func (m *Model) stageValue(key, value string) error {
	return m.stageScalar(key, value)
}

func (m *Model) rebuildDraft() error {
	if m.graph == nil {
		m.draft = m.original.Clone()
	} else {
		m.fileDrafts = make([]configdoc.Document, len(m.fileOriginals))
		for index := range m.fileOriginals {
			m.fileDrafts[index] = *m.fileOriginals[index].Clone()
		}
		m.syncLegacyDocuments()
	}

	keys := m.stagedKeys()
	appendScalars := make([]scalarEdit, 0)
	appendScalarKeys := make([]string, 0)
	for _, key := range keys {
		staged := m.staged[key]
		if staged.Scalar == nil {
			continue
		}
		if staged.Scalar.Line == 0 {
			appendScalars = append(appendScalars, *staged.Scalar)
			appendScalarKeys = append(appendScalarKeys, key)
			continue
		}
		document := m.draftDocument(staged.Scalar.Path)
		if document == nil {
			return fmt.Errorf("draft source file is unavailable: %s", staged.Scalar.Path)
		}
		if _, err := document.SetAssignment(key, staged.Scalar.Line, staged.Scalar.Value); err != nil {
			return fmt.Errorf("stage %s: %w", key, err)
		}
	}

	deletes := make(map[string][]assignmentTarget)
	appends := make([]repeatableItem, 0)
	for _, key := range keys {
		staged := m.staged[key]
		if staged.Repeatable == nil {
			continue
		}
		retained := make(map[string]struct{})
		for _, item := range staged.Repeatable.Items {
			item.Key = key
			if item.Existing && item.Line > 0 {
				target := assignmentTarget{Key: key, Path: item.Path, Line: item.Line}
				retained[target.identity()] = struct{}{}
				document := m.draftDocument(item.Path)
				if document == nil {
					return fmt.Errorf("draft source file is unavailable: %s", item.Path)
				}
				if _, err := document.SetAssignment(key, item.Line, item.Value); err != nil {
					return fmt.Errorf("stage %s: %w", key, err)
				}
			} else {
				item.Path = firstNonEmpty(item.Path, m.configPath)
				appends = append(appends, item)
			}
		}
		for _, original := range m.originalAssignments(key) {
			target := assignmentTarget{Key: key, Path: original.Path, Line: original.Line}
			if _, keep := retained[target.identity()]; keep {
				continue
			}
			deletes[filepath.Clean(original.Path)] = append(deletes[filepath.Clean(original.Path)], target)
		}
	}

	for _, targets := range deletes {
		sort.Slice(targets, func(left, right int) bool { return targets[left].Line > targets[right].Line })
		for _, target := range targets {
			document := m.draftDocument(target.Path)
			if document == nil {
				return fmt.Errorf("draft source file is unavailable: %s", target.Path)
			}
			if err := document.RemoveAssignment(target.Key, target.Line); err != nil {
				return fmt.Errorf("remove %s: %w", target.Key, err)
			}
		}
	}

	for index, edit := range appendScalars {
		document := m.draftDocument(firstNonEmpty(edit.Path, m.configPath))
		if document == nil {
			return fmt.Errorf("draft source file is unavailable: %s", edit.Path)
		}
		if _, err := document.AppendAssignment(appendScalarKeys[index], edit.Value); err != nil {
			return fmt.Errorf("append %s: %w", appendScalarKeys[index], err)
		}
	}
	for _, item := range appends {
		document := m.draftDocument(firstNonEmpty(item.Path, m.configPath))
		if document == nil {
			return fmt.Errorf("draft source file is unavailable: %s", item.Path)
		}
		if _, err := document.AppendAssignment(item.Key, item.Value); err != nil {
			return fmt.Errorf("append repeatable value: %w", err)
		}
	}
	return nil
}

type assignmentTarget struct {
	Key  string
	Path string
	Line int
}

func (t assignmentTarget) identity() string {
	return filepath.Clean(t.Path) + "\x00" + strconv.Itoa(t.Line)
}

func (m Model) stagedKeys() []string {
	keys := make([]string, 0, len(m.staged))
	seen := make(map[string]struct{}, len(m.staged))
	for _, option := range m.options {
		if _, ok := m.staged[option.Key]; ok {
			keys = append(keys, option.Key)
			seen[option.Key] = struct{}{}
		}
	}
	remaining := make([]string, 0)
	for key := range m.staged {
		if _, ok := seen[key]; !ok {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(keys, remaining...)
}

func (m *Model) resetSelected() {
	option, ok := m.selectedOption()
	if !ok || !option.Editable() {
		return
	}
	if m.readOnly {
		m.status = "This editor is configured as read-only"
		return
	}
	if option.Repeatable() {
		if err := m.stageRepeatable(option.Key, nil); err != nil {
			m.status = err.Error()
			return
		}
		m.status = "Staged reset for " + friendlyOptionName(option.Key)
		return
	}
	if err := m.stageScalar(option.Key, ""); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Staged explicit default reset for " + friendlyOptionName(option.Key)
}

func (m *Model) revertSelected() {
	option, ok := m.selectedOption()
	if !ok {
		return
	}
	if _, changed := m.staged[option.Key]; !changed {
		m.status = "No staged change for " + option.Key
		return
	}
	delete(m.staged, option.Key)
	if err := m.rebuildDraft(); err != nil {
		m.status = err.Error()
		return
	}
	m.status = "Reverted " + friendlyOptionName(option.Key)
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

func (m *Model) moveRepeatable(delta int) {
	if len(m.repeatableItems) == 0 {
		return
	}
	if m.repeatableIndex < 0 {
		m.repeatableIndex = 0
		return
	}
	m.repeatableIndex = (m.repeatableIndex + delta + len(m.repeatableItems)) % len(m.repeatableItems)
}

func (m *Model) deleteRepeatableItem() {
	if m.repeatableIndex < 0 || m.repeatableIndex >= len(m.repeatableItems) {
		return
	}
	m.repeatableItems = append(m.repeatableItems[:m.repeatableIndex], m.repeatableItems[m.repeatableIndex+1:]...)
	if len(m.repeatableItems) == 0 {
		m.repeatableIndex = -1
	} else if m.repeatableIndex >= len(m.repeatableItems) {
		m.repeatableIndex = len(m.repeatableItems) - 1
	}
	m.status = "Value removed; press enter to stage"
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

func (m *Model) applyChoiceFilter(current string) {
	query := strings.TrimSpace(m.choiceQuery.Value())
	previous := m.choiceSelectionValue()
	m.choiceMatch = m.choiceMatch[:0]
	for index := 0; index < m.choiceCount(); index++ {
		name, value := m.choiceAt(index)
		if query == "" || containsFold(name, query) || containsFold(value, query) {
			m.choiceMatch = append(m.choiceMatch, index)
		}
	}
	m.choiceIndex = -1
	for position, index := range m.choiceMatch {
		if (previous != "" && m.choiceValue(index) == previous) || (current != "" && m.choiceValueMatches(index, current)) {
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

func (m *Model) openPreview() {
	m.preview.SetContent(m.previewContent())
	m.preview.GotoTop()
	m.mode = ModePreview
}

func (m *Model) saveNow() {
	if !m.HasChanges() {
		m.status = "No staged changes"
		return
	}
	if m.saveFunc == nil {
		m.status = "Save is unavailable for this host; use preview to inspect the candidate"
		return
	}
	snapshots := m.changedFileSnapshots()
	if err := m.saveFunc(snapshots); err != nil {
		m.status = "Save failed: " + err.Error()
		return
	}
	m.markSaved()
	if m.mode == ModePreview {
		m.preview.SetContent(m.previewContent())
		m.preview.GotoTop()
	}
	m.status = fmt.Sprintf("Saved %d config file(s)", len(snapshots))
}

func (m *Model) markSaved() {
	m.staged = make(map[string]stagedOption)
	if m.graph == nil {
		if m.draft == nil {
			return
		}
		m.original = m.draft.Clone()
		m.draft = m.original.Clone()
		return
	}
	for index := range m.fileDrafts {
		if index >= len(m.fileOriginals) {
			continue
		}
		m.fileOriginals[index] = *m.fileDrafts[index].Clone()
	}
	m.syncLegacyDocuments()
}

func (m *Model) requestQuit() tea.Cmd {
	if m.HasChanges() {
		m.returnMode = m.mode
		m.mode = ModeConfirmQuit
		return nil
	}
	return tea.Quit
}

func (m Model) updatePreview(msg tea.Msg) (Model, tea.Cmd) {
	switch msgString(msg) {
	case "esc":
		m.mode = ModeBrowse
		return m, nil
	case "q", "ctrl+c":
		return m, m.requestQuit()
	case "ctrl+s":
		m.saveNow()
		return m, nil
	}

	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}

func (m Model) updateConfirmQuit(msg tea.Msg) (Model, tea.Cmd) {
	switch msgString(msg) {
	case "y", "enter":
		if m.saveFunc == nil {
			return m, tea.Quit
		}
		m.saveNow()
		if strings.HasPrefix(m.status, "Save failed:") {
			m.mode = m.returnMode
			return m, nil
		}
		return m, tea.Quit
	case "d":
		return m, tea.Quit
	case "n", "esc":
		m.mode = m.returnMode
	}
	return m, nil
}

func (m Model) HasChanges() bool {
	if m.graph == nil {
		return m.original != nil && m.draft != nil && !bytes.Equal(m.original.Bytes(), m.draft.Bytes())
	}
	for index := range m.fileOriginals {
		if index >= len(m.fileDrafts) || !bytes.Equal(m.fileOriginals[index].Bytes(), m.fileDrafts[index].Bytes()) {
			return true
		}
	}
	return false
}

func (m Model) changedFileSnapshots() []FileSnapshot {
	if m.graph == nil {
		if m.original == nil || m.draft == nil || bytes.Equal(m.original.Bytes(), m.draft.Bytes()) {
			return nil
		}
		return []FileSnapshot{{Path: m.configPath, Original: m.original.Bytes(), Candidate: m.draft.Bytes()}}
	}
	snapshots := make([]FileSnapshot, 0)
	for index, file := range m.graph.Files {
		if index >= len(m.fileOriginals) || index >= len(m.fileDrafts) {
			continue
		}
		original := m.fileOriginals[index].Bytes()
		candidate := m.fileDrafts[index].Bytes()
		if bytes.Equal(original, candidate) {
			continue
		}
		snapshots = append(snapshots, FileSnapshot{Path: file.Path, Original: original, Candidate: candidate})
	}
	return snapshots
}

func (m Model) Mode() Mode {
	return m.mode
}

func (m Model) DraftBytes() []byte {
	return m.currentDraftValue().Bytes()
}

func (m Model) OriginalBytes() []byte {
	return m.currentOriginalValue().Bytes()
}

func (m Model) Status() string {
	return m.status
}

// ReadOnly reports whether a host explicitly disabled writes. Normal app
// startup leaves graph editing enabled.
func (m Model) ReadOnly() bool {
	return m.readOnly
}

// CanEdit reports whether the current model exposes an option editor.
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

func (m Model) selectedOption() (schema.Option, bool) {
	visible := m.visibleOptionIndexes()
	if m.selected < 0 || m.selected >= len(visible) {
		return schema.Option{}, false
	}
	return m.options[visible[m.selected]], true
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

func (m Model) currentOriginalValue() configdoc.Document {
	if m.graph != nil && m.selectedFile >= 0 && m.selectedFile < len(m.fileOriginals) {
		return m.fileOriginals[m.selectedFile]
	}
	if m.original != nil {
		return *m.original
	}
	return configdoc.Document{}
}

func (m Model) currentDraftValue() configdoc.Document {
	if m.graph != nil && m.selectedFile >= 0 && m.selectedFile < len(m.fileDrafts) {
		return m.fileDrafts[m.selectedFile]
	}
	if m.draft != nil {
		return *m.draft
	}
	return configdoc.Document{}
}

func (m Model) originalDocument(path string) *configdoc.Document {
	if m.graph == nil {
		return m.original
	}
	index := fileIndex(m.graph.Files, path)
	if index < 0 || index >= len(m.fileOriginals) {
		return nil
	}
	return &m.fileOriginals[index]
}

func (m *Model) draftDocument(path string) *configdoc.Document {
	if m.graph == nil {
		return m.draft
	}
	index := fileIndex(m.graph.Files, path)
	if index < 0 || index >= len(m.fileDrafts) {
		return nil
	}
	return &m.fileDrafts[index]
}

func (m *Model) syncLegacyDocuments() {
	if m.graph == nil || m.selectedFile < 0 || m.selectedFile >= len(m.fileDrafts) {
		return
	}
	m.original = &m.fileOriginals[m.selectedFile]
	m.draft = &m.fileDrafts[m.selectedFile]
}

func fileIndex(files []configgraph.File, path string) int {
	for index, file := range files {
		if file.Path == path {
			return index
		}
	}
	clean := filepath.Clean(path)
	for index, file := range files {
		if filepath.Clean(file.Path) == clean {
			return index
		}
	}
	return -1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
