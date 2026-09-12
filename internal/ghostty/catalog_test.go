package ghostty

import "testing"

func TestParseThemeListStripsSourceAnnotationsAndDeduplicates(t *testing.T) {
	got := parseThemeList([]byte("Catppuccin Mocha (resources)\nMy Theme (config)\nCatppuccin Mocha (resources)\n\n"))
	want := []string{"Catppuccin Mocha", "My Theme"}
	if len(got) != len(want) {
		t.Fatalf("theme count = %d, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("theme %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestParseColorListKeepsNamesWithSpaces(t *testing.T) {
	got := parseColorList([]byte("alice blue = #f0f8ff\nAliceBlue = #f0f8ff\nnot a color\n"))
	if len(got) != 2 {
		t.Fatalf("color count = %d, want 2: %+v", len(got), got)
	}
	if got[0].Name != "alice blue" || got[0].Value != "#f0f8ff" {
		t.Fatalf("first color = %+v", got[0])
	}
}

func TestParseFontListKeepsFamiliesAndSkipsFaces(t *testing.T) {
	got := parseFontList([]byte("Andale Mono\n  Andale Mono\n\nMenlo\n\tMenlo Bold\nMenlo\nerror: SentryInitFailed\n"))
	want := []string{"Andale Mono", "Menlo"}
	if len(got) != len(want) {
		t.Fatalf("font count = %d, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("font %d = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestParseActionListSkipsDiagnosticsAndDeduplicates(t *testing.T) {
	got := parseActionList([]byte("error: SentryInitFailed\nnew_window\ncopy_to_clipboard\nnew_window\n\n"))
	want := []string{"new_window", "copy_to_clipboard"}
	if len(got) != len(want) {
		t.Fatalf("action count = %d, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("action %d = %q, want %q", index, got[index], want[index])
		}
	}
}
