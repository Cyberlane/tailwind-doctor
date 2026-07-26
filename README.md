# Tailwind Doctor

Tailwind Doctor (`tw-doctor`) is a fast, read-only CLI that measures design-system debt in Tailwind class lists. It reports a project-wide **Design System Health Score** with file-level evidence.

> **Status: early development.** The tool runs and its output is deterministic, but nothing here is stable yet. Read [What Exists Today](#what-exists-today) before relying on the number it prints.

```bash
go run ./cmd/tw-doctor .
```

```text
Tailwind Doctor: 94/100
Scanned 42 source files

3 finding(s):
- [no-conflicting-utilities] src/card.tsx: p-4 conflicts with p-2 in the same variant.
- [no-arbitrary-value] src/card.tsx: Avoid arbitrary values; prefer a named design token.
- [responsive-bloat] src/card.tsx: Five or more variant utilities make this class list difficult to maintain.
```

## What Exists Today

Three high-confidence rules run over every `.astro`, `.html`, `.jsx`, `.tsx`, `.vue`, and `.svelte` file:

- Conflicting utilities in the same Tailwind variant, such as `p-4 p-2`.
- Arbitrary values, such as `text-[#123456]`.
- Overly dense responsive class lists with five or more variant utilities.

The limitations matter as much as the features, so they are stated plainly:

- **The scoring model is not final.** The score is currently `100 - 2 × findings`, clamped at zero. It is not normalized by project size, so a large codebase bottoms out at zero while a small one with proportionally identical debt still scores well. Treat the number as a relative signal within one project over time, not as something to compare between projects or publish as a badge. A size-normalized model with documented weights and confidence tiers will replace it, and doing so **will change your score**.
- **Extraction is a single regex over `class` and `className` string literals.** Template literals, `clsx`/`cn`/`cva`, Vue `:class`, Svelte `class:foo`, Astro `class:list`, and multiline attributes are not handled yet, so findings are undercounted in projects that use them.
- Findings report a file, not a line and column.
- There is no configuration file, no suppression baseline, and no SARIF output yet. The `--json` schema is not versioned and may change.

Contrast analysis, theme-token discovery, and unused-token detection are planned once structural parsing and the Tailwind configuration adapters are in place.

## CLI

```bash
# Human-readable report
tw-doctor .

# Machine-readable report for CI
tw-doctor --json .

# Fail CI below a score threshold
tw-doctor --fail-under 90 .
```

## Exit Codes

| Code | Meaning |
| ---- | ------- |
| `0`  | The run succeeded and the score met the `--fail-under` threshold. |
| `1`  | The run succeeded but the score was below `--fail-under`. The report is still written to stdout. |
| `2`  | Operational error: an unreadable path, an invalid flag, or a failure writing the report. |

`--fail-under` accepts a value from `0` to `100`. Anything outside that range is rejected with exit code `2` before any files are scanned. The threshold is always applied, so a templated `--fail-under $THRESHOLD` that resolves to `0` is an explicit always-passing gate rather than a silently disabled check. Omitting the flag is equivalent to `--fail-under 0`.

## npm Distribution

The runtime is written in Go. The `npm/tw-doctor` package is the future thin launcher that will make both commands work after release:

```bash
npx tw-doctor
npx tailwind-doctor
```

Release builds will package platform-specific Go binaries alongside that launcher. Until then, use `go run ./cmd/tw-doctor .`.

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/tw-doctor --json .
```

## Privacy

Tailwind Doctor is local, deterministic, and read-only. It does not execute application code, upload source files, or require credentials.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

[MIT](LICENSE)
