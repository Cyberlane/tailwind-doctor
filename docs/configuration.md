# Configuration

Tailwind Doctor runs with no configuration. Everything here is optional, and a
project that needs none of it is analysed with the defaults described below.

Two files are read, both from the directory being analysed:

| File | Purpose |
| --- | --- |
| `twdoctor.toml` | Settings: rule severities, path ignores, Tailwind syntax options |
| `twdoctor-baseline.json` | Debt already accepted, so a build gates on new findings only |

A configuration file that cannot be understood stops the run with exit code 2.
Analysing with settings that were silently half-applied produces a number nobody
can trust and nobody can debug.

## `twdoctor.toml`

```toml
[rules]
# error (default) — reported, and moves the score
# warn            — reported, but does not move the score
# off             — not reported at all
no-arbitrary-value = "warn"
variant-density = "warn"
no-overlapping-utilities = "warn"
# New in this release and disabled by default for one minor release.
unused-token = "warn"
color-contrast = "warn"

[paths]
# Globs, relative to the analysed directory. ** matches any number of segments.
ignore = [
  "generated/**",
  "**/*.min.html",
]
# Read .gitignore files during discovery. Defaults to true.
respect-gitignore = true

[arbitrary-values]
# Exact class strings that no-arbitrary-value should not report. For the one-off
# value that genuinely has no token, so the rule stays on everywhere else.
allow = ["text-[#123456]"]

[tailwind]
# The Tailwind v3 `prefix` option, if the project sets one.
prefix = "tw-"
# The Tailwind v3 `separator` option. Defaults to ":".
separator = ":"

[score]
# The lowest confidence a finding may carry and still move the score.
# high (default) | medium | low
min-confidence = "high"

[baseline]
# Where the suppression file lives. Defaults to twdoctor-baseline.json.
path = "twdoctor-baseline.json"
```

Every table and every key is optional. Unknown tables, settings, and rule IDs;
duplicate keys or tables; invalid globs; and baseline paths outside the analysis
root are errors. This makes misspelled configuration fail closed.

When a prefix is configured or detected, unprefixed class names are treated as
application CSS rather than Tailwind utilities. They produce no findings, do not
consume tokens, and do not enter score exposure.

### Severity

A rule's severity decides what it does when it matches:

- **`error`** — reported, and lowers the score when confidence clears the
  configured threshold. This is the established-rule default; newly introduced
  rules remain off for one minor release.
- **`warn`** — reported, but score-neutral. This is how a rule stays useful
  before a team trusts it enough to gate a build on.
- **`off`** — not reported.

Severity appears in JSON output as the `severity` field on each finding.

### Confidence

Severity is what you configure; confidence is what the tool is willing to stand
behind. They are independent, and a finding is scored only when it clears both:
its severity is `error` *and* its confidence is at least `min-confidence`.

Nothing is ever hidden by confidence. A finding below the threshold is reported,
tagged in every output format, and carries `"scored": false` in JSON — a visibly
uncertain finding costs less trust than a silent miss.

The following evidence can report below `high`, so `min-confidence = "medium"`
is what scores it:

- `variant-density`, because density is a maintainability heuristic rather than
  a defect.
- `no-overlapping-utilities`, because partial property overlap can be intentional.
- an unclassifiable utility conflict that the property taxonomy cannot separate
  conservatively;
- `unused-token` when configuration, plugin, extraction, or package-ownership
  evidence is incomplete.

See [scoring.md](scoring.md) for what the score does with them, and
[rules.md](rules.md) for the per-rule detail.

### Path ignores

Patterns are matched against slash-separated paths relative to the analysed
directory. `*` matches within one path segment; `**` matches any number of
segments, including none, so `dist/**` excludes `dist` and everything under it.

Independently of configuration, discovery always skips `.git`, `node_modules`,
`dist`, `build`, `.next`, and `vendor`.

### `.gitignore`

With `respect-gitignore` left on, `.gitignore` files are loaded before Tailwind
package discovery and apply to both configuration evidence and source analysis.
Supported:
comments, blank lines, `!` negation, a trailing `/` for directories, a leading
`/` to anchor, and `*`/`**` globs.

Not supported: `.git/info/exclude` and the global excludes file.

### Supported TOML

The reader covers tables, string values, booleans, and arrays of strings —
exactly what the settings above need. Integers, floats, dates, inline tables,
nested tables, and multi-line strings are not supported, and a file using them
is rejected rather than half-read.

This is a deliberate trade. The analysis core ships with no third-party
dependencies, which is what makes it a single static binary safe to point at a
private codebase, and a full TOML implementation is not worth spending that on
for a handful of settings.

## `twdoctor-baseline.json`

A project adopting this tool on an existing codebase starts with debt it did not
just write. Without a way to record that, the only threshold that passes today is
one low enough to gate nothing.

Record the current state:

```bash
tw-doctor --write-baseline .
```

That writes every current finding to the baseline file and exits 0. Later runs
report only findings not in it, and the count of suppressed findings appears in
the report as `suppressed`, so accepted debt stays visible rather than vanishing.

```json
{
  "version": 2,
  "note": "Debt accepted at the time this file was written. ...",
  "suppressed": [
    {
      "fingerprint": "sha256:...",
      "rule": "no-conflicting-utilities",
      "file": "src/components/card.tsx",
      "class": "p-4 p-2 rounded",
      "reason": "optional; for humans reading this file"
    }
  ]
}
```

### Stable fingerprints and version 1 migration

A fingerprint is derived from **rule, file, and class evidence** — never by position.
Adding an import above a component would otherwise shift every line in it and
resurrect all of that file's accepted debt as new debt, on a change that altered
nothing. The cost of leaving position out is that two identical class lists in
one file are suppressed together; that is the right trade, because the alternative
makes the file fail on formatting.

Version 2 requires the explicit fingerprint and rejects unknown JSON fields.
The reader recomputes each fingerprint from its visible evidence and refuses a
mismatch rather than accepting an opaque suppression key.
Version 1 files remain readable so the CLI can be upgraded before the baseline
is regenerated; `--write-baseline` always writes version 2. Baseline replacement
uses a same-directory temporary file and atomic rename. `reason` remains solely
for humans.

### Removing debt

Delete an entry to start failing on it again. Regenerating the whole file with
`--write-baseline` also works, but reviewing that diff is harder — it re-accepts
anything added since, which is exactly what the baseline exists to prevent.

## Conservative source fixes

`tw-doctor --fix .` replaces an arbitrary value only when all of the following
are true:

- the `no-arbitrary-value` rule is enabled and the finding is neither
  allowlisted nor baselined;
- the class is a verbatim span of source rather than a value reconstructed
  around a runtime interpolation;
- a non-degraded static Tailwind inventory proves an exact named-token match;
- the source still contains the exact class at the reported byte position when
  the edit is prepared.

Units are never converted, runtime expressions are never evaluated,
symbolic-link source files are not followed, and all candidate edits are
validated before the first file is replaced. The report and `--fail-under` gate
are computed again after the fixes, so JSON and SARIF describe the resulting
source. Each file replacement is atomic. If an operating-system error occurs
after earlier files were replaced, stderr reports the exact applied file and
replacement counts before returning exit code 2.
