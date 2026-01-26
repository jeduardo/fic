package fic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOutputText(t *testing.T) {
	diffs := []Diff{
		{Path: "a.txt", Status: "only_left"},
		{Path: "b.txt", Status: "only_right"},
		{Path: "c.txt", Status: "mismatch", Left: "aaa", Right: "bbb"},
		{Path: "d.txt", Status: "error", LeftError: "left", RightError: "right"},
		{Path: "e.txt", Status: "pending"},
	}

	out := filepath.Join(t.TempDir(), "out.txt")
	if err := writeOutputText(diffs, out); err != nil {
		t.Fatalf("writeOutputText: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "ONLY_LEFT a.txt") {
		t.Fatalf("missing only_left line")
	}
	if !strings.Contains(text, "ONLY_RIGHT b.txt") {
		t.Fatalf("missing only_right line")
	}
	if !strings.Contains(text, "MISMATCH c.txt aaa bbb") {
		t.Fatalf("missing mismatch line")
	}
	if !strings.Contains(text, "ERROR d.txt left=left right=right") {
		t.Fatalf("missing error line")
	}
	if !strings.Contains(text, "PENDING e.txt") {
		t.Fatalf("missing pending line")
	}
}

func TestWriteOutputJSONToStdout(t *testing.T) {
	report := CompareReport{
		Left:  "left.fic",
		Right: "right.fic",
		Diffs: []Diff{{Path: "a.txt", Status: "only_left"}},
	}

	output := captureStdout(t, func() {
		if err := writeOutputJSON(report, ""); err != nil {
			t.Fatalf("writeOutputJSON: %v", err)
		}
	})
	var decoded CompareReport
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if decoded.Left != "left.fic" || decoded.Right != "right.fic" {
		t.Fatalf("unexpected report metadata")
	}
	if len(decoded.Diffs) != 1 || decoded.Diffs[0].Path != "a.txt" {
		t.Fatalf("unexpected diffs")
	}
}

func TestWriteOutputTextStdoutErrors(t *testing.T) {
	cases := []Diff{
		{Path: "a.txt", Status: "only_left"},
		{Path: "b.txt", Status: "only_right"},
		{Path: "c.txt", Status: "mismatch", Left: "aaa", Right: "bbb"},
		{Path: "d.txt", Status: "error", LeftError: "left", RightError: "right"},
		{Path: "e.txt", Status: "pending"},
	}
	for _, diff := range cases {
		withClosedStdout(t, func() {
			if err := writeOutputText([]Diff{diff}, ""); err == nil {
				t.Fatalf("expected stdout error for %s", diff.Status)
			}
		})
	}
}

func TestWriteOutputJSONStdoutError(t *testing.T) {
	report := CompareReport{Left: "left", Right: "right"}
	withClosedStdout(t, func() {
		if err := writeOutputJSON(report, ""); err == nil {
			t.Fatalf("expected stdout error")
		}
	})
}
