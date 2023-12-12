package find

import (
	"os"
	"strings"
)

// String representation of the current system path separator.
var pathSeparator = string(os.PathSeparator)

// Template is a parsed version of each Find filter.
type Template struct {
	and         *Template
	or          *Template
	base        string
	not         bool
	strictLeft  bool
	strictRight bool
}

// NewTemplate creates new Template from the given string.
//
// String can contains:
//
//	*str*    - means that searched path should contain str
//	str      - means that searched path should be str
//	*str     - means that searched path should ends with str
//	str*     - means that searched path should starts with str
//	!*str*   - means that searched path should not contain str
//	!str     - means that searched path should not be str
//	!*str    - means that searched path should not end with str
//	!str*    - means that searched path should not start with str
//	str&str1 - means that searched path should be both str and str1
//	str|str1 - means that searched path should be str or str1
//
// Option '&' defines nested paths e.g., '*str*&*str1*' - Find will search
// for 'str' first and if it was found 'str1' inside it.
//
// Options '|' and '&' can contain as many elements as you need.
func NewTemplate(str string) *Template {
	var t *Template

	sep := strings.IndexFunc(str, func(r rune) bool {
		if r == '&' || r == '|' {
			return true
		}

		return false
	})

	if sep == -1 {
		return parse(str)
	}

	switch str[sep] {
	case '&':
		t = parse(str[:sep])
		t.and = NewTemplate(str[sep+1:])
	case '|':
		t = parse(str[:sep])
		t.or = NewTemplate(str[sep+1:])
	}

	return t
}

// parse parses string into the Template.
func parse(str string) *Template {
	t := &Template{}

	t.not = strings.HasPrefix(str, "!")
	str = strings.TrimPrefix(str, "!")

	// If searched string is '*', then it will match
	// any path it encounters. 'Not' will be ignored
	// in this case.
	if str == "*" {
		t.strictLeft = false
		t.strictRight = false
		t.base = str

		return t
	}

	t.strictLeft = !strings.HasPrefix(str, "*")
	str = strings.TrimPrefix(str, "*")
	t.strictRight = !strings.HasSuffix(str, "*")
	t.base = strings.TrimSuffix(str, "*")

	return t
}

// Match checks if given str matches the [Template].
func (t *Template) Match(str string) bool {
	var match bool

	if t.base == "" {
		match = false
	} else if t.base == "*" {
		match = true
	} else if strings.Contains(str, t.base) {
		match = true
		sub := strings.Split(str, t.base)

		left := len(sub) == 1 ||
			sub[0] == "" ||
			strings.HasSuffix(sub[0], pathSeparator)

		right := len(sub) == 1 ||
			sub[1] == "" ||
			strings.HasPrefix(sub[1], pathSeparator)

		switch {
		case t.strictLeft && t.strictRight:
			match = left && right
		case t.strictLeft:
			match = left
		case t.strictRight:
			match = right
		}

		if t.not {
			match = !match
		}
	} else if t.not {
		match = true
	}

	if t.or != nil && !match {
		match = t.or.Match(str)
	}

	if t.and != nil {
		if !match {
			return match
		}

		match = t.and.Match(str)
	}

	return match
}

type Templates []*Template

// NewTemplates parses slice of strings into slice of Templates.
func NewTemplates(t []string) Templates {
	ts := make(Templates, 0, len(t))
	for _, str := range t {
		ts = append(ts, NewTemplate(str))
	}

	return ts
}
