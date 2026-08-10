package audit

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tailwind"
	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

const (
	minimumContrastRatio = 3.0
	normalContrastRatio  = 4.5
	contrastGuardBand    = 0.01
	largeTextPixels      = 24.0
	largeBoldTextPixels  = 14.0 * 96.0 / 72.0
	largeTextWeight      = 700.0
)

const (
	unknownMissingForeground      = "missing-foreground"
	unknownMissingBackground      = "missing-background"
	unknownMultipleForegrounds    = "multiple-foreground-colors"
	unknownMultipleBackgrounds    = "multiple-background-colors"
	unknownForegroundColor        = "unresolved-foreground-color"
	unknownBackgroundColor        = "unresolved-background-color"
	unknownOpacityContext         = "unresolved-opacity-context"
	unknownTextThreshold          = "unresolved-text-threshold"
	unknownUntrustedTheme         = "untrusted-theme"
	unknownUnsupportedCompositing = "unsupported-color-compositing"
)

type contrastInspection struct {
	findings       []Finding
	resolvedPairs  int
	unknownReasons map[string]int
}

type contrastColorUtility struct {
	token      utilityToken
	meaning    tailwind.UtilityMeaning
	color      cssColor
	resolved   bool
	multiplier float64
}

type contrastContext struct {
	foregrounds []contrastColorUtility
	backgrounds []contrastColorUtility
	fontSizes   []string
	fontWeights []string
}

type opacityContext struct {
	all        bool
	foreground bool
	background bool
}

func summarizeAccessibility(resolved int, unknown map[string]int) AccessibilityReport {
	reasons := make([]AccessibilityUnknownReason, 0, len(unknown))
	total := 0
	for reason, count := range unknown {
		reasons = append(reasons, AccessibilityUnknownReason{Reason: reason, Count: count})
		total += count
	}
	sort.Slice(reasons, func(left, right int) bool {
		return reasons[left].Reason < reasons[right].Reason
	})
	return AccessibilityReport{
		ResolvedColorPairs: resolved,
		UnknownColorPairs:  total,
		UnknownReasons:     reasons,
	}
}

func inspectContrast(file string, list ClassList, syntax tailwind.UtilitySyntax, inventory *tokens.Inventory, trustedTheme bool) contrastInspection {
	inspection := contrastInspection{findings: []Finding{}, unknownReasons: map[string]int{}}
	contexts := map[string]*contrastContext{}
	opacities := map[string]opacityContext{}

	contextFor := func(key string) *contrastContext {
		context, found := contexts[key]
		if !found {
			context = &contrastContext{}
			contexts[key] = context
		}
		return context
	}

	for _, token := range splitUtilities(list) {
		parsed := tailwind.ParseUtility(token.text, syntax)
		if !parsed.Recognized {
			continue
		}
		key := parsed.VariantKey()
		switch {
		case strings.HasPrefix(parsed.Base, "text-opacity-"):
			current := opacities[key]
			current.foreground = true
			opacities[key] = current
			continue
		case strings.HasPrefix(parsed.Base, "bg-opacity-"):
			current := opacities[key]
			current.background = true
			opacities[key] = current
			continue
		case strings.HasPrefix(parsed.Base, "opacity-"):
			current := opacities[key]
			current.all = true
			opacities[key] = current
			continue
		}

		meaning := tailwind.ClassifyUtility(parsed.Base, inventory)
		context := contextFor(key)
		switch meaning.Property {
		case "color":
			context.foregrounds = append(context.foregrounds, resolveContrastColor(token, parsed, meaning, inventory))
		case "background-color":
			context.backgrounds = append(context.backgrounds, resolveContrastColor(token, parsed, meaning, inventory))
		case "font-size":
			value, _ := utilityMeaningValue(meaning, inventory)
			context.fontSizes = append(context.fontSizes, value)
		case "font-weight":
			value, _ := utilityMeaningValue(meaning, inventory)
			context.fontWeights = append(context.fontWeights, value)
		}
	}

	globalOpacity := opacities[""]
	keys := make([]string, 0, len(contexts))
	for key, context := range contexts {
		if len(context.foregrounds) > 0 || len(context.backgrounds) > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		context := contexts[key]
		currentOpacity := opacities[key]
		switch {
		case !trustedTheme:
			inspection.unknownReasons[unknownUntrustedTheme]++
		case len(context.foregrounds) == 0:
			inspection.unknownReasons[unknownMissingForeground]++
		case len(context.backgrounds) == 0:
			inspection.unknownReasons[unknownMissingBackground]++
		case len(context.foregrounds) > 1:
			inspection.unknownReasons[unknownMultipleForegrounds]++
		case len(context.backgrounds) > 1:
			inspection.unknownReasons[unknownMultipleBackgrounds]++
		case !context.foregrounds[0].resolved:
			inspection.unknownReasons[unknownForegroundColor]++
		case !context.backgrounds[0].resolved:
			inspection.unknownReasons[unknownBackgroundColor]++
		case globalOpacity.all || globalOpacity.foreground || globalOpacity.background ||
			currentOpacity.all || currentOpacity.foreground || currentOpacity.background:
			inspection.unknownReasons[unknownOpacityContext]++
		default:
			foreground := context.foregrounds[0]
			background := context.backgrounds[0]
			ratio, resolved := contrastRatio(foreground.color, background.color)
			if !resolved {
				inspection.unknownReasons[unknownUnsupportedCompositing]++
				continue
			}
			threshold, thresholdKnown := contextContrastThreshold(*context)
			if !thresholdKnown {
				switch {
				case ratio >= normalContrastRatio:
					threshold, thresholdKnown = normalContrastRatio, true
				case ratio < minimumContrastRatio:
					threshold, thresholdKnown = minimumContrastRatio, true
				}
			}
			if !thresholdKnown {
				inspection.unknownReasons[unknownTextThreshold]++
				continue
			}

			inspection.resolvedPairs++
			if ratio >= threshold-contrastGuardBand {
				continue
			}
			message := fmt.Sprintf("%s on %s has a %.2f:1 contrast ratio; WCAG 2.2 requires at least %.1f:1.",
				foreground.token.text, background.token.text, ratio, threshold)
			if suggestion, found := contrastSuggestion(foreground, background.color, threshold, inventory); found {
				message += fmt.Sprintf(" Use %s for a passing foreground token.", suggestion)
			}
			inspection.findings = append(inspection.findings, Finding{
				Rule: "color-contrast", Category: CategoryAccessibility,
				Message: message, File: file,
				Class: foreground.token.text + " " + background.token.text,
				Line:  foreground.token.line, Column: foreground.token.column,
				Confidence: ConfidenceHigh,
			})
		}
	}
	return inspection
}

func resolveContrastColor(token utilityToken, parsed tailwind.ParsedUtility, meaning tailwind.UtilityMeaning, inventory *tokens.Inventory) contrastColorUtility {
	resolved := contrastColorUtility{token: token, meaning: meaning, multiplier: 1}
	value, found := utilityMeaningValue(meaning, inventory)
	if !found {
		return resolved
	}
	value = strings.TrimSpace(strings.TrimPrefix(value, "color:"))
	color, found := parseCSSColor(value)
	if !found {
		return resolved
	}
	modifier := colorOpacityModifier(parsed.Base, meaning.Property)
	if modifier != "" {
		multiplier, found := parseOpacityModifier(modifier)
		if !found {
			return resolved
		}
		resolved.multiplier = multiplier
		color.alpha *= multiplier
	}
	resolved.color = color
	resolved.resolved = true
	return resolved
}

func utilityMeaningValue(meaning tailwind.UtilityMeaning, inventory *tokens.Inventory) (string, bool) {
	if meaning.ArbitraryValue != "" {
		return meaning.ArbitraryValue, true
	}
	if inventory == nil || meaning.Family == "" || meaning.TokenName == "" {
		return "", false
	}
	token, found := inventory.ByName(meaning.Family, meaning.TokenName)
	if !found || token.Unresolvable {
		return "", false
	}
	return token.Raw, true
}

func colorOpacityModifier(base, property string) string {
	prefix := "text-"
	if property == "background-color" {
		prefix = "bg-"
	}
	suffix := strings.TrimPrefix(base, prefix)
	if strings.HasPrefix(suffix, "[") {
		if end := strings.LastIndexByte(suffix, ']'); end >= 0 && end+1 < len(suffix) && suffix[end+1] == '/' {
			return suffix[end+2:]
		}
		return ""
	}
	_, modifier, found := strings.Cut(suffix, "/")
	if !found {
		return ""
	}
	return modifier
}

func parseOpacityModifier(raw string) (float64, bool) {
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		raw = strings.TrimSpace(raw[1 : len(raw)-1])
		value, percent, found := numericComponent(raw)
		if !found {
			return 0, false
		}
		if percent {
			value /= 100
		}
		return value, inUnitRange(value)
	}
	value, _, found := numericComponent(raw)
	if !found {
		return 0, false
	}
	value /= 100
	return value, inUnitRange(value)
}

func contextContrastThreshold(context contrastContext) (float64, bool) {
	if len(context.fontSizes) != 1 {
		return 0, false
	}
	size, found := parsePixels(context.fontSizes[0])
	if !found {
		return 0, false
	}
	switch {
	case size < largeBoldTextPixels:
		return normalContrastRatio, true
	case size >= largeTextPixels:
		return minimumContrastRatio, true
	case len(context.fontWeights) != 1:
		return 0, false
	}
	weight, found := parseFontWeight(context.fontWeights[0])
	if !found {
		return 0, false
	}
	if weight >= largeTextWeight {
		return minimumContrastRatio, true
	}
	return normalContrastRatio, true
}

func parsePixels(raw string) (float64, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasSuffix(value, "px") {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "px")), 64)
	return parsed, err == nil && parsed >= 0 && !math.IsInf(parsed, 0) && !math.IsNaN(parsed)
}

func parseFontWeight(raw string) (float64, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if named, found := map[string]float64{"normal": 400, "bold": 700}[value]; found {
		return named, true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil && parsed >= 1 && parsed <= 1000
}

func contrastSuggestion(foreground contrastColorUtility, background cssColor, threshold float64, inventory *tokens.Inventory) (string, bool) {
	if inventory == nil {
		return "", false
	}
	bestName := ""
	bestRatio := 0.0
	for _, token := range inventory.Tokens() {
		if token.Family != tokens.FamilyColor || token.Unresolvable || token.Name == foreground.meaning.TokenName {
			continue
		}
		candidate, found := parseCSSColor(token.Raw)
		if !found {
			continue
		}
		candidate.alpha *= foreground.multiplier
		ratio, resolved := contrastRatio(candidate, background)
		if !resolved || ratio < threshold+contrastGuardBand {
			continue
		}
		if ratio > bestRatio+1e-12 || (math.Abs(ratio-bestRatio) <= 1e-12 && (bestName == "" || token.Name < bestName)) {
			bestName, bestRatio = token.Name, ratio
		}
	}
	if bestName == "" {
		return "", false
	}
	return replaceForegroundToken(foreground.token.text, bestName), true
}

func replaceForegroundToken(utility, tokenName string) string {
	start := strings.LastIndex(utility, "text-")
	if start < 0 {
		return utility
	}
	valueStart := start + len("text-")
	valueEnd := len(utility)
	if valueStart < len(utility) && utility[valueStart] == '[' {
		if close := strings.IndexByte(utility[valueStart:], ']'); close >= 0 {
			valueEnd = valueStart + close + 1
		}
	} else {
		for index := valueStart; index < len(utility); index++ {
			if utility[index] == '/' || utility[index] == '!' {
				valueEnd = index
				break
			}
		}
	}
	return utility[:valueStart] + tokenName + utility[valueEnd:]
}
