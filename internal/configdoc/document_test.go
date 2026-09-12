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
