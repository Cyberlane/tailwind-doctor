# Changelog

All notable changes to Tailwind Doctor are documented here.

## [Unreleased]

## [0.2.0] - 2026-08-10

### Added

- Add global resolved/unresolved class-list coverage and the
  `--require-coverage` CI gate.
- Add JSON schema 4 finding ranges and coverage evidence, plus matching SARIF
  end ranges.
- Add baseline format 2 with explicit stable fingerprints, strict decoding,
  atomic replacement, and read compatibility for version 1.
- Add manifest-only Tailwind package discovery, `.js`, `.ts`, and `.mdx` source
  discovery, Tailwind v3 z-index tokens, inspectable package/version evidence,
  and parser fuzz targets.
- Add opt-in `variant-density` and `no-overlapping-utilities` rules.
- Add `tw-doctor lsp` for unsaved-buffer diagnostics and a dependency-free,
  multi-root VS Code extension shipped as a release VSIX.
- Add native CLI smoke jobs for Linux, macOS, and Windows.

### Changed

- Introduce score model 2: utility exposure now counts recognized Tailwind
  utilities instead of application class names, and categories with zero
  exposure report `null` rather than 100.
- Apply configured and Git ignore rules consistently to Tailwind configuration
  discovery and source analysis.
- Reject unknown or duplicate configuration, unknown rule IDs, invalid globs,
  and baseline paths outside the analysis root.
- Retire `responsive-bloat`; its stable rule ID remains accepted while the more
  precise, opt-in `variant-density` rule replaces it.

### Safety

- Editor analysis reuses immutable static project context, analyzes only the
  supplied buffer, executes no project code, writes no files, and performs no
  network requests.

## [0.1.1] - 2026-08-10

### Fixed

- Preserve the order of stacked Tailwind variants so order-sensitive selectors
  do not produce high-confidence conflict findings.
- Ignore unprefixed application classes when a Tailwind prefix is configured.
- Require import evidence for module-level class-helper calls, preventing an
  unrelated local function named `cn`, `clsx`, or `cva` from fabricating class
  lists while retaining contextual and aliased helper extraction.
- Make npm release recovery resolve paths from the requested output and verify
  that a recovered signed tag targets the workflow commit.
- Select release notes from the release tag instead of a hard-coded version.

## [0.1.0] - 2026-08-10

### Added

- Deterministic human, JSON schema 3, and SARIF 2.1.0 reports with a
  size-normalized Design System Health Score.
- Structural class-list extraction for HTML, JSX/TSX, Vue, Svelte, Astro,
  `clsx`/`cn`/`cva`, template literals, and CSS `@apply`.
- Static Tailwind CSS v3/v4 theme adapters, token usage evidence, exact token
  suggestions, and opt-in unused-token analysis.
- Conservative static WCAG 2.2 text-contrast analysis with explicit unknown
  coverage reasons.
- Rule configuration, confidence-aware scoring, path ignores, suppression
  baselines, and `--fail-under` CI gates.
- `--fix` for arbitrary values proven to exactly match named design tokens.
- Prebuilt npm distribution for macOS arm64/x64, Linux arm64/x64, and Windows
  x64, with checksummed GitHub archives and provenance attestations.

### Safety

- Analysis never executes application configuration or plugin code and never
  makes network calls.
- Source files are read-only unless `--fix` is explicitly supplied; symbolic
  links, runtime expressions, degraded token evidence, allowlists, and baselines
  are excluded from edits.
- `unused-token` and `color-contrast` are disabled by default for their
  introductory release and therefore do not change the default score.

[Unreleased]: https://github.com/Cyberlane/tailwind-doctor/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Cyberlane/tailwind-doctor/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/Cyberlane/tailwind-doctor/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Cyberlane/tailwind-doctor/releases/tag/v0.1.0
