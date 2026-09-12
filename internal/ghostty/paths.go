package ghostty

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigCandidates returns Ghostty's supported config locations in load order.
func ConfigCandidates() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}

	candidates := []string{
		filepath.Join(configHome, "ghostty", "config.ghostty"),
		filepath.Join(configHome, "ghostty", "config"),
	}
	if runtime.GOOS == "darwin" {
		appSupport := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
		candidates = append(candidates,
			filepath.Join(appSupport, "config.ghostty"),
			filepath.Join(appSupport, "config"),
		)
	}
	return candidates
}

// ExistingConfig returns the first regular config file found in load order.
func ExistingConfig() string {
	for _, candidate := range ConfigCandidates() {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}
