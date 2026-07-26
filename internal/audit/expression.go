package audit

import "strings"

// Class lists reach an element through more than a literal attribute. They are
// built by helpers (clsx, cn, cva), bound through framework syntax (:class,
// class:list), and spliced together in template literals. This file reads those
// expressions far enough to recover the strings a developer actually wrote.
//
// It is not an evaluator. Every value that is not a literal in the source is
// recorded as an unresolved site and never guessed at, which is what keeps the
// false-positive rate where the project's policy requires it.

// Helpers whose string arguments are class lists. The value is the shape used
// for anything found inside, unless a framework binding already named one.
var classHelpers = map[string]string{
	"clsx":       shapeClsx,
	"cx":         shapeClsx,
	"classnames": shapeClsx,
	"classNames": shapeClsx,
	"cn":         shapeCn,
}

const cvaHelper = "cva"

// matchingDelimiter returns the offset of the delimiter closing the one at
// start, or -1. Strings and comments inside are stepped over so that a brace in
// a string cannot unbalance the count.
func matchingDelimiter(source string, start int, open, close byte) int {
	depth := 0
	for index := start; index < len(source); {
		switch character := source[index]; {
		case character == '\'' || character == '"' || character == '`':
			index = skipLiteral(source, index)
		case strings.HasPrefix(source[index:], "//"):
			index = skipUntil(source, index, "\n")
		case strings.HasPrefix(source[index:], "/*"):
			index = skipUntil(source, index+2, "*/")
		case character == open:
			depth++
			index++
		case character == close:
			depth--
			index++
			if depth == 0 {
				return index - 1
			}
		default:
			index++
		}
	}
	return -1
}

func skipUntil(source string, index int, marker string) int {
	found := strings.Index(source[index:], marker)
	if found < 0 {
		return len(source)
	}
	return index + found + len(marker)
}

// skipLiteral returns the offset just past the string starting at index.
func skipLiteral(source string, index int) int {
	quote := source[index]
	index++
	for index < len(source) {
		switch {
		case source[index] == '\\':
			index += 2
		case quote == '`' && strings.HasPrefix(source[index:], "${"):
			closing := matchingDelimiter(source, index+1, '{', '}')
			if closing < 0 {
				return len(source)
			}
			index = closing + 1
		case source[index] == quote:
			return index + 1
		default:
			index++
		}
	}
	return len(source)
}

func (s *scanner) advanceTo(offset int) {
	if offset > s.offset {
		s.advance(offset - s.offset)
	}
}

// skipExpressionNoise steps over whitespace and comments, which may appear
// between any two meaningful tokens. Headless UI's clsx call, where every array
// element is introduced by a line comment, is the case that makes this matter.
func (s *scanner) skipExpressionNoise(end int) {
	for s.offset < end {
		switch {
		case s.peek(0) == ' ' || s.peek(0) == '\t' || s.peek(0) == '\n' || s.peek(0) == '\r':
			s.advance(1)
		case s.at("//"):
			s.advanceTo(min(skipUntil(s.source, s.offset, "\n"), end))
		case s.at("/*"):
			s.advanceTo(min(skipUntil(s.source, s.offset+2, "*/"), end))
		default:
			return
		}
	}
}

func isStringQuote(character byte) bool {
	return character == '\'' || character == '"' || character == '`'
}

// readStringLiteral consumes the literal under the cursor and reports its
// content along with the position of the content's first byte.
func (s *scanner) readStringLiteral() (string, int, int) {
	quote := s.peek(0)
	s.advance(1)
	line, column := s.line, s.column
	start := s.offset
	closing := skipLiteral(s.source, start-1)
	content := s.source[start:max(closing-1, start)]
	s.advanceTo(closing)
	_ = quote
	return content, line, column
}

// scanClassExpression walks an expression that yields class lists. Strings are
// classes; object keys are classes and their values are conditions, which is the
// contract clsx, Vue's object syntax, and Astro's class:list all share. Anything
// that is not a literal becomes an unresolved site.
//
// shape names where the classes came from. An empty shape means the expression
// itself decides, which is how className={clsx(...)} is credited to clsx while
// :class="cn(...)" stays a Vue binding.
func (s *scanner) scanClassExpression(end int, shape string) {
	for s.offset < end {
		s.skipExpressionNoise(end)
		if s.offset >= end {
			return
		}

		switch {
		case isStringQuote(s.peek(0)):
			s.readClassStringLiteral(shape, shapeOrDefault(shape, shapeAttributeInterpolated))
		case s.peek(0) == '{':
			s.scanConditionObject(end, shape)
		case s.peek(0) == '[':
			closing := matchingDelimiter(s.source, s.offset, '[', ']')
			if closing < 0 || closing > end {
				s.advance(1)
				continue
			}
			s.advance(1)
			s.scanClassExpression(closing, shape)
			s.advanceTo(closing + 1)
		case isNameStart(s.peek(0)):
			s.scanIdentifierExpression(end, shape)
		default:
			s.advance(1)
		}
	}
}

func shapeOrDefault(shape, fallback string) string {
	if shape == "" {
		return fallback
	}
	return shape
}

// readClassStringLiteral records one string literal as a class list, splitting a
// template literal that contains substitutions into its knowable and unknowable
// halves.
func (s *scanner) readClassStringLiteral(shape, fallback string) {
	isTemplate := s.peek(0) == '`'
	content, line, column := s.readStringLiteral()
	if strings.TrimSpace(content) == "" {
		return
	}
	if isTemplate && strings.Contains(content, "${") {
		s.recordInterpolated(content, line, column, shapeOrDefault(shape, shapeJSXTemplate), "${")
		return
	}
	s.record(ClassList{
		Value: content, Line: line, Column: column,
		Shape: shapeOrDefault(shape, fallback), Resolved: true,
	})
}

// scanConditionObject reads an object whose keys are class names and whose
// values decide at runtime whether they apply. The condition is deliberately not
// examined: which branch wins is exactly the question this tool refuses to guess.
func (s *scanner) scanConditionObject(end int, shape string) {
	closing := matchingDelimiter(s.source, s.offset, '{', '}')
	if closing < 0 || closing > end {
		s.advance(1)
		return
	}
	s.advance(1)
	for s.offset < closing {
		s.skipExpressionNoise(closing)
		if s.offset >= closing {
			break
		}
		switch {
		case isStringQuote(s.peek(0)):
			content, line, column := s.readStringLiteral()
			if strings.TrimSpace(content) != "" {
				s.record(ClassList{
					Value: content, Line: line, Column: column,
					Shape: shapeOrDefault(shape, shapeClsx), Resolved: true,
				})
			}
		case isNameStart(s.peek(0)):
			line, column := s.line, s.column
			name := s.readIdentifier()
			s.skipExpressionNoise(closing)
			if s.peek(0) == ':' && name != "" {
				s.record(ClassList{
					Value: name, Line: line, Column: column,
					Shape: shapeOrDefault(shape, shapeClsx), Resolved: true,
				})
			}
		default:
			s.advance(1)
			continue
		}
		s.skipExpressionNoise(closing)
		if s.peek(0) == ':' {
			s.advance(1)
			s.skipObjectValue(closing)
		}
	}
	s.advanceTo(closing + 1)
}

// skipObjectValue steps over a value without reading it, stopping at the comma
// that ends it or the brace that ends the object.
func (s *scanner) skipObjectValue(end int) {
	for s.offset < end {
		switch {
		case isStringQuote(s.peek(0)):
			s.advanceTo(skipLiteral(s.source, s.offset))
		case s.peek(0) == '{' || s.peek(0) == '[' || s.peek(0) == '(':
			open := s.peek(0)
			closer := map[byte]byte{'{': '}', '[': ']', '(': ')'}[open]
			closing := matchingDelimiter(s.source, s.offset, open, closer)
			if closing < 0 || closing > end {
				s.advanceTo(end)
				return
			}
			s.advanceTo(closing + 1)
		case s.peek(0) == ',':
			s.advance(1)
			return
		default:
			s.advance(1)
		}
	}
}

// An attribute name may contain a colon or a hyphen — class:list, aria-label —
// but a JavaScript name may not. Reading an object key with the attribute rules
// swallows the colon that terminates it, so the key never resolves. A member
// path keeps its dots, so that an unresolved site reads as props.class rather
// than props.
func isExpressionNameCharacter(character byte) bool {
	return isNameStart(character) ||
		character >= '0' && character <= '9' ||
		character == '.'
}

func (s *scanner) readIdentifier() string {
	start := s.offset
	for !s.done() && isExpressionNameCharacter(s.peek(0)) {
		s.advance(1)
	}
	return s.source[start:s.offset]
}

// scanIdentifierExpression handles a name in value position. A call to a known
// class helper is followed into; anything else is a value this tool cannot read,
// and is recorded as such along with its source text so a user can see what was
// skipped.
func (s *scanner) scanIdentifierExpression(end int, shape string) {
	line, column := s.line, s.column
	start := s.offset
	name := s.readIdentifier()
	s.skipExpressionNoise(end)

	if s.peek(0) != '(' {
		if name != "" {
			s.record(ClassList{
				Value: name, Line: line, Column: column,
				Shape: shapeOrDefault(shape, shapeAttributeInterpolated), Resolved: false,
			})
		}
		return
	}

	closing := matchingDelimiter(s.source, s.offset, '(', ')')
	if closing < 0 || closing > end {
		s.advance(1)
		return
	}
	if helper, ok := classHelpers[name]; ok {
		s.advance(1)
		s.scanClassExpression(closing, shapeOrDefault(shape, helper))
		s.advanceTo(closing + 1)
		return
	}
	if name == cvaHelper {
		s.advance(1)
		s.scanCvaArguments(closing)
		s.advanceTo(closing + 1)
		return
	}

	s.record(ClassList{
		Value: strings.TrimSpace(s.source[start : closing+1]), Line: line, Column: column,
		Shape: shapeOrDefault(shape, shapeAttributeInterpolated), Resolved: false,
	})
	s.advanceTo(closing + 1)
}

// scanCvaArguments reads a cva call: a base class list, then a configuration
// object. Only the base and the leaves of `variants` and `compoundVariants` are
// class lists. The variant matrix is never expanded — each leaf stands alone,
// and combining them is a rules question, not an extraction one.
func (s *scanner) scanCvaArguments(end int) {
	s.skipExpressionNoise(end)
	if s.offset < end && isStringQuote(s.peek(0)) {
		s.readClassStringLiteral(shapeCvaLeaf, shapeCvaLeaf)
	}
	for s.offset < end {
		s.skipExpressionNoise(end)
		if s.offset >= end {
			return
		}
		if s.peek(0) == '{' {
			closing := matchingDelimiter(s.source, s.offset, '{', '}')
			if closing < 0 || closing > end {
				return
			}
			s.scanCvaConfiguration(closing)
			s.advanceTo(closing + 1)
			continue
		}
		s.advance(1)
	}
}

func (s *scanner) scanCvaConfiguration(end int) {
	s.advance(1)
	for s.offset < end {
		s.skipExpressionNoise(end)
		if s.offset >= end || s.peek(0) == '}' {
			break
		}
		key := s.readObjectKey(end)
		if key == "" {
			s.advance(1)
			continue
		}
		switch key {
		case "variants":
			s.scanCvaVariants(end)
		case "compoundVariants":
			s.scanCompoundVariants(end)
		default:
			// defaultVariants maps a variant name to a variant key, so its values
			// name variants rather than listing classes. Reading them as class
			// lists would report "default" as a utility.
			s.skipObjectValue(end)
		}
	}
}

// readObjectKey consumes `name:` or `"name":` and reports the name.
func (s *scanner) readObjectKey(end int) string {
	s.skipExpressionNoise(end)
	var name string
	switch {
	case isStringQuote(s.peek(0)):
		name, _, _ = s.readStringLiteral()
	case isNameStart(s.peek(0)):
		name = s.readIdentifier()
	default:
		return ""
	}
	s.skipExpressionNoise(end)
	if s.peek(0) != ':' {
		return ""
	}
	s.advance(1)
	return name
}

// scanCvaVariants records every string in value position inside the variants
// map. A string followed by a colon is a variant key, not a class list.
func (s *scanner) scanCvaVariants(end int) {
	s.skipExpressionNoise(end)
	if s.peek(0) != '{' {
		s.skipObjectValue(end)
		return
	}
	closing := matchingDelimiter(s.source, s.offset, '{', '}')
	if closing < 0 || closing > end {
		s.skipObjectValue(end)
		return
	}
	for s.offset < closing {
		s.skipExpressionNoise(closing)
		if s.offset >= closing {
			break
		}
		if !isStringQuote(s.peek(0)) {
			s.advance(1)
			continue
		}
		content, line, column := s.readStringLiteral()
		s.skipExpressionNoise(closing)
		if s.peek(0) == ':' {
			continue
		}
		if strings.TrimSpace(content) != "" {
			s.record(ClassList{
				Value: content, Line: line, Column: column,
				Shape: shapeCvaLeaf, Resolved: true,
			})
		}
	}
	s.advanceTo(closing + 1)
}

// scanCompoundVariants records only the class or className entry of each
// compound rule. Its other keys name variants that select the rule.
func (s *scanner) scanCompoundVariants(end int) {
	s.skipExpressionNoise(end)
	if s.peek(0) != '[' {
		s.skipObjectValue(end)
		return
	}
	closing := matchingDelimiter(s.source, s.offset, '[', ']')
	if closing < 0 || closing > end {
		s.skipObjectValue(end)
		return
	}
	s.advance(1)
	for s.offset < closing {
		s.skipExpressionNoise(closing)
		if s.offset >= closing {
			break
		}
		key := s.readObjectKey(closing)
		if key == "" {
			s.advance(1)
			continue
		}
		if key == "class" || key == "className" {
			s.skipExpressionNoise(closing)
			if isStringQuote(s.peek(0)) {
				s.readClassStringLiteral(shapeCvaLeaf, shapeCvaLeaf)
				continue
			}
		}
		s.skipObjectValue(closing)
	}
	s.advanceTo(closing + 1)
}

// scanJavaScriptRegion reads a span of JavaScript for helper calls. Class lists
// are frequently defined away from the markup that uses them — a cva call at
// module scope, a clsx call in a component's script block — so the region has to
// be read rather than stepped over.
func (s *scanner) scanJavaScriptRegion(end int) {
	for s.offset < end {
		switch {
		case s.at("//"):
			s.advanceTo(min(skipUntil(s.source, s.offset, "\n"), end))
		case s.at("/*"):
			s.advanceTo(min(skipUntil(s.source, s.offset+2, "*/"), end))
		case isStringQuote(s.peek(0)):
			s.advanceTo(min(skipLiteral(s.source, s.offset), end))
		case isNameStart(s.peek(0)):
			s.scanPossibleHelperCall(end)
		default:
			s.advance(1)
		}
	}
}

func (s *scanner) scanPossibleHelperCall(end int) {
	name := s.readIdentifier()
	_, isHelper := classHelpers[name]
	if !isHelper && name != cvaHelper {
		return
	}
	s.skipExpressionNoise(end)
	if s.peek(0) != '(' {
		return
	}
	closing := matchingDelimiter(s.source, s.offset, '(', ')')
	if closing < 0 || closing > end {
		return
	}
	s.advance(1)
	if name == cvaHelper {
		s.scanCvaArguments(closing)
	} else {
		s.scanClassExpression(closing, classHelpers[name])
	}
	s.advanceTo(closing + 1)
}
