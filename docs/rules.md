# Rule Catalog

## `no-conflicting-utilities`

Reports utility classes in the same variant that target the same simple utility group, such as `p-4 p-2`. Variant-specific utilities such as `p-4 md:p-6` are treated as independent.

Stacked variants are compared as a set, so `hover:md:p-4` and `md:hover:p-2` are seen to select the same elements and do conflict. An important marker (`!p-4` or `p-4!`) and a leading minus (`-mt-2`) change how a utility applies but not which property it sets, so they do not exempt it.

## `no-arbitrary-value`

Reports bracketed arbitrary values, such as `bg-[#fcfcfc]`. These often fragment a design system and should normally be replaced with a named token.

An arbitrary *variant* is not an arbitrary value: `[&_svg]:size-4` selects where a utility applies rather than hard-coding a value that should have been a token, and is not reported.

Legitimate arbitrary values exist. List them under `[arbitrary-values]` in `twdoctor.toml` so the rule stays on everywhere else; see [configuration.md](configuration.md).

## `responsive-bloat`

Reports a class list with five or more variant utilities. It is a maintainability signal, not a claim that responsive styles are invalid.

## Severity

Every rule defaults to `error`: reported, and it lowers the score. A rule can be
set to `warn` (reported, score-neutral) or `off` in `twdoctor.toml`. See
[configuration.md](configuration.md).

## Where Rules Are Applied

Rules run over every class list extraction finds, not only literal `class`
attributes: `cva` and `clsx` leaves, Vue `:class`, Svelte `class:` directives,
Astro `class:list`, the literal part of a template literal, and CSS `@apply`.
A class value that cannot be read statically is reported as unresolved and no
rule is applied to it. See [extraction-accuracy.md](extraction-accuracy.md).
