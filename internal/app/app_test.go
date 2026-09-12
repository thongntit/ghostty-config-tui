package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thongntit/ghostty-config-tui/internal/configgraph"
	"github.com/thongntit/ghostty-config-tui/internal/schema"
)

func TestNewLoadsExplicitFixtureWithoutGhostty(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "config", "basic.ghostty")
	originalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	model, err := New(Options{ConfigPath: path})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if !bytes.Equal(model.ui.OriginalBytes(), originalBytes) {
		t.Fatal("loaded source did not match fixture")
	}
	if !bytes.Equal(model.ui.DraftBytes(), originalBytes) {
		t.Fatal("initial draft did not match fixture")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reread fixture: %v", err)
	}
	if !bytes.Equal(unchanged, originalBytes) {
		t.Fatal("loading changed the fixture")
	}
	if got := model.ui.Status(); got != "Loaded explicit config: "+path {
		t.Fatalf("startup status = %q, want explicit-path status", got)
	}
}

func TestNewDiscoversUserConfig(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	path := filepath.Join(xdg, "ghostty", "config.ghostty")
	originalBytes := []byte("theme = dark\n")
	writeTestConfig(t, path, originalBytes)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	model, err := New(Options{})
	if err != nil {
		t.Fatalf("new app with discovered config: %v", err)
	}
	if !bytes.Equal(model.ui.OriginalBytes(), originalBytes) {
		t.Fatalf("discovered source did not match config")
	}
	if got, want := model.ui.Status(), "Loaded discovered config: "+path; got != want {
		t.Fatalf("startup status = %q, want %q", got, want)
	}
}

func TestNewUsesHighestPrecedenceUserConfigAndExplainsMultipleFiles(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	modern := filepath.Join(xdg, "ghostty", "config.ghostty")
	legacy := filepath.Join(xdg, "ghostty", "config")
	writeTestConfig(t, modern, []byte("theme = dark\n"))
	legacyBytes := []byte("theme = light\n")
	writeTestConfig(t, legacy, legacyBytes)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	model, err := New(Options{})
	if err != nil {
		t.Fatalf("new app with multiple configs: %v", err)
	}
	if !bytes.Equal(model.ui.OriginalBytes(), legacyBytes) {
		t.Fatalf("selected source was not the highest-precedence config")
	}
	wantStatus := "Loaded effective config through " + legacy + " (2 default roots)"
	if got := model.ui.Status(); got != wantStatus {
		t.Fatalf("startup status = %q, want %q", got, wantStatus)
	}
}

func TestNewReportsNoDiscoveredConfig(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	_, err := New(Options{})
	if err == nil || !strings.Contains(err.Error(), "no Ghostty config found") {
		t.Fatalf("unexpected no-config result: %v", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(xdg, "ghostty", "config.ghostty")) || !strings.Contains(err.Error(), "--config PATH") {
		t.Fatalf("no-config error is not actionable: %v", err)
	}
}

func TestNewExplicitPathDoesNotFallBackToDiscoveredConfig(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	writeTestConfig(t, filepath.Join(xdg, "ghostty", "config.ghostty"), []byte("theme = dark\n"))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	explicit := filepath.Join(t.TempDir(), "missing.ghostty")
	_, err := New(Options{ConfigPath: explicit})
	if err == nil || !strings.Contains(err.Error(), explicit) {
		t.Fatalf("explicit missing path unexpectedly succeeded: %v", err)
	}
}

func TestOptionsWithUnknownsAddsEachCustomKeyOnce(t *testing.T) {
	options := []schema.Option{{Key: "theme"}}
	graph := configgraph.Graph{Assignments: []configgraph.Assignment{
		{Key: "theme"},
		{Key: "custom-option"},
		{Key: "custom-option"},
	}}

	got := optionsWithUnknowns(options, graph)
	if len(got) != 2 || got[1].Key != "custom-option" || got[1].Category != "Custom" {
		t.Fatalf("options with unknowns = %+v", got)
	}
}

func TestLoadConfigRejectsMissingPathAndInvalidUTF8(t *testing.T) {
	if _, _, err := LoadConfig(""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("unexpected empty-path result: %v", err)
	}

	path := filepath.Join(t.TempDir(), "invalid.ghostty")
	if err := os.WriteFile(path, []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}
	if _, _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("unexpected invalid-UTF8 result: %v", err)
	}
}

func TestLoadConfigRejectsDirectory(t *testing.T) {
	path := t.TempDir()
	if _, _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("unexpected directory result: %v", err)
	}
}

func writeTestConfig(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
