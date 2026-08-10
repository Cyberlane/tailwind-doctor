package tailwind

import (
	"reflect"
	"testing"
)

func FuzzUtilitySyntaxNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"hover:dark:text-[#123456]", "[&:nth-child(2)]:p-4", "!-mt-[2px]", "tw:p-4!",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, utility string) {
		syntax := DefaultUtilitySyntax()
		first := ParseUtility(utility, syntax)
		second := ParseUtility(utility, syntax)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("non-deterministic parse: %#v then %#v", first, second)
		}
	})
}
