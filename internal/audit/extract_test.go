package audit

import (
	"fmt"
	"strings"
	"testing"
)

// summarise renders one extraction as a single comparable line, so a table case
// asserts position, shape, and resolution together rather than value alone.
func summarise(lists []ClassList) []string {
	rendered := make([]string, 0, len(lists))
	for _, list := range lists {
		state := "resolved"
		if !list.Resolved {
			state = "unresolved"
		}
		rendered = append(rendered, fmt.Sprintf("%d:%d %s %s %q", list.Line, list.Column, state, list.Shape, list.Value))
	}
	return rendered
}

func TestExtractFindsClassAttributes(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		source string
		want   []string
	}{
		{
			name:   "plain html attribute",
			path:   "page.html",
			source: `<div class="flex gap-2"></div>`,
			want:   []string{`1:13 resolved attr-literal "flex gap-2"`},
		},
		{
			name:   "jsx className",
			path:   "card.tsx",
			source: `const Card = () => <div className="p-4 shadow" />`,
			want:   []string{`1:36 resolved attr-literal "p-4 shadow"`},
		},
		{
			name:   "attribute value spanning lines",
			path:   "page.html",
			source: "<div\n  class=\"p-4\n  m-2\"\n></div>",
			want:   []string{"2:10 resolved attr-literal \"p-4\\n  m-2\""},
		},
		{
			name:   "single quoted attribute",
			path:   "page.html",
			source: `<div class='rounded border'></div>`,
			want:   []string{`1:13 resolved attr-literal "rounded border"`},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := summarise(Extract(test.path, test.source))
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

// Everything here looks like a class attribute to a text search and is not one.
// A regression in this test is a regression in the false-positive rate, which is
// the number the project is judged on.
func TestExtractIgnoresTextThatMerelyLooksLikeAnAttribute(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		source string
	}{
		{"line comment", "app.tsx", `// className = "p-4 m-2"`},
		{"block comment", "app.tsx", `/* class="flex gap-2" */`},
		{"string literal", "app.tsx", `const sample = 'class="grid gap-4"'`},
		{"template literal", "app.tsx", "const help = `set className=\"rounded\" here`"},
		{"class declaration", "app.tsx", `class Registry { }`},
		{"html comment", "page.html", `<!-- <div class="flex gap-2"></div> -->`},
		{"script body", "page.html", `<script><li class="hidden"></li></script>`},
		{"style body", "page.html", `<style>.thing { color: red }</style>`},
		{"escaped markup in text", "page.html", `<p>&lt;div class="p-2"&gt;</p>`},
		{"value of another attribute", "page.html", `<p aria-label="write class = 'p-8'"></p>`},
		{"astro frontmatter", "page.astro", "---\nconst a = 'class=\"p-4\"'\n---\n<p>hi</p>"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := Extract(test.path, test.source); len(got) != 0 {
				t.Fatalf("expected nothing extracted, got %v", summarise(got))
			}
		})
	}
}

// A value that is only partly knowable yields both halves of the truth: the
// literal utilities, which are real and must be linted, and the substitution,
// which must be reported as unknown rather than folded in as a utility.
func TestExtractSplitsInterpolatedAttributes(t *testing.T) {
	got := summarise(Extract("box.svelte", `<div class="absolute p-2 {corner}"></div>`))
	want := []string{
		`1:26 unresolved attr-interpolated "{corner}"`,
		`1:13 resolved attr-interpolated "absolute p-2"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A substitution inside a token makes the whole token unknowable. Splitting on
// the substitution invented utilities like "text-[" that the source never
// contained, which were then linted — and scored — as real classes.
func TestExtractDropsTokensSplitByInterpolation(t *testing.T) {
	got := summarise(Extract("bar.svelte", `<div class="absolute text-[{size}px] font-bold"></div>`))
	want := []string{
		`1:22 unresolved attr-interpolated "text-[{size}px]"`,
		`1:13 resolved attr-interpolated "absolute font-bold"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The template-literal form of the same shape, including a space inside the
// substitution, which must not split the token either.
func TestExtractDropsTemplateTokensSplitBySubstitution(t *testing.T) {
	got := summarise(Extract("bar.tsx", "<div className={`absolute text-[${height / 2}px] z-[1]`} />"))
	want := []string{
		`1:27 unresolved jsx-template "text-[${height / 2}px]"`,
		`1:18 resolved jsx-template "absolute z-[1]"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A helper call is read argument by argument, so what is knowable is kept and
// only the specific values that are not are reported as unknown.
func TestExtractReportsExpressionValuesAsUnresolved(t *testing.T) {
	got := summarise(Extract("card.tsx", `<div className={cn(base, className)} />`))
	want := []string{
		`1:20 unresolved cn "base"`,
		`1:26 unresolved cn "className"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A Svelte class: directive names its class in the attribute, so the class is
// knowable even when the condition that toggles it is not.
func TestExtractReadsSvelteClassDirectives(t *testing.T) {
	got := summarise(Extract("panel.svelte", `<div class:hidden={closed} class:compact></div>`))
	want := []string{
		`1:12 resolved svelte-class-directive "hidden"`,
		`1:34 resolved svelte-class-shorthand "compact"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A binding prefix makes the value an expression, so it is read as one. Treating
// :class as the plain class attribute would report the expression's source text
// as a list of utilities.
func TestExtractReadsBoundAttributesAsExpressions(t *testing.T) {
	got := summarise(Extract("nav.vue", `<div :class="cn('flex', props.class)"></div>`))
	want := []string{
		`1:18 resolved vue-bind-class "flex"`,
		`1:25 unresolved vue-bind-class "props.class"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractReadsClassHelpers(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		source string
		want   []string
	}{
		{
			name:   "clsx string arguments",
			path:   "app.tsx",
			source: "import clsx from 'clsx';\nconst a = clsx('p-4', 'm-2')",
			want: []string{
				`2:17 resolved clsx "p-4"`,
				`2:24 resolved clsx "m-2"`,
			},
		},
		{
			name:   "object keys are classes and values are conditions",
			path:   "app.tsx",
			source: "import clsx from 'clsx';\nconst a = clsx({ 'p-4': isWide, 'm-2': isTall })",
			want: []string{
				`2:19 resolved clsx "p-4"`,
				`2:34 resolved clsx "m-2"`,
			},
		},
		{
			name:   "arguments that are not literals are unresolved",
			path:   "app.tsx",
			source: "import { cn } from './utils';\nconst a = cn('p-4', props.class)",
			want: []string{
				`2:15 resolved cn "p-4"`,
				`2:21 unresolved cn "props.class"`,
			},
		},
		{
			name:   "a nested call is reported whole rather than guessed at",
			path:   "app.tsx",
			source: "import { cn } from './utils';\nconst a = cn(buttonVariants({ size }))",
			want:   []string{`2:14 unresolved cn "buttonVariants({ size })"`},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := summarise(Extract(test.path, test.source))
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestExtractRequiresBindingEvidenceForModuleHelpers(t *testing.T) {
	source := `function cn(message: string) { return message }
const diagnostic = cn("text-[#123456] p-4 p-2")`
	if got := Extract("example.tsx", source); len(got) != 0 {
		t.Fatalf("unrelated local cn produced class lists: %v", summarise(got))
	}
}

func TestExtractReadsImportedHelperAliases(t *testing.T) {
	source := `import {
  clsx as mergeClasses,
} from "clsx";
const value = mergeClasses("p-4", "m-2")`
	got := summarise(Extract("example.tsx", source))
	want := []string{
		`4:29 resolved clsx "p-4"`,
		`4:36 resolved clsx "m-2"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A cva call holds class lists in three places and variant names in two others.
// Reading a variant name as a class list is a fabricated finding, so the
// distinction is asserted rather than assumed.
func TestExtractReadsCvaLeavesButNotVariantNames(t *testing.T) {
	source := `import { cva } from "class-variance-authority";
const button = cva("inline-flex", {
  variants: {
    size: { sm: "h-8 px-3", "icon-lg": "size-10" },
  },
  compoundVariants: [{ size: "sm", class: "gap-1" }],
  defaultVariants: { size: "sm" },
})`
	got := summarise(Extract("button.tsx", source))
	want := []string{
		`2:21 resolved cva-leaf "inline-flex"`,
		`4:18 resolved cva-leaf "h-8 px-3"`,
		`4:41 resolved cva-leaf "size-10"`,
		`6:44 resolved cva-leaf "gap-1"`,
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractReadsFrameworkBindings(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		source string
		want   []string
	}{
		{
			name:   "vue bound class keeps the binding shape",
			path:   "nav.vue",
			source: `<template><div :class="cn('flex gap-2', props.class)"></div></template>`,
			want: []string{
				`1:28 resolved vue-bind-class "flex gap-2"`,
				`1:41 unresolved vue-bind-class "props.class"`,
			},
		},
		{
			name:   "astro class:list array and object keys",
			path:   "page.astro",
			source: `<div class:list={['p-4', { active: isOpen }, other]}></div>`,
			want: []string{
				`1:20 resolved astro-class-list "p-4"`,
				`1:28 resolved astro-class-list "active"`,
				`1:46 unresolved astro-class-list "other"`,
			},
		},
		{
			name:   "jsx template literal splits at the substitution",
			path:   "card.tsx",
			source: "const a = <div className={`size-4 rounded ${tone}`} />",
			want: []string{
				`1:43 unresolved jsx-template "${tone}"`,
				`1:28 resolved jsx-template "size-4 rounded"`,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := summarise(Extract(test.path, test.source))
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestExtractReadsApplyRules(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		source string
		want   []string
	}{
		{
			name:   "css file",
			path:   "site.css",
			source: ".card {\n\t@apply rounded-lg p-4;\n}",
			want:   []string{`2:9 resolved css-apply "rounded-lg p-4"`},
		},
		{
			name:   "important keyword is not a utility",
			path:   "site.css",
			source: ".card { @apply p-4 !important; }",
			want:   []string{`1:16 resolved css-apply "p-4"`},
		},
		{
			name:   "style block of a component",
			path:   "card.vue",
			source: "<template><i /></template>\n<style>\n.card { @apply p-2; }\n</style>",
			want:   []string{`3:16 resolved css-apply "p-2"`},
		},
		{
			name:   "commented out apply",
			path:   "site.css",
			source: ".card {\n\t/* @apply p-4; */\n}",
			want:   nil,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := summarise(Extract(test.path, test.source))
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
