package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAllCreatesBackupAndPreservesMode(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.ghostty")
	original := []byte("theme = dark\n")
	candidate := []byte("theme = light\n")
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := SaveAll([]FileChange{{Path: path, Original: original, Candidate: candidate}}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read candidate: %v", err)
	}
	if !bytes.Equal(got, candidate) {
		t.Fatalf("candidate = %q, want %q", got, candidate)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatalf("backup = %q, want %q", backup, original)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat candidate: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Fatalf("candidate mode = %o, want %o", got, want)
	}
}

func TestSaveAllRejectsConcurrentChangeBeforeWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ghostty")
	if err := os.WriteFile(path, []byte("theme = changed\n"), 0o600); err != nil {
		t.Fatalf("write current: %v", err)
	}

	err := SaveAll([]FileChange{
		{Path: path, Original: []byte("theme = old\n"), Candidate: []byte("theme = new\n")},
	})
	if err == nil || !strings.Contains(err.Error(), "changed while it was being edited") {
		t.Fatalf("unexpected conflict result: %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read current after conflict: %v", readErr)
	}
	if string(got) != "theme = changed\n" {
		t.Fatalf("conflict changed file: %q", got)
	}
}

func TestSaveAllFollowsConfigSymlinkTarget(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "real.ghostty")
	link := filepath.Join(directory, "config.ghostty")
	original := []byte("font-size = 13\n")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if err := SaveAll([]FileChange{{Path: link, Original: original, Candidate: []byte("font-size = 14\n")}}); err != nil {
		t.Fatalf("save through symlink: %v", err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config symlink was replaced: info=%v err=%v", info, err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "font-size = 14\n" {
		t.Fatalf("target = %q", got)
	}
}
