// Package find allows to search for files/folders with given options.
package find

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrTemplateType = errors.New("cannot define type of the template")

// String representation of the current system path separator.
var pathSeparator = string(os.PathSeparator)

// Templater defines type constraint for generic Find function.
type Templater interface {
	~string | ~[]string
}

// Type of the searched object.
const (
	File uint8 = iota
	Folder
	Both
)

// options allows to configure Find behavior.
type options struct {
	matchFunc func(Templates, string) bool
	orig      string
	resOrig   string
	fType     uint8
	rec       bool
	name      bool
	relative  bool
	tree      bool
	skip      bool
	log       bool
	output    bool
}

// defaultOptions default Find options.
func defaultOptions() *options {
	return &options{
		matchFunc: MatchAny,
		fType:     Both,
	}
}

type optFunc func(*options)

// Deprecated: use [Only] instead.
func SearchFor(t uint8) optFunc { return Only(t) }

// Only defines if result should contains files, folders or both.
func Only(t uint8) optFunc {
	return func(o *options) {
		o.fType = t
	}
}

// Deprecated: use [Recursively] instead.
func SearchRecursively(o *options) { Recursively(o) }

// Recursively defines recursive search.
func Recursively(o *options) { o.rec = true }

// Deprecated: use [Name] instead.
func SearchName(o *options) { Name(o) }

// Name defines if only names of files/folders should be
// in the output.
func Name(o *options) { o.name = true }

// Deprecated: use [Strict] instead.
func SearchStrict(o *options) { Strict(o) }

// Strict requires all templates to match searched path.
func Strict(o *options) { o.matchFunc = MatchAll }

// MatchTree defines if all path should match the template or only a name.
func MatchTree(o *options) { o.tree = true }

// RelativePaths does not resolve paths in the output.
//
// Note: does not work with [Name] option.
func RelativePaths(o *options) { o.relative = true }

// WithErrorsSkip skips errors during find execution.
//
// Note: if the flag was set, [Find] will return nil error,
// only if the base path was resolved.
func WithErrorsSkip(o *options) { o.skip = true }

// WithErrosLog logs errors during find execution,
// should be used with [WithErrorsSkip], for clear output.
func WithErrosLog(o *options) { o.log = true }

// WithOutput prints found paths as soon as they match.
// Follows all the previous path related options,
// such as names and relative paths.
func WithOutput(o *options) { o.output = true }

// Templates defines slice of templates.
type Templates []*Template

// Find parses given string or slice of strings into templates and
// searches for matching paths in where with given options.
func Find[T Templater](
	ctx context.Context, where string, t T, opts ...optFunc,
) ([]string, error) {
	// Primary path resolution, even if `skip` flag was set,
	// this error is critical and should not be omitted.
	resPath, err := resolvePath(where)
	if err != nil {
		return nil, err
	}

	opt := defaultOptions()

	// Pre-save location file and its resolved path, for further
	// usage if relative paths will be needed.
	opt.orig = where
	opt.resOrig = resPath

	for _, fn := range opts {
		fn(opt)
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

	return find(ctx, resPath, ts, opt)
}

func find(
	ctx context.Context,
	where string,
	ts Templates,
	opt *options,
) ([]string, error) {
	resPath, err := resolvePath(where)
	if err != nil {
		if opt.skip {
			if opt.log {
				fmt.Println("error:", err)
			}

			return nil, nil
		}

		return nil, err
	}

	folders := make([]string, 0)

	data, err := os.ReadDir(resPath)
	if err != nil {
		if opt.skip {
			if opt.log {
				fmt.Println("error:", err)
			}

			return nil, nil
		}

		return nil, err
	}

	for _, f := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			p := filepath.Join(resPath, f.Name())

			var found string

			if (opt.matchFunc(ts, f.Name()) ||
				(opt.tree && opt.matchFunc(ts, p))) &&
				(opt.fType == Both ||
					(opt.fType == File && !f.IsDir()) ||
					(opt.fType == Folder && f.IsDir())) {
				switch {
				case opt.name:
					found = f.Name()
				case opt.relative:
					found = strings.ReplaceAll(p, opt.resOrig, opt.orig)
				default:
					found = p
				}

				if opt.output {
					fmt.Println(found)
				}

				folders = append(folders, found)
			}

			if opt.rec && f.IsDir() {
				recData, err := find(ctx, p, ts, opt)
				if err != nil {
					return nil, err
				}

				folders = append(folders, recData...)
			}
		}
	}

	return folders, nil
}

// resolvePath resolves symlinks and relative paths.
func resolvePath(p string) (string, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return "", err
	}

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		if p, err = filepath.EvalSymlinks(p); err != nil {
			return "", err
		}
	}

	return filepath.Abs(p)
}

// Template is a parsed version of each Find filter.
type Template struct {
	and         *Template
	or          *Template
	base        string
	not         bool
	strictLeft  bool
	strictRight bool
}

// Match checks if given str matches the [Template].
func (t *Template) Match(str string) bool {
	var match bool

	if strings.Contains(str, t.base) {
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
	t.base = strings.TrimFunc(
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

// MatchAny returns true if any of the given templates match the string.
func MatchAny(ts Templates, str string) bool {
	for _, t := range ts {
		if t.Match(str) {
			return true
		}
	}

	return false
}

// MatchAll returns true if all of the given templates match the string.
func MatchAll(ts Templates, str string) bool {
	for _, t := range ts {
		if !t.Match(str) {
			return false
		}
	}

	return true
}
