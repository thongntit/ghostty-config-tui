package ghostty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCandidatesUsesXDGAndPlatformOrder(t *testing.T) {
	home := filepath.FromSlash("/home/tester")
	xdg := filepath.FromSlash("/custom/config")

	linuxCandidates := configCandidates(home, xdg, "linux")
	if want, got := 2, len(linuxCandidates); got != want {
		t.Fatalf("linux candidate count = %d, want %d", got, want)
	}
	if got, want := linuxCandidates[0], filepath.Join(xdg, "ghostty", "config.ghostty"); got != want {
		t.Fatalf("linux modern candidate = %q, want %q", got, want)
	}
	if got, want := linuxCandidates[1], filepath.Join(xdg, "ghostty", "config"); got != want {
		t.Fatalf("linux legacy candidate = %q, want %q", got, want)
	}

	darwinCandidates := configCandidates(home, xdg, "darwin")
	wantDarwin := []string{
		filepath.Join(xdg, "ghostty", "config.ghostty"),
		filepath.Join(xdg, "ghostty", "config"),
		filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config.ghostty"),
		filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config"),
	}
	if len(darwinCandidates) != len(wantDarwin) {
		t.Fatalf("darwin candidates = %v, want %v", darwinCandidates, wantDarwin)
	}
	for index := range wantDarwin {
		if darwinCandidates[index] != wantDarwin[index] {
			t.Errorf("darwin candidate %d = %q, want %q", index, darwinCandidates[index], wantDarwin[index])
		}
	}
}

func TestConfigCandidatesDefaultsToHomeConfigDirectory(t *testing.T) {
	home := filepath.FromSlash("/home/tester")
	candidates := configCandidates(home, "", "linux")
	wantRoot := filepath.Join(home, ".config", "ghostty")
	if got := filepath.Dir(candidates[0]); got != wantRoot {
		t.Fatalf("default config directory = %q, want %q", got, wantRoot)
	}
}

func TestDiscoverConfigSelectsHighestPrecedenceExistingFile(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "config.ghostty")
	second := filepath.Join(root, "config")
	writeConfigFile(t, first, "theme = dark\n")
	writeConfigFile(t, second, "theme = light\n")

	selection, err := discoverConfig([]string{first, second})
	if err != nil {
		t.Fatalf("discover config: %v", err)
	}
	if got, want := selection.Selected, second; got != want {
		t.Fatalf("selected config = %q, want %q", got, want)
	}
	if len(selection.Existing) != 2 {
		t.Fatalf("existing configs = %v, want both candidates", selection.Existing)
	}
}

func TestDiscoverConfigAcceptsSymlinkToRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real-config")
	candidate := filepath.Join(root, "config.ghostty")
	writeConfigFile(t, target, "theme = dark\n")
	if err := os.Symlink(target, candidate); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	selection, err := discoverConfig([]string{candidate})
	if err != nil {
		t.Fatalf("discover symlink config: %v", err)
	}
	if got := selection.Selected; got != candidate {
		t.Fatalf("selected symlink path = %q, want %q", got, candidate)
	}
}

func TestDiscoverConfigRejectsBrokenSymlink(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "config.ghostty")
	if err := os.Symlink(filepath.Join(filepath.Dir(candidate), "missing"), candidate); err != nil {
		t.Fatalf("create broken symlink: %v", err)
	}

	_, err := discoverConfig([]string{candidate})
	if err == nil || !strings.Contains(err.Error(), "broken symlink") {
		t.Fatalf("unexpected broken symlink error: %v", err)
	}
}

func TestDiscoverConfigRejectsNonRegularFile(t *testing.T) {
	candidate := filepath.Join(t.TempDir(), "config.ghostty")
	if err := os.Mkdir(candidate, 0o700); err != nil {
		t.Fatalf("create directory candidate: %v", err)
	}

	_, err := discoverConfig([]string{candidate})
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("unexpected non-regular error: %v", err)
	}
}

func TestDiscoverConfigReportsCheckedPathsWhenNoneExist(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")

	selection, err := discoverConfig([]string{first, second})
	if err == nil || !strings.Contains(err.Error(), "no Ghostty config found") {
		t.Fatalf("unexpected no-config error: %v", err)
	}
	if !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), second) {
		t.Fatalf("no-config error did not list checked paths: %v", err)
	}
	if len(selection.Existing) != 0 || selection.Selected != "" {
		t.Fatalf("empty selection = %+v", selection)
	}
}

func writeConfigFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
}
