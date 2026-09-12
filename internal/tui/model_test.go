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

func TestResetRevertAndReadOnlyGuards(t *testing.T) {
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
	if model.Mode() != ModeBrowse || !strings.Contains(model.Status(), "read-only") {
		t.Fatalf("repeatable option was editable: mode=%v status=%q", model.Mode(), model.Status())
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

func TestReadOnlyGraphRejectsReset(t *testing.T) {
	original := configdoc.Parse([]byte("theme = dark\n"))
	graph := configgraph.Graph{
		Roots:       []string{"config.ghostty"},
		Files:       []configgraph.File{{Path: "config.ghostty", Document: original}},
		Assignments: []configgraph.Assignment{{Key: "theme", Value: "dark", Path: "config.ghostty", Line: 1}},
	}
	options := []schema.Option{{Key: "theme", Kind: schema.KindString, Edit: schema.EditScalar}}
	model := NewGraphModel(options, "config.ghostty", original, graph)
	model, _ = model.Update(printable("r"))
	if model.HasChanges() {
		t.Fatal("read-only graph reset staged a change")
	}
	if !strings.Contains(model.Status(), "read-only") {
		t.Fatalf("read-only reset status = %q", model.Status())
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

func printable(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: value, Code: rune(value[0])})
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
