package fic

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

const stateVersion = 1

// Records are stored as JSONL with a "type" field.
type HeaderRecord struct {
	Type           string `json:"type"`
	Version        int    `json:"version"`
	Root           string `json:"root"`
	Algo           string `json:"algo"`
	CreatedAt      string `json:"created_at"`
	FollowSymlinks bool   `json:"follow_symlinks"`
}

type FileRecord struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Hash string `json:"hash,omitempty"`
}

type ErrorRecord struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Error string `json:"error"`
}

type CheckpointRecord struct {
	Type           string `json:"type"`
	CompletedCount int    `json:"completed_count"`
	DoneB64        string `json:"done_b64"`
	UpdatedAt      string `json:"updated_at"`
}

type CompletedRecord struct {
	Type        string `json:"type"`
	CompletedAt string `json:"completed_at"`
}

type FileEntry struct {
	Path string
	Size int64
	Hash string
	Err  string
}

type State struct {
	Header        HeaderRecord
	HeaderPresent bool
	Files         []FileEntry
	Completed     bool
}

func writeRecord(enc *json.Encoder, v any) error {
	return enc.Encode(v)
}

func loadState(ctx context.Context, path string, progress *atomic.Int64) (*State, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	st := &State{}
	indexByPath := make(map[string]int)
	pendingErrors := make(map[string]string)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if progress != nil {
			progress.Add(1)
		}
		line := scanner.Bytes()
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			return nil, fmt.Errorf("invalid record: %w", err)
		}
		switch head.Type {
		case "header":
			var rec HeaderRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("invalid header: %w", err)
			}
			if rec.Version != stateVersion {
				return nil, fmt.Errorf("unsupported state version: %d", rec.Version)
			}
			st.Header = rec
			st.HeaderPresent = true
		case "file":
			var rec FileRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("invalid file record: %w", err)
			}
			entry := FileEntry{
				Path: rec.Path,
				Size: rec.Size,
				Hash: rec.Hash,
			}
			if errMsg, ok := pendingErrors[rec.Path]; ok {
				entry.Err = errMsg
				delete(pendingErrors, rec.Path)
			}
			indexByPath[rec.Path] = len(st.Files)
			st.Files = append(st.Files, entry)
		case "error":
			var rec ErrorRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, fmt.Errorf("invalid error record: %w", err)
			}
			if idx, ok := indexByPath[rec.Path]; ok {
				st.Files[idx].Err = rec.Error
			} else {
				pendingErrors[rec.Path] = rec.Error
			}
		case "completed":
			st.Completed = true
		case "checkpoint":
			// Optional for resume speed; we ignore for correctness.
		default:
			return nil, fmt.Errorf("unknown record type: %s", head.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return st, nil
}

func WriteStateFile(path string, header HeaderRecord, files []FileEntry, hashes []string, errs []string, completed bool) error {
	if len(hashes) != len(files) || len(errs) != len(files) {
		return fmt.Errorf("hash/error length mismatch")
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".fic.tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	bufw := bufio.NewWriter(tmpFile)
	enc := json.NewEncoder(bufw)

	if err := writeRecord(enc, header); err != nil {
		return err
	}
	for i, f := range files {
		rec := FileRecord{
			Type: "file",
			Path: f.Path,
			Size: f.Size,
		}
		if hashes[i] != "" {
			rec.Hash = hashes[i]
		}
		if err := writeRecord(enc, rec); err != nil {
			return err
		}
	}
	for i, errMsg := range errs {
		if errMsg == "" {
			continue
		}
		rec := ErrorRecord{
			Type:  "error",
			Path:  files[i].Path,
			Error: errMsg,
		}
		if err := writeRecord(enc, rec); err != nil {
			return err
		}
	}
	if completed {
		comp := CompletedRecord{Type: "completed", CompletedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if err := writeRecord(enc, comp); err != nil {
			return err
		}
	}
	if err := bufw.Flush(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
