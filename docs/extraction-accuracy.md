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
| Recall | **1.0000** |
| Expected class lists | 85 |
| Extracted class lists | 85 |
| True positives | 85 |
| False positives | 0 (0 unexplained) |
| False negatives | 0 |
| Skipped (disputed) | 4 |
| Unresolved sites reported | 14 of 14 |

Per shape, every one at 1.0000 recall:

| Shape | Expected | Found |
| --- | --- | --- |
| `attr-literal` | 35 | 35 |
| `cva-leaf` | 22 | 22 |
| `css-apply` | 7 | 7 |
| `astro-class-list` | 5 | 5 |
| `svelte-class-directive` | 5 | 5 |
| `clsx` | 4 | 4 |
| `vue-bind-class` | 3 | 3 |
| `attr-interpolated` | 2 | 2 |
| `jsx-template` | 1 | 1 |
| `svelte-class-shorthand` | 1 | 1 |

**A corpus the extractor scores 100% on has stopped being a scale.** These
numbers say the extractor handles everything this corpus knows how to ask, which
is not the same as saying it handles everything. The corpus is now a regression
test rather than a measurement, and it needs new fixtures to become a measurement
again. The false-positive policy already requires one for every confirmed miss or
fabrication reported from here on; that is the mechanism by which this number
becomes informative again.

### History

| Change | Precision | Recall |
| --- | --- | --- |
| Regex over `class`/`className` | 0.7000 | 0.4118 |
| Structural scanner, attributes and Svelte directives | 1.0000 | 0.5059 |
| Expression scanner: helpers, bindings, `@apply` | 1.0000 | 1.0000 |

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

Line and column are recorded for every record in the corpus. The extractor
reports both, and a measurement counts a class list only when its value, line,
and column all match. A right string attributed to the wrong source site is a
miss, not a true positive, because findings carry positions end to end.

## The Target

The agreed gate is **precision ≥ 0.995 with zero unexplained false positives**.
Every accepted false positive must be named in `testdata/corpus/baseline.json`
with a written reason, so exceptions appear in review rather than hiding inside a
loosened threshold.

That gate is **enforced**: `baseline.json` carries
`enforce_precision_target: true`, set once structural parsing reached the target.
It is never turned off again, so every later extraction change is held to it.

Recall is measured per shape and gated as *must not decrease*. No absolute recall
floor is set, because a floor at the current 1.0000 would only ever be met by a
corpus that never grows — and the corpus is meant to grow.

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

Module-level helper calls require lexical import evidence. Calls to `cn`,
`clsx`, `classnames`, or `cva` that already sit inside a class-bearing framework
expression are contextual, but a bare module-level call without an import may be
an unrelated local function and is not treated as a class list. Imported aliases
are followed without resolving or executing their modules.

## A Note On Running The Tool On This Repository

`testdata/corpus` is excluded by this repository's own `twdoctor.toml`. The
fixtures are deliberately messy real-world files kept verbatim from upstream
projects; they are evidence, not this project's design debt.
