# Rule Catalog

## `color-contrast`

Category `accessibility`, measured per statically resolvable colour pair,
reported at high confidence.

Reports a foreground/background pair whose WCAG 2.2 text contrast ratio is
decisively below its applicable threshold. The rule uses 4.5:1 when explicit
same-context pixel font size proves normal text, 3:1 when size and weight prove
large text, and 3:1 as the universal lower bound when text size is inherited or
relative. A threshold-dependent ratio from 3:1 through 4.5:1 stays unknown when
text size cannot be established.

Named `text-*` and `bg-*` utilities resolve through the detected Tailwind token
inventory. Hexadecimal, RGB, HSL, Oklab, Oklch, basic named CSS colours, and
`color(srgb ...)` arbitrary values are supported. Foreground alpha is composited
over an opaque background. Transparent backgrounds, whole-element opacity,
`currentColor`, runtime variables, unsupported wide-gamut values, multiple
colour declarations, inherited counterparts, and cross-variant pairing remain
coverage gaps rather than findings.

Base, dark, responsive, state, and stacked variants are analyzed when both
colours carry the same normalized variant set in one class list. Ancestors,
computed styles, runtime theme switches, and the CSS cascade are never guessed.
When a token in the same package would pass against the fixed background, the
message names the exact `text-*` replacement.

This rule is disabled by default for its introductory minor release. Enable it
with `color-contrast = "error"` or `"warn"` under `[rules]`.

## `no-conflicting-utilities`

Category `correctness`, measured per utility. Reported at high confidence for
utility groups the tool can separate unambiguously. The token-aware property
taxonomy distinguishes text colour from font size, background colour from size
and position, and border colour from width and style. A lexical conflict that
cannot be classified conservatively remains medium confidence and score-neutral.

Reports utility classes in the same variant that target the same simple utility group, such as `p-4 p-2`. Variant-specific utilities such as `p-4 md:p-6` are treated as independent.

Stacked variants are compared as a set, so `hover:md:p-4` and `md:hover:p-2` are seen to select the same elements and do conflict. An important marker (`!p-4` or `p-4!`) and a leading minus (`-mt-2`) change how a utility applies but not which property it sets, so they do not exempt it.

## `no-arbitrary-value`

Category `consistency`, measured per utility, reported at high confidence.

Reports bracketed arbitrary values, such as `bg-[#fcfcfc]`. These often fragment a design system and should normally be replaced with a named token.

An arbitrary *variant* is not an arbitrary value: `[&_svg]:size-4` selects where a utility applies rather than hard-coding a value that should have been a token, and is not reported.

Legitimate arbitrary values exist. List them under `[arbitrary-values]` in `twdoctor.toml` so the rule stays on everywhere else; see [configuration.md](configuration.md).

When a resolved project token has exactly the same normalized value, the
message names the replacement utility. Tailwind v4's `--spacing` base is treated
as a generator for exact same-unit integer multiples. Units are never converted:
`16px` does not become `1rem` without a declared root font size.

With `--fix`, that exact replacement is applied when the utility is a verbatim
span of source. Suggestions derived from interpolated values, degraded themes,
allowlisted classes, disabled rules, or baseline entries are never changed.

## `unused-token`

Category `consistency`, measured per distinct project token declaration.

Reports a custom theme token that no resolved class list consumes. Default
Tailwind tokens are never reported. A declaration imported by multiple packages
is counted once and is used when any owning package consumes it.

This rule is disabled by default for its introductory minor release. Enable it
explicitly with `unused-token = "error"` or `"warn"` under `[rules]`. A partially
readable theme, incomplete plugin surface, unresolved class list, or ambiguous
monorepo ownership lowers the finding to medium confidence rather than making a
stronger claim than the evidence supports.

## `responsive-bloat`

Category `maintainability`, measured per class list, reported at medium
confidence and therefore score-neutral by default.

Reports a class list with five or more variant utilities. It is a maintainability signal, not a claim that responsive styles are invalid.

## Severity And Confidence

Two independent axes decide what a finding does. **Severity** is configured:
`error` is reported and scored, `warn` is reported and score-neutral, `off` is
not reported. **Confidence** is decided by the tool per finding: only `high`
moves the score by default, and `min-confidence` is how you opt into scoring the
rest. A finding is scored only when it is both an error and confident enough.
See [scoring.md](scoring.md) and [configuration.md](configuration.md).

Rule identifiers are public API and never change meaning. The compatibility
rules for adding, retiring, and re-weighting rules are in
[rule-stability.md](rule-stability.md).

## Where Rules Are Applied

Rules run over every class list extraction finds, not only literal `class`
attributes: `cva` and `clsx` leaves, Vue `:class`, Svelte `class:` directives,
Astro `class:list`, the literal part of a template literal, and CSS `@apply`.
A class value that cannot be read statically is reported as unresolved and no
rule is applied to it. See [extraction-accuracy.md](extraction-accuracy.md).
