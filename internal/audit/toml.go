package audit

import (
	"fmt"
	"strconv"
	"strings"
)

// A deliberately small TOML reader, covering exactly what twdoctor.toml needs:
// tables, string values, booleans, and arrays of strings. The configuration file
// is a handful of settings, and a dependency taken to read it would be a
// dependency shipped in a binary whose selling point is that it has none.
//
// Anything outside the subset is an error rather than a silent misreading, so a
// file written against fuller TOML fails loudly instead of half-applying. The
// supported subset is documented in docs/configuration.md.

type tomlTable map[string]any

func parseTOML(content string) (map[string]tomlTable, error) {
	document := map[string]tomlTable{}
	table := ""
	document[table] = tomlTable{}
	seenTables := map[string]bool{"": true}

	lines := strings.Split(content, "\n")
	for number := 0; number < len(lines); number++ {
		line := stripTOMLComment(strings.TrimSpace(lines[number]))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, fmt.Errorf("line %d: unterminated table header", number+1)
			}
			table = strings.TrimSpace(line[1 : len(line)-1])
			if table == "" {
				return nil, fmt.Errorf("line %d: empty table name", number+1)
			}
			if seenTables[table] {
				return nil, fmt.Errorf("line %d: duplicate table %q", number+1, table)
			}
			seenTables[table] = true
			document[table] = tomlTable{}
			continue
		}

		key, rest, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d: expected key = value", number+1)
		}
		key = strings.TrimSpace(key)
		rest = strings.TrimSpace(rest)

		// An array may run over several lines, which is how anyone writes a list
		// of ignore patterns they expect to keep adding to.
		for strings.HasPrefix(rest, "[") && strings.Count(rest, "[") > strings.Count(rest, "]") {
			number++
			if number >= len(lines) {
				return nil, fmt.Errorf("unterminated array in %q", key)
			}
			rest += " " + stripTOMLComment(strings.TrimSpace(lines[number]))
		}

		value, err := parseTOMLValue(rest)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", number+1, key, err)
		}
		key = unquoteTOMLKey(key)
		if _, exists := document[table][key]; exists {
			return nil, fmt.Errorf("line %d: duplicate key %q", number+1, key)
		}
		document[table][key] = value
	}
	return document, nil
}

// stripTOMLComment removes a trailing comment, leaving a # inside a string alone.
func stripTOMLComment(line string) string {
	var quote byte
	for index := 0; index < len(line); index++ {
		switch {
		case quote != 0:
			if line[index] == quote {
				quote = 0
			}
		case line[index] == '"' || line[index] == '\'':
			quote = line[index]
		case line[index] == '#':
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func unquoteTOMLKey(key string) string {
	if unquoted, err := strconv.Unquote(key); err == nil {
		return unquoted
	}
	return key
}

func parseTOMLValue(text string) (any, error) {
	switch {
	case text == "true":
		return true, nil
	case text == "false":
		return false, nil
	case strings.HasPrefix(text, "["):
		return parseTOMLArray(text)
	case strings.HasPrefix(text, `"`) || strings.HasPrefix(text, "'"):
		return parseTOMLString(text)
	}
	return nil, fmt.Errorf("unsupported value %q; expected a string, a boolean, or an array of strings", text)
}

func parseTOMLArray(text string) (any, error) {
	if !strings.HasSuffix(text, "]") {
		return nil, fmt.Errorf("unterminated array")
	}
	body := strings.TrimSpace(text[1 : len(text)-1])
	items := []string{}
	if body == "" {
		return items, nil
	}
	for _, element := range splitTOMLElements(body) {
		element = strings.TrimSpace(element)
		if element == "" {
			continue
		}
		item, err := parseTOMLString(element)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// splitTOMLElements splits on commas that are not inside a string.
func splitTOMLElements(body string) []string {
	elements := []string{}
	var quote byte
	start := 0
	for index := 0; index < len(body); index++ {
		switch {
		case quote != 0:
			if body[index] == quote {
				quote = 0
			}
		case body[index] == '"' || body[index] == '\'':
			quote = body[index]
		case body[index] == ',':
			elements = append(elements, body[start:index])
			start = index + 1
		}
	}
	return append(elements, body[start:])
}

func parseTOMLString(text string) (string, error) {
	text = strings.TrimSpace(text)
	if len(text) >= 2 && strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'") {
		// A literal string: no escape processing, which is what a glob wants.
		return text[1 : len(text)-1], nil
	}
	if len(text) >= 2 && strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`) {
		unquoted, err := strconv.Unquote(text)
		if err != nil {
			return "", fmt.Errorf("invalid string %s", text)
		}
		return unquoted, nil
	}
	return "", fmt.Errorf("expected a quoted string, got %s", text)
}

func (table tomlTable) stringValue(key string) (string, bool, error) {
	return typedTableValue[string](table, key, "a string")
}

func (table tomlTable) boolValue(key string) (bool, bool, error) {
	return typedTableValue[bool](table, key, "true or false")
}

func (table tomlTable) listValue(key string) ([]string, error) {
	values, _, err := typedTableValue[[]string](table, key, "an array of strings")
	return values, err
}

func typedTableValue[Value any](table tomlTable, key, expected string) (Value, bool, error) {
	var zero Value
	raw, present := table[key]
	if !present {
		return zero, false, nil
	}
	value, ok := raw.(Value)
	if !ok {
		return zero, false, fmt.Errorf("%s must be %s", key, expected)
	}
	return value, true, nil
}
