# Filesystem Integrity Checker

This is a tool to check the integrity between the content of two file systems.

It allows you to:

- Retrieve a list of the checksums of all files in the current file system
- Compare these checksums with a list of another file system
- List which files are present in one file system but not in the other
- Collect these lists in parallel

See `CONTRIBUTING.md` for contribution guidelines.

Disclaimer: This project was built with help from AI.

## Development

Run the full local checks:

```shell
make ci
```

Generate an HTML coverage report:

```shell
make coverage-html
```

## Usage

## Quick start

Run two scans and compare the results:

```shell
fic scan --root /path/to/dirA --out dirA.fic --progress
fic scan --root /path/to/dirB --out dirB.fic --progress
fic compare --left dirA.fic --right dirB.fic --format text
```

For full command help:

```shell
fic --help
fic scan --help
```

Supported algorithms: `sha256` (default) and `md5`.

Scan a directory and write a state file:

```shell
fic scan --root /path/to/dir --out scan.fic --workers 8 --algo sha256 --progress
```

Compare two scans:

```shell
fic compare --left scanA.fic --right scanB.fic --format text
```

View scan contents:

```shell
fic view --state scan.fic --only-done
```

Compact (cleanup checkpoints in) an existing state file:

```shell
fic compact --state scan.fic
```

## State file format (.fic)

The state file is a single JSON Lines (JSONL) file. Each line is a JSON object with
a `type` field. Scans are written once at the end of hashing.

Record types:

- `header`: scan metadata and configuration.
- `file`: file list entry. Includes `hash` when available.
- `error`: error for a file path (e.g., unreadable file).
- `completed`: scan finished marker.

Example records:

```json
{"type":"header","version":1,"root":"/abs/path","algo":"sha256","created_at":"YYYY-MM-DD HH:MM:SS","follow_symlinks":false}
{"type":"file","path":"relative/path.txt","size":123,"hash":"..."}
{"type":"error","path":"relative/other.txt","error":"permission denied"}
{"type":"completed","completed_at":"..."}
```

Notes:

- File entries are ordered by sorted relative path.
- Scans are written once at the end (no incremental checkpointing).
- Unreadable paths are recorded as `error` records and still count toward progress.
