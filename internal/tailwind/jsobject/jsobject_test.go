package jsobject

import "testing"

func TestParseReadsTheExportedObject(t *testing.T) {
	testCases := []struct {
		name   string
		source string
	}{
		{name: "module.exports", source: `module.exports = { prefix: "tw-" }`},
		{name: "export default", source: `export default { prefix: "tw-" }`},
		{name: "typescript export assignment", source: `export = { prefix: "tw-" }`},
		{name: "satisfies", source: `export default { prefix: "tw-" } satisfies Config`},
		{name: "as const", source: `export default { prefix: "tw-" } as const`},
		{name: "leading import is skipped", source: "import type { Config } from 'tailwindcss'\nexport default { prefix: \"tw-\" }"},
		{name: "line comment", source: "// the config\nmodule.exports = { prefix: \"tw-\" }"},
		{name: "block comment", source: "/* the config */\nmodule.exports = { prefix: \"tw-\" }"},
		{name: "trailing comma", source: `module.exports = { prefix: "tw-", }`},
		{name: "single quotes", source: `module.exports = { prefix: 'tw-' }`},
		{name: "quoted key", source: `module.exports = { "prefix": "tw-" }`},
		{name: "template literal", source: "module.exports = { prefix: `tw-` }"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := Parse(testCase.source)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(result.Defeats) != 0 {
				t.Fatalf("unexpected defeats: %+v", result.Defeats)
			}
			prefix, found := result.Root.Get("prefix")
			if !found {
				t.Fatal("prefix not found")
			}
			if prefix.Kind != KindString || prefix.Str != "tw-" {
				t.Errorf("prefix = %s/%q, want string/tw-", prefix.Kind, prefix.Str)
			}
		})
	}
}

func TestParseReadsNestedStructure(t *testing.T) {
	source := `module.exports = {
  content: ["./src/**/*.tsx", "./index.html"],
  theme: {
    extend: {
      colors: {
        brand: { 500: "#3b82f6" },
      },
    },
  },
  darkMode: "class",
  important: true,
  future: null,
  blocklist: [],
  opacity: { 50: 0.5 },
}`

	result, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(result.Defeats) != 0 {
		t.Fatalf("unexpected defeats: %+v", result.Defeats)
	}

	theme, _ := result.Root.Get("theme")
	extend, _ := theme.Get("extend")
	colors, _ := extend.Get("colors")
	brand, found := colors.Get("brand")
	if !found {
		t.Fatal("theme.extend.colors.brand not found")
	}
	shade, found := brand.Get("500")
	if !found {
		t.Fatal("shade 500 not found")
	}
	if shade.Str != "#3b82f6" {
		t.Errorf("shade = %q, want #3b82f6", shade.Str)
	}

	content, _ := result.Root.Get("content")
	if globs := content.Strings(); len(globs) != 2 || globs[0] != "./src/**/*.tsx" {
		t.Errorf("content = %v, want two globs", globs)
	}
	important, _ := result.Root.Get("important")
	if important.Kind != KindBool || !important.Bool {
		t.Errorf("important = %s/%v, want bool/true", important.Kind, important.Bool)
	}
	future, _ := result.Root.Get("future")
	if future.Kind != KindNull {
		t.Errorf("future = %s, want null", future.Kind)
	}
	blocklist, _ := result.Root.Get("blocklist")
	if blocklist.Kind != KindArray || len(blocklist.Items) != 0 {
		t.Errorf("blocklist = %s with %d items, want an empty array", blocklist.Kind, len(blocklist.Items))
	}
	opacity, _ := result.Root.Get("opacity")
	half, _ := opacity.Get("50")
	if half.Kind != KindNumber || half.Num != "0.5" {
		t.Errorf("opacity.50 = %s/%q, want number/0.5", half.Kind, half.Num)
	}
}

func TestParseReadsACallWithoutEvaluatingIt(t *testing.T) {
	result, err := Parse(`module.exports = { plugins: [require("@tailwindcss/typography"), require('daisyui')] }`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(result.Defeats) != 0 {
		t.Fatalf("unexpected defeats: %+v", result.Defeats)
	}
	plugins, _ := result.Root.Get("plugins")
	if len(plugins.Items) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins.Items))
	}
	first := plugins.Items[0]
	if first.Kind != KindCall || first.Callee != "require" {
		t.Fatalf("first plugin = %s/%q, want call/require", first.Kind, first.Callee)
	}
	if len(first.Args) != 1 || first.Args[0].Str != "@tailwindcss/typography" {
		t.Errorf("first plugin args = %+v", first.Args)
	}
}

func TestParseRecordsPositions(t *testing.T) {
	source := "module.exports = {\n  theme: {\n    colors: { brand: \"#000\" },\n  },\n}"
	result, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	theme, _ := result.Root.Get("theme")
	colors := theme.Entries[0]
	if colors.Line != 3 || colors.Column != 5 {
		t.Errorf("colors declared at %d:%d, want 3:5", colors.Line, colors.Column)
	}
}

func TestParseRejectsSourceWithNoObject(t *testing.T) {
	if _, err := Parse("const x = 1\n"); err == nil {
		t.Error("want an error for a source with no exported object")
	}
}

func TestParseResolvesOneExportedIdentifier(t *testing.T) {
	source := "const config: Config = { prefix: \"tw-\" }\nexport default config"
	result, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	prefix, found := result.Root.Get("prefix")
	if !found || prefix.Str != "tw-" {
		t.Fatalf("prefix = %+v, found %v", prefix, found)
	}
}
