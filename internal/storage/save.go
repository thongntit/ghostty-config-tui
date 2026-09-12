// Package storage will own guarded config persistence.
package storage

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// SHA256 returns a content fingerprint used for concurrent-edit detection.
func SHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// CheckUnchanged verifies that path still contains the expected bytes.
func CheckUnchanged(path string, expected [32]byte) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if SHA256(data) != expected {
		return fmt.Errorf("config changed while it was being edited: %s", path)
	}
	return nil
}

// FileChange is one candidate source document. Original is used for
// concurrent-edit detection; Candidate is written only when it differs.
type FileChange struct {
	Path      string
	Original  []byte
	Candidate []byte
}

// SaveAll writes changed config files with a backup and a same-directory
// atomic replacement. All candidates are checked before the first target is
// replaced. If a later replacement fails, already-replaced files are restored
// from their captured originals.
func SaveAll(changes []FileChange) error {
	prepared := make([]preparedChange, 0, len(changes))
	seen := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if change.Path == "" {
			return fmt.Errorf("config save path is empty")
		}
		if bytes.Equal(change.Original, change.Candidate) {
			continue
		}
		target, info, err := resolveTarget(change.Path)
		if err != nil {
			return err
		}
		identity, err := filepath.Abs(target)
		if err != nil {
			identity = filepath.Clean(target)
		}
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("config save target appears more than once: %s", target)
		}
		seen[identity] = struct{}{}
		current, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("read config before save %q: %w", target, err)
		}
		if SHA256(current) != SHA256(change.Original) {
			return fmt.Errorf("config changed while it was being edited: %s", change.Path)
		}
		prepared = append(prepared, preparedChange{
			FileChange: change,
			target:     target,
			mode:       info.Mode().Perm(),
			backup:     target + ".bak",
		})
	}
	if len(prepared) == 0 {
		return nil
	}

	for index := range prepared {
		if err := prepareTemp(&prepared[index]); err != nil {
			cleanupTemps(prepared)
			return err
		}
	}

	for index := range prepared {
		change := &prepared[index]
		if err := writeAtomic(change.backup, change.Original, change.mode); err != nil {
			cleanupTemps(prepared)
			return fmt.Errorf("write backup %q: %w", change.backup, err)
		}
	}

	replaced := 0
	for index := range prepared {
		change := &prepared[index]
		if err := os.Rename(change.temp, change.target); err != nil {
			rollbackErr := rollback(prepared[:replaced])
			cleanupTemps(prepared)
			if rollbackErr != nil {
				return fmt.Errorf("replace %q: %w (rollback failed: %v)", change.target, err, rollbackErr)
			}
			return fmt.Errorf("replace %q: %w", change.target, err)
		}
		change.temp = ""
		replaced++
	}
	if err := syncDirectories(prepared); err != nil {
		return err
	}
	return nil
}

type preparedChange struct {
	FileChange
	target string
	mode   os.FileMode
	backup string
	temp   string
}

func resolveTarget(path string) (string, os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, fmt.Errorf("inspect config save target %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", nil, fmt.Errorf("config save target is not a regular file: %q", path)
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolve config save target %q: %w", path, err)
	}
	resolvedInfo, err := os.Stat(target)
	if err != nil || !resolvedInfo.Mode().IsRegular() {
		if err != nil {
			return "", nil, fmt.Errorf("inspect resolved config save target %q: %w", target, err)
		}
		return "", nil, fmt.Errorf("resolved config save target is not a regular file: %q", target)
	}
	return target, resolvedInfo, nil
}

func prepareTemp(change *preparedChange) error {
	temporary, err := os.CreateTemp(filepath.Dir(change.target), ".ghostty-config-tui-*")
	if err != nil {
		return fmt.Errorf("create temporary config for %q: %w", change.target, err)
	}
	change.temp = temporary.Name()
	if err := temporary.Chmod(change.mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set temporary config permissions for %q: %w", change.target, err)
	}
	if _, err := temporary.Write(change.Candidate); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary config for %q: %w", change.target, err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary config for %q: %w", change.target, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config for %q: %w", change.target, err)
	}
	return nil
}

func writeAtomic(path string, contents []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to replace symlink %q", path)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ghostty-config-tui-backup-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func rollback(changes []preparedChange) error {
	var firstErr error
	for index := len(changes) - 1; index >= 0; index-- {
		change := changes[index]
		if err := writeAtomic(change.target, change.Original, change.mode); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func cleanupTemps(changes []preparedChange) {
	for _, change := range changes {
		if change.temp != "" {
			_ = os.Remove(change.temp)
		}
	}
}

func syncDirectories(changes []preparedChange) error {
	seen := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		directory := filepath.Dir(change.target)
		if _, exists := seen[directory]; exists {
			continue
		}
		seen[directory] = struct{}{}
		directoryFile, err := os.Open(directory)
		if err != nil {
			return fmt.Errorf("open config directory %q: %w", directory, err)
		}
		err = directoryFile.Sync()
		closeErr := directoryFile.Close()
		if err != nil {
			return fmt.Errorf("sync config directory %q: %w", directory, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close config directory %q: %w", directory, closeErr)
		}
	}
	return nil
}
