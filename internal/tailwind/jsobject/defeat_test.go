package jsobject

import "testing"

func TestParseNamesTheConstructThatDefeatedIt(t *testing.T) {
	testCases := []struct {
		name      string
		source    string
		construct string
		line      int
	}{
		{name: "spread", source: "module.exports = {\n  theme: {\n    fontFamily: { ...defaultTheme.fontFamily },\n  },\n}", construct: "spread", line: 3},
		{name: "identifier reference", source: "module.exports = {\n  theme: myTheme,\n}", construct: "identifier reference", line: 2},
		{name: "arrow function", source: "module.exports = {\n  theme: { extend: { colors: (theme) => theme.colors } },\n}", construct: "function", line: 2},
		{name: "function keyword", source: "module.exports = {\n  theme: function () { return {} },\n}", construct: "function", line: 2},
		{name: "template substitution", source: "module.exports = {\n  prefix: `tw-${suffix}`,\n}", construct: "template substitution", line: 2},
		{name: "computed key", source: "module.exports = {\n  theme: { [key]: 1 },\n}", construct: "computed key", line: 2},
		{name: "call with a non-literal argument", source: "module.exports = {\n  plugins: [require(pluginName)],\n}", construct: "call with a non-literal argument", line: 2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := Parse(testCase.source)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(result.Defeats) == 0 {
				t.Fatal("no defeat recorded")
			}
			defeat := result.Defeats[0]
			if defeat.Construct != testCase.construct {
				t.Errorf("Construct = %q, want %q", defeat.Construct, testCase.construct)
			}
			if defeat.Line != testCase.line {
				t.Errorf("Line = %d, want %d", defeat.Line, testCase.line)
			}
			if defeat.Column == 0 {
				t.Error("Column = 0")
			}
		})
	}
}

func TestADefeatDoesNotDiscardTheReadablePart(t *testing.T) {
	source := "module.exports = {\n  prefix: \"tw-\",\n  theme: myTheme,\n  separator: \"_\",\n}"
	result, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(result.Defeats) != 1 {
		t.Fatalf("got %d defeats, want 1", len(result.Defeats))
	}
	prefix, found := result.Root.Get("prefix")
	if !found || prefix.Str != "tw-" {
		t.Error("prefix should remain readable")
	}
	separator, found := result.Root.Get("separator")
	if !found || separator.Str != "_" {
		t.Error("separator should remain readable")
	}
	theme, _ := result.Root.Get("theme")
	if theme.Kind != KindUnreadable {
		t.Errorf("theme = %s, want unreadable", theme.Kind)
	}
}
