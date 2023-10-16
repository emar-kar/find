package find

type (
	optFunc   func(*opts)
	matchFunc func(Templates, string) bool
	options   []optFunc
)

// opts allows to configure Find behavior.
type opts struct {
	matchFunc matchFunc
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

// defaultOptions default Find opts.
func defaultOptions() *opts {
	return &opts{
		matchFunc: MatchAny,
		fType:     Both,
	}
}

// NewOptions creates an empty slice of [optFunc]s to
// create custom options sets.
func NewOptions() options { return make(options, 0) }

// Deprecated: use [Only] instead.
func SearchFor(t uint8) optFunc { return Only(t) }

// Only defines if result should contains files, folders or both.
func Only(t uint8) optFunc {
	return func(o *opts) {
		o.fType = t
	}
}

// Deprecated: use [Recursively] instead.
func SearchRecursively(o *opts) { Recursively(o) }

// Recursively defines recursive search.
func Recursively(o *opts) { o.rec = true }

// Deprecated: use [Name] instead.
func SearchName(o *opts) { Name(o) }

// Name defines if only names of files/folders should be
// in the output.
func Name(o *opts) { o.name = true }

// Deprecated: use [Strict] instead.
func SearchStrict(o *opts) { Strict(o) }

// Strict requires all templates to match searched path.
func Strict(o *opts) { o.matchFunc = MatchAll }

// MatchTree defines if all path should match the template or only a name.
func MatchTree(o *opts) { o.tree = true }

// RelativePaths does not resolve paths in the output.
//
// Note: does not work with [Name] option.
func RelativePaths(o *opts) { o.relative = true }

// WithErrorsSkip skips errors during find execution.
//
// Note: if the flag was set, [Find] will return nil error,
// only if the base path was resolved.
func WithErrorsSkip(o *opts) { o.skip = true }

// WithErrosLog logs errors during find execution,
// should be used with [WithErrorsSkip], for clear output.
func WithErrosLog(o *opts) { o.log = true }

// WithOutput prints found paths as soon as they match.
// Follows all the previous path related opts,
// such as names and relative paths.
func WithOutput(o *opts) { o.output = true }

// WithMatchFunc allows to set custom match function
// for multiple templates.
func WithMatchFunc(fn matchFunc) optFunc {
	return func(o *opts) {
		o.matchFunc = fn
	}
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
