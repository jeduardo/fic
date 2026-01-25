package fic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type ViewEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Hash   string `json:"hash,omitempty"`
	Error  string `json:"error,omitempty"`
}

func RunView(statePath string, onlyDone bool, format string) error {
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

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		for _, f := range st.Files {
			entry := ViewEntry{Path: f.Path, Status: "pending"}
			if f.Hash != "" {
				entry.Status = "done"
				entry.Hash = f.Hash
			} else if f.Err != "" {
				entry.Status = "error"
				entry.Error = f.Err
			}
			if onlyDone && entry.Status == "pending" {
				continue
			}
			if err := enc.Encode(entry); err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range st.Files {
		if f.Hash != "" {
			fmt.Printf("%s %s\n", f.Path, f.Hash)
			continue
		}
		if f.Err != "" {
			fmt.Printf("%s <error: %s>\n", f.Path, f.Err)
			continue
		}
		if onlyDone {
			continue
		}
		fmt.Printf("%s <pending>\n", f.Path)
	}
	return nil
}
