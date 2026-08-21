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

## Documentation site

The GitHub Pages site is a dependency-free static site under `site/`. Its
release badge is filled at build time so the source never carries a stale
version. Preview the generated site from the repository root with:

```bash
preview_directory=$(mktemp -d)
release_version=$(node -p "require('./npm/tw-doctor/package.json').version")
scripts/build-site.sh "v$release_version" "$preview_directory/site"
python3 -m http.server 4173 --directory "$preview_directory/site"
```

Keep asset links relative so the site also works at the repository Pages path.
Run `scripts/test-site.sh` after changes. Site changes merged to `main` are
published with the latest public release version. A successful release calls
the same Pages workflow and verifies the live version before it finishes.

## Rule Design

Each rule should identify its confidence level in its documentation, report file-level evidence, and avoid scoring uncertain static-analysis guesses as failures.
