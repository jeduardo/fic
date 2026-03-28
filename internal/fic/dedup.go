package fic

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
)

var closeFileFn = func(file *os.File) error {
	return file.Close()
}

type duplicateKey struct {
	Size int64
	Hash string
}

type DuplicateGroup struct {
	Hash  string   `json:"hash"`
	Size  int64    `json:"size"`
	Paths []string `json:"paths"`
}

type DedupReport struct {
	State      string           `json:"state"`
	Algo       string           `json:"algo"`
	Duplicates []DuplicateGroup `json:"duplicates"`
}

func RunDedup(statePath, format, out string) error {
	st, err := loadState(context.Background(), statePath, nil)
	if err != nil {
		return err
	}
	if !st.HeaderPresent {
		return fmt.Errorf("missing header in state file")
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unknown format: %s", format)
	}
	if !st.Completed {
		fmt.Fprintln(os.Stderr, "warning: scan is not completed")
	}

	duplicates := buildDuplicateGroups(st)
	if format == "json" {
		report := DedupReport{
			State:      statePath,
			Algo:       st.Header.Algo,
			Duplicates: duplicates,
		}
		return writeOutputJSON(report, out)
	}
	return writeDedupText(duplicates, out)
}

func buildDuplicateGroups(st *State) []DuplicateGroup {
	grouped := make(map[duplicateKey][]string)
	for _, file := range st.Files {
		if file.Hash == "" || file.Err != "" {
			continue
		}
		key := duplicateKey{
			Size: file.Size,
			Hash: file.Hash,
		}
		grouped[key] = append(grouped[key], file.Path)
	}

	duplicates := make([]DuplicateGroup, 0)
	for key, paths := range grouped {
		if len(paths) < 2 {
			continue
		}
		sort.Strings(paths)
		duplicates = append(duplicates, DuplicateGroup{
			Hash:  key.Hash,
			Size:  key.Size,
			Paths: paths,
		})
	}

	sort.Slice(duplicates, func(i, j int) bool {
		if duplicates[i].Hash != duplicates[j].Hash {
			return duplicates[i].Hash < duplicates[j].Hash
		}
		return duplicates[i].Size < duplicates[j].Size
	})

	return duplicates
}

func writeDedupText(groups []DuplicateGroup, out string) (err error) {
	var w io.Writer = os.Stdout
	var file *os.File
	if out == "" {
	} else {
		file, err = os.Create(out)
		if err != nil {
			return err
		}
		w = file
		defer func() {
			if cerr := closeFileFn(file); err == nil && cerr != nil {
				err = cerr
			}
		}()
	}

	return writeDedupTextWriter(w, groups)
}

func writeDedupTextWriter(w io.Writer, groups []DuplicateGroup) error {
	for _, group := range groups {
		if _, err := fmt.Fprintf(w, "DUPLICATE %s size=%d count=%d\n", group.Hash, group.Size, len(group.Paths)); err != nil {
			return err
		}
		for _, path := range group.Paths {
			if _, err := fmt.Fprintf(w, "PATH %s\n", path); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
