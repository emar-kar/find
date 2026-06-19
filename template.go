package find

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

var (
	ErrTemplateType     = errors.New("cannot define type of the template")
	ErrMalformedPattern = errors.New("malformed pattern")
)

// op identifies the kind of a Template node.
type op int8

const (
	// A base pattern with optional wildcards.
	opLeaf op = iota
	// NOT: negates its left operand.
	opNot
	// AND: both left and right must match.
	opAnd
	// OR: either left or right must match.
	opOr
)

// Template is a parsed pattern tree used for matching paths.
type Template struct {
	op          op
	left, right *Template
	base        string
	strictLeft  bool
	strictRight bool
}

// findOuterSep returns the index of the first occurrence of sep at
// parenthesis depth 0, or -1 if none is found.
func findOuterSep(str string, sep byte) int {
	depth := 0

	for i := 0; i < len(str); i++ {
		switch str[i] {
		case '(':
			depth++
		case ')':
			depth--
		case sep:
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// isWrappedInParens reports whether str is entirely wrapped in a matching
// pair of outer parentheses, e.g. "(a|b)" but not "(a)(b)".
func isWrappedInParens(str string) bool {
	if len(str) < 2 || str[0] != '(' || str[len(str)-1] != ')' {
		return false
	}

	// Verify the opening '(' at position 0 is not closed before the last ')'.
	depth := 0

	for i := 0; i < len(str)-1; i++ {
		switch str[i] {
		case '(':
			depth++
		case ')':
			depth--
		}

		if depth == 0 {
			return false // Outer paren closed before end of string.
		}
	}

	return true
}

// buildTemplate parses str into a [Template], returning an error if any
// segment is empty (malformed pattern). Splits on '|' first (lower
// precedence), then delegates to [buildTemplateAnd] for '&'.
func buildTemplate(str string) (*Template, error) {
	if i := findOuterSep(str, '|'); i >= 0 {
		left, err := buildTemplateAnd(str[:i])
		if err != nil {
			return left, err
		}

		right, err := buildTemplate(str[i+1:])
		if err != nil {
			return right, err
		}

		return &Template{op: opOr, left: left, right: right}, nil
	}

	return buildTemplateAnd(str)
}

// buildTemplateAnd handles '&' splitting (higher precedence than '|').
func buildTemplateAnd(str string) (*Template, error) {
	if i := findOuterSep(str, '&'); i >= 0 {
		left, err := buildParse(str[:i])
		if err != nil {
			return left, err
		}

		right, err := buildTemplateAnd(str[i+1:])
		if err != nil {
			return right, err
		}

		return &Template{op: opAnd, left: left, right: right}, nil
	}

	return buildParse(str)
}

// buildParse handles a leading '!' and parenthesised groups before delegating
// to [buildParseLiteral] for plain leaf patterns.
func buildParse(str string) (*Template, error) {
	not := false

	if len(str) > 0 && str[0] == '!' {
		not = true
		str = str[1:]
	}

	if isWrappedInParens(str) {
		inner, err := buildTemplate(str[1 : len(str)-1])
		if err != nil {
			return inner, err
		}

		if not {
			return &Template{op: opNot, left: inner}, nil
		}

		return inner, nil
	}

	leaf, err := buildParseLiteral(str)
	if err != nil {
		return leaf, err
	}

	if not {
		return &Template{op: opNot, left: leaf}, nil
	}

	return leaf, nil
}

// buildParseLiteral parses a single leaf pattern (wildcards and base string).
// Returns [ErrMalformedPattern] if the resulting base is empty (empty segment).
func buildParseLiteral(str string) (*Template, error) {
	t := &Template{op: opLeaf}

	// A lone '*' matches any path.
	if str == "*" {
		t.base = str
		return t, nil
	}

	if len(str) > 0 && str[0] == '*' {
		str = str[1:]
	} else {
		t.strictLeft = true
	}

	n := len(str)
	if n > 0 && str[n-1] == '*' {
		t.base = str[:n-1]
	} else {
		t.strictRight = true
		t.base = str
	}

	// "**" (or similar) produces an empty base after stripping wildcards.
	// Promote to the universal wildcard "*" so it matches everything.
	if t.base == "" && (!t.strictLeft || !t.strictRight) {
		t.base = "*"
		t.strictLeft = false
		t.strictRight = false
	}

	// Genuinely empty segment — e.g. from "a|" or "|b".
	if t.base == "" {
		return t, fmt.Errorf("%w: pattern contains an empty segment", ErrMalformedPattern)
	}

	return t, nil
}

// Match checks if given str matches the [Template].
func (t *Template) Match(str string) bool {
	switch t.op {
	case opLeaf:
		return t.match(str)
	case opNot:
		return !t.left.Match(str)
	case opAnd:
		return t.left.Match(str) && t.right.Match(str)
	case opOr:
		return t.left.Match(str) || t.right.Match(str)
	default:
		return false
	}
}

func (t *Template) match(str string) bool {
	switch t.base {
	case "":
		return false // Empty segment — never matches.
	case "*":
		return true // Universal wildcard - always matches.
	}

	// Fast path: *base* pattern — any occurrence is valid, no boundary checks.
	if !t.strictLeft && !t.strictRight {
		return strings.Contains(str, t.base)
	}

	// Strict path: loop until we find an occurrence that satisfies the
	// required path-segment boundaries, or exhaust all occurrences.
	baselen := len(t.base)
	offset := 0
	s := str

	for {
		idx := strings.Index(s, t.base)
		if idx == -1 {
			return false
		}

		realIdx := offset + idx
		left := str[:realIdx]
		right := str[realIdx+baselen:]

		leftOK := left == "" || left[len(left)-1] == os.PathSeparator
		rightOK := right == "" || right[0] == os.PathSeparator

		var matchOK bool

		switch {
		case t.strictLeft && t.strictRight:
			matchOK = leftOK && rightOK
		case t.strictLeft:
			matchOK = leftOK
		default:
			matchOK = rightOK
		}

		if matchOK {
			return true
		}

		s = s[idx+baselen:]
		offset += idx + baselen
	}
}

type Templates []*Template

// MatchAny returns true if any of the given templates match str.
//
// Deprecated: usage of this functions is discouraged in favor of
// constructing one [Template] from string or []string and use Match
// on it.
func MatchAny(ts Templates, str string) bool {
	for _, t := range ts {
		if t.Match(str) {
			return true
		}
	}

	return false
}

// MatchAll returns true if all of the given templates match str.
//
// Deprecated: usage of this functions is discouraged in favor of
// constructing one [Template] from string or []string and use Match
// on it.
func MatchAll(ts Templates, str string) bool {
	for _, t := range ts {
		if !t.Match(str) {
			return false
		}
	}

	return true
}

// ParsePattern joins one or more pattern strings into a single combined
// pattern suitable for [ParseTemplate].
//
//   - strict=false (default): patterns are OR-joined ("a|b|c"), so a path
//     matching any of the given patterns is accepted.
//   - strict=true: patterns are AND-joined ("(a)&(b)&(c)"), so a path must
//     match every pattern. Each element is wrapped in parentheses to prevent
//     precedence issues with patterns that already contain '|'.
//
// Returns "*" (match everything) if no templates are provided.
// This function removes empty templates, before processing.
func ParsePattern(strict bool, templates ...string) string {
	templates = removeEmpty(templates)

	if len(templates) == 0 {
		return "*"
	}

	builder := new(strings.Builder)
	sep := '|'
	if strict {
		sep = '&'
	}

	for i, t := range templates {
		if i > 0 {
			builder.WriteRune(sep)
		}

		if strict {
			builder.WriteByte('(')
			builder.WriteString(t)
			builder.WriteByte(')')
		} else {
			builder.WriteString(t)
		}
	}

	return builder.String()
}

// CreateTemplate combines parsing pattern and creating [Template].
// Since pattern parsing is a save operation, which guarantees correct
// output string, it must not return any error from template creation
// and thus can be safe to call as is.
func CreateTemplate(strict bool, sl ...string) *Template {
	t, _ := ParseTemplate(ParsePattern(strict, sl...))
	return t
}

// removeEmpty removes empty strings from sl.
func removeEmpty(sl []string) []string {
	i := -1

	for j, s := range sl {
		if s == "" {
			i = j
			break
		}
	}

	if i == -1 {
		return sl
	}

	for j := i + 1; j < len(sl); j++ {
		if v := sl[j]; v != "" {
			sl[i] = v
			i++
		}
	}

	clear(sl[i:])

	return sl[:i]
}

// checkBalancedParens returns [ErrMalformedPattern] if str contains
// unbalanced or improperly nested parentheses.
func checkBalancedParens(str string) error {
	depth := 0

	for i := 0; i < len(str); i++ {
		switch str[i] {
		case '(':
			depth++
		case ')':
			depth--

			if depth < 0 {
				return fmt.Errorf("%w: unmatched ')' in %q", ErrMalformedPattern, str)
			}
		}
	}

	if depth != 0 {
		return fmt.Errorf("%w: unmatched '(' in %q", ErrMalformedPattern, str)
	}

	return nil
}

// ParseTemplate parses str into a [Template] and validates it.
// Returns [ErrMalformedPattern] if the pattern contains an empty segment
// (e.g. "a|", "|b", "a&" or an empty string) or unbalanced parentheses.
//
// Pattern syntax:
//
//	*str*    - path should contain str
//	str      - path should equal str (at a path-segment boundary)
//	*str     - path should end with str
//	str*     - path should start with str
//	!*str*   - path should not contain str
//	!str     - path should not equal str
//	!*str    - path should not end with str
//	!str*    - path should not start with str
//	str&str1 - both str and str1 must match (AND)
//	str|str1 - either str or str1 must match (OR)
//
// '&' has higher precedence than '|', so *go*&!*test*|*mock* is parsed as
// (*go* AND !*test*) OR *mock*, not *go* AND (!*test* OR *mock*).
// Both operators can be chained with as many elements as needed.
//
// Parentheses can be used to override the default '&' > '|' precedence:
//
//	(str|str1)&str2 - (str OR str1) AND str2
//	!(str&str1)     - NOT (str AND str1)
//
// The only all-wildcard pattern that matches everything is "*" (or "**",
// which is normalised to "*").
func ParseTemplate(str string) (*Template, error) {
	if err := checkBalancedParens(str); err != nil {
		return nil, err
	}

	return buildTemplate(str)
}
