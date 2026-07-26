package audit

import "strings"

// A project's utilities do not all live in markup. An @apply rule is a class list
// by another name, and the rules that catch a conflict or an arbitrary value in a
// class attribute catch the same mistakes here. Reading it needs no CSS parser:
// the construct is a keyword, a class list, and a semicolon.
//
// Whether an @apply list names tokens that exist in the project's theme is a
// different question, answered once there is a token inventory to check against.

// scanStyleSheet reads @apply rules between the cursor and end. It is used for a
// .css file and for the <style> block of a component file alike.
func (s *scanner) scanStyleSheet(end int) {
	for s.offset < end {
		switch {
		case s.at("/*"):
			s.advanceTo(min(skipUntil(s.source, s.offset+2, "*/"), end))
		case s.peek(0) == '"' || s.peek(0) == '\'':
			s.advanceTo(min(skipLiteral(s.source, s.offset), end))
		case s.at("@apply"):
			s.scanApplyRule(end)
		default:
			s.advance(1)
		}
	}
}

func (s *scanner) scanApplyRule(end int) {
	s.advance(len("@apply"))
	for s.offset < end && isSpace(s.peek(0)) {
		s.advance(1)
	}

	line, column := s.line, s.column
	start := s.offset
	for s.offset < end && s.peek(0) != ';' && s.peek(0) != '}' {
		s.advance(1)
	}

	value := strings.TrimSpace(s.source[start:s.offset])
	// "@apply p-4 !important" applies the utilities with a raised priority; the
	// keyword is not one of them.
	value = strings.TrimSpace(strings.TrimSuffix(value, "!important"))
	if value == "" {
		return
	}
	s.record(ClassList{
		Value: value, Line: line, Column: column,
		Shape: shapeCSSApply, Resolved: true, Verbatim: true,
	})
}
