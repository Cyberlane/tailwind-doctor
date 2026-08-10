package audit

import "testing"

func FuzzExtractNeverPanics(f *testing.F) {
	for _, seed := range []string{
		`<div class="p-4 md:p-6"></div>`,
		`const value = clsx("p-4", condition && "m-2")`,
		`<div className={` + "`" + `p-[${size}px]` + "`" + `} />`,
		`@apply px-4 hover:px-6;`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		first := Extract("input.tsx", source)
		second := Extract("input.tsx", source)
		if len(first) != len(second) {
			t.Fatalf("non-deterministic extraction length: %d then %d", len(first), len(second))
		}
	})
}

func FuzzTOMLParserNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"[paths]\nignore = [\"dist/**\"]\n",
		"[rules]\nno-arbitrary-value = \"warn\"\n",
		"[tailwind]\nprefix = \"tw-\"\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		first, firstErr := parseTOML(source)
		second, secondErr := parseTOML(source)
		if (firstErr == nil) != (secondErr == nil) || len(first) != len(second) {
			t.Fatal("parser result changed across identical inputs")
		}
	})
}
