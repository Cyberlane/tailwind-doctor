# Tailwind Doctor

Tailwind Doctor (`tw-doctor`) is a fast, read-only CLI that measures design-system debt in Tailwind class lists. It reports a project-wide **Design System Health Score** with file-level evidence.

> **Status: early development.** The tool runs and its output is deterministic, but nothing here is stable yet. Read [What Exists Today](#what-exists-today) before relying on the number it prints.

```bash
go run ./cmd/tw-doctor .
```

```text
Tailwind Doctor: 91/100
Scanned 42 file(s), 310 class list(s), 1840 utilities

  Accessibility    not measured
  Correctness      96
  Consistency      95
  Maintainability  100

3 finding(s):
- [no-conflicting-utilities] src/card.tsx:12:22: p-4 conflicts with p-2 in the same variant.
- [no-arbitrary-value] src/card.tsx:12:31: Avoid arbitrary values; prefer a named design token.
- [responsive-bloat] src/nav.tsx:8:14: Five or more variant utilities make this class list difficult to maintain. (medium confidence, not scored)
```

## What Exists Today

Three established rules run over every `.astro`, `.html`, `.jsx`, `.tsx`, `.vue`, and `.svelte` file, with opt-in token and accessibility rules available for their introductory releases:

- Conflicting utilities in the same Tailwind variant, such as `p-4 p-2`.
- Arbitrary values, such as `text-[#123456]`.
- Overly dense responsive class lists with five or more variant utilities.
- Unused custom theme tokens (`unused-token`, disabled by default for one minor
  release under the rule-stability policy).
- WCAG 2.2 text contrast for statically resolvable foreground/background pairs
  (`color-contrast`, disabled by default for one minor release).

Class lists are read out of `class` and `className` attributes, template literals, `clsx`/`cn`/`cva` leaves, Vue `:class`, Svelte `class:foo`, Astro `class:list`, and CSS `@apply`. A value that cannot be resolved statically is recorded as unresolved and never linted. The measured precision and recall are published in [docs/extraction-accuracy.md](docs/extraction-accuracy.md) and enforced in CI.

**The score is size-normalized and documented.** Debt density is measured per rule against the unit that rule occurs in, then mapped onto 0–100 by `100 × H / (H + D)` with `H = 1/5`. Two projects of very different sizes with proportionally similar debt score alike. The formula, the category weights, and the reasoning behind `H` are in [docs/scoring.md](docs/scoring.md) — arguments welcome.

Scores are comparable across projects only when `scoreModel.version` and the enabled rule set match. Both ship in every report, so a published number is checkable rather than merely asserted.

The limitations matter as much as the features, so they are stated plainly:

- **Accessibility is measured only where static evidence is decisive.** Named
  tokens and common arbitrary CSS colours are resolved without a DOM. Inherited
  colours, runtime themes, uncertain text size, and unsupported compositing are
  counted as unknown rather than turned into findings. The category remains
  `null` until the introductory `color-contrast` rule is explicitly enabled.
- **Token analysis fails closed.** Tailwind v3 and v4 themes are read statically,
  exact arbitrary-value matches name their replacement utility, and token usage
  includes CSS `@apply`. Incomplete configuration, plugin, extraction, or
  monorepo-ownership evidence lowers unused-token confidence.
- **Uncertain signals are score-neutral by default.** This includes
  `responsive-bloat`, unclassifiable utility conflicts, and degraded
  `unused-token` evidence. They are reported and tagged rather than hidden; see
  [docs/configuration.md](docs/configuration.md).
- **Adding a rule will change your score.** New rules ship disabled for one minor release before that happens; the policy is in [docs/rule-stability.md](docs/rule-stability.md).

Contrast coverage, theme-token discovery, and unused-token detection are now
implemented.

Rule severities, path ignores, an arbitrary-value allowlist, and `[score] min-confidence` are configured in `twdoctor.toml`; existing debt is recorded in a baseline. Both are documented in [docs/configuration.md](docs/configuration.md).

## CLI

```bash
# Human-readable report
tw-doctor .

# Machine-readable report for CI
tw-doctor --json .

# SARIF for GitHub code scanning or an editor
tw-doctor --sarif .

tw-doctor --write-baseline .   # record current debt; later runs gate on new findings

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

The runtime is written in Go. Two npm packages under `npm/` will eventually make both commands work without a Go toolchain:

```bash
npx tw-doctor
npx tailwind-doctor
```

Both sit at `0.0.0`, which is a name reservation rather than a release: they contain no binary and exit with a notice saying so. Release builds will attach platform-specific Go binaries as optional dependencies; see [docs/releasing.md](docs/releasing.md). Until then, use `go run ./cmd/tw-doctor .`.

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/tw-doctor --json .
tw-doctor --write-baseline .   # record current debt; later runs gate on new findings
```

## Privacy

Tailwind Doctor is local, deterministic, and read-only. It does not execute application code, upload source files, or require credentials.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

[MIT](LICENSE)
