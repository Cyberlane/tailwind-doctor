// Package cssdecl reads CSS at-rules, rules, and custom-property declarations
// into an ordered tree.
//
// It knows nothing about Tailwind. Order is preserved because it is meaningful to
// the caller: "--color-*: initial" followed by twelve colours means twelve
// colours, and a map would lose the operation that clears the defaults.
package cssdecl

import (
	"fmt"
	"sort"
	"strings"
)

// NodeKind identifies a node in the ordered stylesheet tree.
type NodeKind string

const (
	NodeAtRule      NodeKind = "at-rule"
	NodeRule        NodeKind = "rule"
	NodeDeclaration NodeKind = "declaration"
)

// Node is an at-rule, rule, or declaration.
type Node struct {
	Kind     NodeKind
	Name     string
	Prelude  string
	Selector string
	Property string
	Value    string
	Children []Node
	Line     int
	Column   int
}

// Sheet is a stylesheet in source order.
type Sheet struct {
	Nodes []Node
}

type scanner struct {
	source     string
	offset     int
	lineStarts []int
}

func newScanner(source string) *scanner {
	lineStarts := []int{0}
	for index, character := range []byte(source) {
		if character == '\n' {
			lineStarts = append(lineStarts, index+1)
		}
	}
	return &scanner{source: source, lineStarts: lineStarts}
}

func (scanner *scanner) done() bool {
	return scanner.offset >= len(scanner.source)
}

func (scanner *scanner) peek() byte {
	if scanner.done() {
		return 0
	}
	return scanner.source[scanner.offset]
}

func (scanner *scanner) has(text string) bool {
	return strings.HasPrefix(scanner.source[scanner.offset:], text)
}

func (scanner *scanner) advance() byte {
	if scanner.done() {
		return 0
	}
	character := scanner.source[scanner.offset]
	scanner.offset++
	return character
}

func (scanner *scanner) position() (int, int) {
	lineIndex := sort.Search(len(scanner.lineStarts), func(index int) bool {
		return scanner.lineStarts[index] > scanner.offset
	}) - 1
	return lineIndex + 1, scanner.offset - scanner.lineStarts[lineIndex] + 1
}

func (scanner *scanner) skipSpaceAndComments() error {
	for !scanner.done() {
		if character := scanner.peek(); character == ' ' || character == '\t' || character == '\n' || character == '\r' {
			scanner.advance()
			continue
		}
		if !scanner.has("/*") {
			return nil
		}
		scanner.advance()
		scanner.advance()
		for !scanner.done() && !scanner.has("*/") {
			scanner.advance()
		}
		if scanner.done() {
			line, _ := scanner.position()
			return fmt.Errorf("unterminated CSS comment at line %d", line)
		}
		scanner.advance()
		scanner.advance()
	}
	return nil
}

// Parse reads a stylesheet without interpreting its at-rules.
func Parse(source string) (Sheet, error) {
	scanner := newScanner(source)
	nodes, err := scanner.readNodes(false)
	if err != nil {
		return Sheet{}, err
	}
	return Sheet{Nodes: nodes}, nil
}

func (scanner *scanner) readNodes(inBlock bool) ([]Node, error) {
	nodes := []Node{}
	for {
		if err := scanner.skipSpaceAndComments(); err != nil {
			return nil, err
		}
		if scanner.done() {
			if inBlock {
				return nil, fmt.Errorf("unterminated CSS block")
			}
			return nodes, nil
		}
		if scanner.peek() == '}' {
			if !inBlock {
				line, column := scanner.position()
				return nil, fmt.Errorf("unexpected closing brace at %d:%d", line, column)
			}
			scanner.advance()
			return nodes, nil
		}

		var (
			node Node
			err  error
		)
		if scanner.peek() == '@' {
			node, err = scanner.readAtRule()
		} else {
			node, err = scanner.readRuleOrDeclaration()
		}
		if err != nil {
			return nil, err
		}
		if node.Kind != "" {
			nodes = append(nodes, node)
		}
	}
}

func (scanner *scanner) readAtRule() (Node, error) {
	line, column := scanner.position()
	scanner.advance()
	start := scanner.offset
	for isNameCharacter(scanner.peek()) {
		scanner.advance()
	}
	name := scanner.source[start:scanner.offset]
	if name == "" {
		return Node{}, fmt.Errorf("empty at-rule at %d:%d", line, column)
	}
	prelude, delimiter, err := scanner.readSegment()
	if err != nil {
		return Node{}, err
	}
	node := Node{Kind: NodeAtRule, Name: name, Prelude: strings.TrimSpace(prelude), Line: line, Column: column}
	switch delimiter {
	case ';':
		scanner.advance()
	case '{':
		scanner.advance()
		node.Children, err = scanner.readNodes(true)
		if err != nil {
			return Node{}, err
		}
	case 0:
		return Node{}, fmt.Errorf("unterminated @%s at %d:%d", name, line, column)
	default:
		return Node{}, fmt.Errorf("unexpected %q in @%s at %d:%d", delimiter, name, line, column)
	}
	return node, nil
}

func isNameCharacter(character byte) bool {
	return character == '-' || character == '_' ||
		(character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

func (scanner *scanner) readRuleOrDeclaration() (Node, error) {
	line, column := scanner.position()
	text, delimiter, err := scanner.readSegment()
	if err != nil {
		return Node{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		if delimiter == ';' {
			scanner.advance()
		}
		return Node{}, nil
	}
	if delimiter == '{' {
		scanner.advance()
		children, err := scanner.readNodes(true)
		if err != nil {
			return Node{}, err
		}
		return Node{Kind: NodeRule, Selector: text, Children: children, Line: line, Column: column}, nil
	}

	colon := firstUnquotedColon(text)
	if colon < 0 {
		return Node{}, fmt.Errorf("expected declaration at %d:%d", line, column)
	}
	node := Node{
		Kind:     NodeDeclaration,
		Property: strings.TrimSpace(text[:colon]),
		Value:    strings.TrimSpace(text[colon+1:]),
		Line:     line,
		Column:   column,
	}
	if delimiter == ';' {
		scanner.advance()
	}
	return node, nil
}

// readSegment reads through the next structural delimiter without consuming it.
// Delimiters inside quotes, parentheses, and brackets are literal content.
func (scanner *scanner) readSegment() (string, byte, error) {
	var builder strings.Builder
	depth := 0
	for !scanner.done() {
		if scanner.has("/*") {
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			if err := scanner.skipSpaceAndComments(); err != nil {
				return "", 0, err
			}
			continue
		}
		character := scanner.peek()
		if character == '\'' || character == '"' {
			quoted, err := scanner.readQuoted(character)
			if err != nil {
				return "", 0, err
			}
			builder.WriteString(quoted)
			continue
		}
		if depth == 0 && (character == '{' || character == '}' || character == ';') {
			return builder.String(), character, nil
		}
		switch character {
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		}
		builder.WriteByte(scanner.advance())
	}
	return builder.String(), 0, nil
}

func (scanner *scanner) readQuoted(quote byte) (string, error) {
	var builder strings.Builder
	builder.WriteByte(scanner.advance())
	for !scanner.done() {
		character := scanner.advance()
		builder.WriteByte(character)
		if character == '\\' && !scanner.done() {
			builder.WriteByte(scanner.advance())
			continue
		}
		if character == quote {
			return builder.String(), nil
		}
	}
	return "", fmt.Errorf("unterminated CSS string")
}

func firstUnquotedColon(text string) int {
	var quote byte
	for index := 0; index < len(text); index++ {
		character := text[index]
		if quote != 0 {
			if character == '\\' {
				index++
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		if character == ':' {
			return index
		}
	}
	return -1
}
