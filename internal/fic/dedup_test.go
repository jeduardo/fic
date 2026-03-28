package fic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDedupJSON(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"b.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"same-hash-different-size.txt","size":2,"hash":"aaa"}`,
		`{"type":"file","path":"err.txt","size":1,"hash":"bbb"}`,
		`{"type":"error","path":"err.txt","error":"read failed"}`,
		`{"type":"file","path":"pending.txt","size":1}`,
		`{"type":"file","path":"z.txt","size":3,"hash":"ccc"}`,
		`{"type":"file","path":"y.txt","size":3,"hash":"ccc"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	output := captureStdout(t, func() {
		if err := RunDedup(path, "json", ""); err != nil {
			t.Fatalf("RunDedup: %v", err)
		}
	})

	var report DedupReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.State != path {
		t.Fatalf("unexpected state path: %s", report.State)
	}
	if report.Algo != "sha256" {
		t.Fatalf("unexpected algorithm: %s", report.Algo)
	}
	if len(report.Duplicates) != 2 {
		t.Fatalf("expected 2 duplicate groups, got %d", len(report.Duplicates))
	}
	if report.Duplicates[0].Hash != "aaa" || report.Duplicates[0].Size != 1 {
		t.Fatalf("unexpected first duplicate group: %+v", report.Duplicates[0])
	}
	if got := strings.Join(report.Duplicates[0].Paths, ","); got != "a.txt,b.txt" {
		t.Fatalf("unexpected sorted duplicate paths: %s", got)
	}
	if report.Duplicates[1].Hash != "ccc" || report.Duplicates[1].Size != 3 {
		t.Fatalf("unexpected second duplicate group: %+v", report.Duplicates[1])
	}
}

func TestRunDedupText(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"copy/a file.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"a file.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	output := captureStdout(t, func() {
		if err := RunDedup(path, "text", ""); err != nil {
			t.Fatalf("RunDedup: %v", err)
		}
	})
	if !strings.Contains(output, "DUPLICATE aaa size=1 count=2\n") {
		t.Fatalf("unexpected header output: %s", output)
	}
	if !strings.Contains(output, "PATH a file.txt\n") {
		t.Fatalf("missing first path line: %s", output)
	}
	if !strings.Contains(output, "PATH copy/a file.txt\n") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestRunDedupWarnsOnIncomplete(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":1,"hash":"aaa"}`,
	})

	output := captureStderr(t, func() {
		if err := RunDedup(path, "text", ""); err != nil {
			t.Fatalf("RunDedup: %v", err)
		}
	})
	if !strings.Contains(output, "warning: scan is not completed") {
		t.Fatalf("expected warning output")
	}
}

func TestRunDedupBadFormat(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	if err := RunDedup(path, "nope", ""); err == nil {
		t.Fatalf("expected format error")
	}
}

func TestRunDedupMissingHeader(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	if err := RunDedup(path, "text", ""); err == nil {
		t.Fatalf("expected missing header error")
	}
}

func TestRunDedupMissingStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.fic")
	if err := RunDedup(path, "text", ""); err == nil {
		t.Fatalf("expected missing state file error")
	}
}

func TestBuildDuplicateGroupsSortsAndFilters(t *testing.T) {
	st := &State{
		Files: []FileEntry{
			{Path: "single.txt", Size: 1, Hash: "single"},
			{Path: "skip-error.txt", Size: 1, Hash: "skip", Err: "read failed"},
			{Path: "skip-pending.txt", Size: 1},
			{Path: "z-last.txt", Size: 5, Hash: "bbb"},
			{Path: "a-first.txt", Size: 5, Hash: "bbb"},
			{Path: "size-2-b.txt", Size: 2, Hash: "aaa"},
			{Path: "size-2-a.txt", Size: 2, Hash: "aaa"},
			{Path: "size-1-b.txt", Size: 1, Hash: "aaa"},
			{Path: "size-1-a.txt", Size: 1, Hash: "aaa"},
		},
	}

	groups := buildDuplicateGroups(st)
	if len(groups) != 3 {
		t.Fatalf("expected 3 duplicate groups, got %d", len(groups))
	}

	if groups[0].Hash != "aaa" || groups[0].Size != 1 {
		t.Fatalf("unexpected first group: %+v", groups[0])
	}
	if got := strings.Join(groups[0].Paths, ","); got != "size-1-a.txt,size-1-b.txt" {
		t.Fatalf("unexpected first group paths: %s", got)
	}

	if groups[1].Hash != "aaa" || groups[1].Size != 2 {
		t.Fatalf("unexpected second group: %+v", groups[1])
	}
	if got := strings.Join(groups[1].Paths, ","); got != "size-2-a.txt,size-2-b.txt" {
		t.Fatalf("unexpected second group paths: %s", got)
	}

	if groups[2].Hash != "bbb" || groups[2].Size != 5 {
		t.Fatalf("unexpected third group: %+v", groups[2])
	}
	if got := strings.Join(groups[2].Paths, ","); got != "a-first.txt,z-last.txt" {
		t.Fatalf("unexpected third group paths: %s", got)
	}
}

func TestWriteDedupTextToFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dedup.txt")
	groups := []DuplicateGroup{
		{Hash: "aaa", Size: 1, Paths: []string{"a.txt", "b.txt"}},
	}
	if err := writeDedupText(groups, out); err != nil {
		t.Fatalf("writeDedupText: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "DUPLICATE aaa size=1 count=2\nPATH a.txt\nPATH b.txt\n") {
		t.Fatalf("unexpected file output: %s", string(data))
	}
}

func TestWriteDedupTextCreateError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "missing", "dedup.txt")
	groups := []DuplicateGroup{
		{Hash: "aaa", Size: 1, Paths: []string{"a.txt", "b.txt"}},
	}
	if err := writeDedupText(groups, out); err == nil {
		t.Fatalf("expected create error")
	}
}

func TestWriteDedupTextStdoutError(t *testing.T) {
	group := DuplicateGroup{Hash: "aaa", Size: 1, Paths: []string{"a.txt", "b.txt"}}
	withClosedStdout(t, func() {
		if err := writeDedupText([]DuplicateGroup{group}, ""); err == nil {
			t.Fatalf("expected stdout error")
		}
	})
}

func TestWriteDedupTextWriterPathError(t *testing.T) {
	group := DuplicateGroup{Hash: "aaa", Size: 1, Paths: []string{"a.txt", "b.txt"}}
	writer := &failAfterWritesWriter{failAfter: 1}
	if err := writeDedupTextWriter(writer, []DuplicateGroup{group}); err == nil {
		t.Fatalf("expected path write error")
	}
}

func TestWriteDedupTextWriterSeparatorError(t *testing.T) {
	group := DuplicateGroup{Hash: "aaa", Size: 1, Paths: []string{"a.txt"}}
	writer := &failAfterWritesWriter{failAfter: 2}
	if err := writeDedupTextWriter(writer, []DuplicateGroup{group}); err == nil {
		t.Fatalf("expected separator write error")
	}
}

func TestWriteDedupTextCloseError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "dedup.txt")
	groups := []DuplicateGroup{
		{Hash: "aaa", Size: 1, Paths: []string{"a.txt", "b.txt"}},
	}

	oldCloseFileFn := closeFileFn
	closeFileFn = func(file *os.File) error {
		_ = oldCloseFileFn(file)
		return errors.New("close failed")
	}
	defer func() {
		closeFileFn = oldCloseFileFn
	}()

	if err := writeDedupText(groups, out); err == nil {
		t.Fatalf("expected close error")
	}
}

type failAfterWritesWriter struct {
	writes    int
	failAfter int
}

func (w *failAfterWritesWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("boom")
	}
	w.writes++
	return len(p), nil
}
