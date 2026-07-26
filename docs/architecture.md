# Architecture

Tailwind Doctor is a local static-analysis CLI. The first release keeps the dependency graph intentionally small.

```text
source files -> class attribute extractor -> deterministic rules -> report -> terminal / JSON / CI
```

`internal/audit` owns source discovery, extraction, scoring, and report rendering. The CLI only parses flags, selects an output format, and maps the result onto the documented exit codes.

## Extraction

Extraction is a hand-written scanner, not a parser and not a regular expression. It tracks just enough syntax to know whether the text under the cursor is markup, a string, a comment, or an expression, which is what separates a class attribute from prose that merely mentions one. One tag parser serves both markup files and JSX; the difference between them is only what counts as noise outside a tag.

A second layer reads expressions, because a component library keeps most of its classes outside a literal attribute: `cva`, `clsx`, and `cn` calls, Vue `:class`, Svelte `class:` directives, Astro `class:list`, template literals, and CSS `@apply`. Strings are class lists and object keys are class lists whose values are runtime conditions. Helper calls are read argument by argument, so what is knowable is kept and only the specific values that are not are reported as unknown.

Every class list carries a line and column. Where a value cannot be read statically — a helper call, a variable, a runtime substitution — the site is recorded as unresolved and no classes are invented for it. Unresolved entries never reach the rules.

Accuracy is measured rather than assumed: `testdata/corpus` holds fixtures from real open-source projects with hand-written ground truth, and a test reports precision, recall, and a per-shape breakdown against it. The measured figure and the policy governing it are in [extraction-accuracy.md](extraction-accuracy.md) and [false-positive-policy.md](false-positive-policy.md). CI gates on the committed baseline.

Configuration is read from an optional `twdoctor.toml` and accepted debt from an optional `twdoctor-baseline.json`; both are described in [configuration.md](configuration.md). The TOML reader is a documented subset written for this purpose.

There is deliberately no dependency here. No Go parser exists for JSX, Svelte, Astro, or Vue single-file components, and a tree-sitter grammar would require cgo, which would end the single static binary this project ships.

## JSON Output

`--json` writes a single indented object: `score`, `files`, `suppressed`, and
`findings`. Each finding carries `rule`, `message`, `file`, `class`, `line`,
`column`, and `severity`. A project with no findings emits `"findings": []`,
never `null`, so a consumer can iterate the field unconditionally.

The schema is not versioned yet; versioning lands with the scoring model. Until
then fields may be added, and the ones above may change.

## Scoring

The current score is `100 - 2 × findings`, clamped at zero. This is placeholder arithmetic: it has no size denominator, so it collapses to zero on any codebase with fifty or more findings regardless of how large that codebase is. It exists to make the report shape complete, and will be replaced by a size-normalized model with published weights and confidence tiers before the score is presented as trustworthy.

## Future Adapters

- Tailwind v3 JavaScript configuration parser.
- Tailwind v4 CSS theme parser.
- Framework-aware AST extraction for dynamic `className` expressions.
- Color resolution for contrast analysis.
- npm optional-dependency packages that deliver prebuilt Go binaries.
