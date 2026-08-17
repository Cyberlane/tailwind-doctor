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
taxonomy distinguishes text colour from font size, background colour from size,
position, clip, and origin, and border colour from width and style — per side,
so `border-y` and `border-r` never conflict and `border-l-transparent` is a
colour, not a width. When a file has no resolved project theme, classification
falls back to Tailwind's default theme, so default scale names such as
`text-4xl` and `text-gray-600` keep their meaning. A lexical conflict that
cannot be classified conservatively remains medium confidence and
score-neutral, even when a theme resolved.

Reports utility classes in the same variant that target the same simple utility group, such as `p-4 p-2`. Variant-specific utilities such as `p-4 md:p-6` are treated as independent.

Stacked variants are compared in source order. Tailwind contains order-sensitive
variant combinations, so only identical stacks such as `hover:md:p-4` and
`hover:md:p-2` are treated as the same selector context. An important marker
(`!p-4` or `p-4!`) and a leading minus (`-mt-2`) change how a utility applies but
not which property it sets, so they do not exempt it.

## `no-overlapping-utilities`

Category `correctness`, measured per utility, reported at medium confidence.

Reports utilities in the same ordered variant context whose property sets
partially overlap, such as `px-4 pl-2`. This is separate from
`no-conflicting-utilities`: the latter identifies two utilities that set the
same property group, while partial overlap may be an intentional override and
therefore stays score-neutral at the default confidence threshold.

This rule is disabled by default for its introductory minor release. Enable it
with `no-overlapping-utilities = "error"` or `"warn"` under `[rules]`.

## `no-arbitrary-value`

Category `consistency`, measured per utility, reported at high confidence.

Reports bracketed arbitrary values, such as `bg-[#fcfcfc]`. These often fragment a design system and should normally be replaced with a named token.

Only classes in a recognized Tailwind utility namespace are inspected.
Application classes do not become findings merely because their name contains
brackets, and they do not enter the score denominator.

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

Retired in 0.2.0. The identifier remains reserved and valid in configuration,
but reports nothing. Counting any five variant utilities overstated sparse,
well-factored class lists; `variant-density` replaces it with a proportional
signal under a new public rule ID.

## `variant-density`

Category `maintainability`, measured per class list, reported at medium
confidence and therefore score-neutral by default.

Reports a class list containing at least four variant utilities when they make
up at least 60% of its recognized Tailwind utilities. It is a prompt to consider
extracting a component or variant, not a claim that responsive or state styles
are invalid.

This rule is disabled by default for its introductory minor release. Enable it
with `variant-density = "error"` or `"warn"` under `[rules]`.

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
