package fic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func CompactState(ctx context.Context, statePath, outPath string) error {
	return compactState(ctx, statePath, outPath, false)
}

func CompactStateWithProgress(ctx context.Context, statePath, outPath string, progress bool) error {
	return compactState(ctx, statePath, outPath, progress)
}

func compactState(ctx context.Context, statePath, outPath string, progress bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var readCount atomic.Int64
	var readStop chan struct{}
	if progress {
		readStop = make(chan struct{})
		total, err := countStateRecords(ctx, statePath)
		if err != nil {
			return err
		}
		go countPrinterWithLabel(&readCount, total, "Compressing state", readStop)
	}
	st, err := loadState(ctx, statePath, &readCount)
	if readStop != nil {
		close(readStop)
		fmt.Fprintln(os.Stderr)
	}
	if err != nil {
		return err
	}
	if !st.HeaderPresent {
		return errors.New("missing header in state file")
	}

	writePath := outPath
	inPlace := false
	if writePath == "" {
		writePath = statePath
		inPlace = true
	}

	var done atomic.Int64
	var lastPrint time.Time
	var lastPrinted int64
	total := 0
	if progress {
		total = countCompactRecords(st)
		lastPrint = time.Now()
		lastPrinted = -1
	}

	dir := filepath.Dir(writePath)
	tmpFile, err := os.CreateTemp(dir, ".fic.tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	bufw := bufio.NewWriter(tmpFile)
	enc := json.NewEncoder(bufw)

	if err := writeRecordFn(enc, st.Header); err != nil {
		return err
	}
	if err := writeFileRecordsParallel(ctx, st, bufw, &done, total, &lastPrint, &lastPrinted, progress); err != nil {
		return err
	}
	for idx := range st.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if st.Files[idx].Err != "" {
			rec := ErrorRecord{Type: "error", Path: st.Files[idx].Path, Error: st.Files[idx].Err}
			if err := writeRecordFn(enc, rec); err != nil {
				return err
			}
			done.Add(1)
			if progress {
				maybePrintCompact(&done, total, &lastPrint, &lastPrinted)
			}
		}
	}
	if st.Completed {
		if err := ctx.Err(); err != nil {
			return err
		}
		comp := CompletedRecord{Type: "completed", CompletedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if err := writeRecordFn(enc, comp); err != nil {
			return err
		}
		done.Add(1)
		if progress {
			maybePrintCompact(&done, total, &lastPrint, &lastPrinted)
		}
	}

	if err := bufw.Flush(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if inPlace {
		err = os.Rename(tmpPath, statePath)
	} else {
		err = os.Rename(tmpPath, writePath)
	}
	if progress {
		finalCount := done.Load()
		if lastPrinted != finalCount {
			printCompactLine(finalCount, total)
		}
		fmt.Fprintln(os.Stderr)
	}
	return err
}

type batchResult struct {
	index int
	data  []byte
	count int64
	err   error
}

const compactBatchSize = 2000

func writeFileRecordsParallel(ctx context.Context, st *State, bufw *bufio.Writer, done *atomic.Int64, total int, lastPrint *time.Time, lastPrinted *int64, progress bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(st.Files) == 0 {
		return nil
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	numBatches := (len(st.Files) + compactBatchSize - 1) / compactBatchSize
	jobs := make(chan int, workers)
	results := make(chan batchResult, workers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				if ctx.Err() != nil {
					return
				}
				start := batch * compactBatchSize
				end := start + compactBatchSize
				if end > len(st.Files) {
					end = len(st.Files)
				}
				var buf bytes.Buffer
				enc := json.NewEncoder(&buf)
				var count int64
				for idx := start; idx < end; idx++ {
					if ctx.Err() != nil {
						return
					}
					f := st.Files[idx]
					rec := FileRecord{
						Type: "file",
						Path: f.Path,
						Size: f.Size,
					}
					if f.Hash != "" {
						rec.Hash = f.Hash
					}
					if err := writeRecordFn(enc, rec); err != nil {
						results <- batchResult{index: batch, err: err}
						return
					}
					count++
				}
				results <- batchResult{index: batch, data: buf.Bytes(), count: count}
			}
		}()
	}

	go func() {
		for i := 0; i < numBatches; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	pending := make(map[int]batchResult)
	expected := 0
	for processed := 0; processed < numBatches; processed++ {
		res := <-results
		if res.err != nil {
			cancel()
			wg.Wait()
			return res.err
		}
		pending[res.index] = res
		for {
			next, ok := pending[expected]
			if !ok {
				break
			}
			if len(next.data) > 0 {
				if _, err := bufw.Write(next.data); err != nil {
					cancel()
					wg.Wait()
					return err
				}
			}
			if next.count > 0 {
				done.Add(next.count)
				if progress {
					maybePrintCompact(done, total, lastPrint, lastPrinted)
				}
			}
			delete(pending, expected)
			expected++
		}
	}

	wg.Wait()
	return nil
}

func countCompactRecords(st *State) int {
	total := 0
	for _, f := range st.Files {
		total++
		if f.Err != "" {
			total++
		}
	}
	if st.Completed {
		total++
	}
	return total
}

func countStateRecords(ctx context.Context, path string) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var count int64
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}
