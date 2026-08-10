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
	shapeVueBindClass          = "vue-bind-class"
	shapeAstroClassList        = "astro-class-list"
	shapeJSXTemplate           = "jsx-template"
	shapeClsx                  = "clsx"
	shapeCn                    = "cn"
	shapeCvaLeaf               = "cva-leaf"
	shapeCSSApply              = "css-apply"
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
	// Verbatim records whether Value is a contiguous span of the source file.
	// A list rebuilt from the literal parts of an interpolated value is not, so
	// its utilities cannot be given individual positions.
	Verbatim bool
}

// Extract reads every class list out of one source file. It is deterministic and
// depends on nothing but the path's extension and the content.
func Extract(path, content string) []ClassList {
	current := &scanner{
		source:        content,
		extension:     strings.ToLower(filepath.Ext(path)),
		moduleHelpers: discoverModuleHelpers(content),
		line:          1,
		column:        1,
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
	// moduleHelpers maps a local import binding to the helper it represents.
	// Module-level calls are considered class lists only with this evidence;
	// calls already inside class/className expressions remain contextual.
	moduleHelpers map[string]string
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
	if s.extension == ".css" {
		s.scanStyleSheet(len(s.source))
		return
	}
	s.scanAstroFrontmatter()
	for !s.done() {
		if s.usesJavaScriptSyntax() {
			if s.skipJavaScriptNoise() {
				continue
			}
			if isNameStart(s.peek(0)) {
				s.scanPossibleHelperCall(len(s.source))
				continue
			}
		} else {
			if s.at("<!--") {
				s.advanceThrough("-->")
				continue
			}
			if s.scanRawTextElement() {
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
// no markup but may define class lists through a helper, so it is read rather
// than skipped.
func (s *scanner) scanAstroFrontmatter() {
	if s.extension != ".astro" || !s.at("---") {
		return
	}
	s.advance(3)
	closing := strings.Index(s.source[s.offset:], "\n---")
	if closing < 0 {
		return
	}
	s.scanJavaScriptRegion(s.offset + closing)
	s.advanceThrough("\n---")
}

// A <script> body is JavaScript and may build class lists with a helper, so it
// is scanned as an expression region. A <style> body is CSS: in a component file
// it can hold @apply, so it is scanned as a style sheet. Neither is markup, so
// neither is searched for tags — a template held in a script is inert.
func (s *scanner) scanRawTextElement() bool {
	for _, name := range []string{"script", "style"} {
		if !s.atTagNamed(name) {
			continue
		}
		s.advance(1 + len(name))
		s.advanceThrough(">")
		end := strings.Index(s.source[s.offset:], "</"+name)
		if end < 0 {
			end = len(s.source) - s.offset
		}
		if name == "script" {
			s.scanJavaScriptRegion(s.offset + end)
		} else {
			s.scanStyleSheet(s.offset + end)
		}
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
	case isStringQuote(s.peek(0)):
		s.advanceTo(skipLiteral(s.source, s.offset))
		return true
	}
	return false
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
			closing := matchingDelimiter(s.source, s.offset, '{', '}')
			if closing < 0 {
				s.advance(1)
				continue
			}
			s.advanceTo(closing + 1)
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

// attributeRole says how an attribute's value should be read.
type attributeRole int

const (
	roleIgnored attributeRole = iota
	roleLiteralClasses
	roleExpression
	roleSvelteDirective
)

// classifyAttribute decides what an attribute name means. The shape it returns
// names the binding syntax where one applies, so that classes found inside a
// :class or a class:list are credited to that binding rather than to whatever
// helper happens to be nested in it.
func (s *scanner) classifyAttribute(name string) (attributeRole, string) {
	switch {
	case name == "class" || name == "className":
		return roleLiteralClasses, ""
	case name == ":class" || name == "v-bind:class":
		return roleExpression, shapeVueBindClass
	case name == "class:list" && s.extension == ".astro":
		return roleExpression, shapeAstroClassList
	case s.extension == ".svelte" && strings.HasPrefix(name, "class:") && name != "class:":
		return roleSvelteDirective, shapeSvelteClassDirective
	}
	return roleIgnored, ""
}

func (s *scanner) scanAttribute() {
	nameLine, nameColumn := s.line, s.column
	start := s.offset
	s.advance(1)
	for !s.done() && isNameCharacter(s.peek(0)) {
		s.advance(1)
	}
	name := s.source[start:s.offset]
	role, shape := s.classifyAttribute(name)

	s.skipAttributeWhitespace()
	if s.peek(0) != '=' {
		if role == roleSvelteDirective {
			s.recordDirective(name, nameLine, nameColumn, shapeSvelteClassShorthand)
		}
		return
	}
	s.advance(1)
	s.skipAttributeWhitespace()

	// A directive names its class in the attribute itself, so it is recorded at
	// the name's position; the value only decides whether the class applies,
	// which is a runtime question this tool does not answer.
	if role == roleSvelteDirective {
		s.recordDirective(name, nameLine, nameColumn, shapeSvelteClassDirective)
		s.skipAttributeValue()
		return
	}

	switch {
	case isStringQuote(s.peek(0)) && s.peek(0) != '`':
		quote := s.peek(0)
		s.advance(1)
		valueLine, valueColumn := s.line, s.column
		contentStart := s.offset
		end := strings.IndexByte(s.source[contentStart:], quote)
		if end < 0 {
			s.advanceTo(len(s.source))
			return
		}
		end += contentStart
		switch role {
		case roleLiteralClasses:
			s.recordQuotedClassValue(s.source[contentStart:end], valueLine, valueColumn)
		case roleExpression:
			s.scanClassExpression(end, shape)
		}
		s.advanceTo(end)
		s.advance(1)
	case s.peek(0) == '{':
		closing := matchingDelimiter(s.source, s.offset, '{', '}')
		if closing < 0 {
			s.advance(1)
			return
		}
		if role != roleIgnored {
			s.advance(1)
			s.scanClassExpression(closing, shape)
		}
		s.advanceTo(closing + 1)
	default:
		start := s.offset
		valueLine, valueColumn := s.line, s.column
		for !s.done() && !isAttributeTerminator(s.peek(0)) {
			s.advance(1)
		}
		if role == roleLiteralClasses {
			s.recordQuotedClassValue(s.source[start:s.offset], valueLine, valueColumn)
		}
	}
}

func (s *scanner) skipAttributeValue() {
	switch {
	case isStringQuote(s.peek(0)):
		s.advanceTo(skipLiteral(s.source, s.offset))
	case s.peek(0) == '{':
		closing := matchingDelimiter(s.source, s.offset, '{', '}')
		if closing < 0 {
			s.advance(1)
			return
		}
		s.advanceTo(closing + 1)
	default:
		for !s.done() && !isAttributeTerminator(s.peek(0)) {
			s.advance(1)
		}
	}
}

func isAttributeTerminator(character byte) bool {
	switch character {
	case ' ', '\t', '\n', '\r', '>', '/':
		return true
	}
	return false
}

func (s *scanner) recordDirective(name string, line, column int, shape string) {
	class := strings.TrimPrefix(name, "class:")
	if class == "" {
		return
	}
	s.record(ClassList{
		Value: class, Line: line, Column: column + len("class:"),
		Shape: shape, Resolved: true, Verbatim: true,
	})
}

func (s *scanner) recordQuotedClassValue(value string, line, column int) {
	if !strings.Contains(value, "{") {
		if strings.TrimSpace(value) == "" {
			return
		}
		s.record(ClassList{
			Value: value, Line: line, Column: column,
			Shape: shapeAttributeLiteral, Resolved: true, Verbatim: true,
		})
		return
	}
	s.recordInterpolated(value, line, column, shapeAttributeInterpolated, "{")
}

// recordInterpolated splits a value that mixes literal classes with
// substitutions. The literal part is real Tailwind and must be linted; each
// substitution is recorded as its own unresolved site rather than being folded
// into the class list, which is how a substitution ends up reported as a utility.
// opening is "{" for a markup attribute and "${" for a template literal.
func (s *scanner) recordInterpolated(value string, line, column int, shape, opening string) {
	position := &positionTracker{line: line, column: column}
	literal := strings.Builder{}
	literalLine, literalColumn, haveLiteral := 0, 0, false

	for index := 0; index < len(value); {
		if !strings.HasPrefix(value[index:], opening) {
			if !haveLiteral && !isSpace(value[index]) {
				literalLine, literalColumn = position.line, position.column
				haveLiteral = true
			}
			literal.WriteByte(value[index])
			position.advance(value[index])
			index++
			continue
		}

		segmentLine, segmentColumn := position.line, position.column
		braceStart := index + len(opening) - 1
		end := matchingDelimiter(value, braceStart, '{', '}')
		if end < 0 {
			end = len(value) - 1
		}
		end++
		s.record(ClassList{
			Value: value[index:end], Line: segmentLine, Column: segmentColumn,
			Shape: shape, Resolved: false,
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
		Value: classes, Line: literalLine, Column: literalColumn, Shape: shape, Resolved: true,
	})
}

func isSpace(character byte) bool {
	return character == ' ' || character == '\t' || character == '\n' || character == '\r'
}

type positionTracker struct {
	line   int
	column int
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
