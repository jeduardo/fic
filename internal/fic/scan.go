package fic

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type ScanOptions struct {
	Root           string
	StatePath      string
	Algo           string
	Workers        int
	Progress       bool
	FollowSymlinks bool
}

type scanJob struct {
	Index int
	Path  string
}

type scanResult struct {
	Index int
	Path  string
	Hash  string
	Err   error
}

func RunScan(opts ScanOptions) error {
	if opts.Workers < 1 {
		return errors.New("workers must be >= 1")
	}
	switch opts.Algo {
	case "sha256", "md5":
	default:
		return fmt.Errorf("unsupported algo: %s", opts.Algo)
	}
	if opts.Root == "" || opts.StatePath == "" {
		return errors.New("root and state path are required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return err
	}
	root := absRoot
	statePath := opts.StatePath

	var listCount atomic.Int64
	var listStop chan struct{}
	if opts.Progress {
		listStop = make(chan struct{})
		go listProgressPrinter(&listCount, listStop)
	}
	list, err := buildFileList(ctx, root, opts.FollowSymlinks, &listCount)
	if listStop != nil {
		close(listStop)
		fmt.Fprintf(os.Stderr, "\rIndexing: %d files\n", listCount.Load())
	}
	if err != nil {
		return err
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Path < list[j].Path
	})
	files := list

	hashes := make([]string, len(files))
	errs := make([]string, len(files))
	var doneCount atomic.Int64
	for i, f := range files {
		if f.Err != "" {
			errs[i] = f.Err
			doneCount.Add(1)
		}
	}

	jobs := make(chan scanJob, opts.Workers*2)
	results := make(chan scanResult, opts.Workers*2)

	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fullPath := filepath.Join(root, filepath.FromSlash(job.Path))
				hash, err := hashFile(ctx, fullPath, opts.Algo)
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					results <- scanResult{Index: job.Index, Path: job.Path, Hash: hash, Err: err}
					continue
				}
				results <- scanResult{Index: job.Index, Path: job.Path, Hash: hash, Err: nil}
			}
		}()
	}

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for res := range results {
			if res.Err != nil {
				errs[res.Index] = res.Err.Error()
			} else {
				hashes[res.Index] = res.Hash
			}
			doneCount.Add(1)
		}
	}()

	var (
		hashStop  chan struct{}
		hashDone  chan struct{}
		hashStart time.Time
	)
	if opts.Progress {
		hashStart = time.Now()
		hashStop = make(chan struct{})
		hashDone = make(chan struct{})
		go progressPrinterWithLabel(&doneCount, len(files), "Hashing", hashStart, hashStop, hashDone)
	}

	interrupted := false
enqueue:
	for i, f := range files {
		if errs[i] != "" {
			continue
		}
		select {
		case <-ctx.Done():
			interrupted = true
			break enqueue
		default:
		}
		jobs <- scanJob{Index: i, Path: f.Path}
	}
	close(jobs)

	wg.Wait()
	close(results)
	<-writerDone
	if hashStop != nil {
		close(hashStop)
		<-hashDone
		printProgressLine(doneCount.Load(), len(files), "Hashing", hashStart)
		fmt.Fprintln(os.Stderr)
	}
	if ctx.Err() != nil {
		interrupted = true
	}

	if interrupted {
		return errors.New("scan interrupted; no state written")
	}
	if doneCount.Load() != int64(len(files)) {
		return errors.New("scan incomplete; no state written")
	}
	for i := range files {
		if errs[i] == "" && hashes[i] == "" {
			return errors.New("scan incomplete; no state written")
		}
	}

	if opts.Progress {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Finalizing state...")
	}

	header := HeaderRecord{
		Type:           "header",
		Version:        stateVersion,
		Root:           root,
		Algo:           opts.Algo,
		CreatedAt:      time.Now().UTC().Format("2006-01-02 15:04:05"),
		FollowSymlinks: opts.FollowSymlinks,
	}
	return WriteStateFile(statePath, header, files, hashes, errs, true)
}

func buildFileList(ctx context.Context, root string, followSymlinks bool, count *atomic.Int64) ([]FileEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var files []FileEntry
	addErrorEntry := func(path string, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, FileEntry{
			Path: filepath.ToSlash(rel),
			Err:  err.Error(),
		})
		if count != nil {
			count.Add(1)
		}
		return nil
	}
	walkFn := func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return addErrorEntry(path, err)
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if !followSymlinks {
				return nil
			}
			info, err := os.Stat(path)
			if err != nil {
				return addErrorEntry(path, err)
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, FileEntry{
				Path: filepath.ToSlash(rel),
				Size: info.Size(),
			})
			if count != nil {
				count.Add(1)
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return addErrorEntry(path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, FileEntry{
			Path: filepath.ToSlash(rel),
			Size: info.Size(),
		})
		if count != nil {
			count.Add(1)
		}
		return nil
	}
	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, err
	}
	return files, nil
}

func hashFile(ctx context.Context, path, algo string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	var h hash.Hash
	switch algo {
	case "sha256":
		h = sha256.New()
	case "md5":
		h = md5.New()
	default:
		return "", fmt.Errorf("unsupported algo: %s", algo)
	}
	buf := make([]byte, 256*1024)
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return "", readErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func progressPrinterWithLabel(done *atomic.Int64, total int, label string, start time.Time, stop <-chan struct{}, finished chan<- struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	printProgressLine(done.Load(), total, label, start)
	for {
		select {
		case <-stop:
			if finished != nil {
				close(finished)
			}
			return
		case <-ticker.C:
			printProgressLine(done.Load(), total, label, start)
		}
	}
}

func listProgressPrinter(count *atomic.Int64, stop <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			fmt.Fprintln(os.Stderr)
			return
		case <-ticker.C:
			current := count.Load()
			fmt.Fprintf(os.Stderr, "\rIndexing: %d files", current)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func printProgressLine(completed int64, total int, label string, start time.Time) {
	elapsed := time.Since(start).Seconds()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(completed) / elapsed
	}
	fmt.Fprintf(os.Stderr, "\r%s: %d/%d (%.1f%%) %.1f files/s", label, completed, total, float64(completed)*100/float64(max(total, 1)), rate)
}
