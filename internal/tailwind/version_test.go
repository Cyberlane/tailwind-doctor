package tailwind

import (
	"testing"
	"testing/fstest"
)

func TestDetect(t *testing.T) {
	testCases := []struct {
		name    string
		files   fstest.MapFS
		version Version
		signal  string
	}{
		{name: "v4 css import", files: fstest.MapFS{"src/app.css": &fstest.MapFile{Data: []byte(`@import "tailwindcss";`)}}, version: Version4, signal: "css-import"},
		{name: "v4 theme block", files: fstest.MapFS{"src/app.css": &fstest.MapFile{Data: []byte("@theme { --color-a: red; }")}}, version: Version4, signal: "css-theme"},
		{name: "v3 config file", files: fstest.MapFS{"tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")}}, version: Version3, signal: "config-file"},
		{name: "v3 directive", files: fstest.MapFS{"src/app.css": &fstest.MapFile{Data: []byte("@tailwind base;")}}, version: Version3, signal: "css-directive"},
		{name: "package wins", files: fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(`{"devDependencies":{"tailwindcss":"^4.1.3"}}`)}, "tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")}}, version: Version4, signal: "package-json"},
		{name: "v4 plugin", files: fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(`{"devDependencies":{"@tailwindcss/vite":"^4.0.0"}}`)}}, version: Version4, signal: "package-json"},
		{name: "v3 package", files: fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(`{"dependencies":{"tailwindcss":"~3.4.1"}}`)}}, version: Version3, signal: "package-json"},
		{name: "no signal", files: fstest.MapFS{"src/app.css": &fstest.MapFile{Data: []byte(".card { color: red }")}}, version: VersionUnknown},
		{name: "unparseable range", files: fstest.MapFS{"package.json": &fstest.MapFile{Data: []byte(`{"dependencies":{"tailwindcss":"workspace:*"}}`)}, "tailwind.config.ts": &fstest.MapFile{Data: []byte("export default {}")}}, version: Version3, signal: "config-file"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			detection, err := Detect(testCase.files, ".")
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if detection.Version != testCase.version {
				t.Fatalf("Version = %q, want %q (evidence %+v)", detection.Version, testCase.version, detection.Evidence)
			}
			if testCase.signal != "" && (len(detection.Evidence) == 0 || detection.Evidence[0].Signal != testCase.signal) {
				t.Fatalf("evidence = %+v, want first signal %q", detection.Evidence, testCase.signal)
			}
		})
	}
}

func TestDetectRecordsEveryConflictingSignal(t *testing.T) {
	files := fstest.MapFS{
		"package.json":       &fstest.MapFile{Data: []byte(`{"devDependencies":{"tailwindcss":"^4.1.3"}}`)},
		"tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
		"src/app.css":        &fstest.MapFile{Data: []byte(`@import "tailwindcss";`)},
	}
	detection, err := Detect(files, ".")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if detection.Version != Version4 || len(detection.Evidence) < 3 {
		t.Fatalf("detection = %+v", detection)
	}
}

func TestDetectIsDeterministic(t *testing.T) {
	files := fstest.MapFS{
		"src/a.css":          &fstest.MapFile{Data: []byte("@theme { --color-a: red; }")},
		"src/b.css":          &fstest.MapFile{Data: []byte(`@import "tailwindcss";`)},
		"tailwind.config.js": &fstest.MapFile{Data: []byte("module.exports = {}")},
	}
	first, err := Detect(files, ".")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for attempt := 0; attempt < 20; attempt++ {
		again, err := Detect(files, ".")
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		if len(again.Evidence) != len(first.Evidence) {
			t.Fatal("evidence count varies")
		}
		for index := range first.Evidence {
			if again.Evidence[index] != first.Evidence[index] {
				t.Fatalf("evidence %d varies: %+v vs %+v", index, again.Evidence[index], first.Evidence[index])
			}
		}
	}
}
