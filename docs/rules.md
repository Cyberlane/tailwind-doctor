# Rule Catalog

## `no-conflicting-utilities`

Reports utility classes in the same variant that target the same simple utility group, such as `p-4 p-2`. Variant-specific utilities such as `p-4 md:p-6` are treated as independent.

## `no-arbitrary-value`

Reports bracketed arbitrary values, such as `bg-[#fcfcfc]`. These often fragment a design system and should normally be replaced with a named token. Some legitimate arbitrary values exist; a future configuration file will support explicit allowlists.

## `responsive-bloat`

Reports a class attribute with five or more variant utilities. It is a maintainability signal, not a claim that responsive styles are invalid.
