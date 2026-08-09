# AGENTS.md

Operating guide for automated contributors working in this repository.

## Project

Tailwind Doctor (`tw-doctor`) is a local, deterministic, read-only CLI that measures
design-system debt in Tailwind class lists and reports a project-wide Design System
Health Score with file-level evidence.

Module path: `github.com/Cyberlane/tailwind-doctor`.

### Product boundary

These are hard constraints, not preferences. Do not relax them without an explicit
instruction from a maintainer.

- Never execute user application code. Configuration is parsed statically.
- Never make network calls at analysis time. No telemetry, no update checks, no
  crash reporting.
- Never write to files the user did not ask to be written. Analysis is read-only;
  only an explicit `--fix` invocation may modify sources.
- Output must be deterministic: identical input produces byte-identical output,
  independent of filesystem ordering, map iteration order, or goroutine scheduling.

## Layout

```
cmd/tw-doctor/     CLI entry point; flag parsing and output selection only
internal/audit/    source discovery, extraction, rules, scoring, report rendering
internal/tailwind/ Tailwind version discovery and static v3/v4 configuration adapters
internal/tokens/   canonical design-token inventory and value matching
docs/              architecture notes and the public rule catalog
npm/               npm launcher packages that ship prebuilt Go binaries
.github/           CI workflows and contribution templates
```

Keep the CLI layer thin. Analysis logic belongs in `internal/`.

## Commands

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...
gofmt -l .            # must print nothing
go run ./cmd/tw-doctor .
go run ./cmd/tw-doctor --json .
go run ./cmd/tw-doctor --sarif .
go test ./internal/audit/ -run TestGoldenReports -update   # regenerate committed expectations
```

Run `gofmt -l .`, `go vet ./...` and `go test ./...` before proposing any change.
Do not claim work is complete without running them and reading the output.

## Structural similarity review

- Before implementing behavior, search existing symbols, callers, helpers,
  adapters, transports, decoders, queries, and contracts that may already provide
  it.
- At the start of source-changing work, compare `.mori-version` with the latest
  official Mori GitHub release when network access is available. If a newer
  release exists, complete a separate reviewed upgrade before continuing.
- Before committing supported source, read
  `.agents/skills/mori-review-similarity/SKILL.md`, verify that `mori version`
  matches `.mori-version`, and run
  `mori scan --changed-since HEAD --format json .`.
- Inspect both source locations for every focused group, including literals,
  nested functions or callbacks, types, control flow, side effects, error
  handling, authorization, transactions, callers, and tests.
- Reuse or extract shared behavior only after manual source review. Record why
  intentional similarity remains separate.
- Never describe a similarity score as semantic equivalence or a confirmed
  defect.
- Do not create or update a baseline, weaken thresholds or scope, enable noisy
  statement blocks, or bypass the hook merely to make a commit pass.
- Treat warnings, parse diagnostics, skipped fragments, zero-fragment files,
  insufficient coverage, and truncation as incomplete evidence.
- Do not use `git commit --no-verify` without explicit maintainer approval and a
  recorded rationale.

## Code conventions

- Go 1.26. Standard library first. Do not add a dependency without a stated reason
  and maintainer agreement; the zero-dependency analysis core is a deliberate
  property, not an accident.
- Table-driven tests. Every rule change ships with fixture coverage.
- Full words for identifiers, matching the existing code (`utility`, `finding`,
  `report` — not `u`, `f`, `r`).
- Errors are wrapped with context and returned, not logged and swallowed.
- Sort any collection before it reaches output. Never range over a map to produce
  user-visible ordering.

## Rules

Every diagnostic has a stable, kebab-case rule identifier (`no-arbitrary-value`).

- Rule IDs are a public API. Never rename or repurpose one; retire it and add a
  new ID instead.
- Every rule carries a confidence level. Only high-confidence rules may affect the
  score by default.
- A new rule changes every existing user's score and can break their `--fail-under`
  gate. New rules ship disabled by default for one minor release, then become
  default-on in the following minor.
- Document each rule in `docs/rules.md` in the same change that introduces it.

## Output contract

- `--json` output is versioned. Any field removal or type change is a breaking
  change and requires a schema version bump.
- Exit codes: `0` success, `1` score below `--fail-under`, `2` operational error.
- Human output is for humans; JSON and SARIF are for machines. Do not make the
  human format parseable-by-contract.

## Commits and pull requests

- Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`,
  `build:`, `ci:`. Subject in imperative mood, 50 characters or fewer.
- Write commits in a normal human voice. Explain *why* in the body when the reason
  is not obvious from the diff.
- **Never add AI, agent, or assistant attribution of any kind.** No trailers
  crediting a model or tool, no "generated with" footers, no naming of any assistant
  product — in commit messages, pull request bodies, issue comments, code comments,
  or documentation. Commits are authored by people.
- Do not reference external planning systems, personal note-taking tools, private
  trackers, or local filesystem paths anywhere in this repository. Everything
  committed here must make sense to an outside contributor reading only this repo.
- One logical change per pull request. Update `docs/` in the same change.

## Versioning and releases

- Semantic Versioning from the first tag.
- Pre-1.0, rule additions and scoring changes are minor bumps; document score-affecting
  changes prominently in release notes.
- Distribution names are independent of the GitHub org: the npm packages are
  `tw-doctor` and `tailwind-doctor`, published in lockstep at the same version.

## Naming

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or
endorsed by Tailwind Labs, and user-facing documentation must say so.
