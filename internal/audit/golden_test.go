package audit

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// A golden file is the only test that notices an unintended change to output
// nobody asserted directly — a reordered field, a lost line, a moved score on a
// fixture whose debt did not change.
//
// One flag regenerates every committed expectation in the package: the golden
// reports here and the extraction accuracy baseline. Two flags doing the same
// job would mean a contributor regenerating half of them.
var updateFixtures = flag.Bool("update", false,
	"rewrite the golden reports and the extraction accuracy baseline from the current output")

func TestGoldenReports(t *testing.T) {
	for _, project := range []string{"clean", "mixed", "baselined"} {
		t.Run(project, func(t *testing.T) {
			report, err := Run(filepath.Join("testdata", "projects", project))
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			var human, jsonReport, sarifReport bytes.Buffer
			WriteHuman(&human, report)
			if err := WriteJSON(&jsonReport, report); err != nil {
				t.Fatalf("WriteJSON: %v", err)
			}
			if err := WriteSARIF(&sarifReport, report); err != nil {
				t.Fatalf("WriteSARIF: %v", err)
			}

			compareGolden(t, project+".txt", human.Bytes())
			compareGolden(t, project+".json", jsonReport.Bytes())
			compareGolden(t, project+".sarif", sarifReport.Bytes())
		})
	}
}

func compareGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)

	if *updateFixtures {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with: go test ./internal/audit/ -run TestGoldenReports -update)",
			name, err)
	}
	if !bytes.Equal(expected, actual) {
		t.Errorf("%s differs from the golden file.\n--- want ---\n%s\n--- got ---\n%s", name, expected, actual)
	}
}

// Determinism is a product boundary, not an aspiration: the same tree analysed
// twice must produce identical bytes, whatever order the filesystem hands files
// over in.
func TestOutputIsByteIdenticalAcrossRuns(t *testing.T) {
	render := func() []byte {
		report, err := Run(filepath.Join("testdata", "projects", "mixed"))
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var buffer bytes.Buffer
		if err := WriteJSON(&buffer, report); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		if err := WriteSARIF(&buffer, report); err != nil {
			t.Fatalf("WriteSARIF: %v", err)
		}
		WriteHuman(&buffer, report)
		return buffer.Bytes()
	}

	if !bytes.Equal(render(), render()) {
		t.Error("two runs over the same tree produced different bytes")
	}
}
