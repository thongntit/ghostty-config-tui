package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
