package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStateFile(t *testing.T) {
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "state.fic")
	header := HeaderRecord{
		Type:           "header",
		Version:        Version,
		Root:           "/tmp",
		Algo:           "sha256",
		CreatedAt:      "2026-02-01T00:00:00Z",
		FollowSymlinks: false,
	}
	files := []FileEntry{
		{Path: "a.txt", Size: 1},
		{Path: "b.txt", Size: 2},
	}
	hashes := []string{"aa", "bb"}
	errs := []string{"", "oops"}

	if err := WriteStateFile(outPath, header, files, hashes, errs, true); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty output file")
	}
}

func TestWriteStateFileErrors(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "missing", "state.fic")
	header := HeaderRecord{Type: "header", Version: Version, Root: "/tmp", Algo: "sha256", CreatedAt: "2026-02-01T00:00:00Z"}
	files := []FileEntry{{Path: "a.txt", Size: 1}}

	if err := WriteStateFile(outPath, header, files, []string{}, []string{}, false); err == nil {
		t.Fatalf("expected length mismatch error")
	}
	if err := WriteStateFile(outPath, header, files, []string{"aa"}, []string{""}, false); err == nil {
		t.Fatalf("expected error for missing directory")
	}
}
