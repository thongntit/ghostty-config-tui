package configdoc

import (
	"bytes"
	"errors"
	"testing"
)

func TestSetScalarPreservesOnlyTheSelectedValue(t *testing.T) {
	document := Parse([]byte("# keep\r\ntheme    =    old # value text\r\nunknown line\r\n"))

	change, err := document.SetScalar("theme", "new")
	if err != nil {
		t.Fatalf("set scalar: %v", err)
	}
	if change.Line != 2 || change.Appended {
		t.Fatalf("unexpected change metadata: %+v", change)
	}
	want := []byte("# keep\r\ntheme    =    new\r\nunknown line\r\n")
	if got := document.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected edit:\n got %q\nwant %q", got, want)
	}
}

func TestSetScalarAppendsMultipleKeysWithoutBlankLines(t *testing.T) {
	document := Parse([]byte("last line"))
	if _, err := document.SetScalar("theme", "one"); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if _, err := document.SetScalar("font-size", "14"); err != nil {
		t.Fatalf("second append: %v", err)
	}

	want := []byte("last line\ntheme = one\nfont-size = 14\n")
	if got := document.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("unexpected append sequence:\n got %q\nwant %q", got, want)
	}
}

func TestSetScalarRejectsUnsafeValues(t *testing.T) {
	document := Parse([]byte("theme = old\n"))
	_, err := document.SetScalar("theme", "bad\nvalue")
	if !errors.Is(err, ErrUnsafeValue) {
		t.Fatalf("expected unsafe value error, got %v", err)
	}
	if got := string(document.Bytes()); got != "theme = old\n" {
		t.Fatalf("unsafe edit changed document: %q", got)
	}
}

func TestSetScalarCandidateIsStableAfterReparse(t *testing.T) {
	document := Parse([]byte("theme = old\nfont-size = 14\n"))
	if _, err := document.SetScalar("theme", "new"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	if _, err := document.SetScalar("font-size", "16"); err != nil {
		t.Fatalf("set font size: %v", err)
	}

	candidate := document.Bytes()
	reparsed := Parse(candidate)
	if got := reparsed.Bytes(); !bytes.Equal(got, candidate) {
		t.Fatalf("candidate is not stable after reparse:\n got %q\nwant %q", got, candidate)
	}
}

func TestTargetedAssignmentsSupportDuplicateAndRepeatableEdits(t *testing.T) {
	document := Parse([]byte("# keep\nkeybind = ctrl+a=first\nkeybind = ctrl+b=second\n"))
	if _, err := document.SetAssignment("keybind", 2, "ctrl+a=updated"); err != nil {
		t.Fatalf("targeted edit: %v", err)
	}
	if _, err := document.AppendAssignment("keybind", "ctrl+c=third"); err != nil {
		t.Fatalf("append repeatable: %v", err)
	}
	if err := document.RemoveAssignment("keybind", 3); err != nil {
		t.Fatalf("remove repeatable: %v", err)
	}
	want := "# keep\nkeybind = ctrl+a=updated\nkeybind = ctrl+c=third\n"
	if got := string(document.Bytes()); got != want {
		t.Fatalf("targeted document = %q, want %q", got, want)
	}
}

func TestTargetedAssignmentRejectsStaleLine(t *testing.T) {
	document := Parse([]byte("theme = dark\n"))
	if _, err := document.SetAssignment("font-size", 1, "14"); err != ErrAssignmentTarget {
		t.Fatalf("stale target error = %v, want %v", err, ErrAssignmentTarget)
	}
}
