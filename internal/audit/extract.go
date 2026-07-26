package audit

import (
	"path/filepath"
	"strings"
)

// Extraction is a scanner, not a parser. It tracks just enough syntax to know
// whether the text under the cursor is markup, a string, a comment, or an
// expression, which is what separates a class attribute from prose that happens
// to mention one. Nothing here evaluates user code or resolves a value that is
// only known at runtime; where a value cannot be read statically the site is
// recorded as unresolved and no classes are invented for it.

// Shape names are the vocabulary the extraction corpus uses. Adding one means
// adding it to testdata/corpus and to docs/extraction-accuracy.md.
const (
	shapeAttributeLiteral      = "attr-literal"
	shapeAttributeInterpolated = "attr-interpolated"
	shapeSvelteClassDirective  = "svelte-class-directive"
	shapeSvelteClassShorthand  = "svelte-class-shorthand"
)

// ClassList is one class list attributed to one source position. Resolved is
// false when the site was found but its value is not statically knowable; such
// an entry carries the expression text for reporting and must never be treated
// as a list of utilities.
type ClassList struct {
	Value    string
	Line     int
	Column   int
	Shape    string
	Resolved bool
}

// Extract reads every class list out of one source file. It is deterministic and
// depends on nothing but the path's extension and the content.
func Extract(path, content string) []ClassList {
	current := &scanner{
		source:    content,
		extension: strings.ToLower(filepath.Ext(path)),
		line:      1,
		column:    1,
	}
	current.run()
	return current.found
}

type scanner struct {
	source    string
	extension string
	offset    int
	line      int
	column    int
	found     []ClassList
}

func (s *scanner) done() bool {
	return s.offset >= len(s.source)
}

func (s *scanner) at(prefix string) bool {
	return strings.HasPrefix(s.source[s.offset:], prefix)
}

func (s *scanner) peek(ahead int) byte {
	if s.offset+ahead >= len(s.source) {
		return 0
	}
	return s.source[s.offset+ahead]
}

// advance moves the cursor and keeps the line and column with it. Column is a
// byte offset within the line, one-based, matching the corpus convention.
func (s *scanner) advance(count int) {
	for range count {
		if s.done() {
			return
		}
		if s.source[s.offset] == '\n' {
			s.line++
			s.column = 1
		} else {
			s.column++
		}
		s.offset++
	}
}

func (s *scanner) advanceThrough(marker string) {
	index := strings.Index(s.source[s.offset:], marker)
	if index < 0 {
		s.advance(len(s.source) - s.offset)
		return
	}
	s.advance(index + len(marker))
}

func (s *scanner) usesJavaScriptSyntax() bool {
	return s.extension == ".jsx" || s.extension == ".tsx"
}

func (s *scanner) run() {
	s.skipAstroFrontmatter()
	for !s.done() {
		if s.usesJavaScriptSyntax() {
			if s.skipJavaScriptNoise() {
				continue
			}
		} else {
			if s.at("<!--") {
				s.advanceThrough("-->")
				continue
			}
			if s.skipRawTextElement() {
				continue
			}
		}
		if s.startsTag() {
			s.scanTag()
			continue
		}
		s.advance(1)
	}
}

// Astro frontmatter is JavaScript fenced by --- at the top of the file. It holds
// no markup, so the cheapest correct thing is to step over it whole.
func (s *scanner) skipAstroFrontmatter() {
	if s.extension != ".astro" || !s.at("---") {
		return
	}
	s.advance(3)
	s.advanceThrough("\n---")
}

// A <script> or <style> body is not markup, and in a template-holding script it
// is not live markup either. Either way its contents are not class attributes on
// this document's elements. Helper calls inside a component's script block are a
// separate extraction shape and are not handled here.
func (s *scanner) skipRawTextElement() bool {
	for _, name := range []string{"script", "style"} {
		if !s.atTagNamed(name) {
			continue
		}
		s.advance(1 + len(name))
		s.advanceThrough(">")
		s.advanceThrough("</" + name)
		s.advanceThrough(">")
		return true
	}
	return false
}

func (s *scanner) atTagNamed(name string) bool {
	if !s.at("<" + name) {
		return false
	}
	next := s.peek(1 + len(name))
	return next == '>' || next == ' ' || next == '\t' || next == '\n' || next == '\r'
}

// skipJavaScriptNoise steps over the constructs that can contain text looking
// like markup without being markup: comments, strings, and template literals.
// It reports whether it consumed anything.
func (s *scanner) skipJavaScriptNoise() bool {
	switch {
	case s.at("//"):
		s.advanceThrough("\n")
		return true
	case s.at("/*"):
		s.advance(2)
		s.advanceThrough("*/")
		return true
	case s.peek(0) == '"' || s.peek(0) == '\'' || s.peek(0) == '`':
		s.skipStringLiteral()
		return true
	}
	return false
}

func (s *scanner) skipStringLiteral() {
	quote := s.peek(0)
	s.advance(1)
	for !s.done() {
		switch {
		case s.peek(0) == '\\':
			s.advance(2)
		case quote == '`' && s.at("${"):
			s.advance(2)
			s.skipBalancedBraces()
		case s.peek(0) == quote:
			s.advance(1)
			return
		default:
			s.advance(1)
		}
	}
}

// skipBalancedBraces consumes up to and including the brace that closes the one
// already consumed, stepping over strings so that a brace inside a string does
// not unbalance the count.
func (s *scanner) skipBalancedBraces() {
	depth := 1
	for !s.done() {
		switch {
		case s.peek(0) == '"' || s.peek(0) == '\'' || s.peek(0) == '`':
			s.skipStringLiteral()
		case s.peek(0) == '{':
			depth++
			s.advance(1)
		case s.peek(0) == '}':
			depth--
			s.advance(1)
			if depth == 0 {
				return
			}
		default:
			s.advance(1)
		}
	}
}

func isNameStart(character byte) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character == '_' || character == '$'
}

func isNameCharacter(character byte) bool {
	return isNameStart(character) ||
		character >= '0' && character <= '9' ||
		character == '-' || character == ':' || character == '.' || character == '@'
}

// Vue, Svelte, and Astro all prefix attributes with punctuation that binds them
// to an expression: :class, @click, #slot. The prefix has to be part of the name,
// because an attribute called ":class" is a binding whose value is code, and
// reading it as the plain "class" attribute reports the expression as utilities.
func isAttributeNameStart(character byte) bool {
	return isNameStart(character) || character == ':' || character == '@' || character == '#'
}

// startsTag reports whether the cursor is on an opening tag. A closing tag is
// consumed here too, since it carries no attributes and skipping it keeps the
// caller's loop simple.
func (s *scanner) startsTag() bool {
	if s.peek(0) != '<' {
		return false
	}
	if s.peek(1) == '/' {
		s.advanceThrough(">")
		return false
	}
	return isNameStart(s.peek(1))
}

func (s *scanner) scanTag() {
	s.advance(1)
	for !s.done() && isNameCharacter(s.peek(0)) {
		s.advance(1)
	}

	for !s.done() {
		s.skipAttributeWhitespace()
		switch {
		case s.done():
			return
		case s.peek(0) == '>':
			s.advance(1)
			return
		case s.at("/>"):
			s.advance(2)
			return
		case s.peek(0) == '{':
			// A spread such as {...props}, which names no attribute.
			s.advance(1)
			s.skipBalancedBraces()
			continue
		case !isAttributeNameStart(s.peek(0)):
			s.advance(1)
			continue
		}
		s.scanAttribute()
	}
}

func (s *scanner) skipAttributeWhitespace() {
	for !s.done() {
		switch s.peek(0) {
		case ' ', '\t', '\n', '\r':
			s.advance(1)
		default:
			return
		}
	}
}

func (s *scanner) scanAttribute() {
	nameLine, nameColumn := s.line, s.column
	start := s.offset
	s.advance(1)
	for !s.done() && isNameCharacter(s.peek(0)) {
		s.advance(1)
	}
	name := s.source[start:s.offset]

	s.skipAttributeWhitespace()
	if s.peek(0) != '=' {
		s.recordValuelessAttribute(name, nameLine, nameColumn)
		return
	}
	s.advance(1)
	s.skipAttributeWhitespace()

	switch {
	case s.peek(0) == '"' || s.peek(0) == '\'':
		quote := s.peek(0)
		s.advance(1)
		valueLine, valueColumn := s.line, s.column
		start := s.offset
		for !s.done() && s.peek(0) != quote {
			s.advance(1)
		}
		value := s.source[start:s.offset]
		s.advance(1)
		if s.recordDirective(name, nameLine, nameColumn, shapeSvelteClassDirective) {
			return
		}
		s.recordQuotedAttribute(name, value, valueLine, valueColumn)
	case s.peek(0) == '{':
		s.advance(1)
		valueLine, valueColumn := s.line, s.column
		start := s.offset
		s.skipBalancedBraces()
		expression := strings.TrimSpace(s.source[start:max(s.offset-1, start)])
		if s.recordDirective(name, nameLine, nameColumn, shapeSvelteClassDirective) {
			return
		}
		s.recordExpressionAttribute(name, expression, valueLine, valueColumn)
	default:
		start := s.offset
		valueLine, valueColumn := s.line, s.column
		for !s.done() && !isAttributeTerminator(s.peek(0)) {
			s.advance(1)
		}
		s.recordQuotedAttribute(name, s.source[start:s.offset], valueLine, valueColumn)
	}
}

func isAttributeTerminator(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '>', '/':
		return true
	}
	return false
}

func isClassAttribute(name string) bool {
	return name == "class" || name == "className"
}

// A Svelte class: directive names the class in the attribute itself, so the
// class is knowable even when the condition controlling it is not. Astro's
// class:list shares the prefix but is an expression, and is not this shape.
func (s *scanner) svelteDirectiveClass(name string) (string, bool) {
	if s.extension != ".svelte" || !strings.HasPrefix(name, "class:") {
		return "", false
	}
	class := strings.TrimPrefix(name, "class:")
	if class == "" {
		return "", false
	}
	return class, true
}

// recordDirective reports a Svelte class: directive. The class is named in the
// attribute itself, so it is recorded at the name's position rather than the
// value's; the value only decides whether the class is applied, which is a
// runtime question this tool does not answer.
func (s *scanner) recordDirective(name string, line, column int, shape string) bool {
	class, ok := s.svelteDirectiveClass(name)
	if !ok {
		return false
	}
	s.record(ClassList{
		Value: class, Line: line, Column: column + len("class:"),
		Shape: shape, Resolved: true,
	})
	return true
}

func (s *scanner) recordValuelessAttribute(name string, line, column int) {
	s.recordDirective(name, line, column, shapeSvelteClassShorthand)
}

func (s *scanner) recordQuotedAttribute(name, value string, line, column int) {
	if !isClassAttribute(name) {
		return
	}
	if !strings.Contains(value, "{") {
		if strings.TrimSpace(value) == "" {
			return
		}
		s.record(ClassList{
			Value: value, Line: line, Column: column, Shape: shapeAttributeLiteral, Resolved: true,
		})
		return
	}
	s.recordInterpolatedValue(value, line, column)
}

func (s *scanner) recordExpressionAttribute(name, expression string, line, column int) {
	if !isClassAttribute(name) || expression == "" {
		return
	}
	// Helper calls and template literals are read by a later pass; until then the
	// honest answer for any expression is that its value is not known here.
	s.record(ClassList{
		Value: expression, Line: line, Column: column, Shape: shapeAttributeInterpolated, Resolved: false,
	})
}

// recordInterpolatedValue splits a quoted attribute that mixes literal classes
// with interpolations. The literal part is real Tailwind and must be linted; each
// interpolation is recorded as its own unresolved site rather than being folded
// into the class list, which is how a substitution ends up reported as a utility.
func (s *scanner) recordInterpolatedValue(value string, line, column int) {
	position := newPositionTracker(line, column)
	literal := strings.Builder{}
	literalLine, literalColumn, haveLiteral := 0, 0, false

	for index := 0; index < len(value); {
		if value[index] != '{' {
			if !haveLiteral && value[index] != ' ' && value[index] != '\t' && value[index] != '\n' {
				literalLine, literalColumn = position.line, position.column
				haveLiteral = true
			}
			literal.WriteByte(value[index])
			position.advance(value[index])
			index++
			continue
		}

		segmentLine, segmentColumn := position.line, position.column
		depth, end := 0, index
		for end < len(value) {
			if value[end] == '{' {
				depth++
			}
			if value[end] == '}' {
				depth--
				if depth == 0 {
					end++
					break
				}
			}
			end++
		}
		s.record(ClassList{
			Value: value[index:end], Line: segmentLine, Column: segmentColumn,
			Shape: shapeAttributeInterpolated, Resolved: false,
		})
		for ; index < end; index++ {
			position.advance(value[index])
		}
		literal.WriteByte(' ')
	}

	classes := strings.Join(strings.Fields(literal.String()), " ")
	if classes == "" {
		return
	}
	s.record(ClassList{
		Value: classes, Line: literalLine, Column: literalColumn,
		Shape: shapeAttributeInterpolated, Resolved: true,
	})
}

type positionTracker struct {
	line   int
	column int
}

func newPositionTracker(line, column int) *positionTracker {
	return &positionTracker{line: line, column: column}
}

func (p *positionTracker) advance(character byte) {
	if character == '\n' {
		p.line++
		p.column = 1
		return
	}
	p.column++
}

func (s *scanner) record(list ClassList) {
	s.found = append(s.found, list)
}
