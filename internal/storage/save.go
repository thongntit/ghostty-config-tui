// Package storage will own guarded config persistence.
package storage

import (
	"crypto/sha256"
	"fmt"
	"os"
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
