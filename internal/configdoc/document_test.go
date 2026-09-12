package configdoc

import (
	"bytes"
	"testing"
)

func TestRoundTripPreservesSource(t *testing.T) {
	input := []byte("# Ghostty\r\nbackground = 282c34 # this is value text\r\nkeybind = ctrl+d=\"new_split:right\"\r\nunknown syntax\r\nfont-family =\r\n")
	document := Parse(input)

	if got := document.Render(); !bytes.Equal(got, input) {
		t.Fatalf("round trip changed source:\n got %q\nwant %q", got, input)
	}
	if got, ok := document.Lookup("keybind"); !ok || got != `ctrl+d="new_split:right"` {
		t.Fatalf("unexpected keybind: %q, %v", got, ok)
	}
}

func TestRoundTripHandlesWhitespaceOnlyValues(t *testing.T) {
	input := []byte("font-family =    \nbackground =\t\r\n")
	document := Parse(input)

	if got := document.Render(); !bytes.Equal(got, input) {
		t.Fatalf("whitespace-only values changed source:\n got %q\nwant %q", got, input)
	}
	if got, ok := document.Lookup("font-family"); !ok || got != "" {
		t.Fatalf("unexpected whitespace-only value: %q, %v", got, ok)
	}
}

func TestSetPreservesAssignmentShape(t *testing.T) {
	document := Parse([]byte("theme    =    old-theme  \r\n# keep me\r\n"))
	document.Set("theme", "new-theme")

	want := []byte("theme    =    new-theme  \r\n# keep me\r\n")
	if got := document.Render(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected edit:\n got %q\nwant %q", got, want)
	}
}

func TestSetUsesLastRepeatedAssignment(t *testing.T) {
	document := Parse([]byte("keybind = ctrl+a=first\nkeybind = ctrl+b=second\n"))
	document.Set("keybind", "ctrl+b=updated")

	want := []byte("keybind = ctrl+a=first\nkeybind = ctrl+b=updated\n")
	if got := document.Render(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected repeated-key edit:\n got %q\nwant %q", got, want)
	}
}

func TestSetAppendsWithDetectedLineEnding(t *testing.T) {
	document := Parse([]byte("theme = rose-pine\r\nlast-line"))
	document.Set("font-size", "14")

	want := []byte("theme = rose-pine\r\nlast-line\r\nfont-size = 14\r\n")
	if got := document.Render(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected appended config:\n got %q\nwant %q", got, want)
	}
}

func TestSetScalarRejectsDuplicateAssignments(t *testing.T) {
	document := Parse([]byte("theme = one\ntheme = two\n"))

	if _, err := document.SetScalar("theme", "three"); err != ErrAmbiguousAssignment {
		t.Fatalf("expected duplicate assignment error, got %v", err)
	}
	if got := string(document.Bytes()); got != "theme = one\ntheme = two\n" {
		t.Fatalf("duplicate rejection changed document: %q", got)
	}
}

func TestSetScalarAppendsAndSupportsReset(t *testing.T) {
	document := Parse([]byte("theme = old\n"))
	if _, err := document.SetScalar("font-size", "14"); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := document.SetScalar("theme", ""); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	want := "theme =\nfont-size = 14\n"
	if got := string(document.Bytes()); got != want {
		t.Fatalf("unexpected scalar edit:\n got %q\nwant %q", got, want)
	}
}

func TestCloneKeepsOriginalUnchanged(t *testing.T) {
	original := Parse([]byte("theme = old\n"))
	draft := original.Clone()
	if _, err := draft.SetScalar("theme", "new"); err != nil {
		t.Fatalf("edit failed: %v", err)
	}

	if got := string(original.Bytes()); got != "theme = old\n" {
		t.Fatalf("original changed through clone: %q", got)
	}
	if got := string(draft.Bytes()); got != "theme = new\n" {
		t.Fatalf("draft did not change: %q", got)
	}
}
