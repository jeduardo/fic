package fic

import (
	"context"
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

func TestLoadState(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":2}`,
		`{"type":"error","path":"b.txt","error":"missing"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if !st.HeaderPresent || st.Header.Root != "/tmp" {
		t.Fatalf("unexpected header: %+v", st.Header)
	}
	if !st.Completed {
		t.Fatalf("expected completed")
	}
	if len(st.Files) < 2 {
		t.Fatalf("expected 2 files, got %d", len(st.Files))
	}
	if st.Files[0].Hash != "aaa" {
		t.Fatalf("expected hash for first file")
	}
	if st.Files[1].Err != "missing" {
		t.Fatalf("expected error for second file")
	}
}

func TestLoadStateFileRecordHash(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"abc"}`,
	})

	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if st.Files[0].Hash != "abc" {
		t.Fatalf("expected hash to be loaded from file record")
	}
}

func TestCompareDiffs(t *testing.T) {
	leftPath := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"hash-a"}`,
		`{"type":"file","path":"b.txt","size":1,"hash":"hash-b"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	rightPath := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"hash-a"}`,
		`{"type":"file","path":"c.txt","size":1,"hash":"hash-c"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	outPath := filepath.Join(t.TempDir(), "report.json")
	if err := RunCompare(leftPath, rightPath, "json", outPath); err != nil {
		t.Fatalf("runCompare: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report CompareReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if len(report.Diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(report.Diffs))
	}
}

func TestCompareAlgoMismatch(t *testing.T) {
	leftPath := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"hash-a"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	rightPath := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha1","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"hash-a"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	if err := RunCompare(leftPath, rightPath, "text", ""); err == nil {
		t.Fatalf("expected algo mismatch error")
	}
}

func TestLoadStateVersionMismatch(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":2,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected version mismatch error")
	}
}
