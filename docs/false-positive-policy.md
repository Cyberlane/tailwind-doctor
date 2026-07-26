# False-Positive Policy

A tool that reports debt which is not there is worse than a tool that misses some.
A missed finding costs a user nothing they did not already have; a fabricated one
costs them time, and costs the score its credibility. This page defines what
counts as a fabrication, what counts as a deliberate omission, and how an argument
about a specific case is settled.

The measured rate this policy governs is in [extraction-accuracy.md](extraction-accuracy.md).

## What Counts As A False Positive

A false positive is any class list the extractor reports that is not a class list
written on an element in the source. Concretely:

- **Text that is not an attribute.** A class list inside a comment, inside an
  ordinary string, inside prose, or inside an HTML-escaped code sample. None of it
  reaches the browser, so none of it is debt.
- **Markup that does not ship.** A commented-out block, or a template that is inert
  at the point the file is read.
- **A value invented for an unknowable site.** Where a class value is computed at
  runtime, the correct output is "unresolved". Emitting a partial or guessed class
  list instead is a false positive even if the guess is plausible.
- **A fragment of a larger expression.** Reporting `cn(` because a scanner stopped
  at the wrong delimiter is a false positive, not a formatting nuisance.
- **A string in the right file but the wrong role.** A `cva` variant key, a
  `defaultVariants` value naming a variant rather than listing classes, or a data
  attribute whose value merely looks like utilities.

## What Counts As A Deliberate Skip

A skip is a case where the *correct answer itself* is arguable, not one where the
extractor is merely known to be wrong. Skips are recorded in the corpus with
status `skip`, are counted and printed by the harness, and never enter the
precision or recall figures.

The distinction that matters:

- **Known miss.** The correct answer is agreed, the extractor does not produce it.
  This is a false negative and it counts. Known misses are never skipped, because
  the whole purpose of the number is to make them visible and to let a later
  change show that it fixed them.
- **Disputed case.** Reasonable maintainers disagree about what should happen. This
  is a skip until the argument is settled.

An example currently in the corpus: a plain object whose values are class-list
strings, read by a computed key. A tool could follow it; whether it should is a
real question about how far static analysis goes before it starts guessing. It is
skipped rather than silently decided.

Out-of-scope shapes are not skips. Class lists passed to a third-party component's
props are recorded as `ignore`: the decision is that they must not be extracted,
which is a definite answer, not an open one.

## Accepted False Positives

The gate is precision ≥ 0.995 **and** zero unexplained false positives. Any false
positive that is to be tolerated must be listed in
`testdata/corpus/baseline.json` under `accepted_false_positives` with a written
reason.

This exists so that an exception is a visible line in a diff that a reviewer can
argue with, rather than a threshold quietly loosened by a decimal place. An
accepted false positive is a debt, not a resolution; the reason should say what
would be needed to remove it.

## Settling A Disputed Case

1. Open an issue naming the fixture and the exact record, and state what you think
   the correct extraction is and why.
2. Change the record's status to `skip` in the fixture's `expected.txt`. This takes
   it out of the measured rate immediately, so the argument does not block work or
   distort the number while it runs.
3. A maintainer decides.
4. Write the decision into the comment header of that fixture's `expected.txt`, so
   the reasoning travels with the file rather than living only in an issue, and set
   the record's status to the agreed value.
5. Regenerate the baseline in the same change, so the effect of the decision on the
   published number is visible in the same diff as the decision.

If a case stays disputed, it stays skipped. A permanent skip is an honest statement
that the project has not decided; a coin-flip recorded as ground truth is not.

## Reporting A False Positive As A User

Open an issue with the smallest source file that reproduces it and the output you
received. A confirmed false positive becomes a corpus fixture in the change that
fixes it, so the same mistake cannot return unnoticed.
