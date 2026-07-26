# Contributing

Thanks for helping make Tailwind projects easier to maintain. Participation in
this project is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Setup

Install Go 1.26 or newer, then run:

```bash
go test ./...
go vet ./...
```

## Pull Requests

- Keep changes focused and include tests for rule behavior.
- Preserve deterministic output: same input must produce the same report.
- Do not execute scanned projects or send source code anywhere.
- Use Conventional Commit-style subjects: `feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`, `perf:`, `ci:`, or `build:`.

## Rule Design

Each rule should identify its confidence level in its documentation, report file-level evidence, and avoid scoring uncertain static-analysis guesses as failures.
