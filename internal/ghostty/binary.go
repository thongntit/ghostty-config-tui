package ghostty

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Find returns a usable Ghostty executable path when one is discoverable.
// An empty result is expected on machines that do not have Ghostty installed.
func Find() string {
	if path, err := exec.LookPath("ghostty"); err == nil {
		return path
	}

	if runtime.GOOS != "darwin" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, path := range []string{
		"/Applications/Ghostty.app/Contents/MacOS/ghostty",
		filepath.Join(home, "Applications", "Ghostty.app", "Contents", "MacOS", "ghostty"),
	} {
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
