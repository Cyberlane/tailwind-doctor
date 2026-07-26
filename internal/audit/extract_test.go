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

func TestExtractReportsExpressionValuesAsUnresolved(t *testing.T) {
	got := summarise(Extract("card.tsx", `<div className={cn(base, className)} />`))
	want := []string{`1:17 unresolved attr-interpolated "cn(base, className)"`}
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

// A binding prefix makes the value an expression. Reading :class as class would
// report the expression's source text as a list of utilities.
func TestExtractDoesNotTreatBoundAttributesAsPlainClass(t *testing.T) {
	got := Extract("nav.vue", `<div :class="cn('flex', props.class)"></div>`)
	if len(got) != 0 {
		t.Fatalf("expected nothing extracted from a bound attribute, got %v", summarise(got))
	}
}
