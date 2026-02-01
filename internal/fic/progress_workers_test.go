package fic

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerStatusHelpers(t *testing.T) {
	workers := make([]atomic.Value, 2)
	if got := workerStatusValue(0, workers); got != "" {
		t.Fatalf("expected empty worker status, got %q", got)
	}
	if got := workerStatusValue(1, workers); got != "" {
		t.Fatalf("expected empty worker status out of range, got %q", got)
	}
	workers[1].Store(struct{}{})
	if got := workerStatusValue(1, workers); got != "" {
		t.Fatalf("expected empty worker status for non-string, got %q", got)
	}
	workers[0].Store("[H] file.txt")
	if got := workerStatusValue(0, workers); got != "[H] file.txt" {
		t.Fatalf("unexpected worker status: %q", got)
	}

	status, path := parseWorkerStatus("")
	if status != "I" || path != "idle" {
		t.Fatalf("expected idle status, got %q %q", status, path)
	}
	status, path = parseWorkerStatus("[H] file.txt")
	if status != "H" || path != "file.txt" {
		t.Fatalf("expected hash status, got %q %q", status, path)
	}
	status, path = parseWorkerStatus("[D]file.txt")
	if status != "D" || path != "file.txt" {
		t.Fatalf("expected done status without space, got %q %q", status, path)
	}
	status, path = parseWorkerStatus("weird")
	if status != "?" || path != "weird" {
		t.Fatalf("expected fallback status, got %q %q", status, path)
	}
}

func TestWorkerLineFormatting(t *testing.T) {
	display1 := workerDisplay{name: "Worker 1", status: "H", path: "a.txt"}
	display2 := workerDisplay{name: "Worker 12", status: "D", path: "b.txt"}

	line1 := formatWorkerLine(display1, len("Worker 12"), 80)
	line2 := formatWorkerLine(display2, len("Worker 12"), 80)
	if !strings.Contains(line1, "Worker 1  [H]:") {
		t.Fatalf("expected padded worker label, got %q", line1)
	}
	if !strings.Contains(line2, "Worker 12 [D]:") {
		t.Fatalf("expected aligned worker label, got %q", line2)
	}

	if got := formatWorkerLine(display1, len("Worker 12"), 3); got != "[H]" {
		t.Fatalf("expected compact worker prefix, got %q", got)
	}
	if got := formatWorkerLine(display1, len("Worker 12"), 4); got != "[H]:" {
		t.Fatalf("expected compact worker prefix with colon, got %q", got)
	}
}

func TestBuildWorkerProgressLines(t *testing.T) {
	start := time.Now().Add(-time.Second)
	workers := make([]atomic.Value, 2)
	workers[0].Store("[H] a.txt")
	workers[1].Store("[D] b.txt")

	lines := buildWorkerProgressLines(1, 2, "Hashing", start, workers, 80, 4)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
	if lines[2] != "" {
		t.Fatalf("expected blank separator line")
	}
	if !strings.Contains(lines[3], "Hashing: 1/2") {
		t.Fatalf("expected progress line, got %q", lines[3])
	}

	lines = buildWorkerProgressLines(1, 2, "Hashing", start, workers, 80, 2)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines with limited height, got %d", len(lines))
	}

	lines = buildWorkerProgressLines(1, 2, "Hashing", start, workers, 80, 1)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line with minimal height, got %d", len(lines))
	}
}

func TestProgressWorkerUtilities(t *testing.T) {
	if got := displayRows([]string{"123456", ""}, 5); got != 3 {
		t.Fatalf("expected 3 display rows, got %d", got)
	}
	if got := truncateLine("abcdef", 3); got != "abc" {
		t.Fatalf("unexpected truncate result: %q", got)
	}
	if got := truncateLine("abc", 0); got != "abc" {
		t.Fatalf("expected no truncation on width 0, got %q", got)
	}
	if got := padLine("abc", 5); got != "abc  " {
		t.Fatalf("unexpected pad result: %q", got)
	}

	tmp, err := os.CreateTemp(t.TempDir(), "terminal")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer func() {
		_ = tmp.Close()
	}()
	if w, h := terminalSize(tmp); w != 0 || h != 0 {
		t.Fatalf("expected zero size for non-terminal, got %d x %d", w, h)
	}
}

func TestWorkerProgressPrinter(t *testing.T) {
	start := time.Now().Add(-time.Second)
	workers := make([]atomic.Value, 1)
	workers[0].Store("[H] a.txt")
	done := newAtomicInt64(1)

	output := captureStderr(t, func() {
		stopFn := startWorkerProgress(context.Background(), done, 1, "Hashing", start, nil)
		if stopFn == nil {
			t.Fatalf("expected stop function")
		}
		time.Sleep(10 * time.Millisecond)
		stopFn()
	})
	if !strings.Contains(output, "Hashing") {
		t.Fatalf("expected progress output, got %q", output)
	}

	output = captureStderr(t, func() {
		stop := make(chan struct{})
		finished := make(chan struct{})
		renderer := &workerProgressRenderer{prevLines: []string{"prev", "line"}}
		go progressPrinterWithWorkers(done, 2, "Hashing", start, stop, finished, workers, renderer)
		time.Sleep(10 * time.Millisecond)
		close(stop)
		<-finished
	})
	if !strings.Contains(output, "Hashing") {
		t.Fatalf("expected worker progress output, got %q", output)
	}
}

func TestTruncateWorkerPath(t *testing.T) {
	if got := truncateWorkerPath("abc", 0); got != "" {
		t.Fatalf("expected empty when maxLen=0, got %q", got)
	}
	if got := truncateWorkerPath("abc", 2); got != "ab" {
		t.Fatalf("expected short truncation, got %q", got)
	}
	if got := truncateWorkerPath("abcdefgh", 5); got != "...gh" {
		t.Fatalf("expected ellipsis truncation, got %q", got)
	}
}
