// Package find allows to search for files/folders with given options.
package find

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ErrTemplateType = errors.New("cannot define type of the template")

// pathSeparator string representation of the current system path separator.
var pathSeparator = string(os.PathSeparator)

// Templater defines type constraint for generic Find function.
type Templater interface {
	string | []string
}

// Allows to define searched object.
const (
	File uint8 = iota
	Folder
	Both
)

// options allows to configure Find behavior.
type options struct {
	matchFunc func(Templates, string) bool
	fType     uint8
	rec       bool
	name      bool
}

// defaultOptions default Find options.
func defaultOptions() *options {
	return &options{
		matchFunc: MatchAny,
		fType:     Both,
		rec:       false,
		name:      false,
	}
}

type optFunc func(*options)

// SearchFor defines if result should contains files, folders or both.
func SearchFor(t uint8) optFunc {
	return func(o *options) {
		o.fType = t
	}
}

// SearchRecursively defines recursive search.
func SearchRecursively(o *options) { o.rec = true }

// SearchName defines if only result names should be found.
func SearchName(o *options) { o.name = true }

// SearchStrict requires all templates to match searched path.
func SearchStrict(o *options) { o.matchFunc = MatchAll }

// Templates defines slice of templates.
type Templates []*Template

// Find parses given string or slice of strings into template and
// searches for matching paths with given options.
func Find[T Templater](
	ctx context.Context, where string, t T, opts ...optFunc,
) ([]string, error) {
	opt := defaultOptions()
	for _, fn := range opts {
		fn(opt)
	}

	if strings.HasPrefix(where, ".") {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		where = strings.ReplaceAll(where, ".", cwd)
	}

	var ts Templates

	switch any(t).(type) {
	case string:
		ts = Templates{NewTemplate(any(t).(string))}
	case []string:
		ts = NewTemplates(any(t).([]string))
	default:
		return nil, fmt.Errorf("%w: %v", ErrTemplateType, t)
	}

	return find(ctx, where, ts, opt)
}

// findTemplate searches specific Template in where with given options.
func find(
	ctx context.Context,
	p string,
	ts Templates,
	opt *options,
) ([]string, error) {
	p, err := evalSymlink(p)
	if err != nil {
		return nil, err
	}

	folders := make([]string, 0)

	data, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}

	for _, f := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if (opt.matchFunc(ts, f.Name()) ||
				opt.matchFunc(ts, path.Join(p, f.Name()))) &&
				(opt.fType == Both || (opt.fType == File && !f.IsDir()) ||
					(opt.fType == Folder && f.IsDir())) {
				if opt.name {
					folders = append(folders, f.Name())
				} else {
					folders = append(folders, path.Join(p, f.Name()))
				}
			}

			if opt.rec && f.IsDir() {
				recData, err := find(
					ctx, path.Join(p, f.Name()), ts, opt,
				)
				if err != nil {
					return nil, err
				}

				folders = append(folders, recData...)
			}
		}
	}

	return folders, nil
}

// evalSymlink returns the path name after the evaluation
// of any symbolic links.
// Check [filepath.EvalSymlinks] for details.
func evalSymlink(p string) (string, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", fmt.Errorf("cannot get %s info: %w", p, err)
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		return filepath.EvalSymlinks(p)
	}

	return p, nil
}

// Template is a parsed version of each Find filter.
type Template struct {
	and         *Template
	or          *Template
	str         string
	not         bool
	strictLeft  bool
	strictRight bool
}

// Match checks if basename of the given path matches the template.
func (t *Template) Match(str string) bool {
	var match bool

	if strings.Contains(str, t.str) {
		match = true
		sub := strings.Split(str, t.str)

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

// NewTemplate creates new Template from the given string.
//
// String can contains:
//
//	str&str1 - means that searched path should be both str and str1
//	str|str1 - means that searched path should be str or str1
//	*str     - means that searched path should ends with str
//	str*     - means that searched path should starts with str
//	*str*    - means that searched path should contain str
//	str      - means that searched path should be str
//	!str     - means that searched path should not be str
//	!*str    - means that searched path should not end with str
//	!str*    - means that searched path should not start with str
//	!*str*   - means that searched path should not contain str
//
// Option '&' defines nested paths e.g., 'str&str1' - Find will search
// for 'str' first and if it's found 'str1' inside it.
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

// parse parses basic fields of the Template.
func parse(str string) *Template {
	t := &Template{}
	t.str = strings.TrimFunc(
		str, func(r rune) bool {
			return r == '!' || r == '*'
		},
	)

	t.not = strings.HasPrefix(str, "!")

	str = strings.TrimPrefix(str, "!")
	t.strictLeft = !strings.HasPrefix(str, "*")
	t.strictRight = !strings.HasSuffix(str, "*")

	return t
}

// NewTemplates parses slice of strings into slice of Templates.
func NewTemplates(t []string) Templates {
	ts := make(Templates, 0, len(t))
	for _, str := range t {
		ts = append(ts, NewTemplate(str))
	}

	return ts
}

// Any returns true if any of the templates match the given string.
func MatchAny(ts Templates, str string) bool {
	for _, t := range ts {
		if t.Match(str) {
			return true
		}
	}

	return false
}

// All returns true if all of the templates match the given string.
func MatchAll(ts Templates, str string) bool {
	for _, t := range ts {
		if !t.Match(str) {
			return false
		}
	}

	return true
}
