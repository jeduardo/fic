package fic

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildFileListWithUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are unreliable on windows")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	badDir := filepath.Join(root, "nope")
	if err := os.Mkdir(badDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(badDir, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() {
		_ = os.Chmod(badDir, 0700)
	}()

	files, err := buildFileList(context.Background(), root, false, nil)
	if err != nil {
		t.Fatalf("buildFileList: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Err != "" && strings.Contains(f.Path, "nope") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unreadable path to be recorded as error entry")
	}
}

func TestHashFileAlgorithms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte("abc"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	gotSHA, err := hashFile(context.Background(), path, "sha256")
	if err != nil {
		t.Fatalf("sha256 hashFile: %v", err)
	}
	if gotSHA != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("unexpected sha256: %s", gotSHA)
	}

	gotMD5, err := hashFile(context.Background(), path, "md5")
	if err != nil {
		t.Fatalf("md5 hashFile: %v", err)
	}
	if gotMD5 != "900150983cd24fb0d6963f7d28e17f72" {
		t.Fatalf("unexpected md5: %s", gotMD5)
	}
}
