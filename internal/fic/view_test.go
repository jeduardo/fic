package fic

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunViewJSON(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":2}`,
		`{"type":"error","path":"b.txt","error":"missing"}`,
	})

	output := captureStdout(t, func() {
		if err := RunView(path, false, "json"); err != nil {
			t.Fatalf("RunView: %v", err)
		}
	})

	scanner := bufio.NewScanner(strings.NewReader(output))
	entries := make(map[string]ViewEntry)
	for scanner.Scan() {
		var entry ViewEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("unmarshal view entry: %v", err)
		}
		entries[entry.Path] = entry
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan output: %v", err)
	}

	if got := entries["a.txt"].Status; got != "done" {
		t.Fatalf("expected a.txt done, got %s", got)
	}
	if got := entries["b.txt"].Status; got != "error" {
		t.Fatalf("expected b.txt error, got %s", got)
	}
	if entries["b.txt"].Error == "" {
		t.Fatalf("expected b.txt error message")
	}
}

func TestRunViewTextOnlyDone(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":2}`,
		`{"type":"error","path":"b.txt","error":"missing"}`,
		`{"type":"file","path":"c.txt","size":3}`,
	})

	output := captureStdout(t, func() {
		if err := RunView(path, true, "text"); err != nil {
			t.Fatalf("RunView: %v", err)
		}
	})

	if strings.Contains(output, "c.txt") {
		t.Fatalf("expected pending entries to be omitted")
	}
	if !strings.Contains(output, "a.txt aaa") {
		t.Fatalf("expected hash output for a.txt")
	}
	if !strings.Contains(output, "b.txt <error: missing>") {
		t.Fatalf("expected error output for b.txt")
	}
}

func TestRunViewTextPending(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"pending.txt","size":1}`,
	})
	output := captureStdout(t, func() {
		if err := RunView(path, false, "text"); err != nil {
			t.Fatalf("RunView: %v", err)
		}
	})
	if !strings.Contains(output, "pending.txt <pending>") {
		t.Fatalf("expected pending output")
	}
}

func TestRunViewBadFormat(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
	})
	if err := RunView(path, false, "nope"); err == nil {
		t.Fatalf("expected error for bad format")
	}
}

func TestRunViewMissingHeader(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	if err := RunView(path, false, "text"); err == nil {
		t.Fatalf("expected missing header error")
	}
}
