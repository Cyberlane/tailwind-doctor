package audit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind/plugins"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

type tokenAnalysisResult struct {
	packages               []TokenPackageReport
	findings               []Finding
	projectTokens          int
	highConfidenceTokens   int
	mediumConfidenceTokens int
}

func analyzeTokens(themes []resolvedTheme) tokenAnalysisResult {
	result := tokenAnalysisResult{
		packages: []TokenPackageReport{},
		findings: []Finding{},
	}
	usedEverywhere := map[string]bool{}
	projectTokens := map[string]tokens.Token{}
	owners := map[string][]int{}

	for themeIndex := range themes {
		for identity := range themes[themeIndex].usedTokens {
			usedEverywhere[identity] = true
		}
		for _, token := range themes[themeIndex].theme.Inventory.Tokens() {
			if token.Origin != tokens.OriginProject {
				continue
			}
			identity := tokenIdentity(token)
			projectTokens[identity] = token
			owners[identity] = append(owners[identity], themeIndex)
		}
	}

	identities := make([]string, 0, len(projectTokens))
	for identity := range projectTokens {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	result.projectTokens = len(identities)
	for _, identity := range identities {
		if tokenDeclarationConfidence(owners[identity], themes) == ConfidenceHigh {
			result.highConfidenceTokens++
		} else {
			result.mediumConfidenceTokens++
		}
	}

	for themeIndex := range themes {
		resolved := &themes[themeIndex]
		confidence, reasons := tokenConfidence(*resolved)
		packageReport := TokenPackageReport{
			Package: resolved.packageDirectory, TailwindVersion: resolved.version,
			Confidence: confidence, ConfidenceReasons: reasons,
			ResolvedClassLists: resolved.resolvedLists, UnresolvedClassLists: resolved.unresolvedLists,
			Plugins:   append([]plugins.Coverage{}, resolved.theme.PluginCoverage...),
			Inventory: []TokenReport{}, Unused: []TokenReport{},
		}
		for _, token := range resolved.theme.Inventory.Tokens() {
			identity := tokenIdentity(token)
			used := resolved.usedTokens[identity]
			if token.Origin == tokens.OriginProject {
				used = usedEverywhere[identity]
			}
			reported := reportToken(token, used)
			packageReport.Inventory = append(packageReport.Inventory, reported)
			if token.Origin == tokens.OriginProject && !used {
				packageReport.Unused = append(packageReport.Unused, reported)
			}
		}
		result.packages = append(result.packages, packageReport)
	}

	for _, identity := range identities {
		if usedEverywhere[identity] {
			continue
		}
		token := projectTokens[identity]
		confidence := tokenDeclarationConfidence(owners[identity], themes)
		result.findings = append(result.findings, Finding{
			Rule: "unused-token", Category: CategoryConsistency,
			Message: fmt.Sprintf("Custom token %s is not used by any resolved class list.", tokenLabel(token)),
			File:    token.Decl.File, Class: token.Path, Line: token.Decl.Line, Column: token.Decl.Column,
			Confidence: confidence,
		})
	}
	return result
}

func tokenDeclarationConfidence(owners []int, themes []resolvedTheme) Confidence {
	for _, owner := range owners {
		ownerConfidence, _ := tokenConfidence(themes[owner])
		if ownerConfidence != ConfidenceHigh {
			return ConfidenceMedium
		}
	}
	return ConfidenceHigh
}

func tokenConfidence(theme resolvedTheme) (Confidence, []string) {
	reasons := []string{}
	if theme.theme.Degraded {
		reasons = append(reasons, "theme configuration was only partially readable")
	}
	if !plugins.Complete(theme.theme.PluginCoverage) {
		reasons = append(reasons, "one or more configured plugin surfaces are incomplete")
	}
	if theme.unresolvedLists > 0 {
		reasons = append(reasons, fmt.Sprintf("%d class list(s) could not be resolved statically", theme.unresolvedLists))
	}
	if theme.ambiguousLists > 0 {
		reasons = append(reasons, fmt.Sprintf("%d class list(s) could not be assigned to one Tailwind package", theme.ambiguousLists))
	}
	if len(reasons) > 0 {
		return ConfidenceMedium, reasons
	}
	return ConfidenceHigh, reasons
}

func reportToken(token tokens.Token, used bool) TokenReport {
	return TokenReport{
		Family: token.Family, Name: token.Name, Path: token.Path,
		Value: token.Value, Raw: token.Raw, Origin: token.Origin,
		Declaration: token.Decl, Unresolvable: token.Unresolvable, Used: used,
	}
}

func tokenIdentity(token tokens.Token) string {
	return strings.Join([]string{
		string(token.Family), token.Name, token.Path, token.Decl.File,
		strconv.Itoa(token.Decl.Line), strconv.Itoa(token.Decl.Column),
	}, "\x00")
}
