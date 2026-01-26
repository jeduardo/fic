package fic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync/atomic"
	"time"
)

type fileValue struct {
	Present bool
	HasSum  bool
	HasErr  bool
	Hash    string
	Err     string
}

type Diff struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	Left       string `json:"left,omitempty"`
	Right      string `json:"right,omitempty"`
	LeftError  string `json:"left_error,omitempty"`
	RightError string `json:"right_error,omitempty"`
}

type CompareReport struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	Diffs []Diff `json:"diffs"`
}

func RunCompare(leftPath, rightPath, format, out string) error {
	leftHeader, leftOK, err := readHeader(context.Background(), leftPath)
	if err != nil {
		return err
	}
	rightHeader, rightOK, err := readHeader(context.Background(), rightPath)
	if err != nil {
		return err
	}
	if !leftOK || !rightOK {
		return errors.New("missing header in one or both state files")
	}
	if leftHeader.Algo != rightHeader.Algo {
		return fmt.Errorf("algorithm mismatch: left=%s right=%s", leftHeader.Algo, rightHeader.Algo)
	}

	left, err := loadState(context.Background(), leftPath, nil)
	if err != nil {
		return err
	}
	right, err := loadState(context.Background(), rightPath, nil)
	if err != nil {
		return err
	}
	if !left.HeaderPresent || !right.HeaderPresent {
		return errors.New("missing header in one or both state files")
	}

	if !left.Completed || !right.Completed {
		fmt.Fprintln(os.Stderr, "warning: one or both scans are not completed")
	}

	leftMap := buildPathMap(left)
	rightMap := buildPathMap(right)

	leftPaths := make([]string, 0, len(leftMap))
	for p := range leftMap {
		leftPaths = append(leftPaths, p)
	}
	sort.Strings(leftPaths)

	rightOnly := make([]string, 0)
	for p := range rightMap {
		if _, ok := leftMap[p]; ok {
			continue
		}
		rightOnly = append(rightOnly, p)
	}
	sort.Strings(rightOnly)

	total := len(leftPaths)
	var processed atomic.Int64
	var stop chan struct{}
	var done chan struct{}
	var start time.Time
	if total > 0 {
		start = time.Now()
		stop = make(chan struct{})
		done = make(chan struct{})
		go progressPrinterWithLabel(&processed, total, "Comparing", start, stop, done)
	}

	diffs := make([]Diff, 0)
	for _, p := range leftPaths {
		lv := leftMap[p]
		rv, rOk := rightMap[p]
		if !rOk {
			diffs = append(diffs, Diff{Path: p, Status: "only_left"})
			processed.Add(1)
			continue
		}
		if lv.HasErr || rv.HasErr {
			diffs = append(diffs, Diff{
				Path:       p,
				Status:     "error",
				LeftError:  lv.Err,
				RightError: rv.Err,
			})
			processed.Add(1)
			continue
		}
		if lv.HasSum && rv.HasSum {
			if lv.Hash != rv.Hash {
				diffs = append(diffs, Diff{Path: p, Status: "mismatch", Left: lv.Hash, Right: rv.Hash})
			}
			processed.Add(1)
			continue
		}
		diffs = append(diffs, Diff{Path: p, Status: "pending"})
		processed.Add(1)
	}
	for _, p := range rightOnly {
		diffs = append(diffs, Diff{Path: p, Status: "only_right"})
	}
	if stop != nil {
		close(stop)
		<-done
		printProgressLine(processed.Load(), total, "Comparing", start)
		fmt.Fprintln(os.Stderr)
	}

	if format != "text" && format != "json" {
		return fmt.Errorf("unknown format: %s", format)
	}

	if format == "json" {
		report := CompareReport{Left: leftPath, Right: rightPath, Diffs: diffs}
		return writeOutputJSON(report, out)
	}
	return writeOutputText(diffs, out)
}

func buildPathMap(st *State) map[string]fileValue {
	m := make(map[string]fileValue)
	for _, f := range st.Files {
		v := m[f.Path]
		v.Present = true
		if f.Hash != "" {
			v.HasSum = true
			v.Hash = f.Hash
		}
		if f.Err != "" {
			v.HasErr = true
			v.Err = f.Err
		}
		m[f.Path] = v
	}
	return m
}

func writeOutputJSON(report CompareReport, out string) (err error) {
	var w *os.File
	if out == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(out)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeOutputText(diffs []Diff, out string) (err error) {
	var w *os.File
	if out == "" {
		w = os.Stdout
	} else {
		w, err = os.Create(out)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := w.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
	}
	for _, d := range diffs {
		switch d.Status {
		case "only_left":
			if _, err = fmt.Fprintf(w, "ONLY_LEFT %s\n", d.Path); err != nil {
				return err
			}
		case "only_right":
			if _, err = fmt.Fprintf(w, "ONLY_RIGHT %s\n", d.Path); err != nil {
				return err
			}
		case "mismatch":
			if _, err = fmt.Fprintf(w, "MISMATCH %s %s %s\n", d.Path, d.Left, d.Right); err != nil {
				return err
			}
		case "error":
			if _, err = fmt.Fprintf(w, "ERROR %s left=%s right=%s\n", d.Path, d.LeftError, d.RightError); err != nil {
				return err
			}
		case "pending":
			if _, err = fmt.Fprintf(w, "PENDING %s\n", d.Path); err != nil {
				return err
			}
		}
	}
	return nil
}
