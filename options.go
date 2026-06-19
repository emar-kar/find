package find

import "strings"

// Type of the searched object.
const (
	File uint8 = iota
	Folder
	Both
)

type (
	// Option configures [Find] and [FindSeq] Behavior.
	Option func(*options)

	caseFunc func(string) string
)

// options configures the Behavior of [Find] and [FindSeq].
type options struct {
	caseFunc caseFunc
	tmpl     *Template
	orig     string
	resOrig  string
	max      int
	fType    uint8
	rec      bool
	name     bool
	relative bool
	full     bool
	skip     bool
	symlinks bool
}

// defaultOptions returns default options.
func defaultOptions() *options {
	return &options{
		// Sensetive case by default.
		caseFunc: func(s string) string { return s },
		max:      -1,
		fType:    Both,
	}
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

// Only defines if result should contains files, folders or both.
func Only(t uint8) Option {
	return func(o *options) {
		o.fType = t
	}
}

// Recursive enables recursive directory search.
func Recursive(o *options) { o.rec = true }

// FollowSymlinks resolves symlinks during traversal.
// When enabled, symlinks pointing to directories are recursed into
// (requires [Recursive]) and reported as folders when [Only](Folder)
// is set. Disabled by default to avoid unexpected Behavior with
// circular symlinks.
func FollowSymlinks(o *options) { o.symlinks = true }

// NamesOnly restricts the output to entry names only, omitting the path.
func NamesOnly(o *options) { o.name = true }

// MatchFullPath matches the full path instead of just the entry name.
func MatchFullPath(o *options) { o.full = true }

// RelativePaths does not resolve paths in the output.
//
// Note: does not work with [NamesOnly] option.
func RelativePaths(o *options) { o.relative = true }

// SkipErrors silently ignores non-critical errors during search.
// Has no effect on [FindSeq], which always yields errors to the caller.
//
// Note: if set, [Find] returns nil even when errors occur, as long
// as the root path was resolved.
func SkipErrors(o *options) { o.skip = true }

// Max sets the maximum number of results returned.
// Traversal stops as soon as it reaches the limit.
// Values <= 0 are ignored; the default Behavior is unlimited (-1).
func Max(i int) Option {
	return func(o *options) {
		if i > 0 {
			o.max = i
		}
	}
}

// CaseInsensitive enables case-insensitive pattern matching.
func CaseInsensitive(o *options) {
	o.caseFunc = strings.ToLower
}
