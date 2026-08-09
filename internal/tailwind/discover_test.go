package tailwind

import (
	"slices"
	"testing"
	"testing/fstest"
)

func TestDiscoverFindsOnePackagePerConfig(t *testing.T) {
	files := fstest.MapFS{
		"package.json":                   &fstest.MapFile{Data: []byte(`{"workspaces":["packages/*"]}`)},
		"packages/ui/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"packages/ui/src/button.tsx":     &fstest.MapFile{Data: []byte(`<button className="p-4" />`)},
		"packages/web/package.json":      &fstest.MapFile{Data: []byte(`{"dependencies":{"tailwindcss":"^4.1.0"}}`)},
		"packages/web/src/app.css":       &fstest.MapFile{Data: []byte(`@import "tailwindcss";`)},
		"packages/web/src/page.tsx":      &fstest.MapFile{Data: []byte(`<div className="p-2" />`)},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(layout.Packages) != 2 {
		t.Fatalf("got %d packages, want 2: %+v", len(layout.Packages), layout.Packages)
	}
	if layout.Packages[0].Dir != "packages/ui" || layout.Packages[0].Version != Version3 ||
		layout.Packages[0].ManifestFile != "package.json" {
		t.Errorf("package 0 = %+v", layout.Packages[0])
	}
	if layout.Packages[1].Dir != "packages/web" || layout.Packages[1].Version != Version4 ||
		layout.Packages[1].ManifestFile != "packages/web/package.json" ||
		len(layout.Packages[1].Entries) != 1 || layout.Packages[1].Entries[0] != "packages/web/src/app.css" {
		t.Errorf("package 1 = %+v", layout.Packages[1])
	}
}

func TestDiscoverKeepsEveryVersion4EntrySorted(t *testing.T) {
	files := fstest.MapFS{
		"package.json": &fstest.MapFile{Data: []byte(`{"dependencies":{"tailwindcss":"4.1.0"}}`)},
		"src/z.css":    &fstest.MapFile{Data: []byte(`@theme { --color-z: red; }`)},
		"src/a.css":    &fstest.MapFile{Data: []byte(`@import "tailwindcss";`)},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(layout.Packages) != 1 {
		t.Fatalf("packages = %+v", layout.Packages)
	}
	want := []string{"src/a.css", "src/z.css"}
	if !slices.Equal(layout.Packages[0].Entries, want) {
		t.Fatalf("entries = %v, want %v", layout.Packages[0].Entries, want)
	}
}

func TestPackageForBindsToTheNearestAncestor(t *testing.T) {
	files := fstest.MapFS{
		"tailwind.config.js":             &fstest.MapFile{Data: []byte("module.exports = {}")},
		"src/root.tsx":                   &fstest.MapFile{},
		"packages/ui/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"packages/ui/src/button.tsx":     &fstest.MapFile{},
		"packages/ui/deep/a/b/c.tsx":     &fstest.MapFile{},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	testCases := []struct{ file, dir string }{
		{"src/root.tsx", "."},
		{"packages/ui/src/button.tsx", "packages/ui"},
		{"packages/ui/deep/a/b/c.tsx", "packages/ui"},
		{"tailwind.config.js", "."},
	}
	for _, testCase := range testCases {
		pkg, found := layout.PackageFor(testCase.file)
		if !found || pkg.Dir != testCase.dir {
			t.Errorf("%s bound to %+v, found %v; want %q", testCase.file, pkg, found, testCase.dir)
		}
	}
}

func TestPackageForReportsNoPackageWhenThereIsNone(t *testing.T) {
	files := fstest.MapFS{
		"packages/ui/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"apps/docs/src/page.tsx":         &fstest.MapFile{},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, found := layout.PackageFor("apps/docs/src/page.tsx"); found {
		t.Error("file bound to an unrelated package")
	}
}

func TestDiscoverSkipsVendoredDirectories(t *testing.T) {
	files := fstest.MapFS{
		"tailwind.config.js":                      &fstest.MapFile{Data: []byte("module.exports = {}")},
		"node_modules/some-ui/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"dist/tailwind.config.js":                 &fstest.MapFile{Data: []byte("module.exports = {}")},
		".git/tailwind.config.js":                 &fstest.MapFile{Data: []byte("module.exports = {}")},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(layout.Packages) != 1 || layout.Packages[0].Dir != "." {
		t.Errorf("packages = %+v", layout.Packages)
	}
}

func TestPackagesAreSortedByDir(t *testing.T) {
	files := fstest.MapFS{
		"packages/z/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"packages/a/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"packages/m/tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for index, want := range []string{"packages/a", "packages/m", "packages/z"} {
		if layout.Packages[index].Dir != want {
			t.Errorf("package %d = %q, want %q", index, layout.Packages[index].Dir, want)
		}
	}
}

func TestV4EntryWalksUpToNearestPackageManifest(t *testing.T) {
	files := fstest.MapFS{
		"package.json":                 &fstest.MapFile{Data: []byte(`{"private":true}`)},
		"packages/web/package.json":    &fstest.MapFile{Data: []byte(`{"dependencies":{"tailwindcss":"4.1.0"}}`)},
		"packages/web/src/css/app.css": &fstest.MapFile{Data: []byte(`@theme { --color-brand: red; }`)},
	}
	layout, err := Discover(files)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(layout.Packages) != 1 || layout.Packages[0].Dir != "packages/web" ||
		layout.Packages[0].ManifestFile != "packages/web/package.json" {
		t.Fatalf("packages = %+v", layout.Packages)
	}
}
