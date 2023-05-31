// Package find allows to search for files/folders paths with given options.
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

// fType defines types of possibly searching objects.
// To set up type on place you can use predefined constants or
// just use iota numeric.
type fType uint8

const (
	// File represents file like object.
	File fType = iota
	// Folder represents directory like object.
	Folder
	// Both represents both files or directories.
	Both
)

// options allows to configure Find behavior.
type options struct {
	fType fType
	rec   bool
	name  bool
}

// defaultOptions default Find options.
func defaultOptions() *options {
	return &options{
		fType: Both,
	}
}

type optFunc func(*options)

// SearchFor sets if result should contains files, folders or both.
func SearchFor(t fType) optFunc {
	return func(o *options) {
		o.fType = t
	}
}

// SearchRecursively sets the search depth.
func SearchRecursively(o *options) { o.rec = true }

// SearchName sets if return slice should contain only names.
func SearchName(o *options) { o.name = true }

// Templates slice of the templates.
type Templates []*Template

// ParseTemplates parses slice of strings into slice of Templates.
func ParseTemplates(t []string) Templates {
	ts := make(Templates, 0, len(t))
	for _, str := range t {
		ts = append(ts, NewTemplate(str))
	}

	return ts
}

// Any return true if any of the templates match given string, false otherwise.
func (ts Templates) Any(str string) bool {
	for _, t := range ts {
		if t.Match(str) {
			return true
		}
	}

	return false
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

// NewTemplate creates new Template from the given string.
//
// String can contain:
//
//	str&str1 - means that searched path should contain str and str1
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
//
// Each element of the '|' or '&' option can specificators special options
// from the list above.
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

// Match checks if basename of the given path matches the template.
func (t *Template) Match(str string) bool {
	var match bool
	if strings.Contains(str, t.str) {
		match = true
		sub := strings.Split(str, t.str)
		left := sub[0] == "" || strings.HasSuffix(sub[0], pathSeparator)
		right := sub[1] == "" || strings.HasPrefix(sub[1], pathSeparator)

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

// Templater defines type constraint for generic Find function.
type Templater interface {
	string | []string
}

// Find is a generic function which allows to accept string or slice of strings
// as a template.
//
// If options not defined, default will be used.
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
		ts = ParseTemplates(any(t).([]string))
	default:
		return nil, fmt.Errorf("%w: %v", ErrTemplateType, t)
	}

	return find(ctx, where, ts, opt)
}

// findTemplate searches specific Template in where with given options.
func find(ctx context.Context, where string, ts Templates, opt *options) ([]string, error) {
	where, err := evalSymlink(where)
	if err != nil {
		return nil, err
	}

	folders := make([]string, 0)

	data, err := os.ReadDir(where)
	if err != nil {
		return nil, err
	}

	for _, f := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if ts.Any(f.Name()) && (opt.fType == Both ||
				(opt.fType == File && !f.IsDir()) ||
				(opt.fType == Folder && f.IsDir())) {
				if opt.name {
					folders = append(folders, f.Name())
				} else {
					folders = append(folders, path.Join(where, f.Name()))
				}
			}

			if opt.rec && f.IsDir() {
				recData, err := find(
					ctx, path.Join(where, f.Name()), ts, opt,
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

// evalSymlink returns the path name after the evaluation of any symbolic links.
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
