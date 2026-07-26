# Extraction Accuracy

Every rule, every finding, and the health score itself rest on one question: how
much of a project's Tailwind class usage does the extractor actually find, and how
much does it invent? This page carries the measured answer.

The number is produced by a test, not by hand, and it is deliberately unflattering.

## Current Measurement

Measured against the 16-fixture corpus in `testdata/corpus`.

| Metric | Value |
| --- | --- |
| Precision | **1.0000** |
| Recall | **0.5059** |
| Expected class lists | 85 |
| Extracted class lists | 43 |
| True positives | 43 |
| False positives | 0 (0 unexplained) |
| False negatives | 42 |
| Skipped (disputed) | 4 |
| Unresolved sites reported | 8 of 12 |

Per shape:

| Shape | Expected | Found | Recall |
| --- | --- | --- | --- |
| `attr-literal` | 35 | 35 | 1.0000 |
| `attr-interpolated` | 2 | 2 | 1.0000 |
| `svelte-class-directive` | 5 | 5 | 1.0000 |
| `svelte-class-shorthand` | 1 | 1 | 1.0000 |
| `astro-class-list` | 5 | 0 | 0.0000 |
| `clsx` | 4 | 0 | 0.0000 |
| `css-apply` | 7 | 0 | 0.0000 |
| `cva-leaf` | 22 | 0 | 0.0000 |
| `jsx-template` | 1 | 0 | 0.0000 |
| `vue-bind-class` | 3 | 0 | 0.0000 |

Read plainly: everything the extractor reports is real, and it finds about half of
what is there. What it misses is composition — `cva`, `clsx`, and the framework
binding syntaxes — which is where a component library keeps most of its classes.

### History

| Change | Precision | Recall |
| --- | --- | --- |
| Regex over `class`/`className` | 0.7000 | 0.4118 |
| Structural scanner, attributes and Svelte directives | 1.0000 | 0.5059 |

The regex's 15 false positives were not near-misses. They included `cn(` reported
as a class list, because its character class stopped at the first single quote
inside `:class="cn('flex items-center', props.class)"`, and four class lists
lifted out of a commented-out block of HTML that never reaches the browser. The
scanner knows the difference between markup, a string, and a comment, so all of
them are gone.

Two shapes changed meaning rather than regressing. `vue-bind-class` previously
read 0.3333 because a value-only match credited it with another shape's class
list; matching now prefers an exact position, and its true score with no `:class`
support is zero. The scanner also stopped treating `:class` as `class`, which is
what removed the `cn(` false positives.

## What Is Measured

The unit is a **class list**: one string of space-separated utilities attributed to
one source location. Comparison is a multiset match on the exact string.

- **True positive** — an extracted value matching a class list the corpus says exists.
- **False negative** — a class list the corpus says exists that was not extracted.
- **False positive** — an extracted value with no matching class list. This includes
  values matching a decoy, and values produced for a site the corpus marks as not
  statically knowable, because inventing classes for an unknowable site is the
  fabrication the project's rules forbid.

Line and column are recorded for every record in the corpus. The extractor now
reports both, and matching prefers an exact position before falling back to
matching on value alone, so a fixture holding the same class list under two
shapes credits each to the right one. Position is not yet *required* for a match;
that gate turns on once findings carry positions end to end.

## The Target

The agreed gate is **precision ≥ 0.995 with zero unexplained false positives**.
Every accepted false positive must be named in `testdata/corpus/baseline.json`
with a written reason, so exceptions appear in review rather than hiding inside a
loosened threshold.

That gate is **enforced**: `baseline.json` carries
`enforce_precision_target: true`, set once structural parsing reached the target.
It is never turned off again, so every later extraction change is held to it.

Recall is measured per shape and gated as *must not decrease*. An absolute recall
floor is not useful while whole shapes sit at zero.

## How CI Uses This

The `extraction-accuracy` job fails when, against the committed baseline:

- false positives rise, or
- false negatives rise, or
- precision falls, or
- recall falls, or
- unexplained false positives rise.

A change that improves extraction therefore fails CI until the baseline is
regenerated, which is intended: the new number is reviewed in the diff alongside
the change that produced it.

## Reproducing

```bash
go test ./internal/audit -run TestExtractionAccuracy -v      # measure
go test ./internal/audit -run TestExtractionAccuracy -update # rewrite the baseline
```

The measurement is deterministic and makes no network calls. Corpus provenance,
including the upstream commit for every fixture, is in
`testdata/corpus/MANIFEST.md`.

`TestCorpusGroundTruthIsWellFormed` runs first and checks the corpus against
itself: every record's recorded position must still point at its recorded string.
Ground truth is hand-written, so it can drift; if it has, the accuracy figure is
meaningless and the suite says so before reporting a number.

## A Note On Running The Tool On This Repository

`tw-doctor .` scans `testdata/corpus` and reports a large number of findings
against the fixtures. They are deliberately messy real-world files. Path ignores
arrive with the configuration file.
