# Architecture

Tailwind Doctor is a local static-analysis CLI. The first release keeps the dependency graph intentionally small.

```text
source files -> class attribute extractor -> deterministic rules -> report -> terminal / JSON / CI
```

`internal/audit` owns source discovery, extraction, scoring, and report rendering. The CLI only parses flags, selects an output format, and maps the result onto the documented exit codes.

## Extraction

Extraction is currently a single regular expression matching a literal `class` or `className` attribute. Its accuracy is measured rather than assumed: `testdata/corpus` holds fixtures from real open-source projects with hand-written ground truth, and a test reports precision, recall, and a per-shape breakdown against it. The measured figure and the policy governing it are in [extraction-accuracy.md](extraction-accuracy.md) and [false-positive-policy.md](false-positive-policy.md).

The measurement exists so that replacing the regex with structural parsing can be shown to be an improvement rather than asserted to be one. CI gates on the committed baseline.

## JSON Output

`--json` writes a single indented object: `score`, `files`, and `findings`. A
project with no findings emits `"findings": []`, never `null`, so a consumer can
iterate the field unconditionally. The schema is not versioned yet; versioning
lands with the scoring model.

## Scoring

The current score is `100 - 2 × findings`, clamped at zero. This is placeholder arithmetic: it has no size denominator, so it collapses to zero on any codebase with fifty or more findings regardless of how large that codebase is. It exists to make the report shape complete, and will be replaced by a size-normalized model with published weights and confidence tiers before the score is presented as trustworthy.

## Future Adapters

- Tailwind v3 JavaScript configuration parser.
- Tailwind v4 CSS theme parser.
- Framework-aware AST extraction for dynamic `className` expressions.
- Color resolution for contrast analysis.
- npm optional-dependency packages that deliver prebuilt Go binaries.
