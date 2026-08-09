package tailwind

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

var updateGolden = flag.Bool("update", false, "rewrite the committed golden inventories")

// The golden format is one token per line, sorted, so a diff shows exactly which
// token changed rather than a reordered blob.
func TestGoldenInventories(t *testing.T) {
	fixtures := []string{"v3-basic", "v4-basic", "v3-unreadable", "v4-cleared", "monorepo"}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			root := filepath.Join("testdata", "projects", fixture)
			rendered, err := renderInventories(root)
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			golden := filepath.Join("testdata", "golden", "inventory-"+fixture+".txt")
			if *updateGolden {
				if err := os.WriteFile(golden, []byte(rendered), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update to create it): %v", err)
			}
			if rendered != string(want) {
				t.Errorf("inventory changed:\n--- want ---\n%s\n--- got ---\n%s", want, rendered)
			}
		})
	}
}

// renderInventories writes every package's project tokens, its diagnostics, and
// its degraded flag. Default tokens are summarised by count rather than listed.
func renderInventories(root string) (string, error) {
	layout, err := Discover(os.DirFS(root))
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, pkg := range layout.Packages {
		fmt.Fprintf(&builder, "package %s\n", pkg.Dir)
		fmt.Fprintf(&builder, "  version %s\n", pkg.Version)

		adapter, found := AdapterFor(pkg.Version)
		if !found {
			fmt.Fprintln(&builder, "  no adapter")
			continue
		}
		theme, err := adapter.Load(os.DirFS(root), pkg)
		if err != nil {
			return "", err
		}

		fmt.Fprintf(&builder, "  degraded %v\n", theme.Degraded)
		if theme.Syntax.Prefix != "" {
			fmt.Fprintf(&builder, "  prefix %s variant=%v\n", theme.Syntax.Prefix, theme.Syntax.PrefixIsVariant)
		}
		for _, plugin := range theme.Plugins {
			fmt.Fprintf(&builder, "  plugin %s\n", plugin)
		}
		for _, diagnostic := range theme.Diagnostics {
			fmt.Fprintf(&builder, "  diagnostic %s %s:%d:%d %s\n",
				diagnostic.Kind, diagnostic.File, diagnostic.Line, diagnostic.Column, diagnostic.Message)
		}

		defaultCounts := map[string]int{}
		for _, token := range theme.Inventory.Tokens() {
			if token.Origin == tokens.OriginDefault {
				defaultCounts[string(token.Family)]++
				continue
			}
			fmt.Fprintf(&builder, "  token %s %s = %s (%s) %s:%d:%d\n",
				token.Family, token.Name, token.Value, token.Path,
				token.Decl.File, token.Decl.Line, token.Decl.Column)
		}

		families := make([]string, 0, len(defaultCounts))
		for family := range defaultCounts {
			families = append(families, family)
		}
		sort.Strings(families)
		for _, family := range families {
			fmt.Fprintf(&builder, "  defaults %s %d\n", family, defaultCounts[family])
		}
	}
	return builder.String(), nil
}

func TestMonorepoFileScoping(t *testing.T) {
	root := filepath.Join("testdata", "projects", "monorepo")
	layout, err := Discover(os.DirFS(root))
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	testCases := []struct {
		file  string
		dir   string
		found bool
	}{
		{file: "packages/ui/src/button.tsx", dir: "packages/ui", found: true},
		{file: "packages/web/src/page.tsx", dir: "packages/web", found: true},
		{file: "src/loose.tsx", found: false},
	}

	for _, testCase := range testCases {
		pkg, found := layout.PackageFor(testCase.file)
		if found != testCase.found {
			t.Errorf("%s: found = %v, want %v", testCase.file, found, testCase.found)
			continue
		}
		if found && pkg.Dir != testCase.dir {
			t.Errorf("%s bound to %q, want %q", testCase.file, pkg.Dir, testCase.dir)
		}
	}
}

func TestGoldenInventoriesAreDeterministic(t *testing.T) {
	for _, fixture := range []string{"v3-basic", "v4-basic", "monorepo"} {
		root := filepath.Join("testdata", "projects", fixture)
		first, err := renderInventories(root)
		if err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		for attempt := 0; attempt < 10; attempt++ {
			again, err := renderInventories(root)
			if err != nil {
				t.Fatalf("%s: %v", fixture, err)
			}
			if again != first {
				t.Fatalf("%s: inventory rendering varies between runs", fixture)
			}
		}
	}
}
