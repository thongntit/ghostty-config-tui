package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/thongntit/ghostty-config-tui/internal/configdoc"
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

func printable(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: value, Code: rune(value[0])})
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code})
}
