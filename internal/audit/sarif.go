package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// SARIF is how a finding reaches GitHub code scanning and most editors. The
// structs below are the subset of SARIF 2.1.0 this tool produces, written out
// rather than assembled from maps so that field order — and therefore the output
// bytes — is fixed by the source.

const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"

const sarifInformationURI = "https://github.com/Cyberlane/tailwind-doctor"

// sarifFingerprintKey names the fingerprint scheme. The version suffix is part
// of the contract: changing how a fingerprint is computed requires a new key, or
// consumers silently treat every old finding as new.
const sarifFingerprintKey = "twDoctorRuleFileClass/v1"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Invocations []sarifInvocation `json:"invocations"`
	Results     []sarifResult     `json:"results"`
	Properties  sarifRunProps     `json:"properties"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications"`
}

type sarifNotification struct {
	Descriptor sarifDescriptor `json:"descriptor"`
	Level      string          `json:"level"`
	Message    sarifText       `json:"message"`
	Locations  []sarifLocation `json:"locations"`
}

type sarifDescriptor struct {
	ID string `json:"id"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	ShortDescription     sarifText       `json:"shortDescription"`
	HelpURI              string          `json:"helpUri"`
	DefaultConfiguration sarifRuleConfig `json:"defaultConfiguration"`
	Properties           sarifRuleProps  `json:"properties"`
}

type sarifRuleConfig struct {
	Level string `json:"level"`
}

type sarifRuleProps struct {
	Category   Category   `json:"category"`
	Exposure   Exposure   `json:"exposure"`
	Confidence Confidence `json:"confidence"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
	Properties          sarifResultProps  `json:"properties"`
}

type sarifResultProps struct {
	Category   Category   `json:"category"`
	Confidence Confidence `json:"confidence"`
	Scored     bool       `json:"scored"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

type sarifRunProps struct {
	Score                  int             `json:"score"`
	ScoreExcludingBaseline int             `json:"scoreExcludingBaseline"`
	ScoreModelVersion      int             `json:"scoreModelVersion"`
	Categories             []CategoryScore `json:"categories"`
	Scanned                Scanned         `json:"scanned"`
}

// sarifLevel maps a finding onto SARIF's severity vocabulary. A finding the
// score does not count reports as a note, so a code-scanning gate configured on
// errors agrees with the score about what matters.
func sarifLevel(finding Finding) string {
	switch {
	case finding.Severity == SeverityWarn:
		return "warning"
	case !finding.Scored:
		return "note"
	default:
		return "error"
	}
}

// sarifFingerprint keys a result by rule, file, and class list, never by
// position — the same key the baseline uses. Reformatting a file then leaves
// deduplication intact instead of presenting every finding in it as new.
func sarifFingerprint(finding Finding) string {
	sum := sha256.Sum256([]byte(finding.Rule + "\x00" + finding.File + "\x00" + finding.Class))
	return hex.EncodeToString(sum[:])
}

func WriteSARIF(writer io.Writer, report Report) error {
	rules := make([]sarifRule, 0, len(ruleRegistry))
	for _, rule := range ruleRegistry {
		level := "error"
		if !rule.DefaultOn {
			level = "none"
		} else if rule.DefaultConfidence != ConfidenceHigh {
			level = "note"
		}
		rules = append(rules, sarifRule{
			ID:                   rule.ID,
			Name:                 rule.ID,
			ShortDescription:     sarifText{Text: fmt.Sprintf("%s (%s)", rule.ID, rule.Category)},
			HelpURI:              sarifInformationURI + "/blob/main/docs/rules.md#" + rule.ID,
			DefaultConfiguration: sarifRuleConfig{Level: level},
			Properties: sarifRuleProps{
				Category: rule.Category, Exposure: rule.Exposure, Confidence: rule.DefaultConfidence,
			},
		})
	}

	results := make([]sarifResult, 0, len(report.Findings))
	for _, finding := range report.Findings {
		results = append(results, sarifResult{
			RuleID:  finding.Rule,
			Level:   sarifLevel(finding),
			Message: sarifText{Text: finding.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifact{URI: finding.File},
					Region:           sarifRegion{StartLine: finding.Line, StartColumn: finding.Column},
				},
			}},
			PartialFingerprints: map[string]string{sarifFingerprintKey: sarifFingerprint(finding)},
			Properties: sarifResultProps{
				Category: finding.Category, Confidence: finding.Confidence, Scored: finding.Scored,
			},
		})
	}

	notifications := make([]sarifNotification, 0, len(report.Diagnostics))
	for _, diagnostic := range report.Diagnostics {
		notifications = append(notifications, sarifNotification{
			Descriptor: sarifDescriptor{ID: diagnostic.Kind},
			Level:      "warning",
			Message:    sarifText{Text: diagnostic.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifact{URI: diagnostic.File},
					Region: sarifRegion{
						StartLine: diagnostic.Line, StartColumn: diagnostic.Column,
					},
				},
			}},
		})
	}

	log := sarifLog{
		Schema:  sarifSchema,
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name: "tw-doctor", Version: Version,
				InformationURI: sarifInformationURI, Rules: rules,
			}},
			Invocations: []sarifInvocation{{
				ExecutionSuccessful: true, ToolExecutionNotifications: notifications,
			}},
			Results: results,
			Properties: sarifRunProps{
				Score:                  report.Score,
				ScoreExcludingBaseline: report.ScoreExcludingBaseline,
				ScoreModelVersion:      report.ScoreModel.Version,
				Categories:             report.Categories,
				Scanned:                report.Scanned,
			},
		}},
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(log)
}
