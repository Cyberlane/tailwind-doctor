# Rule Stability Policy

Adding a rule changes every user's score and can turn a passing `--fail-under`
gate red without anyone touching their code. That makes a rule addition a
compatibility event, and this document is the contract around it.

## Rule Identifiers Are Public API

A rule identifier is never renamed and never repurposed. It appears in
`twdoctor.toml`, in baseline files, in JSON reports, and in SARIF results, all
of which outlive any single release.

When a rule is retired, its identifier stays reserved and reports nothing.
Configuration naming a retired rule remains valid rather than failing — a
project should not break because it was careful enough to configure something.

A rule whose meaning needs to change gets a new identifier. The old one is
retired.

## New Rules Ship Disabled For One Minor Release

A new rule enters the registry with `DefaultOn: false`. It is fully implemented,
documented, and opt-in: a project can enable it in `twdoctor.toml` immediately.

In the following minor release it becomes default-on.

This gives every project one release in which to discover the rule, measure what
it would do to their score, and either fix the findings or record them in a
baseline — before it can break a build. The registry records `Since` and
`DefaultOn` for every rule, so the current state of each is one table lookup
rather than release-note archaeology.

## The Score Model Is Frozen Per Version

These are the score model, and none of them change within a version:

- category weights
- `H`, the half-score density
- the transfer function
- which category a rule belongs to
- which exposure unit a rule is measured against

Changing any of them requires a minor release and a changelog entry stating
which direction scores are expected to move. `scoreModel.version` appears in
every report, so a consumer comparing two numbers can tell whether they were
produced by the same arithmetic.

Two scores are comparable only when `scoreModel.version` *and* the enabled rule
set match. Both are published in every report for exactly this reason.

## Quality gates

`--fail-under` applies to the headline score. Per-category sub-scores are
published but not gateable.

`--require-coverage` independently gates the percentage of candidate class lists
that extraction resolved. Both gates use exit code 1 after the report is written;
an invalid threshold remains an operational error with exit code 2. Coverage
does not alter score arithmetic.

Sub-scores are new, and a gate is a promise to keep a number stable. The
headline score is the one this project is prepared to make that promise about
today.

## Fixing A False Positive Is Not A Compatibility Event

If a rule reports something it should not, the fix ships as a patch and scores
may rise. This is stated explicitly so that nobody treats a precision fix as
breaking and defers it — a fabricated finding costs more trust than a score
that improved for a good reason.

The same applies to a confidence promotion: when a rule becomes reliable enough
to move from `medium` to `high`, scores may fall. That change is a minor release
with a changelog entry, because it has the same effect on a CI gate as adding a
rule.
