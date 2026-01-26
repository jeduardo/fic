package fic

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCompareWarnsOnIncomplete(t *testing.T) {
	left := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	right := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	output := captureStderr(t, func() {
		if err := RunCompare(left, right, "text", ""); err != nil {
			t.Fatalf("RunCompare: %v", err)
		}
	})
	if !strings.Contains(output, "warning: one or both scans are not completed") {
		t.Fatalf("expected warning output")
	}
}

func TestRunCompareBadFormat(t *testing.T) {
	left := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	right := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	if err := RunCompare(left, right, "nope", ""); err == nil {
		t.Fatalf("expected format error")
	}
}

func TestRunCompareNoLeftEntries(t *testing.T) {
	left := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	right := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	output := captureStdout(t, func() {
		if err := RunCompare(left, right, "json", ""); err != nil {
			t.Fatalf("RunCompare: %v", err)
		}
	})
	if !strings.Contains(output, `"diffs": []`) && !strings.Contains(output, `"diffs":[]`) {
		t.Fatalf("expected empty diffs")
	}
}

func TestWriteOutputTextToStdout(t *testing.T) {
	diffs := []Diff{{Path: "a.txt", Status: "only_left"}}
	output := captureStdout(t, func() {
		if err := writeOutputText(diffs, ""); err != nil {
			t.Fatalf("writeOutputText: %v", err)
		}
	})
	if !strings.Contains(output, "ONLY_LEFT a.txt") {
		t.Fatalf("expected output line")
	}
}

func TestRunCompareMismatchAndPending(t *testing.T) {
	left := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/left","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":1}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	right := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/right","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"bbb"}`,
		`{"type":"file","path":"b.txt","size":1}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})

	output := captureStdout(t, func() {
		if err := RunCompare(left, right, "text", ""); err != nil {
			t.Fatalf("RunCompare: %v", err)
		}
	})
	if !strings.Contains(output, "MISMATCH a.txt aaa bbb") {
		t.Fatalf("expected mismatch output")
	}
	if !strings.Contains(output, "PENDING b.txt") {
		t.Fatalf("expected pending output")
	}
}

func TestWriteOutputTextCreateError(t *testing.T) {
	diffs := []Diff{{Path: "a.txt", Status: "only_left"}}
	if err := writeOutputText(diffs, filepath.Join(t.TempDir(), "nope", "out.txt")); err == nil {
		t.Fatalf("expected create error")
	}
}

func TestWriteOutputJSONCreateError(t *testing.T) {
	report := CompareReport{Left: "left", Right: "right"}
	if err := writeOutputJSON(report, filepath.Join(t.TempDir(), "nope", "out.json")); err == nil {
		t.Fatalf("expected create error")
	}
}

func TestBuildPathMapErrorEntry(t *testing.T) {
	st := &State{
		Files: []FileEntry{
			{Path: "a.txt", Err: "missing"},
		},
	}
	m := buildPathMap(st)
	v := m["a.txt"]
	if !v.HasErr || v.Err != "missing" {
		t.Fatalf("expected error entry")
	}
}
