# Rule Catalog

## `no-conflicting-utilities`

Category `correctness`, measured per utility. Reported at high confidence for
utility groups the tool can separate unambiguously, and at medium confidence —
therefore score-neutral — for `text-`, `bg-`, and `border-`, where a shorthand
and a colour currently land in the same group.

Reports utility classes in the same variant that target the same simple utility group, such as `p-4 p-2`. Variant-specific utilities such as `p-4 md:p-6` are treated as independent.

Stacked variants are compared as a set, so `hover:md:p-4` and `md:hover:p-2` are seen to select the same elements and do conflict. An important marker (`!p-4` or `p-4!`) and a leading minus (`-mt-2`) change how a utility applies but not which property it sets, so they do not exempt it.

## `no-arbitrary-value`

Category `consistency`, measured per utility, reported at high confidence.

Reports bracketed arbitrary values, such as `bg-[#fcfcfc]`. These often fragment a design system and should normally be replaced with a named token.

An arbitrary *variant* is not an arbitrary value: `[&_svg]:size-4` selects where a utility applies rather than hard-coding a value that should have been a token, and is not reported.

Legitimate arbitrary values exist. List them under `[arbitrary-values]` in `twdoctor.toml` so the rule stays on everywhere else; see [configuration.md](configuration.md).

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
