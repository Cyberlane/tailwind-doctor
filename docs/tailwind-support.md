# Tailwind version support

Tailwind Doctor supports both Tailwind CSS v3 and v4 without executing project
code.

| Tailwind version | Status | Theme source | Degraded behaviour |
|---|---|---|---|
| v3 (3.x) | Supported | `tailwind.config.{js,cjs,mjs,ts,mts,cts}` is read statically; a manifest-only package uses v3 defaults | If a construct cannot be read without executing it, the package uses the default theme and reports why |
| v4 (4.x) | Supported | CSS `@theme` blocks and custom properties across every package entry; a manifest-only package uses v4 defaults | Theme declarations that can be read statically form the inventory |
| Anything else, or no version signal | No token inventory | None | Token-dependent rules do not run; an explicitly unsupported major is diagnosed |

## Version detection

Tailwind Doctor records every signal it finds. A version is never guessed: no
signal means no token inventory.

Tailwind v4 signals are:

- a CSS file containing `@import "tailwindcss"` or an `@theme` block;
- `tailwindcss` at major version 4 in `package.json`; or
- a dependency on `@tailwindcss/vite` or `@tailwindcss/postcss`.

Tailwind v3 signals are:

- a `tailwind.config.{js,cjs,mjs,ts,mts,cts}` file;
- `tailwindcss` at major version 3 in `package.json`; or
- `@tailwind base;` in CSS.

`package.json` is authoritative when it resolves a major version. Reports carry
the complete evidence trail so the decision remains inspectable. A declared
unsupported major never falls through to a v3 config or v4 stylesheet and gets
misidentified as a supported project.

Mixed signals are normal. Tailwind v4 supports `@config
"tailwind.config.js"`, so a genuine v4 project may contain a v3-style config,
and a repository partway through a migration may contain both kinds of signal.
The detected Tailwind version and the dialect from which a theme is read are
therefore separate facts.

## Token families

Both dialects become the same canonical inventory.

| Family | Tailwind v3 theme key | Tailwind v4 namespace |
|---|---|---|
| Colour | `colors` | `--color-*` |
| Spacing | `spacing` | `--spacing-*` |
| Font family | `fontFamily` | `--font-*` |
| Font size | `fontSize` | `--text-*` |
| Font weight | `fontWeight` | `--font-weight-*` |
| Line height | `lineHeight` | `--leading-*` |
| Letter spacing | `letterSpacing` | `--tracking-*` |
| Border radius | `borderRadius` | `--radius-*` |
| Box shadow | `boxShadow` | `--shadow-*` |
| Breakpoint | `screens` | `--breakpoint-*` |
| Container | No tokens | `--container-*` |
| Z-index | `zIndex` | Dynamic values; no theme namespace |

Tailwind v3's `theme.container` is an options object rather than a token scale,
so it does not produce container tokens.

## Deliberate limits

Tailwind `content` globs do not control source discovery. Relative `@import`
and `presets` references are followed only while they remain inside the
project; imports from `node_modules` are never read. Additional v4 theme
namespaces remain candidates for later conservative inventory coverage.

The analyzer never runs a project's JavaScript. A configuration assembled by a
function, spread, or imported value cannot be interpreted safely. When that
happens, the report names the unsupported construct and its position, uses the
default theme for that package, and lowers the confidence of token findings.
The diagnostic does not turn an otherwise successful analysis into an
operational error.

## Plugin coverage

Configured plugins are identified lexically from v3 `plugins` arrays and v4
`@plugin` directives. Tailwind Doctor never imports a plugin or reads its code
from `node_modules`. Instead, a small curated registry describes reviewed
utility surfaces for `@tailwindcss/typography`, `@tailwindcss/forms`, and
`@tailwindcss/aspect-ratio`. The package manifest supplies the declared version
range used to select that coverage.

The registry also recognizes current daisyUI and Flowbite major versions, but
marks their broad utility surfaces as intentionally partial. An unknown plugin,
a missing version, or a version outside the reviewed range is explicit
incomplete coverage. Token findings use that evidence to lower confidence rather
than pretending the analyzer saw utilities it cannot classify safely.

Contrast analysis requires a non-degraded theme and complete configured-plugin
coverage before it trusts named colour utilities. A partial static read or an
unknown plugin makes candidate pairs explicit unknown coverage instead of
allowing a possibly overridden colour value to produce a false failure.

## Monorepos

Every source file is scoped to the nearest Tailwind configuration or manifest
in an ancestor directory. Inventories remain package-local and are never merged, so
two packages may define the same token name with different values.

Tailwind is a trademark of Tailwind Labs. This project is not affiliated with
or endorsed by Tailwind Labs.

## Default theme values

Tailwind's default theme values are compiled into this tool so that a project's
`theme.extend` can be resolved against the same base Tailwind uses, and so that a
configuration this tool cannot read still has a theme to fall back on. Those
values are taken from Tailwind CSS, which is MIT licensed, Copyright (c) Tailwind
Labs, Inc. The exact upstream version each table was taken from is recorded in a
comment beside it.
