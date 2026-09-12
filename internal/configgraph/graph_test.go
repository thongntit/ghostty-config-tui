package configgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesLocalAssignmentsBeforeIncludesAndRootsInOrder(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.ghostty")
	child := filepath.Join(root, "child.ghostty")
	second := filepath.Join(root, "second.ghostty")
	writeGraphConfig(t, first, "theme = dark\nconfig-file = child.ghostty\ntheme = blue\n")
	writeGraphConfig(t, child, "theme = light\nfont-size = 14\n")
	writeGraphConfig(t, second, "theme = rose-pine\n")

	graph, err := Load([]string{first, second})
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	theme := graph.AssignmentsFor("theme")
	if len(theme) != 4 {
		t.Fatalf("theme assignments = %d, want 4", len(theme))
	}
	wantValues := []string{"dark", "blue", "light", "rose-pine"}
	for index, want := range wantValues {
		if theme[index].Value != want {
			t.Errorf("theme assignment %d = %q, want %q", index, theme[index].Value, want)
		}
	}
	if effective, ok := graph.Effective("theme"); !ok || effective.Value != "rose-pine" || effective.Path != second {
		t.Fatalf("effective theme = %+v, %v", effective, ok)
	}
	if len(graph.Files) != 3 {
		t.Fatalf("loaded files = %d, want 3", len(graph.Files))
	}
	if len(graph.Includes) != 1 || !graph.Includes[0].Loaded || graph.Includes[0].ResolvedPath != child {
		t.Fatalf("includes = %+v", graph.Includes)
	}
}

func TestLoadReportsOptionalMissingIncludeAndRequiredMissingInclude(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.ghostty")
	writeGraphConfig(t, path, "config-file = ?optional.ghostty\nconfig-file = required.ghostty\n")

	graph, err := Load([]string{path})
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	if len(graph.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %+v, want optional and required include diagnostics", graph.Diagnostics)
	}
	if graph.Diagnostics[0].Severity != SeverityInfo || !strings.Contains(graph.Diagnostics[0].Message, "optional") {
		t.Errorf("optional diagnostic = %+v", graph.Diagnostics[0])
	}
	if graph.Diagnostics[1].Severity != SeverityError || !strings.Contains(graph.Diagnostics[1].Message, "required") {
		t.Errorf("required diagnostic = %+v", graph.Diagnostics[1])
	}
}

func TestLoadDetectsIncludeCycle(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.ghostty")
	second := filepath.Join(root, "second.ghostty")
	writeGraphConfig(t, first, "theme = dark\nconfig-file = second.ghostty\n")
	writeGraphConfig(t, second, "config-file = first.ghostty\n")

	graph, err := Load([]string{first})
	if err != nil {
		t.Fatalf("load cyclic graph: %v", err)
	}
	if graph.ErrorCount() != 0 {
		t.Fatalf("cycle produced error diagnostics: %+v", graph.Diagnostics)
	}
	if len(graph.Diagnostics) != 1 || graph.Diagnostics[0].Severity != SeverityWarning || !strings.Contains(graph.Diagnostics[0].Message, "cycle") {
		t.Fatalf("cycle diagnostics = %+v", graph.Diagnostics)
	}
}

func TestLoadDecodesQuotedIncludeAndPreservesSourceDocument(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent.ghostty")
	child := filepath.Join(root, "child file.ghostty")
	contents := []byte("# keep\nconfig-file = \"child file.ghostty\"\r\n")
	if err := os.WriteFile(parent, contents, 0o600); err != nil {
		t.Fatalf("write parent: %v", err)
	}
	writeGraphConfig(t, child, "foreground = ffffff\n")

	graph, err := Load([]string{parent})
	if err != nil {
		t.Fatalf("load quoted graph: %v", err)
	}
	if len(graph.Includes) != 1 || graph.Includes[0].Optional || graph.Includes[0].ResolvedPath != child {
		t.Fatalf("quoted include = %+v", graph.Includes)
	}
	document, ok := graph.DocumentFor(parent)
	if !ok || string(document.Bytes()) != string(contents) {
		t.Fatalf("parent document was not preserved: %q", document.Bytes())
	}
}

func TestLoadRejectsInvalidRoot(t *testing.T) {
	_, err := Load([]string{filepath.Join(t.TempDir(), "missing.ghostty")})
	if err == nil || !strings.Contains(err.Error(), "config root") {
		t.Fatalf("unexpected invalid-root result: %v", err)
	}
}

func writeGraphConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create graph directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write graph config: %v", err)
	}
}
