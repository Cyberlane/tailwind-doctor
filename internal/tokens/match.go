package tokens

import (
	"math/big"
	"sort"
	"strings"
)

// Normalization exists so that text-[#123456] can be recognised as a value the
// project already has a name for. It is deliberately narrow. Every transformation
// below is one that cannot change which colour or length a value denotes. Exact
// matches may become source fixes, so a wrong match is a wrong edit rather than
// merely a wrong message.
//
// Notably absent: unit conversion. 16px and 1rem are equal only under a root font
// size nobody declared, so they stay different values.

// Normalize reduces a CSS value to a comparable form. It reports false for a value
// this tool will not compare at all.
func Normalize(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", false
	}
	// A value computed from other values is not knowable without evaluating the
	// cascade, which this tool does not do. "Conservative over complete".
	if strings.Contains(value, "var(") || strings.Contains(value, "calc(") {
		return "", false
	}

	fields := strings.Fields(value)
	for index, field := range fields {
		fields[index] = normalizeField(field)
	}
	return strings.Join(fields, " "), true
}

func normalizeField(field string) string {
	if expanded, ok := expandHex(field); ok {
		return expanded
	}
	return trimNumericNoise(field)
}

// expandHex rewrites #abc as #aabbcc and #abcd as #aabbccdd, so a shorthand and a
// longhand spelling of one colour compare equal.
func expandHex(field string) (string, bool) {
	digits, found := strings.CutPrefix(field, "#")
	if !found {
		return "", false
	}
	for index := 0; index < len(digits); index++ {
		if !isHexDigit(digits[index]) {
			return "", false
		}
	}
	switch len(digits) {
	case 3, 4:
		var builder strings.Builder
		builder.WriteByte('#')
		for index := 0; index < len(digits); index++ {
			builder.WriteByte(digits[index])
			builder.WriteByte(digits[index])
		}
		return builder.String(), true
	case 6, 8:
		return "#" + digits, true
	}
	return "", false
}

func isHexDigit(character byte) bool {
	return (character >= '0' && character <= '9') ||
		(character >= 'a' && character <= 'f')
}

// trimNumericNoise makes .25rem, 0.25rem, and 0.250rem one value. The unit is
// carried through untouched.
func trimNumericNoise(field string) string {
	number, unit := splitNumber(field)
	if number == "" {
		return field
	}
	if strings.HasPrefix(number, ".") {
		number = "0" + number
	}
	if strings.Contains(number, ".") {
		number = strings.TrimRight(number, "0")
		number = strings.TrimSuffix(number, ".")
	}
	if number == "" || number == "-" {
		number += "0"
	}
	return number + unit
}

// splitNumber divides a leading decimal number from its unit. It returns an empty
// number for a field that does not start with one, which is how a keyword such as
// "solid" passes through unchanged.
func splitNumber(field string) (string, string) {
	index := 0
	if index < len(field) && (field[index] == '-' || field[index] == '+') {
		index++
	}
	start := index
	seenPoint := false
	for index < len(field) {
		character := field[index]
		switch {
		case character >= '0' && character <= '9':
			index++
		case character == '.' && !seenPoint:
			seenPoint, index = true, index+1
		default:
			if index == start {
				return "", field
			}
			return field[:index], field[index:]
		}
	}
	if index == start {
		return "", field
	}
	return field, ""
}

// Lookup finds a token in one family whose value equals raw. A tie resolves by
// name order so the suggestion is stable between runs; two tokens sharing a value
// is common in a real theme, where "brand" and "primary" are often the same blue.
func (inventory *Inventory) Lookup(family Family, raw string) (Token, bool) {
	wanted, resolvable := Normalize(raw)
	if !resolvable {
		return Token{}, false
	}

	named := inventory.byFamily[family]
	names := make([]string, 0, len(named))
	for name := range named {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		token := named[name]
		if token.Unresolvable {
			continue
		}
		if token.Value == wanted {
			return token, true
		}
	}
	return Token{}, false
}

// SpacingMultiple returns the integer Tailwind spacing multiplier represented
// by raw. It compares only values with the same unit and never assumes a root
// font size, so px and rem remain deliberately incomparable.
func (inventory *Inventory) SpacingMultiple(raw string) (string, bool) {
	base, found := inventory.ByName(FamilySpacing, "DEFAULT")
	if !found || base.Unresolvable {
		return "", false
	}
	wanted, wantedOK := Normalize(raw)
	if !wantedOK {
		return "", false
	}
	wantedNumber, wantedUnit := splitNumber(wanted)
	baseNumber, baseUnit := splitNumber(base.Value)
	if wantedNumber == "" || baseNumber == "" || wantedUnit != baseUnit || strings.ContainsAny(wantedUnit, " ") {
		return "", false
	}
	wantedRat, wantedOK := new(big.Rat).SetString(wantedNumber)
	baseRat, baseOK := new(big.Rat).SetString(baseNumber)
	if !wantedOK || !baseOK || baseRat.Sign() <= 0 || wantedRat.Sign() < 0 {
		return "", false
	}
	multiple := new(big.Rat).Quo(wantedRat, baseRat)
	if !multiple.IsInt() {
		return "", false
	}
	return multiple.Num().String(), true
}

// IsSpacingMultiplier reports whether a utility suffix can be generated by the
// v4 --spacing base. Keywords such as auto, full, and px are not multipliers.
func IsSpacingMultiplier(name string) bool {
	if name == "" {
		return false
	}
	seenDigit := false
	seenPoint := false
	for _, character := range name {
		switch {
		case character >= '0' && character <= '9':
			seenDigit = true
		case character == '.' && !seenPoint:
			seenPoint = true
		default:
			return false
		}
	}
	return seenDigit
}
