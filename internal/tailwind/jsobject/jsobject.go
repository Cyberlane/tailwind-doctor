// Package jsobject reads a JavaScript or TypeScript literal object into Go values.
//
// It knows nothing about Tailwind. It finds an exported object and reads only
// values knowable without evaluation. An unsupported construct becomes an
// unreadable value with a positioned defeat; it is never guessed or executed.
package jsobject

import (
	"fmt"
	"strings"
)

// Kind identifies the statically readable shape of a value.
type Kind string

const (
	KindObject     Kind = "object"
	KindArray      Kind = "array"
	KindString     Kind = "string"
	KindNumber     Kind = "number"
	KindBool       Kind = "bool"
	KindNull       Kind = "null"
	KindCall       Kind = "call"
	KindUnreadable Kind = "unreadable"
)

// Value is one statically read JavaScript value.
type Value struct {
	Kind    Kind
	Str     string
	Num     string
	Bool    bool
	Callee  string
	Args    []Value
	Items   []Value
	Entries []Entry
	Line    int
	Column  int
}

// Entry is one object property or spread.
type Entry struct {
	Key    string
	Spread bool
	Value  Value
	Line   int
	Column int
}

// Defeat names a construct that cannot be read without evaluation.
type Defeat struct {
	Construct string
	Line      int
	Column    int
}

// Result is the exported root and every unreadable construct encountered.
type Result struct {
	Root    Value
	Defeats []Defeat
}

// Get returns the last object entry for key, matching JavaScript object-literal
// semantics when a key is repeated.
func (value Value) Get(key string) (Value, bool) {
	entry, found := value.Entry(key)
	return entry.Value, found
}

// Entry returns the last full object entry for key.
func (value Value) Entry(key string) (Entry, bool) {
	for index := len(value.Entries) - 1; index >= 0; index-- {
		if value.Entries[index].Key == key && !value.Entries[index].Spread {
			return value.Entries[index], true
		}
	}
	return Entry{}, false
}

// Strings returns the readable string items in an array.
func (value Value) Strings() []string {
	stringsOnly := make([]string, 0, len(value.Items))
	for _, item := range value.Items {
		if item.Kind == KindString {
			stringsOnly = append(stringsOnly, item.Str)
		}
	}
	return stringsOnly
}

type parser struct {
	source  string
	offset  int
	line    int
	column  int
	defeats []Defeat
}

func newParser(source string) *parser {
	return &parser{source: source, line: 1, column: 1}
}

func newParserAt(source string, offset int) *parser {
	reader := newParser(source)
	for reader.offset < offset {
		reader.advance()
	}
	return reader
}

func (reader *parser) done() bool {
	return reader.offset >= len(reader.source)
}

func (reader *parser) peek() byte {
	if reader.done() {
		return 0
	}
	return reader.source[reader.offset]
}

func (reader *parser) has(text string) bool {
	return strings.HasPrefix(reader.source[reader.offset:], text)
}

func (reader *parser) advance() byte {
	if reader.done() {
		return 0
	}
	character := reader.source[reader.offset]
	reader.offset++
	if character == '\n' {
		reader.line++
		reader.column = 1
	} else {
		reader.column++
	}
	return character
}

func (reader *parser) advanceText(text string) {
	for range len(text) {
		reader.advance()
	}
}

func (reader *parser) skipSpaceAndComments() {
	for !reader.done() {
		switch {
		case isSpace(reader.peek()):
			reader.advance()
		case reader.has("//"):
			for !reader.done() && reader.advance() != '\n' {
			}
		case reader.has("/*"):
			reader.advanceText("/*")
			for !reader.done() && !reader.has("*/") {
				reader.advance()
			}
			if reader.has("*/") {
				reader.advanceText("*/")
			}
		default:
			return
		}
	}
}

func isSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

func isIdentifierStart(character byte) bool {
	return character == '_' || character == '$' || isASCIILetter(character)
}

func isASCIILetter(character byte) bool {
	lowercase := character | 0x20
	return lowercase >= 'a' && lowercase <= 'z'
}

func isIdentifierPart(character byte) bool {
	return isIdentifierStart(character) || (character >= '0' && character <= '9')
}

func (reader *parser) identifier() string {
	start := reader.offset
	if isIdentifierStart(reader.peek()) {
		reader.advance()
		for isIdentifierPart(reader.peek()) {
			reader.advance()
		}
	}
	return reader.source[start:reader.offset]
}

func (reader *parser) word(word string) bool {
	if !reader.has(word) {
		return false
	}
	beforeOK := reader.offset == 0 || !isIdentifierPart(reader.source[reader.offset-1])
	after := reader.offset + len(word)
	afterOK := after >= len(reader.source) || !isIdentifierPart(reader.source[after])
	return beforeOK && afterOK
}

// Parse finds and statically reads the exported object.
func Parse(source string) (Result, error) {
	reader := newParser(source)
	for !reader.done() {
		reader.skipSpaceAndComments()
		if reader.done() {
			break
		}

		if reader.has("module.exports") {
			reader.advanceText("module.exports")
			reader.skipSpaceAndComments()
			if reader.peek() != '=' {
				reader.advance()
				continue
			}
			reader.advance()
			return reader.readExport()
		}
		if reader.word("export") {
			reader.advanceText("export")
			reader.skipSpaceAndComments()
			if reader.word("default") {
				reader.advanceText("default")
				return reader.readExport()
			}
			if reader.peek() == '=' {
				reader.advance()
				return reader.readExport()
			}
		}

		if reader.peek() == '\'' || reader.peek() == '"' || reader.peek() == '`' {
			reader.skipQuoted(reader.peek())
		} else {
			reader.advance()
		}
	}
	return Result{}, fmt.Errorf("no exported object found")
}

func (reader *parser) readExport() (Result, error) {
	reader.skipSpaceAndComments()
	if reader.peek() == '{' {
		root := reader.readValue()
		return Result{Root: root, Defeats: reader.defeats}, nil
	}
	if isIdentifierStart(reader.peek()) {
		line, column := reader.line, reader.column
		identifier := reader.identifier()
		root, defeats, found := resolveIdentifier(reader.source, reader.offset, identifier)
		if found {
			return Result{Root: root, Defeats: defeats}, nil
		}
		return Result{
			Root:    Value{Kind: KindUnreadable, Line: line, Column: column},
			Defeats: []Defeat{{Construct: "identifier reference", Line: line, Column: column}},
		}, nil
	}
	line, column := reader.line, reader.column
	return Result{
		Root:    Value{Kind: KindUnreadable, Line: line, Column: column},
		Defeats: []Defeat{{Construct: "exported non-object", Line: line, Column: column}},
	}, nil
}

func resolveIdentifier(source string, limit int, wanted string) (Value, []Defeat, bool) {
	scanner := newParser(source)
	candidate := -1
	assignments := 0
	for scanner.offset < limit {
		scanner.skipSpaceAndComments()
		if scanner.offset >= limit {
			break
		}
		if scanner.word("const") || scanner.word("let") || scanner.word("var") {
			keyword := scanner.identifier()
			_ = keyword
			scanner.skipSpaceAndComments()
			name := scanner.identifier()
			if name != wanted {
				scanner.skipToStatementEnd()
				continue
			}
			scanner.skipSpaceAndComments()
			if scanner.peek() == ':' {
				for !scanner.done() && scanner.peek() != '=' && scanner.offset < limit {
					scanner.advance()
				}
			}
			scanner.skipSpaceAndComments()
			if scanner.peek() != '=' {
				return Value{}, nil, false
			}
			scanner.advance()
			scanner.skipSpaceAndComments()
			assignments++
			candidate = scanner.offset
			scanner.skipToStatementEnd()
			continue
		}
		if scanner.peek() == '\'' || scanner.peek() == '"' || scanner.peek() == '`' {
			scanner.skipQuoted(scanner.peek())
		} else {
			scanner.advance()
		}
	}
	if assignments != 1 || candidate < 0 || source[candidate] != '{' {
		return Value{}, nil, false
	}
	resolved := newParserAt(source, candidate)
	root := resolved.readValue()
	return root, resolved.defeats, true
}

func (reader *parser) skipToStatementEnd() {
	reader.skipNestedUntil(func(character byte, depth int) bool {
		return depth == 0 && (character == ';' || character == '\n')
	})
	if reader.peek() == ';' || reader.peek() == '\n' {
		reader.advance()
	}
}

func (reader *parser) readValue() Value {
	reader.skipSpaceAndComments()
	line, column := reader.line, reader.column
	if reader.done() {
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	}

	switch character := reader.peek(); {
	case character == '{':
		return reader.readObject()
	case character == '[':
		return reader.readArray()
	case character == '\'' || character == '"' || character == '`':
		text, substituted := reader.readString(character)
		if substituted {
			reader.defeat("template substitution", line, column)
			return Value{Kind: KindUnreadable, Line: line, Column: column}
		}
		return Value{Kind: KindString, Str: text, Line: line, Column: column}
	case character == '(':
		reader.defeat("function", line, column)
		reader.skipExpression()
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	case (character >= '0' && character <= '9') || character == '-' || character == '+':
		return reader.readNumber()
	case isIdentifierStart(character):
		return reader.readIdentifierValue()
	default:
		reader.defeat("unsupported expression", line, column)
		reader.skipExpression()
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	}
}

func (reader *parser) readObject() Value {
	line, column := reader.line, reader.column
	reader.advance()
	value := Value{Kind: KindObject, Line: line, Column: column, Entries: []Entry{}}

	for !reader.done() {
		reader.skipSpaceAndComments()
		if reader.peek() == '}' {
			reader.advance()
			return value
		}
		entryLine, entryColumn := reader.line, reader.column
		entry := Entry{Line: entryLine, Column: entryColumn}

		if reader.has("...") {
			reader.advanceText("...")
			reader.defeat("spread", entryLine, entryColumn)
			reader.skipExpression()
			entry.Spread = true
			entry.Value = Value{Kind: KindUnreadable, Line: entryLine, Column: entryColumn}
			value.Entries = append(value.Entries, entry)
			reader.consumeComma()
			continue
		}
		if reader.peek() == '[' {
			reader.defeat("computed key", entryLine, entryColumn)
			reader.skipBalanced('[', ']')
			reader.skipSpaceAndComments()
			if reader.peek() == ':' {
				reader.advance()
				_ = reader.readValue()
			}
			entry.Value = Value{Kind: KindUnreadable, Line: entryLine, Column: entryColumn}
			value.Entries = append(value.Entries, entry)
			reader.consumeComma()
			continue
		}

		switch {
		case reader.peek() == '\'' || reader.peek() == '"' || reader.peek() == '`':
			entry.Key, _ = reader.readString(reader.peek())
		case isIdentifierStart(reader.peek()):
			entry.Key = reader.identifier()
		case reader.peek() >= '0' && reader.peek() <= '9':
			entry.Key = reader.readNumber().Num
		default:
			reader.defeat("object key", entryLine, entryColumn)
			reader.skipExpression()
			reader.consumeComma()
			continue
		}
		reader.skipSpaceAndComments()
		if reader.peek() != ':' {
			reader.defeat("object shorthand", entryLine, entryColumn)
			entry.Value = Value{Kind: KindUnreadable, Line: entryLine, Column: entryColumn}
			value.Entries = append(value.Entries, entry)
			reader.skipExpression()
			reader.consumeComma()
			continue
		}
		reader.advance()
		entry.Value = reader.readValue()
		value.Entries = append(value.Entries, entry)
		reader.consumeComma()
	}
	return value
}

func (reader *parser) readArray() Value {
	line, column := reader.line, reader.column
	reader.advance()
	value := Value{Kind: KindArray, Line: line, Column: column, Items: []Value{}}
	for !reader.done() {
		reader.skipSpaceAndComments()
		if reader.peek() == ']' {
			reader.advance()
			return value
		}
		value.Items = append(value.Items, reader.readValue())
		reader.consumeComma()
	}
	return value
}

func (reader *parser) consumeComma() {
	reader.skipSpaceAndComments()
	if reader.peek() == ',' {
		reader.advance()
	}
}

func (reader *parser) readNumber() Value {
	line, column := reader.line, reader.column
	start := reader.offset
	if reader.peek() == '-' || reader.peek() == '+' {
		reader.advance()
	}
	for !reader.done() {
		character := reader.peek()
		if (character >= '0' && character <= '9') || character == '.' || character == 'e' || character == 'E' || character == '+' || character == '-' {
			reader.advance()
			continue
		}
		break
	}
	return Value{Kind: KindNumber, Num: reader.source[start:reader.offset], Line: line, Column: column}
}

func (reader *parser) readIdentifierValue() Value {
	line, column := reader.line, reader.column
	identifier := reader.identifier()
	switch identifier {
	case "true":
		return Value{Kind: KindBool, Bool: true, Line: line, Column: column}
	case "false":
		return Value{Kind: KindBool, Line: line, Column: column}
	case "null", "undefined":
		return Value{Kind: KindNull, Line: line, Column: column}
	case "function":
		reader.defeat("function", line, column)
		reader.skipExpression()
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	}

	reader.skipSpaceAndComments()
	if reader.has("=>") {
		reader.defeat("function", line, column)
		reader.skipExpression()
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	}
	if reader.peek() == '(' {
		return reader.readCall(identifier, line, column)
	}
	reader.defeat("identifier reference", line, column)
	reader.skipExpression()
	return Value{Kind: KindUnreadable, Line: line, Column: column}
}

func (reader *parser) readCall(callee string, line, column int) Value {
	reader.advance()
	arguments := []Value{}
	readable := true
	defeatsBefore := len(reader.defeats)
	for !reader.done() {
		reader.skipSpaceAndComments()
		if reader.peek() == ')' {
			reader.advance()
			break
		}
		argument := reader.readValue()
		arguments = append(arguments, argument)
		if argument.Kind == KindUnreadable {
			readable = false
		}
		reader.skipSpaceAndComments()
		if reader.peek() == ',' {
			reader.advance()
			continue
		}
		if reader.peek() == ')' {
			reader.advance()
			break
		}
	}
	if !readable {
		reader.defeats = reader.defeats[:defeatsBefore]
		reader.defeat("call with a non-literal argument", line, column)
		return Value{Kind: KindUnreadable, Line: line, Column: column}
	}
	return Value{Kind: KindCall, Callee: callee, Args: arguments, Line: line, Column: column}
}

func (reader *parser) readString(quote byte) (string, bool) {
	reader.advance()
	var builder strings.Builder
	substituted := false
	for !reader.done() {
		if reader.peek() == quote {
			reader.advance()
			break
		}
		if quote == '`' && reader.has("${") {
			substituted = true
		}
		character := reader.advance()
		if character == '\\' && !reader.done() {
			escaped := reader.advance()
			switch escaped {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteByte(escaped)
			}
			continue
		}
		builder.WriteByte(character)
	}
	return builder.String(), substituted
}

func (reader *parser) skipQuoted(quote byte) {
	reader.advance()
	for !reader.done() {
		character := reader.advance()
		if character == '\\' && !reader.done() {
			reader.advance()
			continue
		}
		if character == quote {
			return
		}
	}
}

func (reader *parser) skipBalanced(open, close byte) {
	if reader.peek() != open {
		return
	}
	depth := 0
	for !reader.done() {
		if reader.peek() == '\'' || reader.peek() == '"' || reader.peek() == '`' {
			reader.skipQuoted(reader.peek())
			continue
		}
		character := reader.advance()
		if character == open {
			depth++
		} else if character == close {
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

// skipExpression advances to the next comma or enclosing close delimiter without
// consuming it, respecting nested literals well enough to resume the parent.
func (reader *parser) skipExpression() {
	reader.skipNestedUntil(func(character byte, depth int) bool {
		return depth == 0 && (character == ',' || character == '}' || character == ']' || character == ')')
	})
}

func (reader *parser) skipNestedUntil(stop func(character byte, depth int) bool) {
	depth := 0
	for !reader.done() {
		if reader.peek() == '\'' || reader.peek() == '"' || reader.peek() == '`' {
			reader.skipQuoted(reader.peek())
			continue
		}
		if stop(reader.peek(), depth) {
			return
		}
		switch reader.peek() {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
		}
		reader.advance()
	}
}

func (reader *parser) defeat(construct string, line, column int) {
	reader.defeats = append(reader.defeats, Defeat{Construct: construct, Line: line, Column: column})
}
