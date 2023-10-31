package find

import "strings"

// Type of the searched object.
const (
	File uint8 = iota
	Folder
	Both
)

var sensitive = func(s string) string { return s }

type (
	optFunc   func(*options)
	matchFunc func(Templates, string) bool
	caseFunc  func(string) string
)

type Options []optFunc

// NewDefaultOptions returns [Options] to create custom sets.
//
// Can be used like:
//
//	 opts := find.NewOptions()
//	 opts = append(opts, find.Only(find.File), find.Recursively)
//	 res, err := find.Find(
//		 	context.TODO(),
//		 	"path/to/where",
//		 	"template",
//		 	opts...,
//	 )
func NewOptions() Options { return make(Options, 0) }

// options allows to configure Find behavior.
type options struct {
	matchFunc matchFunc
	caseFunc  caseFunc
	max       int
	orig      string
	resOrig   string
	fType     uint8
	rec       bool
	name      bool
	relative  bool
	full      bool
	skip      bool
	log       bool
	output    bool
}

// defaultOptions default [Find] options.
func defaultOptions() *options {
	return &options{
		matchFunc: MatchAny,
		caseFunc:  sensitive,
		fType:     Both,
		max:       -1,
	}
}

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

// MatchFullPath matches full path not just the name.
func MatchFullPath(o *options) { o.full = true }

// RelativePaths does not resolve paths in the output.
//
// Note: does not work with [Name] option.
func RelativePaths(o *options) { o.relative = true }

// WithErrorsSkip skips errors during find execution.
//
// Note: if the flag was set, [Find] will return nil error,
// only if the base path was resolved.
func WithErrorsSkip(o *options) { o.skip = true }

// WithErrorsLog logs errors during find execution,
// should be used with [WithErrorsSkip], for clear output.
func WithErrorsLog(o *options) { o.log = true }

// WithOutput prints found paths as soon as they match.
// Follows all the previous path related options,
// such as names and relative paths.
func WithOutput(o *options) { o.output = true }

// Max set maximum ammount of searched objects. [Find] will stop as
// soon as reach the limitation.
func Max(i int) optFunc {
	return func(o *options) {
		o.max = i
	}
}

// Insensitive sets case insensitive search.
func Insensitive(o *options) {
	o.caseFunc = strings.ToLower
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
