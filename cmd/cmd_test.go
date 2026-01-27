package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestScanCommand(t *testing.T) {
	_ = scanCmd.Flags().Set("root", "")
	_ = scanCmd.Flags().Set("out", "")
	if err := scanCmd.RunE(scanCmd, nil); err == nil {
		t.Fatalf("expected missing root/out error")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	out := filepath.Join(t.TempDir(), "state.fic")
	_ = scanCmd.Flags().Set("root", root)
	_ = scanCmd.Flags().Set("out", out)
	_ = scanCmd.Flags().Set("workers", "1")
	_ = scanCmd.Flags().Set("algo", "sha256")
	_ = scanCmd.Flags().Set("progress", "false")
	_ = scanCmd.Flags().Set("follow-symlinks", "false")
	if err := scanCmd.RunE(scanCmd, nil); err != nil {
		t.Fatalf("scan RunE: %v", err)
	}
}

func TestCompareCommand(t *testing.T) {
	_ = compareCmd.Flags().Set("left", "")
	_ = compareCmd.Flags().Set("right", "")
	if err := compareCmd.RunE(compareCmd, nil); err == nil {
		t.Fatalf("expected missing left/right error")
	}

	left := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	right := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	out := filepath.Join(t.TempDir(), "out.txt")
	_ = compareCmd.Flags().Set("left", left)
	_ = compareCmd.Flags().Set("right", right)
	_ = compareCmd.Flags().Set("format", "text")
	_ = compareCmd.Flags().Set("out", out)
	if err := compareCmd.RunE(compareCmd, nil); err != nil {
		t.Fatalf("compare RunE: %v", err)
	}

	_ = compareCmd.Flags().Set("format", "nope")
	if err := compareCmd.RunE(compareCmd, nil); err == nil {
		t.Fatalf("expected bad format error")
	}
}

func TestViewCommand(t *testing.T) {
	_ = viewCmd.Flags().Set("state", "")
	if err := viewCmd.RunE(viewCmd, nil); err == nil {
		t.Fatalf("expected missing state error")
	}

	state := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	_ = viewCmd.Flags().Set("state", state)
	_ = viewCmd.Flags().Set("only-done", "true")
	_ = viewCmd.Flags().Set("format", "text")
	if err := viewCmd.RunE(viewCmd, nil); err != nil {
		t.Fatalf("view RunE: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	out := captureStdout(t, func() {
		if err := versionCmd.RunE(versionCmd, nil); err != nil {
			t.Fatalf("version RunE: %v", err)
		}
	})
	if !strings.Contains(out, "FIC (Filesystem Integrity Checker) - version") {
		t.Fatalf("expected version prefix")
	}
	if !strings.Contains(out, "https://github.com/jeduardo/fic/") {
		t.Fatalf("expected repo URL")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()

	_ = w.Close()
	os.Stdout = old
	<-done
	_ = r.Close()

	return buf.String()
}

func TestExecute(t *testing.T) {
	oldExit := exitFunc
	defer func() {
		exitFunc = oldExit
	}()

	calls := 0
	exitFunc = func(code int) {
		calls = code
	}

	rootCmd.SetArgs([]string{"--bad-flag"})
	Execute()
	if calls != 1 {
		t.Fatalf("expected exit code 1, got %d", calls)
	}

	calls = 0
	rootCmd.SetArgs([]string{"--help"})
	Execute()
	if calls != 0 {
		t.Fatalf("expected no exit on success")
	}
}

func TestSetExitFuncForTest(t *testing.T) {
	restore := SetExitFuncForTest(func(int) {})
	restore()
}
