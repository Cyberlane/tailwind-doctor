package audit

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSARIFIsValidAndCarriesTheScore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "page.html",
		`<div class="p-4 p-2 text-[#123456] sm:p-2 md:p-4 lg:m-6 xl:m-8 2xl:mt-10"></div>`)

	report, err := Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var buffer bytes.Buffer
	if err := WriteSARIF(&buffer, report); err != nil {
		t.Fatalf("WriteSARIF: %v", err)
	}

	var log struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name           string `json:"name"`
					Version        string `json:"version"`
					InformationURI string `json:"informationUri"`
					Rules          []struct {
						ID                   string `json:"id"`
						HelpURI              string `json:"helpUri"`
						DefaultConfiguration struct {
							Level string `json:"level"`
						} `json:"defaultConfiguration"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine   int `json:"startLine"`
							StartColumn int `json:"startColumn"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				PartialFingerprints map[string]string `json:"partialFingerprints"`
				Properties          struct {
					Category   string `json:"category"`
					Confidence string `json:"confidence"`
					Scored     bool   `json:"scored"`
				} `json:"properties"`
			} `json:"results"`
			Properties struct {
				Score                  int `json:"score"`
				ScoreExcludingBaseline int `json:"scoreExcludingBaseline"`
				ScoreModelVersion      int `json:"scoreModelVersion"`
			} `json:"properties"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &log); err != nil {
		t.Fatalf("decode SARIF: %v", err)
	}

	if log.Version != "2.1.0" || log.Schema == "" {
		t.Errorf("version = %q, schema = %q", log.Version, log.Schema)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(log.Runs))
	}
	run := log.Runs[0]

	if run.Tool.Driver.Name != "tw-doctor" || run.Tool.Driver.Version != Version {
		t.Errorf("driver = %#v", run.Tool.Driver)
	}
	if len(run.Tool.Driver.Rules) != len(ruleRegistry) {
		t.Errorf("declared %d rules, want %d", len(run.Tool.Driver.Rules), len(ruleRegistry))
	}
	for _, rule := range run.Tool.Driver.Rules {
		if rule.HelpURI == "" {
			t.Errorf("rule %q has no helpUri", rule.ID)
		}
	}

	if len(run.Results) != len(report.Findings) {
		t.Fatalf("got %d results for %d findings", len(run.Results), len(report.Findings))
	}
	fingerprints := map[string]bool{}
	sawNote := false
	for _, result := range run.Results {
		location := result.Locations[0].PhysicalLocation
		if location.ArtifactLocation.URI != "page.html" {
			t.Errorf("uri = %q", location.ArtifactLocation.URI)
		}
		if location.Region.StartLine < 1 || location.Region.StartColumn < 1 {
			t.Errorf("region = %#v: SARIF regions are one-based", location.Region)
		}
		if result.Message.Text == "" {
			t.Errorf("result %q has no message", result.RuleID)
		}
		switch result.Level {
		case "error":
			if !result.Properties.Scored {
				t.Errorf("%q is level error but not scored", result.RuleID)
			}
		case "note":
			sawNote = true
			if result.Properties.Scored {
				t.Errorf("%q is level note but scored", result.RuleID)
			}
		default:
			t.Errorf("unexpected level %q", result.Level)
		}
		if len(result.PartialFingerprints) == 0 {
			t.Errorf("%q has no partialFingerprints", result.RuleID)
		}
		for _, value := range result.PartialFingerprints {
			fingerprints[value] = true
		}
	}
	if !sawNote {
		t.Error("the fixture includes responsive-bloat, which must report as a note")
	}
	if len(fingerprints) != len(run.Results) {
		t.Errorf("%d distinct fingerprints for %d results", len(fingerprints), len(run.Results))
	}

	if run.Properties.Score != report.Score ||
		run.Properties.ScoreExcludingBaseline != report.ScoreExcludingBaseline ||
		run.Properties.ScoreModelVersion != ScoreModelVersion {
		t.Errorf("run properties = %#v", run.Properties)
	}
}

// A fingerprint keyed on position would change every time a file was
// reformatted, and code-scanning deduplication would then treat old debt as new.
func TestSARIFFingerprintsIgnorePosition(t *testing.T) {
	first := t.TempDir()
	writeFile(t, first, "page.html", `<div class="p-4 p-2"></div>`)
	second := t.TempDir()
	writeFile(t, second, "page.html", "\n\n\n<div class=\"p-4 p-2\"></div>")

	fingerprint := func(root string) string {
		report, err := Run(root)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var buffer bytes.Buffer
		if err := WriteSARIF(&buffer, report); err != nil {
			t.Fatalf("WriteSARIF: %v", err)
		}
		var log struct {
			Runs []struct {
				Results []struct {
					PartialFingerprints map[string]string `json:"partialFingerprints"`
				} `json:"results"`
			} `json:"runs"`
		}
		if err := json.Unmarshal(buffer.Bytes(), &log); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, value := range log.Runs[0].Results[0].PartialFingerprints {
			return value
		}
		t.Fatal("no fingerprint")
		return ""
	}

	if fingerprint(first) != fingerprint(second) {
		t.Error("moving a class list down the file changed its fingerprint")
	}
}
