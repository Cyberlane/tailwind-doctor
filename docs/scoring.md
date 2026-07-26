# The Design System Health Score

This document is the score's argument, not its changelog. Everything here is
open to being disagreed with; if the reasoning is wrong, the number is wrong,
and both should be said out loud.

## The Formula

```
rate_r        = weight(category(r)) × scored_findings(r) / exposure(unit(r))
D             = Σ_r rate_r
D_c           = Σ_{r ∈ category c} rate_r

Score         = round(100 × H / (H + D))
sub_score(c)  = round(100 × H / (H + D_c))   when c has an enabled scoring rule
              = null                         otherwise

H             = 1/5
```

`D` is *debt density*: how much weighted debt a project carries per unit of the
thing that debt occurs in. It has no units of size, which is the whole point.

## What Counts As Scored

A finding is scored if and only if all three hold:

1. Its severity is `error`. `warn` is reported and score-neutral; `off` is not
   reported at all.
2. Its confidence is at or above `min-confidence`, which defaults to `high`.
3. The baseline did not suppress it.

Everything else is still reported. A finding that does not move the score is
tagged as such in every output format.

## Each Rule Declares Its Own Denominator

| Rule | Exposure unit |
|---|---|
| `no-arbitrary-value` | utility |
| `no-conflicting-utilities` | utility |
| `responsive-bloat` | class list |

There is deliberately no single global denominator, for two reasons.

The rules genuinely disagree about their unit. An arbitrary value is a property
of one utility; responsive bloat is a property of a whole class list. Any one
global denominator misprices one of them from the first release.

And a global denominator would not survive the next milestone. An unused custom
token has a denominator of *tokens declared*; a contrast failure has one of
*resolvable colour pairs*. Neither means anything per utility scanned. Choosing
a single denominator now would mean re-opening the score model later, and a
change to the score model is a compatibility event for everyone gating CI on it.

### Exposure counts resolved class lists only

A `className` the tool cannot read statically contributes to no denominator. It
is reported as unresolved and excluded from both sides of the fraction.

Counting it would dilute measured debt in proportion to how much of a codebase
cannot be analysed — that is, it would reward writing code this tool cannot
read. The same argument is why an unresolved class list is never linted: see
[extraction-accuracy.md](extraction-accuracy.md).

## Category Weights

Ordered by what it costs a user to ignore them.

| Category | Weight | Why |
|---|---|---|
| `accessibility` | 4 | Excludes users, carries external legal exposure, and is close to invisible in code review |
| `correctness` | 3 | CSS that provably does not apply. A real defect, but self-contained |
| `consistency` | 2 | Token drift. Erodes the design system over time with no immediate defect |
| `maintainability` | 1 | A smell, and judgment-dependent |

Accessibility outranks correctness because a conflicting utility is harmless
dead code, while an unreadable colour pair excludes a person. Correctness
outranks consistency because a provable defect should outweigh a stylistic
drift, even though drift is what this tool exists to measure.

The weights are coarse on purpose. A finer scale would imply a precision the
underlying judgement does not have.

## How `H` Was Chosen

`H` is the debt density at which a project scores 50. It is set from a stated
principle:

> A project where one utility in ten is a consistency violation is
> half-healthy.

Consistency carries weight 2, so `0.10 × 2 = 0.20 = 1/5`.

`H` is frozen in score model v1. It was not fitted to any repository, and it
will not be adjusted to make one repository feel right — that is how a metric
ends up working only on the codebase it was tuned against. The single validation
permitted later is a distribution check across the fixture corpus: that real
projects do not all collapse into one bucket. If that check fails, the change
ships as a score-model version bump with a changelog entry stating which
direction scores move, not as a quiet tweak.

## Why The Curve Is Hyperbolic, And Exact

**No clamp.** The formula this replaced was `100 − 2 × findings` clamped at
zero, which awarded zero to every codebase with fifty or more findings
regardless of size. Past the clamp, a bad codebase and an awful one were
indistinguishable, and neither could show improvement — a metric that cannot
register progress is not worth measuring. `100 × H / (H + D)` approaches zero
without reaching it, so there is always room to get better and always a gap
between bad and worse.

**Exact arithmetic.** The score is computed in rationals (`math/big`), with no
floating point anywhere in the path. `math.Exp` and `math.Pow` have
per-architecture implementations, so an exponential curve would make "identical
input produces byte-identical output" true per machine rather than absolutely.
That guarantee is a product boundary, so the curve was chosen to fit it rather
than the other way round.

**Ties round up**, toward the more favourable score. Stated so nobody has to
guess when reproducing the arithmetic by hand.

## Worked Values

A project whose only debt is arbitrary values, as a share of utilities scanned:

| Arbitrary share | `D` | Score |
|---|---|---|
| 0% | 0 | 100 |
| 0.5% | 0.01 | 95 |
| 1% | 0.02 | 91 |
| 2% | 0.04 | 83 |
| 5% | 0.10 | 67 |
| 10% | 0.20 | 50 |
| 20% | 0.40 | 33 |
| 50% | 1.00 | 17 |

A mixed project: 2000 utilities across 400 class lists, with 20 arbitrary
values, 10 high-confidence conflicts, and 8 responsive-bloat findings.

```
consistency      2 × 20/2000 = 0.020    sub-score  91
correctness      3 × 10/2000 = 0.015    sub-score  93
maintainability                0        sub-score 100   (8 findings, none scored)
accessibility                  —        sub-score null  (no enabled rule)

D = 0.035                               Score      85
```

Responsive bloat reports at medium confidence, so those eight findings are
visible and score-neutral. Maintainability therefore scores 100 while carrying
eight findings, which is not a contradiction: the category is measured, and
nothing in it currently counts.

## Edge Cases

**Nothing scanned.** Every exposure is zero, so `D` is zero and the score is
100, with `scanned.utilities: 0` in the report. A `--fail-under` gate on an
empty repository passes. There is no debt to find in a codebase with no class
lists, and inventing a penalty for that would be theatre.

**A category with no enabled scoring rule reports `null`, not 100.** Before the
accessibility rules exist, an accessibility sub-score of 100 would read as "this
project is accessible" when it means "this was not measured". That single
misreading would cost more credibility than the sub-scores are worth.

**Turning a rule off raises the score.** This cannot be prevented, so it is
disclosed instead. Every report carries `configuredRules` and
`scoreModel.version`. Two scores are comparable only when both match — which is
what makes a published badge checkable rather than merely asserted.

## The Heavy Tail

The curve is forgiving at the extreme: a genuinely terrible codebase scores
around 10 to 17 rather than 0. Someone will reasonably call that generous.

It is the accepted cost of having no clamp. Being able to distinguish degrees of
bad, and to show a project moving from 12 to 19, is worth more than the
rhetorical satisfaction of printing a zero.

## Stability

Weights, `H`, the transfer function, category assignments, and exposure units
are frozen per `scoreModel.version`. The rules for changing them, and for adding
or retiring a rule, are in [rule-stability.md](rule-stability.md).
