# Architecture

Tailwind Doctor is a local static-analysis CLI. The first release keeps the dependency graph intentionally small.

```text
Tailwind config -> static adapters -> token inventory
source files    -> class extractor -> deterministic rules -> report -> terminal / JSON / CI
```

`internal/audit` owns source discovery, extraction, scoring, and report rendering. The CLI only parses flags, selects an output format, and maps the result onto the documented exit codes.

## Extraction

Extraction is a hand-written scanner, not a parser and not a regular expression. It tracks just enough syntax to know whether the text under the cursor is markup, a string, a comment, or an expression, which is what separates a class attribute from prose that merely mentions one. One tag parser serves both markup files and JSX; the difference between them is only what counts as noise outside a tag.

A second layer reads expressions, because a component library keeps most of its classes outside a literal attribute: `cva`, `clsx`, and `cn` calls, Vue `:class`, Svelte `class:` directives, Astro `class:list`, template literals, and CSS `@apply`. Strings are class lists and object keys are class lists whose values are runtime conditions. Helper calls are read argument by argument, so what is knowable is kept and only the specific values that are not are reported as unknown.

Every class list carries a line and column. Where a value cannot be read statically — a helper call, a variable, a runtime substitution — the site is recorded as unresolved and no classes are invented for it. Unresolved entries never reach the rules.

Accuracy is measured rather than assumed: `testdata/corpus` holds fixtures from real open-source projects with hand-written ground truth, and a test reports precision, recall, and a per-shape breakdown against it. The measured figure and the policy governing it are in [extraction-accuracy.md](extraction-accuracy.md) and [false-positive-policy.md](false-positive-policy.md). CI gates on the committed baseline.

Configuration is read from an optional `twdoctor.toml` and accepted debt from an optional `twdoctor-baseline.json`; both are described in [configuration.md](configuration.md). The TOML reader is a documented subset written for this purpose.

There is deliberately no dependency here. No Go parser exists for JSX, Svelte, Astro, or Vue single-file components, and a tree-sitter grammar would require cgo, which would end the single static binary this project ships.

## Output Formats

`--json` and `--sarif` are mutually exclusive; passing both exits 2 rather than
interleaving two documents on one stream.

`--json` writes a single indented object at `schemaVersion` 3:

| Field | Meaning |
|---|---|
| `schemaVersion` | Read this first. The shape below only changes with it |
| `tool` | `name` and `version` of the build that produced the report |
| `score` | The headline score, 0–100 |
| `scoreExcludingBaseline` | The score with no suppressions applied |
| `scoreModel` | `version`, `transferFunction`, `halfScoreDensity`, `weights` |
| `categories` | Per-category `score`, ordered `exposures`, and finding counts. `score` is `null` for a category with no enabled scoring rule |
| `scanned` | `files`, `classLists`, `utilities`, `tokens`, token confidence counts, and `colorPairs` — the score's denominators |
| `configuredRules` | Every rule with the `severity` and `confidence` it ran at |
| `diagnostics` | Configuration constructs the static adapters could not read. Always an array and never score-affecting |
| `tokens` | Per-package inventory, usage, unused entries, plugin coverage, and confidence evidence. Always an array |
| `accessibility` | Resolved and unknown colour-pair counts plus sorted coverage-gap reasons |
| `findings` | Always an array, never `null` |
| `suppressed` | How many findings the baseline absorbed |

Each finding carries `rule`, `category`, `message`, `file`, `class`, `line`,
`column`, `severity`, `confidence`, and `scored`.

SARIF output is 2.1.0. Results carry `partialFingerprints` keyed on rule, file,
and class list — the same key the baseline uses — so reformatting a file does not
present old debt to code scanning as new. A finding the score does not count
reports at level `note`, which keeps a code-scanning gate and the score in
agreement about what matters. Configuration diagnostics travel as warning-level
tool execution notifications, not results, because they describe analysis
coverage rather than defects in the project.

## Design Tokens

Tailwind configuration is read statically and never evaluated. Discovery scopes
each source file to its nearest Tailwind package, then a version-specific adapter
builds a canonical inventory for colours, spacing, typography, radii, shadows,
breakpoints, and containers. The inventory starts from the defaults for the
detected Tailwind line and applies readable project declarations in source order.

The v3 adapter reads the supported JavaScript object subset from
`tailwind.config.*`, including local presets. The v4 adapter reads `@theme`,
relative CSS imports, `@config`, and prefix declarations. Package imports,
functions, spreads, computed properties, and other runtime constructs are not
executed or guessed. They produce deterministic configuration diagnostics and
leave the report degraded but usable; diagnostics never become findings or alter
the score.

Detected prefix and separator settings control utility parsing for files in that
package. Explicit `[tailwind]` settings in `twdoctor.toml` take precedence.

Configured plugins are resolved against a curated, version-banded lexical
registry. Unknown, out-of-range, missing-version, and intentionally partial
surfaces lower unused-token confidence. No plugin module or `node_modules`
source is loaded.

## Scoring

Each rule declares the unit it is measured against, so its rate is weighted
findings over the count of that unit: utilities for `no-arbitrary-value` and
`no-conflicting-utilities`, class lists for `responsive-bloat`, distinct
project declarations for `unused-token`, and statically resolvable colour pairs
for `color-contrast`. Those rates sum
into a debt density `D`, which maps onto 0–100 by `100 × H / (H + D)` with
`H = 1/5`.

The arithmetic runs in `math/big` rationals with no floating point, because
`math.Exp` and `math.Pow` are implemented per architecture and byte-identical
output is a product boundary rather than a nicety.

Exposure counts resolved class lists only, and only high-confidence findings move
the score by default. The formula, the weights, and the reasoning behind `H` are
argued in [scoring.md](scoring.md); the compatibility rules around changing any
of it are in [rule-stability.md](rule-stability.md).

## Accessibility

Contrast analysis resolves same-context `text-*` and `bg-*` utilities through
the package token inventory or a supported arbitrary CSS colour. It calculates
WCAG 2.2 relative luminance, composites translucent foregrounds over opaque
backgrounds, and suggests a passing foreground token when one exists.

Coverage is deliberately narrower than CSS. Variant sets must match exactly;
ancestors and inherited colours are not traversed; text-size thresholds must be
provable without inventing a root font size; and unsupported gamut mapping,
runtime variables, background transparency, and opacity composition remain
unknown. Unknown contexts are summarized in every report and never become
findings or score exposure.

## Future Analysis

- Framework-aware AST extraction for dynamic `className` expressions.
- Wider CSS colour-space and conservative gamut-mapping support.
- npm optional-dependency packages that deliver prebuilt Go binaries.
