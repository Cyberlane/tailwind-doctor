# Tailwind Doctor

Tailwind Doctor (`tw-doctor`) is a fast, read-only-by-default CLI that measures design-system debt in Tailwind class lists. It reports a project-wide **Design System Health Score** with file-level evidence.

[Explore the documentation site](https://cyberlane.github.io/tailwind-doctor/) · [Rule catalog](docs/rules.md) · [Configuration](docs/configuration.md)

> **Status: v0.4.0.** The project is still pre-1.0 and intentionally conservative.
> Read [What Exists Today](#what-exists-today) and the documented coverage gaps
> before using its score as a gate.

```bash
npx tw-doctor .
```

```text
Tailwind Doctor: 91/100
✓ 1 Tailwind package detected
✓ Theme inventoried: 24 project tokens
✓ Scanned 42 files: 310 class lists, 1840 utilities
✗ 12 of 322 class lists (4%) were not analyzed: dynamic expressions
✓ Every resolved class list matched a Tailwind package

  Accessibility    not measured
  Correctness      96
  Consistency      95
  Maintainability  not measured

Findings by rule:
- no-arbitrary-value: 1
- no-conflicting-utilities: 1

2 findings:
src/card.tsx
  12:22 [no-conflicting-utilities] p-4 conflicts with p-2 in the same variant.
  12:31 [no-arbitrary-value] Avoid arbitrary values; replace w-[137px] with a named design token.

Tailwind Doctor: 91/100 — 2 findings, 2 scored
```

## What Exists Today

Established rules run over `.astro`, `.cjs`, `.css`, `.cts`, `.html`, `.js`,
`.jsx`, `.mdx`, `.mjs`, `.mts`, `.ts`, `.tsx`, `.vue`, and `.svelte` files, with additional rules available as
opt-ins during their introductory releases:

- Conflicting utilities in the same Tailwind variant, such as `p-4 p-2`.
- Arbitrary values, such as `text-[#123456]`.
- Partially overlapping utilities such as `px-4 pl-2`
  (`no-overlapping-utilities`, opt-in and medium confidence).
- Variant-heavy class lists (`variant-density`, opt-in and medium confidence).
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
  `variant-density`, overlapping utility scopes, unclassifiable utility conflicts, and degraded
  `unused-token` evidence. The `--json` and `--sarif` reports carry every one,
  tagged with its confidence; the human report summarizes them as a count so
  they never bury the scored findings. See
  [docs/configuration.md](docs/configuration.md).
- **Adding a rule will change your score.** New rules ship disabled for one minor release before that happens; the policy is in [docs/rule-stability.md](docs/rule-stability.md).

Contrast coverage, theme-token discovery, and unused-token detection are now
implemented.

Rule severities, path ignores, an arbitrary-value allowlist, and `[score] min-confidence` are configured in `twdoctor.toml`; existing debt is recorded in a baseline. Both are documented in [docs/configuration.md](docs/configuration.md).

## CLI

Install or run without a global install:

```bash
npx tw-doctor .
npx tailwind-doctor .
```

Or install the Go binary directly:

```bash
go install github.com/Cyberlane/tailwind-doctor/cmd/tw-doctor@v0.4.0
```

```bash
# Human-readable report
tw-doctor .

# Machine-readable report for CI
tw-doctor --json .

# SARIF for GitHub code scanning or an editor
tw-doctor --sarif .

tw-doctor --write-baseline .   # record current debt; later runs gate on new findings

# Replace arbitrary values only when they exactly match a named token
tw-doctor --fix .

# Fail CI below a score threshold
tw-doctor --fail-under 90 .

# Fail CI when too much dynamic class construction is unresolved
tw-doctor --require-coverage 95 .
```

## Editor Integration

`tw-doctor lsp` analyzes unsaved buffers over the Language Server Protocol. The
VS Code extension in [`editors/vscode`](editors/vscode) launches one local server
per workspace folder, and the native Neovim configuration in
[`lsp/tailwind_doctor.lua`](lsp/tailwind_doctor.lua) supports Neovim 0.11 and
newer. Both display the same rule IDs, confidence, and source ranges as the CLI.
A ready-to-install VSIX is attached to each GitHub release. See
[docs/editor-integration.md](docs/editor-integration.md).

## Exit Codes

| Code | Meaning |
| ---- | ------- |
| `0`  | The run succeeded and all requested gates passed. |
| `1`  | The run succeeded but the score or extraction coverage missed its threshold. The report is still written to stdout. |
| `2`  | Operational error: an unreadable path, an invalid flag, or a failure writing the report. |

`--fail-under` and `--require-coverage` accept values from `0` to `100`.
Anything outside that range is rejected with exit code `2` before files are
scanned. Zero is an explicit always-passing gate.

## npm Distribution

The runtime is written in Go. Both npm command names install the same prebuilt
binary:

```bash
npx tw-doctor
npx tailwind-doctor
```

The launcher selects one optional binary package for macOS arm64/x64, Linux
arm64/x64, or Windows x64. There is no install-time download script. Release
archives and checksums are also attached to the GitHub release; see
[docs/releasing.md](docs/releasing.md).

## Development

```bash
go test ./...
go vet ./...
go run ./cmd/tw-doctor --json .
tw-doctor --write-baseline .   # record current debt; later runs gate on new findings
npm test --prefix npm
scripts/test-release.sh
```

## Privacy

Tailwind Doctor is local, deterministic, and read-only unless `--fix` or
`--write-baseline` is explicitly requested. The human report uses ANSI color
only when writing to a terminal; piped or redirected output — and all `--json`
and `--sarif` output — is always plain text, and `NO_COLOR` or `TERM=dumb`
disables color entirely. It does not execute application
code, upload source files, or require credentials. `--fix` changes only
verbatim class utilities whose value exactly matches a statically resolved
named token; runtime values, uncertain configuration, allowlisted findings,
and baselined debt are left untouched.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Trademark

"Tailwind" is a trademark of Tailwind Labs. This project is not affiliated with or endorsed by Tailwind Labs.

## License

[MIT](LICENSE)
