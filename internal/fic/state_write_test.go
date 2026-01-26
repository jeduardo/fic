package fic

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWriteStateFileAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.fic")

	header := HeaderRecord{
		Type:           "header",
		Version:        stateVersion,
		Root:           "/tmp",
		Algo:           "sha256",
		CreatedAt:      "2024-01-01 12:00:00",
		FollowSymlinks: false,
	}
	files := []FileEntry{
		{Path: "a.txt", Size: 1},
		{Path: "b.txt", Size: 2},
	}
	hashes := []string{"aaa", "bbb"}
	errs := []string{"", "missing"}

	if err := WriteStateFile(path, header, files, hashes, errs, true); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if !st.Completed {
		t.Fatalf("expected completed")
	}
	if len(st.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(st.Files))
	}
	if st.Files[0].Hash != "aaa" || st.Files[1].Err != "missing" {
		t.Fatalf("unexpected file data: %+v", st.Files)
	}
}

func TestWriteStateFileMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.fic")
	header := HeaderRecord{Type: "header", Version: stateVersion}
	if err := WriteStateFile(path, header, []FileEntry{}, []string{"a"}, []string{}, true); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestWriteStateFileCreateTempError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope", "state.fic")
	header := HeaderRecord{Type: "header", Version: stateVersion}
	err := WriteStateFile(path, header, []FileEntry{}, []string{}, []string{}, false)
	if err == nil {
		t.Fatalf("expected create temp error")
	}
}

func TestWriteStateFileRenameError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	header := HeaderRecord{Type: "header", Version: stateVersion}
	err := WriteStateFile(dir, header, []FileEntry{}, []string{}, []string{}, false)
	if err == nil {
		t.Fatalf("expected rename error")
	}
}

func TestLoadStateUnknownType(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"nope","data":1}`,
	})
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected unknown record error")
	}
}

func TestLoadStateInvalidRecord(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{`,
	})
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected invalid record error")
	}
}

func TestLoadStateMissingHeader(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
	})
	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if st.HeaderPresent {
		t.Fatalf("expected missing header")
	}
}

func TestLoadStatePendingErrorAndCheckpoint(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"error","path":"b.txt","error":"missing"}`,
		`{"type":"checkpoint","completed_count":1,"done_b64":"AA==","updated_at":"2024-01-01T00:00:01Z"}`,
		`{"type":"file","path":"b.txt","size":2,"hash":"bbb"}`,
	})
	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if len(st.Files) != 1 || st.Files[0].Err != "missing" {
		t.Fatalf("expected pending error to attach")
	}
}

func TestWriteStateFileIncomplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.fic")
	header := HeaderRecord{
		Type:      "header",
		Version:   stateVersion,
		Root:      "/tmp",
		Algo:      "sha256",
		CreatedAt: "2024-01-01 12:00:00",
	}
	files := []FileEntry{{Path: "a.txt", Size: 1}}
	hashes := []string{""}
	errs := []string{""}
	if err := WriteStateFile(path, header, files, hashes, errs, false); err != nil {
		t.Fatalf("WriteStateFile: %v", err)
	}
	st, err := loadState(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if st.Completed {
		t.Fatalf("expected not completed")
	}
	if st.Files[0].Hash != "" {
		t.Fatalf("expected empty hash")
	}
}

func TestLoadStateWithProgress(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":"a.txt","size":1,"hash":"aaa"}`,
		`{"type":"completed","completed_at":"2024-01-01T00:00:01Z"}`,
	})
	var count atomic.Int64
	st, err := loadState(context.Background(), path, &count)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 records, got %d", count.Load())
	}
	if !st.Completed {
		t.Fatalf("expected completed")
	}
}

func TestLoadStateInvalidFileRecord(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"file","path":1,"size":1}`,
	})
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected invalid file record error")
	}
}

func TestLoadStateInvalidErrorRecord(t *testing.T) {
	path := writeStateFile(t, []string{
		`{"type":"header","version":1,"root":"/tmp","algo":"sha256","created_at":"2024-01-01 12:00:00","follow_symlinks":false}`,
		`{"type":"error","path":1,"error":"bad"}`,
	})
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected invalid error record error")
	}
}

func TestLoadStateTooLongLine(t *testing.T) {
	path := writeLargeLineFile(t)
	if _, err := loadState(context.Background(), path, nil); err == nil {
		t.Fatalf("expected scanner error")
	}
}

func TestWriteRecord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() {
		_ = f.Close()
	}()
	enc := json.NewEncoder(f)
	if err := writeRecord(enc, map[string]string{"type": "header"}); err != nil {
		t.Fatalf("writeRecord: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"type":"header"`) {
		t.Fatalf("expected record output")
	}
}

func TestWriteStateFileWriteRecordErrors(t *testing.T) {
	header := HeaderRecord{
		Type:      "header",
		Version:   stateVersion,
		Root:      "/tmp",
		Algo:      "sha256",
		CreatedAt: "2024-01-01 12:00:00",
	}
	files := []FileEntry{{Path: "a.txt", Size: 1}}

	withWriteRecordFn(t, func(_ *json.Encoder, _ any) error {
		return errors.New("boom")
	}, func() {
		if err := WriteStateFile(filepath.Join(t.TempDir(), "out.fic"), header, files, []string{""}, []string{""}, false); err == nil {
			t.Fatalf("expected header write error")
		}
	})

	calls := 0
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		calls++
		if calls == 1 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := WriteStateFile(filepath.Join(t.TempDir(), "out.fic"), header, files, []string{""}, []string{""}, false); err == nil {
			t.Fatalf("expected file record write error")
		}
	})

	calls = 0
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		calls++
		if calls < 3 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := WriteStateFile(filepath.Join(t.TempDir(), "out.fic"), header, files, []string{""}, []string{"missing"}, false); err == nil {
			t.Fatalf("expected error record write error")
		}
	})

	calls = 0
	withWriteRecordFn(t, func(enc *json.Encoder, v any) error {
		calls++
		if calls < 3 {
			return writeRecord(enc, v)
		}
		return errors.New("boom")
	}, func() {
		if err := WriteStateFile(filepath.Join(t.TempDir(), "out.fic"), header, files, []string{""}, []string{""}, true); err == nil {
			t.Fatalf("expected completed write error")
		}
	})
}
