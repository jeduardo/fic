package fic

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

type workerProgressRenderer struct {
	prevLines []string
}

type workerDisplay struct {
	name   string
	status string
	path   string
}

func startWorkerProgress(ctx context.Context, done *atomic.Int64, total int, label string, start time.Time, workers []atomic.Value) func() {
	if total == 0 {
		return nil
	}
	if !isTerminal(os.Stderr) || len(workers) == 0 {
		stop := make(chan struct{})
		finished := make(chan struct{})
		go progressPrinterWithLabel(done, total, label, start, stop, finished)
		return func() {
			close(stop)
			<-finished
			printProgressLine(done.Load(), total, label, start)
		}
	}
	stop := make(chan struct{})
	finished := make(chan struct{})
	renderer := &workerProgressRenderer{}
	go progressPrinterWithWorkers(done, total, label, start, stop, finished, workers, renderer)

	var stopOnce sync.Once
	stopFn := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}
	if ctx != nil {
		go func() {
			<-ctx.Done()
			stopFn()
		}()
	}
	return func() {
		stopFn()
		<-finished
	}
}

func progressPrinterWithWorkers(done *atomic.Int64, total int, label string, start time.Time, stop <-chan struct{}, finished chan<- struct{}, workers []atomic.Value, renderer *workerProgressRenderer) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	renderer.render(done.Load(), total, label, start, workers)
	for {
		select {
		case <-stop:
			renderer.render(done.Load(), total, label, start, workers)
			if finished != nil {
				close(finished)
			}
			return
		case <-ticker.C:
			renderer.render(done.Load(), total, label, start, workers)
		}
	}
}

func (r *workerProgressRenderer) render(completed int64, total int, label string, start time.Time, workers []atomic.Value) {
	width, height := terminalSize(os.Stderr)
	lines := buildWorkerProgressLines(completed, total, label, start, workers, width, height)

	// Disable line wrapping to avoid accidental extra lines on narrow terminals.
	fmt.Fprint(os.Stderr, "\x1b[?7l")
	defer fmt.Fprint(os.Stderr, "\x1b[?7h")

	prevRows := displayRows(r.prevLines, width)
	if prevRows > 0 {
		fmt.Fprint(os.Stderr, "\r")
		if prevRows > 1 {
			fmt.Fprintf(os.Stderr, "\033[%dA", prevRows-1)
		}
	}

	for i, line := range lines {
		fmt.Fprint(os.Stderr, "\r\033[2K")
		fmt.Fprint(os.Stderr, line)
		if i < len(lines)-1 {
			fmt.Fprint(os.Stderr, "\n")
		}
	}

	newRows := displayRows(lines, width)
	if prevRows > newRows {
		extra := prevRows - newRows
		for i := 0; i < extra; i++ {
			fmt.Fprint(os.Stderr, "\n\r\033[2K")
		}
		fmt.Fprintf(os.Stderr, "\033[%dA", extra)
	}

	r.prevLines = lines
}

func buildWorkerProgressLines(completed int64, total int, label string, start time.Time, workers []atomic.Value, width, height int) []string {
	progress := formatProgressLine(completed, total, label, start)
	progress = truncateLine(progress, width)

	if height <= 1 {
		return []string{progress}
	}

	workerLines := len(workers)
	includeBlank := workerLines > 0
	if height > 0 {
		switch {
		case height <= 1:
			workerLines = 0
			includeBlank = false
		case height == 2:
			includeBlank = false
			if workerLines > 1 {
				workerLines = 1
			}
		default:
			maxWorkers := height - 2
			if workerLines > maxWorkers {
				workerLines = maxWorkers
			}
		}
	}

	lines := make([]string, 0, workerLines+2)
	displays := make([]workerDisplay, 0, workerLines)
	maxNameLen := 0
	for i := 0; i < workerLines; i++ {
		status, path := parseWorkerStatus(workerStatusValue(i, workers))
		name := fmt.Sprintf("Worker %d", i+1)
		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
		displays = append(displays, workerDisplay{name: name, status: status, path: path})
	}
	for i := 0; i < workerLines; i++ {
		lines = append(lines, formatWorkerLine(displays[i], maxNameLen, width))
	}
	if includeBlank && workerLines > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, progress)
	return lines
}

func formatWorkerProgress(completed int64, total int, label string, start time.Time, workers []atomic.Value, width, height int) string {
	lines := buildWorkerProgressLines(completed, total, label, start, workers, width, height)
	return strings.Join(lines, "\n")
}

func formatWorkerLine(display workerDisplay, nameWidth int, width int) string {
	prefixName := display.name
	if nameWidth > len(prefixName) {
		prefixName += strings.Repeat(" ", nameWidth-len(prefixName))
	}
	prefix := fmt.Sprintf("%s [%s]: ", prefixName, display.status)
	path := display.path
	if width <= 0 {
		return prefix + path
	}
	if width <= 2 {
		return strings.Repeat(" ", width)
	}
	if width <= len(prefix) {
		return padLine(compactWorkerPrefix(display, width), width)
	}
	maxPathLen := width - len(prefix) - 2
	if maxPathLen < 0 {
		maxPathLen = 0
	}
	line := prefix + truncateWorkerPath(path, maxPathLen)
	return padLine(line, width)
}

func workerStatusValue(index int, workers []atomic.Value) string {
	if index < 0 || index >= len(workers) {
		return ""
	}
	val := workers[index].Load()
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

func parseWorkerStatus(raw string) (string, string) {
	if raw == "" {
		return "I", "idle"
	}
	if len(raw) >= 4 && raw[0] == '[' && raw[2] == ']' && raw[3] == ' ' {
		status := raw[1:2]
		path := raw[4:]
		if path == "" {
			path = "idle"
		}
		return status, path
	}
	if len(raw) >= 3 && raw[0] == '[' && raw[2] == ']' {
		status := raw[1:2]
		path := raw[3:]
		if path == "" {
			path = "idle"
		}
		return status, path
	}
	return "?", raw
}

func truncateLine(line string, width int) string {
	if width <= 0 || len(line) <= width {
		return line
	}
	return line[:width]
}

func padLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	if len(line) >= width {
		return line[:width]
	}
	return line + strings.Repeat(" ", width-len(line))
}

func compactWorkerPrefix(display workerDisplay, width int) string {
	if width <= 0 {
		return ""
	}
	status := display.status
	if status == "" {
		status = "?"
	}
	min := fmt.Sprintf("[%s]", status)
	if width <= len(min) {
		return min[:width]
	}
	min += ":"
	if width <= len(min) {
		return min[:width]
	}
	return min
}

func displayRows(lines []string, width int) int {
	if len(lines) == 0 {
		return 0
	}
	if width <= 0 {
		return len(lines)
	}
	rows := 0
	for _, line := range lines {
		if line == "" {
			rows++
			continue
		}
		length := len(line)
		if length <= width {
			rows++
			continue
		}
		quot := length / width
		rem := length % width
		rows += quot
		if rem > 0 {
			rows++
		}
	}
	return rows
}

func terminalSize(file *os.File) (int, int) {
	if !isTerminal(file) {
		return 0, 0
	}
	w, h, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0, 0
	}
	return w, h
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
