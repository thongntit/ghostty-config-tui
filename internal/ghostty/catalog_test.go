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
