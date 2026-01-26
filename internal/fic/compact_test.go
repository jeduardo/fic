package fic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompactStateWritesCleanFile(t *testing.T) {
	input := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"file","path":"b.txt","size":2}`,
		`{"type":"error","path":"b.txt","error":"missing"}`,
		`{"type":"checkpoint","completed_count":1,"done_b64":"AA==","updated_at":"2024-01-01T00:00:01Z"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:02Z"}`,
	})

	out := filepath.Join(t.TempDir(), "out.fic")
	if err := CompactState(context.Background(), input, out); err != nil {
		t.Fatalf("CompactState: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(data), `"checkpoint"`) {
		t.Fatalf("expected checkpoints to be removed")
	}

	st, err := loadState(context.Background(), out, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if !st.HeaderPresent || !st.Completed {
		t.Fatalf("expected header and completed")
	}
	if len(st.Files) != 2 || st.Files[1].Err == "" {
		t.Fatalf("unexpected compacted state")
	}
}

func TestCompactStateInPlaceWithProgress(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	output := captureStderr(t, func() {
		if err := CompactStateWithProgress(context.Background(), path, "", true); err != nil {
			t.Fatalf("CompactStateWithProgress: %v", err)
		}
	})
	if !strings.Contains(output, "Compacting") {
		t.Fatalf("expected compacting output")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected in-place file to exist: %v", err)
	}
}

func TestCompactStateMissingHeader(t *testing.T) {
	input := writeStateFile(t, []string{
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	out := filepath.Join(t.TempDir(), "out.fic")
	if err := CompactState(context.Background(), input, out); err == nil {
		t.Fatalf("expected missing header error")
	}
}

func TestCompactStateNoCompleted(t *testing.T) {
	input := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	out := filepath.Join(t.TempDir(), "out.fic")
	if err := CompactState(context.Background(), input, out); err != nil {
		t.Fatalf("CompactState: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(data), `"completed"`) {
		t.Fatalf("expected no completed record")
	}
}

func TestCompactStateWithContext(t *testing.T) {
	input := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	out := filepath.Join(t.TempDir(), "out.fic")
	if err := compactState(context.TODO(), input, out, false); err != nil {
		t.Fatalf("compactState: %v", err)
	}
}

func TestCountStateRecords(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	count, err := countStateRecords(context.Background(), path)
	if err != nil {
		t.Fatalf("countStateRecords: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 records, got %d", count)
	}
}

func TestCountStateRecordsMissingFile(t *testing.T) {
	if _, err := countStateRecords(context.Background(), filepath.Join(t.TempDir(), "nope.fic")); err == nil {
		t.Fatalf("expected open error")
	}
}

func TestCountStateRecordsTooLongLine(t *testing.T) {
	path := writeLargeLineFile(t)
	if _, err := countStateRecords(context.Background(), path); err == nil {
		t.Fatalf("expected scanner error")
	}
}

func TestCountStateRecordsCanceled(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := countStateRecords(ctx, path); err == nil {
		t.Fatalf("expected canceled context error")
	}
}

func TestCountCompactRecords(t *testing.T) {
	st := &State{
		Files: []FileEntry{
			{Path: "a.txt", Size: 1},
			{Path: "b.txt", Size: 2, Err: "missing"},
		},
		Completed: true,
	}
	if got := countCompactRecords(st); got != 4 {
		t.Fatalf("expected 4 records, got %d", got)
	}
}

func TestWriteFileRecordsParallelEmpty(t *testing.T) {
	st := &State{}
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	done := newAtomicInt64(0)
	if err := writeFileRecordsParallel(context.Background(), st, writer, done, 0, nil, nil, false); err != nil {
		t.Fatalf("writeFileRecordsParallel: %v", err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
}

func TestCompactStateProgressCanceled(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CompactStateWithProgress(ctx, path, "", true); err == nil {
		t.Fatalf("expected canceled context error")
	}
}

func TestCompactStateWriteRecordErrors(t *testing.T) {
	input := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"error","path":"a.txt","error":"missing"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	noErrorInput := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	out := filepath.Join(t.TempDir(), "out.fic")

	withWriteRecordFn(t, func(_ *json.Encoder, _ any) error {
		return errors.New("boom")
	}, func() {
		if err := compactState(context.Background(), input, out, false); err == nil {
			t.Fatalf("expected header write error")
		}
	})

	var calls atomic.Int32
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		if calls.Add(1) == 1 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := compactState(context.Background(), input, out, false); err == nil {
			t.Fatalf("expected file record write error")
		}
	})

	calls.Store(0)
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		if calls.Add(1) < 3 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := compactState(context.Background(), input, out, false); err == nil {
			t.Fatalf("expected error record write error")
		}
	})

	calls.Store(0)
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		if calls.Add(1) < 3 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := compactState(context.Background(), noErrorInput, out, false); err == nil {
			t.Fatalf("expected completed record write error")
		}
	})
}
