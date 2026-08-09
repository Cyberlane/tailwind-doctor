package tailwind

import (
	"sort"
	"strings"
)

// A utility is not just a string with colons in it. It carries stacked variants,
// an optional important marker written before it in Tailwind v3 and after it in
// v4, an optional leading minus, and — where a project configures them — a
// prefix and a separator other than the colon. Splitting on the last colon gets
// all of this wrong, and gets it wrong most often on arbitrary values, which are
// exactly the utilities the rules care about: text-[color:red] is one utility
// with a colon inside it, not a variant applied to "red]".

// UtilitySyntax carries the project-level options that change how a utility is
// spelled. The zero value is invalid; use DefaultUtilitySyntax.
type UtilitySyntax struct {
	// Prefix is the Tailwind v3 `prefix` option, for example "tw-" giving
	// "tw-p-4". Tailwind v4 spells its prefix as a leading variant instead.
	Prefix string
	// PrefixIsVariant records the Tailwind v4 spelling "tw:p-4" rather than
	// the v3 name prefix "tw-p-4".
	PrefixIsVariant bool
	// Separator is the Tailwind v3 `separator` option, ":" unless configured.
	Separator string
}

// DefaultUtilitySyntax returns Tailwind's default utility syntax.
func DefaultUtilitySyntax() UtilitySyntax {
	return UtilitySyntax{Separator: ":"}
}

func (syntax UtilitySyntax) separator() string {
	if syntax.Separator == "" {
		return ":"
	}
	return syntax.Separator
}

// ParsedUtility is a utility separated into the parts that affect its meaning.
type ParsedUtility struct {
	// Variants are the stacked conditions, in source order.
	Variants []string
	// Base is the utility with variants, prefix, important marker, and leading
	// minus removed: the part that names a property.
	Base string
	// Negative records a leading minus, which changes the value's sign but not
	// which property the utility sets.
	Negative bool
	// Important records ! in either position.
	Important bool
}

// SplitOnSeparator splits a utility into its variant segments, ignoring any
// separator that sits inside brackets, parentheses, or quotes. Arbitrary
// variants and arbitrary values both contain characters that would otherwise
// split a utility in the wrong place.
func SplitOnSeparator(raw, separator string) []string {
	segments := make([]string, 0, 2)
	depth := 0
	var quote byte
	start := 0

	for index := 0; index < len(raw); index++ {
		character := raw[index]
		switch {
		case quote != 0:
			if character == quote {
				quote = 0
			}
		case character == '\'' || character == '"':
			quote = character
		case character == '[' || character == '(':
			depth++
		case character == ']' || character == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && strings.HasPrefix(raw[index:], separator):
			segments = append(segments, raw[start:index])
			index += len(separator) - 1
			start = index + 1
		}
	}
	return append(segments, raw[start:])
}

// ParseUtility parses a utility using project-level syntax options.
func ParseUtility(raw string, syntax UtilitySyntax) ParsedUtility {
	segments := SplitOnSeparator(raw, syntax.separator())
	prefixNegative := false
	if syntax.PrefixIsVariant && syntax.Prefix != "" && len(segments) > 1 {
		switch segments[0] {
		case syntax.Prefix:
			segments = segments[1:]
		case "-" + syntax.Prefix:
			segments = segments[1:]
			prefixNegative = true
		}
	}
	parsed := ParsedUtility{Base: segments[len(segments)-1]}
	if len(segments) > 1 {
		parsed.Variants = segments[:len(segments)-1]
	}

	// Tailwind v3 writes the important marker before the utility and v4 after
	// it. Both forms appear in the wild, often in the same codebase mid-upgrade.
	if trimmed, found := strings.CutPrefix(parsed.Base, "!"); found {
		parsed.Base, parsed.Important = trimmed, true
	}
	if trimmed, found := strings.CutSuffix(parsed.Base, "!"); found {
		parsed.Base, parsed.Important = trimmed, true
	}

	// The minus comes before the configured prefix: -tw-mt-2, not tw--mt-2.
	if trimmed, found := strings.CutPrefix(parsed.Base, "-"); found {
		parsed.Base, parsed.Negative = trimmed, true
	}
	if prefixNegative {
		parsed.Negative = true
	}
	if syntax.Prefix != "" && !syntax.PrefixIsVariant {
		parsed.Base = strings.TrimPrefix(parsed.Base, syntax.Prefix)
	}

	return parsed
}

// VariantKey identifies the condition under which a utility applies. Variants
// are sorted because hover:md:p-4 and md:hover:p-4 select the same elements, so
// two utilities carrying them conflict with each other.
func (parsed ParsedUtility) VariantKey() string {
	if len(parsed.Variants) == 0 {
		return ""
	}
	sorted := append([]string(nil), parsed.Variants...)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}

// HasArbitraryValue reports a bracketed value such as text-[#123456]. An
// arbitrary variant — [&_svg]:size-4 — is not one: it selects where a utility
// applies rather than hard-coding a value that should have been a token.
func (parsed ParsedUtility) HasArbitraryValue() bool {
	return strings.Contains(parsed.Base, "[")
}

// UtilityGroup names the property a utility sets, so that two utilities setting
// the same property under the same variants can be reported as conflicting.
func UtilityGroup(base string) string {
	for _, prefix := range []string{
		"px-", "py-", "pt-", "pr-", "pb-", "pl-", "p-",
		"mx-", "my-", "mt-", "mr-", "mb-", "ml-", "m-",
		"text-", "bg-", "border-",
	} {
		if strings.HasPrefix(base, prefix) {
			return prefix
		}
	}
	return ""
}
