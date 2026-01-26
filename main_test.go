package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduardo/fic/cmd"
	"github.com/jeduardo/fic/internal/fic"
)

func TestMainRuns(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = []string{"fic", "--help"}
	main()
}

func TestCLIRoundTrip(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	root := t.TempDir()
	if err := os.WriteFile(root+"/a.txt", []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	state := root + "/state.fic"
	out := root + "/compare.txt"

	os.Args = []string{"fic", "scan", "--root", root, "--out", state, "--workers", "1"}
	cmd.Execute()

	os.Args = []string{"fic", "view", "--state", state, "--only-done"}
	cmd.Execute()

	os.Args = []string{"fic", "compare", "--left", state, "--right", state, "--format", "text", "--out", out}
	cmd.Execute()

}

func TestCLIErrorPaths(t *testing.T) {
	restore := cmd.SetExitFuncForTest(func(int) {})
	defer restore()

	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	os.Args = []string{"fic", "scan"}
	cmd.Execute()

	os.Args = []string{"fic", "compare"}
	cmd.Execute()

	os.Args = []string{"fic", "compare", "--left", "a", "--right", "b", "--format", "nope"}
	cmd.Execute()

	os.Args = []string{"fic", "view"}
	cmd.Execute()

}

func TestInternalCoveragePaths(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	broken := filepath.Join(root, "broken.txt")
	if err := os.Symlink(filepath.Join(root, "missing.txt"), broken); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	state := filepath.Join(root, "state.fic")
	if err := fic.RunScan(fic.ScanOptions{
		Root:           root,
		StatePath:      state,
		Algo:           "sha256",
		Workers:        1,
		Progress:       true,
		FollowSymlinks: true,
	}); err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	if err := fic.RunView(state, false, "text"); err != nil {
		t.Fatalf("RunView text: %v", err)
	}
	if err := fic.RunView(state, false, "json"); err != nil {
		t.Fatalf("RunView json: %v", err)
	}
	if err := fic.RunView(state, false, "nope"); err == nil {
		t.Fatalf("expected bad format error")
	}

	leftMismatch := filepath.Join(root, "left.fic")
	rightMismatch := filepath.Join(root, "right.fic")
	header := fic.HeaderRecord{
		Type:      "header",
		Version:   1,
		Root:      root,
		Algo:      "sha256",
		CreatedAt: "2024-01-01 12:00:00",
	}
	files := []fic.FileEntry{{Path: "a.txt", Size: 1}}
	if err := fic.WriteStateFile(leftMismatch, header, files, []string{"aaa"}, []string{""}, true); err != nil {
		t.Fatalf("WriteStateFile left: %v", err)
	}
	header.Algo = "md5"
	if err := fic.WriteStateFile(rightMismatch, header, files, []string{"bbb"}, []string{""}, true); err != nil {
		t.Fatalf("WriteStateFile right: %v", err)
	}
	if err := fic.RunCompare(leftMismatch, rightMismatch, "text", ""); err == nil {
		t.Fatalf("expected algo mismatch")
	}

	rightPending := filepath.Join(root, "right-pending.fic")
	header.Algo = "sha256"
	if err := fic.WriteStateFile(rightPending, header, files, []string{""}, []string{""}, true); err != nil {
		t.Fatalf("WriteStateFile pending: %v", err)
	}
	if err := fic.RunCompare(leftMismatch, rightPending, "text", ""); err != nil {
		t.Fatalf("RunCompare: %v", err)
	}
	if err := fic.RunCompare(leftMismatch, rightPending, "nope", ""); err == nil {
		t.Fatalf("expected format error")
	}

	if err := fic.WriteStateFile(filepath.Join(root, "nope", "bad.fic"), header, files, []string{""}, []string{""}, true); err == nil {
		t.Fatalf("expected write state error")
	}
	if err := fic.WriteStateFile(filepath.Join(root, "mismatch.fic"), header, files, []string{}, []string{}, true); err == nil {
		t.Fatalf("expected length mismatch error")
	}
}

func writeStateFile(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.fic")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}
