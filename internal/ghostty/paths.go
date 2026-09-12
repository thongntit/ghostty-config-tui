package ghostty

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ConfigSelection describes the default config files found for the current
// user. Selected is the last existing candidate because Ghostty loads later
// config files with higher precedence.
type ConfigSelection struct {
	Candidates []string
	Existing   []string
	Selected   string
}

// ConfigCandidates returns Ghostty's supported config locations in load order.
func ConfigCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}

	return configCandidates(home, os.Getenv("XDG_CONFIG_HOME"), runtime.GOOS)
}

func configCandidates(home, xdgConfigHome, goos string) []string {
	configHome := xdgConfigHome
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	candidates := []string{
		filepath.Join(configHome, "ghostty", "config.ghostty"),
		filepath.Join(configHome, "ghostty", "config"),
	}
	if goos == "darwin" {
		appSupport := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
		candidates = append(candidates,
			filepath.Join(appSupport, "config.ghostty"),
			filepath.Join(appSupport, "config"),
		)
	}
	return candidates
}

// DiscoverConfig finds the default Ghostty config files without reading their
// contents. It follows symlinks to regular files, preserves the candidate
// path for display, and reports invalid existing candidates instead of
// silently falling back to another file.
func DiscoverConfig() (ConfigSelection, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		if err == nil {
			err = fmt.Errorf("home directory is empty")
		}
		return ConfigSelection{}, fmt.Errorf("cannot determine user home directory: %w", err)
	}

	candidates := configCandidates(home, os.Getenv("XDG_CONFIG_HOME"), runtime.GOOS)
	return discoverConfig(candidates)
}

func discoverConfig(candidates []string) (ConfigSelection, error) {
	selection := ConfigSelection{
		Candidates: append([]string(nil), candidates...),
	}
	for _, candidate := range candidates {
		linkInfo, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return selection, fmt.Errorf("inspect Ghostty config %q: %w", candidate, err)
		}

		info, err := os.Stat(candidate)
		if err != nil {
			if linkInfo.Mode()&os.ModeSymlink != 0 && os.IsNotExist(err) {
				return selection, fmt.Errorf("Ghostty config path is a broken symlink: %q", candidate)
			}
			return selection, fmt.Errorf("inspect Ghostty config %q: %w", candidate, err)
		}
		if !info.Mode().IsRegular() {
			return selection, fmt.Errorf("Ghostty config path is not a regular file: %q", candidate)
		}
		selection.Existing = append(selection.Existing, candidate)
	}

	if len(selection.Existing) == 0 {
		return selection, fmt.Errorf("no Ghostty config found; checked: %s; create one or pass --config PATH", strings.Join(selection.Candidates, ", "))
	}
	selection.Selected = selection.Existing[len(selection.Existing)-1]
	return selection, nil
}

// ExistingConfig returns the highest-precedence regular config file found in
// the default search locations. It is retained as a small compatibility
// helper; callers that need diagnostics should use DiscoverConfig.
func ExistingConfig() string {
	selection, err := DiscoverConfig()
	if err != nil {
		return ""
	}
	return selection.Selected
}
