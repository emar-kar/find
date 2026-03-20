package find

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Type of the searched object.
const (
	File uint8 = iota
	Folder
	Both
)

type (
	// Option configures [Find], [FindWithIterator], and [FindSeq] behaviour.
	Option func(*options)

	caseFunc func(string) string
)

// options configures the behaviour of [Find], [FindWithIterator], and [FindSeq].
type options struct {
	caseFunc caseFunc
	tmpl     *Template
	logger   io.Writer
	output   io.Writer
	orig     string
	resOrig  string
	max      int
	maxIter  int
	fType    uint8
	rec      bool
	name     bool
	relative bool
	full     bool
	skip     bool
	strict   bool
	symlinks bool
}

// defaultOptions returns default options.
func defaultOptions() *options {
	return &options{
		// Sensetive case by default.
		caseFunc: func(s string) string { return s },
		maxIter:  100,
		max:      -1,
		fType:    Both,
	}
}

func (o *options) logError(e error) error {
	if o.logger != nil {
		if _, err := fmt.Fprintf(o.logger, "error: %s\n", e); err != nil {
			return fmt.Errorf("%w: %w", e, err)
		}
	}

	if o.skip {
		return nil
	}

	return e
}

func (o *options) printOutput(str string) error {
	if o.output != nil {
		if _, err := fmt.Fprintln(o.output, str); err != nil {
			return err
		}
	}

	return nil
}

func (o *options) isSearchedType(isDir bool) bool {
	switch o.fType {
	case Folder:
		return isDir
	case File:
		return !isDir
	default:
		return true
	}
}

// match checks whether the entry at fullPath/name matches the template.
// When [MatchFullPath] is set, the full path is matched; otherwise only the
// entry name is used.
func (o *options) match(fullPath, name string) bool {
	if o.tmpl == nil {
		return false
	}

	if o.full {
		return o.tmpl.Match(o.caseFunc(fullPath))
	}

	return o.tmpl.Match(o.caseFunc(name))
}

// Deprecated: use [Only] instead.
func SearchFor(t uint8) Option { return Only(t) }

// Only defines if result should contains files, folders or both.
func Only(t uint8) Option {
	return func(o *options) {
		o.fType = t
	}
}

// Deprecated: use [Recursive] instead.
func Recursively(o *options) { Recursive(o) }

// Recursive enables recursive directory search.
func Recursive(o *options) { o.rec = true }

// FollowSymlinks resolves symlinks during traversal.
// When enabled, symlinks pointing to directories are recursed into
// (requires [Recursive]) and reported as folders when [Only](Folder)
// is set. Disabled by default to avoid unexpected behaviour with
// circular symlinks.
func FollowSymlinks(o *options) { o.symlinks = true }

// Deprecated: use [NamesOnly] instead.
func Name(o *options) { NamesOnly(o) }

// NamesOnly restricts the output to entry names only, omitting the path.
func NamesOnly(o *options) { o.name = true }

// Strict changes the behaviour of [FindWithIterator] when a
// []string pattern is given: instead of joining the patterns with OR
// (any must match), they are joined with AND (all must match).
// Has no effect on [Find] and [FindSeq], which accept a single string pattern.
func Strict(o *options) { o.strict = true }

// MatchFullPath matches the full path instead of just the entry name.
func MatchFullPath(o *options) { o.full = true }

// RelativePaths does not resolve paths in the output.
//
// Note: does not work with [NamesOnly] option.
func RelativePaths(o *options) { o.relative = true }

// Deprecated: use [SkipErrors] instead.
func WithErrorsSkip(o *options) { SkipErrors(o) }

// SkipErrors silently ignores non-critical errors during search.
// Has no effect on [FindSeq], which always yields errors to the caller.
//
// Note: if set, [Find] returns nil even when errors occur, as long
// as the root path was resolved.
func SkipErrors(o *options) { o.skip = true }

// Deprecated: use [LogErrors] instead.
func WithErrorsLog(o *options) { LogErrors(o) }

// LogErrors logs errors to stderr (or the writer set by [WithLogger])
// during search. Defaults to [os.Stderr].
//
// Deprecated: use [FindSeq] and handle errors directly in the iteration loop.
func LogErrors(o *options) { o.logger = os.Stderr }

// WithOutput prints results to [os.Stdout] as soon as they match.
// Use [WithWriter] to set a custom writer.
//
// Deprecated: use [FindSeq] and write matches directly in the iteration loop.
func WithOutput(o *options) { o.output = os.Stdout }

// WithWriter sets a custom [io.Writer] for output. Also enables output.
// Ignores nil writers.
//
// Note: write error counts as critical and will be returned
// even if [SkipErrors] was set.
//
// Deprecated: use [FindSeq] and write matches directly in the iteration loop.
func WithWriter(out io.Writer) Option {
	return func(o *options) {
		if out != nil {
			o.output = out
		}
	}
}

// WithLogger sets a custom [io.Writer] for error logging. Also enables
// logging. Ignores nil writers.
//
// Note: write error counts as critical and will be returned
// even if [SkipErrors] was set.
//
// Deprecated: use [FindSeq] and handle errors directly in the iteration loop.
func WithLogger(l io.Writer) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithMaxIterator sets the buffer size of the output channel.
// Values <= 0 are ignored; the default buffer size is 100.
//
// Deprecated: use [FindSeq] instead, which does not use channels.
func WithMaxIterator(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.maxIter = n
		}
	}
}

// Max sets the maximum number of results returned.
// Traversal stops as soon as it reaches the limit.
// Values <= 0 are ignored; the default behaviour is unlimited (-1).
func Max(i int) Option {
	return func(o *options) {
		if i > 0 {
			o.max = i
		}
	}
}

// Deprecated: use [CaseInsensitive] instead.
func Insensitive(o *options) { CaseInsensitive(o) }

// CaseInsensitive enables case-insensitive pattern matching.
func CaseInsensitive(o *options) {
	o.caseFunc = strings.ToLower
}

// MatchAny returns true if any of the given templates match str.
func MatchAny(ts Templates, str string) bool {
	for _, t := range ts {
		if t.Match(str) {
			return true
		}
	}

	return false
}

// MatchAll returns true if all of the given templates match str.
func MatchAll(ts Templates, str string) bool {
	for _, t := range ts {
		if !t.Match(str) {
			return false
		}
	}

	return true
}
