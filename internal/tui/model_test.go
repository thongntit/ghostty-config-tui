package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
	"github.com/thongntit/ghostty-config-tui/internal/configgraph"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func TestEditPreviewKeepsOriginalUntouched(t *testing.T) {
	original := configdoc.Parse([]byte("theme = old\nfont-size = 14\n"))
	model := NewModel(schema.BootstrapOptions(), "fixture.ghostty", original)

	model, _ = model.Update(printable("e"))
	if model.Mode() != ModeEdit {
		t.Fatalf("expected edit mode, got %v", model.Mode())
	}
	model.input.SetValue("rose-pine")
	model, _ = model.Update(special(tea.KeyEnter))
	if model.Mode() != ModeBrowse || !model.HasChanges() {
		t.Fatalf("expected staged browse state, mode=%v changes=%v", model.Mode(), model.HasChanges())
	}

	model, _ = model.Update(printable("p"))
	if model.Mode() != ModePreview {
		t.Fatalf("expected preview mode, got %v", model.Mode())
	}
	if !strings.Contains(model.preview.GetContent(), "DRY RUN — no file will be written") {
		t.Fatal("preview did not identify dry-run mode")
	}
	if !strings.Contains(model.preview.GetContent(), "theme = rose-pine") {
		t.Fatal("preview did not contain the staged value")
	}

	if got, want := model.OriginalBytes(), original.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("original document changed:\n got %q\nwant %q", got, want)
	}
	if got, want := string(model.DraftBytes()), "theme = rose-pine\nfont-size = 14\n"; got != want {
		t.Fatalf("unexpected draft:\n got %q\nwant %q", got, want)
	}
}

func TestResetRevertAndRepeatableEditor(t *testing.T) {
	original := configdoc.Parse([]byte("theme = old\nfont-size = 14\nkeybind = ctrl+a=first\n"))
	model := NewModel(schema.BootstrapOptions(), "fixture.ghostty", original)

	model, _ = model.Update(printable("r"))
	if got := string(model.DraftBytes()); !strings.Contains(got, "theme =\n") {
		t.Fatalf("reset did not stage an explicit default: %q", got)
	}
	model, _ = model.Update(printable("u"))
	if got := string(model.DraftBytes()); got != string(original.Bytes()) {
		t.Fatalf("revert did not restore original draft:\n got %q\nwant %q", got, original.Bytes())
	}

	for index := 0; index < 5; index++ {
		model, _ = model.Update(special(tea.KeyDown))
	}
	if model.SelectedKey() != "keybind" {
		t.Fatalf("expected keybind selection, got %q", model.SelectedKey())
	}
	model, _ = model.Update(printable("e"))
	if model.Mode() != ModeEdit {
		t.Fatalf("repeatable option did not open an editor: mode=%v status=%q", model.Mode(), model.Status())
	}
	model, _ = model.Update(special(tea.KeyEscape))
	if model.Mode() != ModeBrowse {
		t.Fatalf("repeatable editor did not cancel: mode=%v", model.Mode())
	}
}

func TestInvalidValueStaysInEditAndDirtyQuitConfirms(t *testing.T) {
	original := configdoc.Parse([]byte("theme = old\nfont-size = 14\n"))
	model := NewModel(schema.BootstrapOptions(), "fixture.ghostty", original)
	model, _ = model.Update(special(tea.KeyDown))
	model, _ = model.Update(special(tea.KeyDown))
	model, _ = model.Update(printable("e"))
	model.input.SetValue("NaN")
	model, cmd := model.Update(special(tea.KeyEnter))
	if cmd != nil || model.Mode() != ModeEdit || model.EditError() == nil {
		t.Fatalf("invalid value did not stay in edit: mode=%v err=%v cmd=%v", model.Mode(), model.EditError(), cmd)
	}

	model.input.SetValue("16")
	model, _ = model.Update(special(tea.KeyEnter))
	model, cmd = model.Update(printable("q"))
	if cmd != nil || model.Mode() != ModeConfirmQuit {
		t.Fatalf("dirty quit skipped confirmation: mode=%v cmd=%v", model.Mode(), cmd)
	}
	model, _ = model.Update(printable("n"))
	if model.Mode() != ModeBrowse {
		t.Fatalf("cancelled quit did not return to browse: %v", model.Mode())
	}
	model, _ = model.Update(printable("q"))
	model, cmd = model.Update(printable("y"))
	if cmd == nil {
		t.Fatal("confirmed quit did not return a quit command")
	}
}

func TestBooleanEditorCyclesAndStagesTypedValue(t *testing.T) {
	original := configdoc.Parse([]byte("confirm-close-surface = false\n"))
	options := []schema.Option{{
		Key:         "confirm-close-surface",
		Category:    "General",
		Kind:        schema.KindBoolean,
		Edit:        schema.EditScalar,
		Description: "Confirm before closing.",
	}}
	model := NewModel(options, "fixture.ghostty", original)

	model, _ = model.Update(printable("e"))
	if model.Mode() != ModeEdit {
		t.Fatalf("expected edit mode, got %v", model.Mode())
	}
	model, _ = model.Update(special(tea.KeyRight))
	if got := model.input.Value(); got != "true" {
		t.Fatalf("boolean toggle value = %q, want true", got)
	}
	model, _ = model.Update(special(tea.KeyEnter))
	if got := string(model.DraftBytes()); got != "confirm-close-surface = true\n" {
		t.Fatalf("unexpected boolean draft: %q", got)
	}
}

func TestFriendlyThemeChooserStagesSelectionWithoutRawSyntax(t *testing.T) {
	original := configdoc.Parse([]byte("theme = old\n"))
	options := []schema.Option{{
		Key:         "theme",
		Category:    "Appearance",
		Kind:        schema.KindString,
		Edit:        schema.EditScalar,
		Description: "A theme to use.",
	}}
	model := NewModel(options, "fixture.ghostty", original)
	model.SetThemeChoices([]string{"Rose Pine", "Tokyo Night"})

	model, _ = model.Update(printable("e"))
	if model.choiceMode != choiceTheme {
		t.Fatalf("expected theme chooser, got %v", model.choiceMode)
	}
	model, _ = model.Update(printable("tokyo"))
	model, _ = model.Update(special(tea.KeyEnter))
	if model.Mode() != ModeBrowse {
		t.Fatalf("theme chooser did not return to browse: %v", model.Mode())
	}
	if got := string(model.DraftBytes()); got != "theme = Tokyo Night\n" {
		t.Fatalf("unexpected chosen theme draft: %q", got)
	}
	if got := model.optionStatus(options[0]); got != "Tokyo Night" {
		t.Fatalf("staged theme status = %q, want Tokyo Night", got)
	}
}

func TestFriendlyColorChooserStagesSwatchValue(t *testing.T) {
	original := configdoc.Parse([]byte("background = 282c34\n"))
	options := []schema.Option{{
		Key:         "background",
		Category:    "Appearance",
		Kind:        schema.KindColor,
		Edit:        schema.EditScalar,
		Description: "Background color.",
	}}
	model := NewModel(options, "fixture.ghostty", original)
	model.SetColorChoices([]ColorChoice{{Name: "Ocean", Value: "#123456"}})

	model, _ = model.Update(printable("e"))
	if model.choiceMode != choiceColor {
		t.Fatalf("expected color chooser, got %v", model.choiceMode)
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if lines := len(strings.Split(model.render(), "\n")); lines > 24 {
		t.Fatalf("color chooser rendered %d lines on an 80x24 terminal", lines)
	}
	model, _ = model.Update(special(tea.KeyDown))
	model, _ = model.Update(special(tea.KeyEnter))
	if got := string(model.DraftBytes()); got != "background = #123456\n" {
		t.Fatalf("unexpected chosen color draft: %q", got)
	}
}

func TestFontSizeStepperStagesHalfPoint(t *testing.T) {
	original := configdoc.Parse([]byte("font-size = 13\n"))
	step := 0.5
	options := []schema.Option{{
		Key:         "font-size",
		Category:    "Appearance",
		Kind:        schema.KindNumber,
		Edit:        schema.EditScalar,
		Step:        &step,
		Description: "Font size in points.",
	}}
	model := NewModel(options, "fixture.ghostty", original)

	model, _ = model.Update(printable("e"))
	model, _ = model.Update(special(tea.KeyRight))
	model, _ = model.Update(special(tea.KeyEnter))
	if got := string(model.DraftBytes()); got != "font-size = 13.5\n" {
		t.Fatalf("unexpected stepped font size draft: %q", got)
	}
	model, _ = model.Update(printable("p"))
	if !strings.Contains(model.preview.GetContent(), "font-size = 13.5") {
		t.Fatalf("numeric preview did not include staged value: %q", model.preview.GetContent())
	}
}

func TestFriendlyEditorDoesNotDirtyOnNoopSelection(t *testing.T) {
	original := configdoc.Parse([]byte("background = #123456\n"))
	options := []schema.Option{{Key: "background", Kind: schema.KindColor, Edit: schema.EditScalar}}
	model := NewModel(options, "fixture.ghostty", original)
	model.SetColorChoices([]ColorChoice{{Name: "Ocean", Value: "#123456"}})

	model, _ = model.Update(printable("e"))
	model, _ = model.Update(special(tea.KeyEnter))
	if model.HasChanges() {
		t.Fatal("reselecting the current color created a staged change")
	}
	if got := model.Status(); got != "No change for background" {
		t.Fatalf("noop status = %q", got)
	}
}

func TestSuccessfulSavePromotesDraftBaseline(t *testing.T) {
	original := configdoc.Parse([]byte("theme = old\n"))
	model := NewModel([]schema.Option{{Key: "theme", Kind: schema.KindString, Edit: schema.EditScalar}}, "fixture.ghostty", original)
	var calls [][]FileSnapshot
	model.SetSaveFunc(func(changes []FileSnapshot) error {
		calls = append(calls, changes)
		return nil
	})

	if err := model.stageScalar("theme", "new"); err != nil {
		t.Fatalf("stage first value: %v", err)
	}
	model.saveNow()
	if model.HasChanges() {
		t.Fatal("successful save left the first draft dirty")
	}
	if len(calls) != 1 || string(calls[0][0].Candidate) != "theme = new\n" {
		t.Fatalf("first save snapshots = %+v", calls)
	}

	if err := model.stageScalar("theme", "newer"); err != nil {
		t.Fatalf("stage second value: %v", err)
	}
	model.saveNow()
	if model.HasChanges() {
		t.Fatal("successful second save left the draft dirty")
	}
	if len(calls) != 2 || string(calls[1][0].Original) != "theme = new\n" || string(calls[1][0].Candidate) != "theme = newer\n" {
		t.Fatalf("second save snapshots = %+v", calls)
	}
}

func TestFriendlyOptionName(t *testing.T) {
	for key, want := range map[string]string{
		"font-size":    "Font Size",
		"gtk-titlebar": "GTK Titlebar",
		"macos-window": "macOS Window",
		"window-title": "Window Title",
	} {
		if got := friendlyOptionName(key); got != want {
			t.Errorf("friendlyOptionName(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestGraphResetStagesAgainstSourceDocument(t *testing.T) {
	original := configdoc.Parse([]byte("theme = dark\n"))
	graph := configgraph.Graph{
		Roots:       []string{"config.ghostty"},
		Files:       []configgraph.File{{Path: "config.ghostty", Document: original}},
		Assignments: []configgraph.Assignment{{Key: "theme", Value: "dark", Path: "config.ghostty", Line: 1}},
	}
	options := []schema.Option{{Key: "theme", Kind: schema.KindString, Edit: schema.EditScalar}}
	model := NewGraphModel(options, "config.ghostty", original, graph)
	model, _ = model.Update(printable("r"))
	if !model.HasChanges() {
		t.Fatal("graph reset did not stage a change")
	}
	if got := string(model.DraftBytes()); got != "theme =\n" {
		t.Fatalf("graph reset draft = %q", got)
	}
}

func TestGraphModelSearchesCatalogAndShowsEffectiveSource(t *testing.T) {
	original := configdoc.Parse([]byte("theme = dark\nfont-size = 14\n"))
	graph := configgraph.Graph{
		Roots: []string{"config.ghostty"},
		Files: []configgraph.File{{Path: "config.ghostty", Document: original}},
		Assignments: []configgraph.Assignment{
			{Key: "theme", Value: "dark", Path: "config.ghostty", Line: 1},
			{Key: "font-size", Value: "14", Path: "config.ghostty", Line: 2},
		},
	}
	options := []schema.Option{
		{Key: "theme", Category: "Appearance", Kind: schema.KindString, Edit: schema.EditReadOnlyRepeatable, Description: "Theme."},
		{Key: "font-size", Category: "Appearance", Kind: schema.KindNumber, Edit: schema.EditReadOnlyRepeatable, Description: "Font size."},
	}
	model := NewGraphModel(options, "config.ghostty", original, graph)

	model, _ = model.Update(printable("/"))
	model.search.SetValue("font")
	model, _ = model.Update(special(tea.KeyEnter))
	if got := model.SelectedKey(); got != "font-size" {
		t.Fatalf("search selected %q, want font-size", got)
	}
	if got := model.optionStatus(options[1]); got != "14" {
		t.Fatalf("effective option status = %q, want 14", got)
	}
	if got := model.optionSource(options[1]); got != "Effective source: config.ghostty:2" {
		t.Fatalf("effective source = %q", got)
	}
	model, _ = model.Update(printable("p"))
	if !strings.Contains(model.preview.GetContent(), "Effective Ghostty configuration graph") || !strings.Contains(model.preview.GetContent(), "config.ghostty:2") {
		t.Fatal("graph preview did not include effective source information")
	}
}

func TestGraphScalarEditTargetsEffectiveSourceFile(t *testing.T) {
	rootPath := "root.ghostty"
	childPath := "child.ghostty"
	root := configdoc.Parse([]byte("theme = dark\nconfig-file = child.ghostty\n"))
	child := configdoc.Parse([]byte("theme = light\n"))
	graph := configgraph.Graph{
		Roots: []string{rootPath},
		Files: []configgraph.File{
			{Path: rootPath, Document: root},
			{Path: childPath, Document: child},
		},
		Assignments: []configgraph.Assignment{
			{Key: "theme", Value: "dark", Path: rootPath, Line: 1, SourceFile: 0},
			{Key: "theme", Value: "light", Path: childPath, Line: 1, SourceFile: 1},
		},
	}
	options := []schema.Option{{Key: "theme", Kind: schema.KindString, Edit: schema.EditScalar}}
	model := NewGraphModel(options, rootPath, root, graph)
	if err := model.stageScalar("theme", "rose-pine"); err != nil {
		t.Fatalf("stage graph scalar: %v", err)
	}
	if got := string(model.fileDrafts[0].Bytes()); got != string(root.Bytes()) {
		t.Fatalf("root changed instead of effective source: %q", got)
	}
	if got := string(model.fileDrafts[1].Bytes()); got != "theme = rose-pine\n" {
		t.Fatalf("child draft = %q", got)
	}
	if got := model.optionStatus(options[0]); got != "rose-pine (2 assignments)" {
		t.Fatalf("effective graph status = %q", got)
	}
}

func TestRepeatableEditorUpdatesAndAddsAcrossFiles(t *testing.T) {
	rootPath := "root.ghostty"
	childPath := "child.ghostty"
	root := configdoc.Parse([]byte("config-file = child.ghostty\n"))
	child := configdoc.Parse([]byte("keybind = ctrl+a=one\nkeybind = ctrl+b=two\n"))
	graph := configgraph.Graph{
		Roots: []string{rootPath},
		Files: []configgraph.File{
			{Path: rootPath, Document: root},
			{Path: childPath, Document: child},
		},
		Assignments: []configgraph.Assignment{
			{Key: "keybind", Value: "ctrl+a=one", Path: childPath, Line: 1, SourceFile: 1},
			{Key: "keybind", Value: "ctrl+b=two", Path: childPath, Line: 2, SourceFile: 1},
		},
	}
	options := []schema.Option{{Key: "keybind", Kind: schema.KindKeybind, Edit: schema.EditRepeatable}}
	model := NewGraphModel(options, rootPath, root, graph)
	items := model.repeatableItemsFor("keybind")
	items = append(items[1:], repeatableItem{Key: "keybind", Path: rootPath, Value: "ctrl+c=three"})
	if err := model.stageRepeatable("keybind", items); err != nil {
		t.Fatalf("stage repeatable graph value: %v", err)
	}
	if got := string(model.fileDrafts[1].Bytes()); got != "keybind = ctrl+b=two\n" {
		t.Fatalf("child repeatable draft = %q", got)
	}
	if got := string(model.fileDrafts[0].Bytes()); got != "config-file = child.ghostty\nkeybind = ctrl+c=three\n" {
		t.Fatalf("root repeatable draft = %q", got)
	}
}

func TestRepeatableEditorAddsValueThroughListControls(t *testing.T) {
	original := configdoc.Parse([]byte("keybind = ctrl+a=one\n"))
	options := []schema.Option{{Key: "keybind", Kind: schema.KindKeybind, Edit: schema.EditRepeatable}}
	model := NewModel(options, "fixture.ghostty", original)

	model, _ = model.Update(printable("e"))
	if model.Mode() != ModeEdit || !model.repeatableMode {
		t.Fatalf("repeatable editor did not open: mode=%v repeatable=%v", model.Mode(), model.repeatableMode)
	}
	model, _ = model.Update(printable("a"))
	if !model.repeatableEditing {
		t.Fatal("add control did not open the value input")
	}
	model.input.SetValue("ctrl+b=two")
	model, _ = model.Update(special(tea.KeyEnter))
	model, _ = model.Update(special(tea.KeyEnter))
	if model.Mode() != ModeBrowse || !model.HasChanges() {
		t.Fatalf("repeatable list did not stage: mode=%v changes=%v", model.Mode(), model.HasChanges())
	}
	if got := string(model.DraftBytes()); got != "keybind = ctrl+a=one\nkeybind = ctrl+b=two\n" {
		t.Fatalf("repeatable list draft = %q", got)
	}
}

func printable(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: value, Code: rune(value[0])})
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
