package plugins

import "testing"

func TestResolve(t *testing.T) {
	testCases := []struct {
		name        string
		specifier   string
		versions    map[string]string
		support     Support
		complete    bool
		match       string
		shouldMatch bool
	}{
		{name: "typography", specifier: "@tailwindcss/typography", versions: map[string]string{"@tailwindcss/typography": "^0.5.20"}, support: SupportComplete, complete: true, match: "prose-lg", shouldMatch: true},
		{name: "forms", specifier: "@tailwindcss/forms", versions: map[string]string{"@tailwindcss/forms": "~0.5.11"}, support: SupportComplete, complete: true, match: "form-input", shouldMatch: true},
		{name: "aspect ratio", specifier: "@tailwindcss/aspect-ratio", versions: map[string]string{"@tailwindcss/aspect-ratio": "0.4.2"}, support: SupportComplete, complete: true, match: "aspect-w-16", shouldMatch: true},
		{name: "partial ecosystem", specifier: "daisyui/plugin", versions: map[string]string{"daisyui": "^5.7.16"}, support: SupportPartial},
		{name: "out of range", specifier: "@tailwindcss/forms", versions: map[string]string{"@tailwindcss/forms": "^1.0.0"}, support: SupportOutOfRange},
		{name: "missing version", specifier: "flowbite/plugin", versions: map[string]string{}, support: SupportMissingVersion},
		{name: "unknown", specifier: "@acme/tailwind", versions: map[string]string{"@acme/tailwind": "1.0.0"}, support: SupportUnknown},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolved := Resolve([]string{testCase.specifier}, testCase.versions)
			if len(resolved) != 1 || resolved[0].Support != testCase.support || resolved[0].Complete != testCase.complete {
				t.Fatalf("coverage = %+v", resolved)
			}
			if resolved[0].Matches(testCase.match) != testCase.shouldMatch {
				t.Errorf("Matches(%q) = %v, want %v", testCase.match, resolved[0].Matches(testCase.match), testCase.shouldMatch)
			}
		})
	}
}

func TestComplete(t *testing.T) {
	if !Complete(nil) {
		t.Fatal("an empty plugin list should be complete")
	}
	if Complete([]Coverage{{Complete: true}, {Support: SupportPartial}}) {
		t.Fatal("partial plugin coverage should not be complete")
	}
}

func TestResolveIsSortedAndDeduplicated(t *testing.T) {
	resolved := Resolve([]string{"flowbite", "@tailwindcss/forms", "flowbite"}, map[string]string{
		"flowbite": "4.0.2", "@tailwindcss/forms": "0.5.11",
	})
	if len(resolved) != 2 || resolved[0].Specifier != "@tailwindcss/forms" || resolved[1].Specifier != "flowbite" {
		t.Fatalf("coverage = %+v", resolved)
	}
}
