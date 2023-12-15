package find

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

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

	// Type to create custom slices of find options.
	Options []optFunc
)

// options allows to configure Find behavior.
type options struct {
	matchFunc matchFunc
	caseFunc  caseFunc
	logger    io.Writer
	output    io.Writer
	orig      string
	resOrig   string
	max       int
	maxIter   int
	fType     uint8
	iterCh    chan string
	errCh     chan error
	rec       bool
	name      bool
	relative  bool
	full      bool
	skip      bool
	log       bool
	iter      bool
	out       bool
}

// defaultOptions default [Find] options.
func defaultOptions() *options {
	return &options{
		matchFunc: MatchAny,
		caseFunc:  sensitive,
		logger:    os.Stdout,
		output:    os.Stdout,
		maxIter:   100,
		max:       -1,
		fType:     Both,
	}
}

func defaultOptionsWithCustom(opts ...optFunc) *options {
	opt := defaultOptions()

	for _, fn := range opts {
		fn(opt)
	}

	return opt
}

func (o *options) logError(e error) error {
	var err error

	if o.log && !o.skip {
		_, err = fmt.Fprintf(o.logger, "error: %s\n", e)
		if err != nil {
			return fmt.Errorf("%w: %w", e, err)
		}
	}

	return err
}

func (o *options) printOutput(str string) error {
	var err error

	if o.out {
		_, err = fmt.Println(o.output, str)
	}

	return err
}

func (o *options) isSearchedType(isDir bool) bool {
	switch {
	case o.fType == Folder:
		return isDir
	case o.fType == File:
		return !isDir
	default:
		return true
	}
}

func (o *options) match(ts Templates, fullPath string) bool {
	if o.full {
		return o.matchFunc(ts, o.caseFunc(fullPath))
	}

	return o.matchFunc(ts, o.caseFunc(path.Base(fullPath)))
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

// WithErrorsLog logs errors during find execution.
func WithErrorsLog(o *options) { o.log = true }

// WithOutput prints results as soon as they match given [Templates].
func WithOutput(o *options) { o.out = true }

// WithWriter allows to set custom [io.Writer] for [WithPrint].
//
// Note: write errors count as critical and will be returned
// even if [WithErrorsSkip] was set.
func WithWriter(out io.Writer) optFunc {
	return func(o *options) {
		o.output = out
		o.out = true
	}
}

// WithLogger allows to set custom logger for [WithErrorsLog].
//
// Note: write errors count as critical and will be returned
// even if [WithErrorsSkip] was set.
func WithLogger(l io.Writer) optFunc {
	return func(o *options) {
		o.logger = l
	}
}

// WithMaxIterator allows to set custom output channel buffer.
//
// Note: can be used only with [FindWithIterator].
func WithMaxIterator(max int) optFunc {
	return func(o *options) {
		o.maxIter = max
	}
}

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
