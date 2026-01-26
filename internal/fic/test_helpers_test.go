package fic

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func writeLargeLineFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "large.fic")
	size := 11 * 1024 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = 'a'
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatalf("write large file: %v", err)
	}
	return path
}

func withWriteRecordFn(t *testing.T, fn func(*json.Encoder, any) error, testFn func()) {
	t.Helper()
	old := writeRecordFn
	writeRecordFn = fn
	defer func() {
		writeRecordFn = old
	}()
	testFn()
}
