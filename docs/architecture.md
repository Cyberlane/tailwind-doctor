# Architecture

Tailwind Doctor is a local static-analysis CLI. The first release keeps the dependency graph intentionally small.

```text
source files -> class attribute extractor -> deterministic rules -> report -> terminal / JSON / CI
```

`internal/audit` owns source discovery, extraction, scoring, and report rendering. The CLI only parses flags and selects an output format.

## Future Adapters

- Tailwind v3 JavaScript configuration parser.
- Tailwind v4 CSS theme parser.
- Framework-aware AST extraction for dynamic `className` expressions.
- Color resolution for contrast analysis.
- npm optional-dependency packages that deliver prebuilt Go binaries.
