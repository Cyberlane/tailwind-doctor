package tailwind

import (
	"strings"

	"github.com/Cyberlane/tailwind-doctor/internal/tokens"
)

// UtilityMeaning is the statically knowable design-system meaning of one base
// utility. Empty fields mean the utility cannot be classified conservatively.
type UtilityMeaning struct {
	Property         string
	Family           tokens.Family
	TokenName        string
	ArbitraryValue   string
	SuggestionPrefix string
	SuggestionSuffix string
}

// IsKnownUtility reports whether a base class is part of Tailwind's utility
// namespace. Prefix recognition alone is insufficient: application classes
// such as "card" must not dilute the score denominator or trigger rules.
func IsKnownUtility(base string, inventory *tokens.Inventory) bool {
	meaning := ClassifyUtility(base, inventory)
	if meaning.Property != "" || meaning.Family != "" {
		return true
	}
	if strings.HasPrefix(base, "[") && strings.Contains(base, ":") && strings.HasSuffix(base, "]") {
		return true
	}
	for _, exact := range []string{
		"block", "contents", "flex", "flow-root", "grid", "hidden", "inline",
		"inline-block", "inline-flex", "inline-grid", "list-item", "table",
		"static", "fixed", "absolute", "relative", "sticky", "visible", "invisible",
		"isolate", "isolation-auto", "truncate", "antialiased", "subpixel-antialiased",
		"container", "sr-only", "not-sr-only", "overflow-ellipsis", "overflow-clip",
		"italic", "not-italic", "uppercase", "lowercase", "capitalize", "normal-case",
		"underline", "overline", "line-through", "no-underline", "grow", "shrink",
		"transform", "transform-cpu", "transform-gpu", "transform-none", "transition",
		"resize", "resize-none", "resize-x", "resize-y", "blur", "grayscale", "invert",
		"sepia", "drop-shadow", "filter", "backdrop-filter", "group", "peer",
	} {
		if base == exact {
			return true
		}
	}
	for _, prefix := range []string{
		"accent-", "align-", "animate-", "appearance-", "aspect-", "backdrop-",
		"basis-", "bg-", "bg-blend-", "blur-", "border-", "box-", "break-", "brightness-", "caption-",
		"caret-", "clear-", "col-", "columns-", "container-", "content-", "contrast-",
		"cursor-", "decoration-", "delay-", "divide-", "drop-shadow-", "duration-",
		"ease-", "fill-", "flex-", "float-", "font-", "from-", "gap-", "grayscale-", "grid-",
		"grow-", "hue-rotate-", "hyphens-", "indent-", "inset-", "invert-", "items-",
		"justify-", "line-clamp-", "list-", "max-h-", "max-w-", "min-h-", "min-w-",
		"mix-blend-", "object-", "opacity-", "order-", "origin-", "outline-", "overflow-", "overscroll-",
		"place-", "pointer-events-", "resize-", "ring-", "rotate-", "row-", "saturate-",
		"scale-", "scroll-", "select-", "self-", "sepia-", "shrink-", "size-", "skew-", "snap-",
		"stroke-", "table-", "text-", "to-", "touch-", "transform-", "transition-",
		"translate-", "underline-", "via-", "whitespace-", "will-change-", "z-",
	} {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

// ClassifyUtility separates property families that share a lexical prefix and
// identifies token-backed values without executing Tailwind or project code.
func ClassifyUtility(base string, inventory *tokens.Inventory) UtilityMeaning {
	if prefix, suffix, found := spacingUtility(base); found {
		return namedOrArbitrary(base, prefix, suffix, prefix, tokens.FamilySpacing)
	}
	if base == "rounded" {
		return UtilityMeaning{Property: "border-radius", Family: tokens.FamilyRadius, TokenName: "DEFAULT"}
	}
	if strings.HasPrefix(base, "rounded-") {
		return classifyRadius(base)
	}
	if base == "shadow" {
		return UtilityMeaning{Property: "box-shadow", Family: tokens.FamilyShadow, TokenName: "DEFAULT"}
	}
	if strings.HasPrefix(base, "shadow-") {
		return classifyShadow(base, inventory)
	}

	for _, mapping := range []struct {
		prefix   string
		property string
		family   tokens.Family
	}{
		{"leading-", "line-height", tokens.FamilyLineHeight},
		{"tracking-", "letter-spacing", tokens.FamilyLetterSpacing},
	} {
		if strings.HasPrefix(base, mapping.prefix) {
			return namedOrArbitrary(base, mapping.prefix, strings.TrimPrefix(base, mapping.prefix), mapping.property, mapping.family)
		}
	}

	if strings.HasPrefix(base, "max-w-") {
		meaning := namedOrArbitrary(base, "max-w-", strings.TrimPrefix(base, "max-w-"), "max-width", tokens.FamilyContainer)
		if meaning.ArbitraryValue == "" && !hasToken(inventory, meaning.Family, meaning.TokenName) {
			meaning.Family = tokens.FamilySpacing
		}
		return meaning
	}
	if strings.HasPrefix(base, "z-") {
		return namedOrArbitrary(base, "z-", strings.TrimPrefix(base, "z-"), "z-index", tokens.FamilyZIndex)
	}
	if strings.HasPrefix(base, "text-") {
		return classifyText(base, inventory)
	}
	if strings.HasPrefix(base, "font-") {
		return classifyFont(base, inventory)
	}
	if strings.HasPrefix(base, "bg-") {
		return classifyBackground(base, inventory)
	}
	if base == "border" || strings.HasPrefix(base, "border-") {
		return classifyBorder(base, inventory)
	}
	return UtilityMeaning{}
}

func spacingUtility(base string) (string, string, bool) {
	for _, prefix := range []string{
		"px-", "py-", "pt-", "pr-", "pb-", "pl-", "p-",
		"mx-", "my-", "mt-", "mr-", "mb-", "ml-", "m-",
		"space-x-", "space-y-", "gap-x-", "gap-y-", "gap-",
		"inset-x-", "inset-y-", "inset-", "top-", "right-", "bottom-", "left-",
		"min-w-", "min-h-", "w-", "h-",
	} {
		if strings.HasPrefix(base, prefix) {
			return prefix, strings.TrimPrefix(base, prefix), true
		}
	}
	return "", "", false
}

func classifyText(base string, inventory *tokens.Inventory) UtilityMeaning {
	suffix := strings.TrimPrefix(base, "text-")
	if arbitrary, _, found := arbitraryParts(suffix); found {
		family, property := tokens.FamilyFontSize, "font-size"
		if looksLikeColor(arbitrary) {
			family, property = tokens.FamilyColor, "color"
		}
		return namedOrArbitrary(base, "text-", suffix, property, family)
	}
	name := withoutModifier(suffix)
	switch {
	case contains([]string{"left", "right", "center", "justify", "start", "end"}, name):
		return UtilityMeaning{Property: "text-align"}
	case hasToken(inventory, tokens.FamilyColor, name):
		return namedMeaning("text-", suffix, "color", tokens.FamilyColor)
	case hasToken(inventory, tokens.FamilyFontSize, name):
		return namedMeaning("text-", suffix, "font-size", tokens.FamilyFontSize)
	}
	return UtilityMeaning{}
}

func classifyBackground(base string, inventory *tokens.Inventory) UtilityMeaning {
	suffix := strings.TrimPrefix(base, "bg-")
	if arbitrary, _, found := arbitraryParts(suffix); found {
		if looksLikeColor(arbitrary) {
			return namedOrArbitrary(base, "bg-", suffix, "background-color", tokens.FamilyColor)
		}
		return UtilityMeaning{}
	}
	name := withoutModifier(suffix)
	switch {
	case contains([]string{"auto", "cover", "contain"}, name):
		return UtilityMeaning{Property: "background-size"}
	case strings.HasPrefix(name, "repeat"):
		return UtilityMeaning{Property: "background-repeat"}
	case contains([]string{"bottom", "center", "left", "left-bottom", "left-top", "right", "right-bottom", "right-top", "top"}, name):
		return UtilityMeaning{Property: "background-position"}
	case contains([]string{"fixed", "local", "scroll"}, name):
		return UtilityMeaning{Property: "background-attachment"}
	case hasToken(inventory, tokens.FamilyColor, name):
		return namedMeaning("bg-", suffix, "background-color", tokens.FamilyColor)
	}
	return UtilityMeaning{}
}

func classifyBorder(base string, inventory *tokens.Inventory) UtilityMeaning {
	if base == "border" {
		return UtilityMeaning{Property: "border-width"}
	}
	suffix := strings.TrimPrefix(base, "border-")
	if arbitrary, _, found := arbitraryParts(suffix); found {
		if looksLikeColor(arbitrary) {
			return namedOrArbitrary(base, "border-", suffix, "border-color", tokens.FamilyColor)
		}
		return UtilityMeaning{Property: "border-width", ArbitraryValue: arbitrary}
	}
	name := withoutModifier(suffix)
	if contains([]string{"solid", "dashed", "dotted", "double", "hidden", "none"}, name) {
		return UtilityMeaning{Property: "border-style"}
	}
	if isBorderWidth(name) {
		return UtilityMeaning{Property: "border-width"}
	}
	if hasToken(inventory, tokens.FamilyColor, name) {
		return namedMeaning("border-", suffix, "border-color", tokens.FamilyColor)
	}
	return UtilityMeaning{}
}

func classifyFont(base string, inventory *tokens.Inventory) UtilityMeaning {
	suffix := strings.TrimPrefix(base, "font-")
	name := withoutModifier(suffix)
	if contains([]string{"thin", "extralight", "light", "normal", "medium", "semibold", "bold", "extrabold", "black"}, name) ||
		hasToken(inventory, tokens.FamilyFontWeight, name) {
		return namedMeaning("font-", suffix, "font-weight", tokens.FamilyFontWeight)
	}
	if hasToken(inventory, tokens.FamilyFontFamily, name) {
		return namedMeaning("font-", suffix, "font-family", tokens.FamilyFontFamily)
	}
	return UtilityMeaning{}
}

func classifyRadius(base string) UtilityMeaning {
	suffix := strings.TrimPrefix(base, "rounded-")
	for _, direction := range []struct {
		prefix   string
		property string
	}{
		{"tl-", "border-top-left-radius"}, {"tr-", "border-top-right-radius"},
		{"br-", "border-bottom-right-radius"}, {"bl-", "border-bottom-left-radius"},
		{"ss-", "border-start-start-radius"}, {"se-", "border-start-end-radius"},
		{"ee-", "border-end-end-radius"}, {"es-", "border-end-start-radius"},
		{"t-", "border-top-radius"}, {"r-", "border-right-radius"},
		{"b-", "border-bottom-radius"}, {"l-", "border-left-radius"},
		{"s-", "border-start-radius"}, {"e-", "border-end-radius"},
	} {
		if strings.HasPrefix(suffix, direction.prefix) {
			return namedOrArbitrary(base, "rounded-"+direction.prefix, strings.TrimPrefix(suffix, direction.prefix), direction.property, tokens.FamilyRadius)
		}
	}
	if contains([]string{"t", "r", "b", "l", "s", "e", "tl", "tr", "br", "bl", "ss", "se", "ee", "es"}, suffix) {
		return UtilityMeaning{Property: "border-" + suffix + "-radius", Family: tokens.FamilyRadius, TokenName: "DEFAULT"}
	}
	return namedOrArbitrary(base, "rounded-", suffix, "border-radius", tokens.FamilyRadius)
}

func classifyShadow(base string, inventory *tokens.Inventory) UtilityMeaning {
	suffix := strings.TrimPrefix(base, "shadow-")
	if arbitrary, _, found := arbitraryParts(suffix); found && looksLikeColor(arbitrary) {
		return namedOrArbitrary(base, "shadow-", suffix, "box-shadow-color", tokens.FamilyColor)
	}
	name := withoutModifier(suffix)
	if hasToken(inventory, tokens.FamilyColor, name) {
		return namedMeaning("shadow-", suffix, "box-shadow-color", tokens.FamilyColor)
	}
	return namedOrArbitrary(base, "shadow-", suffix, "box-shadow", tokens.FamilyShadow)
}

func namedOrArbitrary(base, prefix, suffix, property string, family tokens.Family) UtilityMeaning {
	if value, trailing, found := arbitraryParts(suffix); found {
		return UtilityMeaning{
			Property: property, Family: family, ArbitraryValue: decodeArbitraryValue(value),
			SuggestionPrefix: prefix, SuggestionSuffix: trailing,
		}
	}
	return namedMeaning(prefix, suffix, property, family)
}

func decodeArbitraryValue(value string) string {
	var builder strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) && value[index+1] == '_' {
			builder.WriteByte('_')
			index++
			continue
		}
		if value[index] == '_' {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteByte(value[index])
	}
	return builder.String()
}

func namedMeaning(prefix, suffix, property string, family tokens.Family) UtilityMeaning {
	return UtilityMeaning{
		Property: property, Family: family, TokenName: withoutModifier(suffix),
		SuggestionPrefix: prefix,
	}
}

func arbitraryParts(suffix string) (string, string, bool) {
	if !strings.HasPrefix(suffix, "[") {
		return "", "", false
	}
	end := strings.LastIndexByte(suffix, ']')
	if end < 1 {
		return "", "", false
	}
	return suffix[1:end], suffix[end+1:], true
}

func withoutModifier(name string) string {
	if before, _, found := strings.Cut(name, "/"); found {
		return before
	}
	return name
}

func looksLikeColor(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "#") || strings.HasPrefix(value, "rgb(") ||
		strings.HasPrefix(value, "rgba(") || strings.HasPrefix(value, "hsl(") ||
		strings.HasPrefix(value, "hsla(") || strings.HasPrefix(value, "oklch(") ||
		strings.HasPrefix(value, "oklab(") || strings.HasPrefix(value, "color(") ||
		strings.HasPrefix(value, "color:") || isBasicNamedColor(value)
}

func isBasicNamedColor(value string) bool {
	return contains([]string{
		"aqua", "black", "blue", "fuchsia", "gray", "green", "grey", "lime",
		"maroon", "navy", "olive", "orange", "purple", "red", "silver", "teal",
		"transparent", "white", "yellow",
	}, value)
}

func hasToken(inventory *tokens.Inventory, family tokens.Family, name string) bool {
	if inventory == nil {
		return false
	}
	_, found := inventory.ByName(family, name)
	return found
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func isBorderWidth(name string) bool {
	if contains([]string{"x", "y", "s", "e", "t", "r", "b", "l", "0", "2", "4", "8"}, name) {
		return true
	}
	for _, direction := range []string{"x-", "y-", "s-", "e-", "t-", "r-", "b-", "l-"} {
		if strings.HasPrefix(name, direction) {
			return true
		}
	}
	return false
}
