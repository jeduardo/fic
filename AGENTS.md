# Agent Guidelines

This file defines how the coding agent should work in this repository.

## Goals

- Build a Go-based file system integrity checker as described in `README.md`.
- Prioritize correctness, deterministic results, and resumability.
- Keep the UX simple and scriptable.

## Operating Principles

- Read the README before planning or making changes.
- Prefer small, reviewable changes and explain intent briefly.
- Avoid breaking changes to file formats once published; version them.
- Be conservative with filesystem operations; never delete or modify user data.
- Keep parallel worker opportunities in mind, since the program only reads and checksums files.
- Always include the syntax highlight type on fenced code blocks in Markdown (for example, ```shell, ```go, ```json).

## Implementation Notes

- Use Go standard library when possible.
- Ensure deterministic ordering of results regardless of concurrency.
- Keep checksum and metadata formats stable and documented.
- Make resume state robust to partial writes (write temp + atomic rename).
- Progress reporting must not significantly slow down checksumming.
- Run `make lint` after you finish a change and fix any linting finds.
- Run `make test` after you finish a change.
- Build the binary with `make build` after you finish a change.

## CLI and I/O

- Provide clear subcommands and flags.
- Support piping/redirecting output (no forced interactive prompts).
- Emit machine-friendly output formats when requested (e.g., JSON/CSV).

## Tests

- Ensure all code is written with test coverage.
- Add unit tests for checksum, compare, and resume logic.
- Use small fixtures; avoid large binaries or flaky tests.
- Prefer table-driven tests in Go.

## Safety

- Never run destructive commands without explicit user request.
- If unsure about a requirement, ask before implementing.
