# Contributing

Thanks for contributing!

## Expectations

- Keep changes small, focused, and reviewable.
- Avoid breaking published file formats; version them if changes are required.
- Never delete or modify user data.
- Use the Go standard library when possible.

## Tests

- All new or changed code must include test coverage.
- Prefer table-driven tests.
- Use small fixtures; avoid large binaries or flaky tests.

To run tests with coverage locally:
```shell
make coverage
```

To run the full local checks:

```shell
make ci
```

To generate an HTML coverage report:

```shell
make coverage-html
```

## Development

- Ensure deterministic output regardless of concurrency.
- Keep resume state robust (write temp + atomic rename).
