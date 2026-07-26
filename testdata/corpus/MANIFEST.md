# Extraction Corpus

Sixteen source files used to measure how much of a project's Tailwind class usage
the extractor actually finds. Fixtures are chosen to cover distinct extraction
shapes, not to be representative of any codebase.

Every file except the two decoys is real code from a public repository under a
permissive licence, copied verbatim. Where a file was too large to review by hand
it was trimmed to a contiguous range, recorded below; content inside a kept range
is never edited. A fixture that has been touched is no longer evidence about real
code.

`manifest.json` carries the same data in machine-readable form. Ground truth for
each fixture is in its `expected.txt`, hand-written and reviewed — never generated
from the extractor, which would make the measurement circular.

## Fixtures

| Fixture | Upstream | Path | Licence | Trimmed |
| --- | --- | --- | --- | --- |
| `astro-blog-header-link` | [withastro/astro](https://github.com/withastro/astro) | `examples/blog/src/components/HeaderLink.astro` | MIT | — |
| `decoy-commented-markup` | authored for this corpus | — | MIT | — |
| `decoy-non-attributes` | authored for this corpus | — | MIT | — |
| `flowbite-astro-sidebar` | [themesberg/flowbite-astro-admin-dashboard](https://github.com/themesberg/flowbite-astro-admin-dashboard) | `src/app/SideBar.astro` | MIT | lines 1–30 |
| `flowbite-docs-toc` | [themesberg/flowbite](https://github.com/themesberg/flowbite) | `src/docs.css` | MIT | lines 39–65 |
| `flowbite-svelte-kbd` | [themesberg/flowbite-svelte](https://github.com/themesberg/flowbite-svelte) | `src/lib/kbd/Kbd.svelte` | MIT | — |
| `gradio-walkthrough` | [gradio-app/gradio](https://github.com/gradio-app/gradio) | `js/tabs/shared/Walkthrough.svelte` | Apache-2.0 | lines 182–220 |
| `headlessui-clsx` | [tailwindlabs/headlessui](https://github.com/tailwindlabs/headlessui) | `playgrounds/react/page-examples/transitions/both-apis.tsx` | MIT | — |
| `shadcn-badge-cva` | [shadcn-ui/ui](https://github.com/shadcn-ui/ui) | `apps/v4/registry/new-york-v4/ui/badge.tsx` | MIT | — |
| `shadcn-button-cva` | [shadcn-ui/ui](https://github.com/shadcn-ui/ui) | `apps/v4/registry/new-york-v4/ui/button.tsx` | MIT | — |
| `shadcn-empty-directory` | [shadcn-ui/ui](https://github.com/shadcn-ui/ui) | `apps/v4/app/(app)/(styles)/sera/empty-state/components/empty-directory.tsx` | MIT | lines 46–95 |
| `shadcn-vue-colors-nav` | [unovue/shadcn-vue](https://github.com/unovue/shadcn-vue) | `apps/v4/components/ColorsNav.vue` | MIT | — |
| `shadcn-vue-page-nav` | [unovue/shadcn-vue](https://github.com/unovue/shadcn-vue) | `apps/v4/components/PageNav.vue` | MIT | — |
| `skeleton-decor-corners` | [skeletonlabs/skeleton](https://github.com/skeletonlabs/skeleton) | `sites/plus.skeleton.dev/src/lib/components/layout/decor-corners.svelte` | MIT | — |
| `tailwindcss-playground` | [tailwindlabs/tailwindcss](https://github.com/tailwindlabs/tailwindcss) | `playgrounds/vite/src/index.html` | MIT | — |
| `webcoreui-box` | [Frontendland/webcoreui](https://github.com/Frontendland/webcoreui) | `src/static/Box.astro` | MIT | — |

Exact commits are pinned in `manifest.json`.

## Shape Coverage

| Shape | Fixtures |
| --- | --- |
| `attr-literal` | 8 |
| `attr-interpolated` | 3 |
| `attr-multiline` | 1 |
| `jsx-template` | 1 |
| `clsx` | 2 |
| `cn` | 3 |
| `cva-leaf` | 2 |
| `cva-variant-key` | 1 |
| `cva-default-variant` | 2 |
| `vue-bind-class` | 2 |
| `svelte-class-directive` | 1 |
| `svelte-class-shorthand` | 1 |
| `astro-class-list` | 3 |
| `css-apply` | 1 |
| `object-map` | 1 |
| `out-of-scope-prop` | 1 |
| `decoy` | 4 |

Four shapes rest on a single fixture. They are the shapes for which no second
small, permissively licensed real-world example was found, and padding the corpus
with invented files would have traded evidence for the appearance of coverage.
Adding a second real fixture for each is worth doing when one turns up.

## Attribution

Fixture files remain under their original licences and copyrights, held by their
respective authors. They are included here for the sole purpose of testing this
project's extraction accuracy. The two `decoy-*` fixtures were written for this
corpus and are covered by this project's licence.

Tailwind is a trademark of Tailwind Labs. This project is not affiliated with or
endorsed by Tailwind Labs.
