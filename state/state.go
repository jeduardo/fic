// Package state exposes the .fic state file structures and helpers.
package state

import "github.com/jeduardo/fic/internal/fic"

// Version is the current .fic state file format version.
const Version = 1

type HeaderRecord = fic.HeaderRecord
type FileRecord = fic.FileRecord
type ErrorRecord = fic.ErrorRecord
type CompletedRecord = fic.CompletedRecord
type FileEntry = fic.FileEntry
type State = fic.State

// WriteStateFile writes a .fic state file with the provided entries.
func WriteStateFile(path string, header HeaderRecord, files []FileEntry, hashes []string, errs []string, completed bool) error {
	return fic.WriteStateFile(path, header, files, hashes, errs, completed)
}
