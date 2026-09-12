package main

import (
	"strings"
	"testing"

	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func TestParseVersionIgnoresDiagnosticPrefix(t *testing.T) {
	got, err := parseVersion([]byte("error: SentryInitFailed\nGhostty 1.3.2-main-+abc\n"))
	if err != nil {
		t.Fatalf("parse version: %v", err)
	}
	if got != "1.3.2-main-+abc" {
		t.Fatalf("version = %q", got)
	}
}

func TestSourceRevisionExtractsBuildRevision(t *testing.T) {
	if got := sourceRevision("1.3.2-main-+44f2a44df"); got != "44f2a44df" {
		t.Fatalf("source revision = %q", got)
	}
	if got := sourceRevision("1.3.2"); got != "" {
		t.Fatalf("release source revision = %q, want empty", got)
	}
}

func TestCatalogFromDocsDeduplicatesAndClassifiesOptions(t *testing.T) {
	docs := []byte("# A theme.\ntheme =\n\n# Toggle it.\nenable-feature = true\n\n# Font size.\nfont-size = 13\n\n# Additional configuration files to read.\nconfig-file = ?optional.conf\n\n# Binding.\nkeybind = ctrl+a=ignore\n\n# duplicate docs entry\ntheme =\n")
	catalog, err := catalogFromDocs(docs, "test")
	if err != nil {
		t.Fatalf("catalog from docs: %v", err)
	}
	if len(catalog.Options) != 5 {
		t.Fatalf("options = %d, want 5", len(catalog.Options))
	}
	byKey := make(map[string]schema.Option, len(catalog.Options))
	for _, option := range catalog.Options {
		byKey[option.Key] = option
	}
	if byKey["enable-feature"].Kind != schema.KindBoolean {
		t.Fatalf("boolean classification = %q", byKey["enable-feature"].Kind)
	}
	if byKey["config-file"].Kind != schema.KindRepeatable {
		t.Fatalf("config-file classification = %q", byKey["config-file"].Kind)
	}
	if byKey["keybind"].Kind != schema.KindKeybind {
		t.Fatalf("keybind classification = %q", byKey["keybind"].Kind)
	}
	if byKey["keybind"].Edit != schema.EditRepeatable || byKey["theme"].Edit != schema.EditScalar {
		t.Fatalf("editor classification = keybind:%q theme:%q", byKey["keybind"].Edit, byKey["theme"].Edit)
	}
	if byKey["font-size"].Step == nil || *byKey["font-size"].Step != 0.5 {
		t.Fatalf("font-size step = %+v, want 0.5", byKey["font-size"].Step)
	}
	if !strings.Contains(byKey["theme"].Docs, "#theme") {
		t.Fatalf("theme docs link = %q", byKey["theme"].Docs)
	}
}
