package fic

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestRunScanValidation(t *testing.T) {
	if err := RunScan(ScanOptions{Workers: 0, Algo: "sha256", Root: "x", StatePath: "y"}); err == nil {
		t.Fatalf("expected workers error")
	}
	if err := RunScan(ScanOptions{Workers: 1, Algo: "nope", Root: "x", StatePath: "y"}); err == nil {
		t.Fatalf("expected algo error")
	}
	if err := RunScan(ScanOptions{Workers: 1, Algo: "sha256"}); err == nil {
		t.Fatalf("expected missing root/out error")
	}
}

func TestRunScanBasic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatalf("write b: %v", err)
	}
	out := filepath.Join(t.TempDir(), "state.fic")
	opts := ScanOptions{
		Root:      root,
		StatePath: out,
		Algo:      "sha256",
		Workers:   2,
		Progress:  false,
	}
	if err := RunScan(opts); err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	st, err := loadState(context.Background(), out, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if !st.Completed || !st.HeaderPresent {
		t.Fatalf("expected completed scan")
	}
	if len(st.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(st.Files))
	}
	if st.Files[0].Hash == "" || st.Files[1].Hash == "" {
		t.Fatalf("expected hashes")
	}
}

func TestBuildFileListSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	files, err := buildFileList(context.Background(), root, false, nil)
	if err != nil {
		t.Fatalf("buildFileList: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file without following symlinks, got %d", len(files))
	}

	files, err = buildFileList(context.Background(), root, true, nil)
	if err != nil {
		t.Fatalf("buildFileList: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files with symlink follow, got %d", len(files))
	}

	dirTarget := filepath.Join(root, "dir")
	if err := os.Mkdir(dirTarget, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dirLink := filepath.Join(root, "dir-link")
	if err := os.Symlink(dirTarget, dirLink); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	files, err = buildFileList(context.Background(), root, true, nil)
	if err != nil {
		t.Fatalf("buildFileList: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected symlink to dir to be skipped, got %d", len(files))
	}
}

func TestHashFileErrors(t *testing.T) {
	if _, err := hashFile(context.Background(), "/nope", "sha256"); err == nil {
		t.Fatalf("expected error for missing file")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte("a"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := hashFile(context.Background(), path, "nope"); err == nil {
		t.Fatalf("expected unsupported algo error")
	}
	if _, err := hashFile(ctx, path, "sha256"); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestProgressHelpers(t *testing.T) {
	start := time.Now().Add(-time.Second)
	output := captureStderr(t, func() {
		printProgressLine(5, 10, "Hashing", start)
	})
	if !strings.Contains(output, "Hashing: 5/10") {
		t.Fatalf("unexpected progress output: %s", output)
	}

	workerStatus := make([]atomic.Value, 2)
	for i := range workerStatus {
		workerStatus[i].Store("")
	}
	workerStatus[0].Store("[H] a.txt")
	workerStatus[1].Store("[D] b.txt")
	output = formatWorkerProgress(1, 3, "Hashing", start, workerStatus, 80, 4)
	if !strings.Contains(output, "Worker 1 [H]:") || !strings.Contains(output, "a.txt") {
		t.Fatalf("expected worker status output, got: %s", output)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	output = captureStderr(t, func() {
		go progressPrinterWithLabel(newAtomicInt64(1), 3, "Hashing", start, stop, done)
		time.Sleep(10 * time.Millisecond)
		close(stop)
		<-done
	})
	if !strings.Contains(output, "Hashing") {
		t.Fatalf("expected progress output")
	}

	stop = make(chan struct{})
	output = captureStderr(t, func() {
		go listProgressPrinter(newAtomicInt64(2), stop)
		time.Sleep(600 * time.Millisecond)
		close(stop)
		time.Sleep(20 * time.Millisecond)
	})
	if !strings.Contains(output, "Indexing") {
		t.Fatalf("expected indexing output")
	}
}

func TestRunScanProgressAndBrokenSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	broken := filepath.Join(root, "broken.txt")
	if err := os.Symlink(filepath.Join(root, "missing.txt"), broken); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	out := filepath.Join(t.TempDir(), "state.fic")

	output := captureStderr(t, func() {
		opts := ScanOptions{
			Root:           root,
			StatePath:      out,
			Algo:           "sha256",
			Workers:        1,
			Progress:       true,
			FollowSymlinks: true,
		}
		if err := RunScan(opts); err != nil {
			t.Fatalf("RunScan: %v", err)
		}
	})
	if !strings.Contains(output, "Indexing") || !strings.Contains(output, "Hashing") {
		t.Fatalf("expected progress output")
	}

	st, err := loadState(context.Background(), out, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	foundErr := false
	for _, f := range st.Files {
		if f.Path == "broken.txt" && f.Err != "" {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Fatalf("expected broken symlink to be recorded as error")
	}
}

func TestBuildFileListContextHandling(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if _, err := buildFileList(context.TODO(), root, false, nil); err != nil {
		t.Fatalf("buildFileList with nil ctx: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := buildFileList(ctx, root, false, nil); err == nil {
		t.Fatalf("expected canceled context error")
	}
}

func TestRunScanUnreadableFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(path, []byte("secret"), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer func() {
		_ = os.Chmod(path, 0600)
	}()
	if _, err := os.Open(path); err == nil {
		t.Skip("unreadable file not enforced for current user")
	}

	out := filepath.Join(t.TempDir(), "state.fic")
	if err := RunScan(ScanOptions{
		Root:      root,
		StatePath: out,
		Algo:      "sha256",
		Workers:   1,
	}); err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	st, err := loadState(context.Background(), out, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if len(st.Files) != 1 || st.Files[0].Err == "" {
		t.Fatalf("expected error entry for unreadable file")
	}
}

func TestBuildFileListNonRegular(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mkfifo not supported on windows")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	fifo := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Skipf("mkfifo failed: %v", err)
	}
	files, err := buildFileList(context.Background(), root, false, nil)
	if err != nil {
		t.Fatalf("buildFileList: %v", err)
	}
	if len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("expected fifo to be skipped")
	}
}

func newAtomicInt64(v int64) *atomic.Int64 {
	var a atomic.Int64
	a.Store(v)
	return &a
}
