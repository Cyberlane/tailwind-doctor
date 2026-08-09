package cssdecl

import "testing"

func TestParseReadsAtRulesAndDeclarationsInOrder(t *testing.T) {
	source := `@import "tailwindcss" prefix(tw);

@theme {
  --color-*: initial;
  --color-brand-500: #3b82f6;
  --spacing-gutter: 1.5rem;
}

:root {
  --brand-raw: #3b82f6;
}
`
	sheet, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Nodes) != 3 {
		t.Fatalf("got %d top-level nodes, want 3: %+v", len(sheet.Nodes), sheet.Nodes)
	}
	importRule := sheet.Nodes[0]
	if importRule.Kind != NodeAtRule || importRule.Name != "import" || importRule.Prelude != `"tailwindcss" prefix(tw)` {
		t.Fatalf("import = %+v", importRule)
	}
	theme := sheet.Nodes[1]
	if theme.Kind != NodeAtRule || theme.Name != "theme" || theme.Prelude != "" {
		t.Fatalf("theme = %+v", theme)
	}
	if len(theme.Children) != 3 {
		t.Fatalf("theme has %d children, want 3", len(theme.Children))
	}
	if theme.Children[0].Property != "--color-*" || theme.Children[0].Value != "initial" {
		t.Errorf("first declaration = %+v", theme.Children[0])
	}
	if theme.Children[1].Line != 5 {
		t.Errorf("brand declared on line %d, want 5", theme.Children[1].Line)
	}
	root := sheet.Nodes[2]
	if root.Kind != NodeRule || root.Selector != ":root" || len(root.Children) != 1 || root.Children[0].Property != "--brand-raw" {
		t.Fatalf("root = %+v", root)
	}
}

func TestParseReadsThemeOptions(t *testing.T) {
	testCases := []struct{ name, source, prelude string }{
		{name: "plain", source: "@theme { --color-a: red; }", prelude: ""},
		{name: "inline", source: "@theme inline { --font-sans: var(--font-inter); }", prelude: "inline"},
		{name: "static", source: "@theme static { --color-a: red; }", prelude: "static"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sheet, err := Parse(testCase.source)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if sheet.Nodes[0].Prelude != testCase.prelude {
				t.Errorf("Prelude = %q, want %q", sheet.Nodes[0].Prelude, testCase.prelude)
			}
		})
	}
}

func TestParseIgnoresCommentsAndStrings(t *testing.T) {
	source := `/* @theme { --color-decoy: red; } */
@theme {
  /* --color-also-decoy: red; */
  --color-real: #000;
  --content-quoted: "a; b { }";
}
`
	sheet, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Nodes) != 1 || len(sheet.Nodes[0].Children) != 2 {
		t.Fatalf("sheet = %+v", sheet)
	}
	children := sheet.Nodes[0].Children
	if children[0].Property != "--color-real" || children[1].Value != `"a; b { }"` {
		t.Fatalf("children = %+v", children)
	}
}

func TestParseReadsSemicolonAtRules(t *testing.T) {
	sheet, err := Parse("@plugin \"@tailwindcss/typography\";\n@config \"../tailwind.config.js\";\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(sheet.Nodes) != 2 || sheet.Nodes[0].Name != "plugin" || sheet.Nodes[0].Prelude != `"@tailwindcss/typography"` || sheet.Nodes[1].Name != "config" {
		t.Fatalf("nodes = %+v", sheet.Nodes)
	}
}

func TestParseReadsNestedAtRules(t *testing.T) {
	sheet, err := Parse("@media (min-width: 48rem) {\n  @theme {\n    --spacing-gutter: 2rem;\n  }\n}\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	media := sheet.Nodes[0]
	if media.Name != "media" || len(media.Children) != 1 || media.Children[0].Name != "theme" {
		t.Fatalf("media = %+v", media)
	}
}

func TestParseRejectsAnUnterminatedBlock(t *testing.T) {
	if _, err := Parse("@theme { --color-a: red;"); err == nil {
		t.Error("want an unterminated-block error")
	}
}
