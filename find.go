// Package find searches for files and folders matching configurable patterns.
package find

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
)

// FindSeq returns an iterator over all matches of pattern in where.
// It can be used directly with range:
//
//	for path, err := range FindSeq(ctx, "/some/dir", "*.go") {
//	    if err != nil {
//	        // handle or break
//	        continue
//	    }
//	    fmt.Println(path)
//	}
//
// Setup errors (unresolvable path, malformed pattern) are yielded as
// ("", err) on the first iteration. Mid-walk errors (e.g. permission
// denied on a subdirectory) are yielded and traversal continues; use
// break to stop early at any point. Empty pattern breaks loop immediately.
//
// See [ParseTemplate] for the full pattern syntax.
func FindSeq(
	ctx context.Context, where, pattern string, opts ...Option,
) iter.Seq2[string, error] {
	if pattern == "" {
		return func(yield func(string, error) bool) {
			yield("", nil)
		}
	}

	opt := defaultOptions()

	for _, fn := range opts {
		fn(opt)
	}

	return findSeqWithOpts(ctx, where, pattern, opt)
}

// findSeqWithOpts is the internal implementation of [FindSeq] that accepts
// a pre-built *options, allowing [Find] to share the same options struct
// without double-instantiation.
func findSeqWithOpts(
	ctx context.Context, where, pattern string, opt *options,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		rp, err := resolvePath(where)
		if err != nil {
			yield("", err)
			return
		}

		opt.orig = where
		opt.resOrig = rp

		tmpl, err := ParseTemplate(opt.caseFunc(pattern))
		if err != nil {
			yield("", err)
			return
		}

		opt.tmpl = tmpl

		findSeq(ctx, rp, opt, yield)
	}
}

// resolvePath returns the absolute real path for p. It calls [os.Lstat] to
// check for symlinks, resolves them via [filepath.EvalSymlinks] if present,
// and converts the result to an absolute path.
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

// findSeq reads rp and processes each entry, yielding matches via yield.
// Returns false as soon as yield signals the caller wants to stop.
func findSeq(
	ctx context.Context,
	rp string,
	opt *options,
	yield func(string, error) bool,
) bool {
	entries, err := os.ReadDir(rp)
	if err != nil {
		// Yield the error but keep walking the parent directory.
		return yield("", err)
	}

	for _, f := range entries {
		select {
		case <-ctx.Done():
			yield("", ctx.Err())
			return false
		default:
			if opt.max == 0 {
				return false
			}

			p := filepath.Join(rp, f.Name())

			if !processEntrySeq(ctx, f, p, opt, yield) {
				return false
			}
		}
	}

	return true
}

// processEntrySeq handles a single directory entry for [FindSeq]: match,
// yield, and optionally recurse. Returns false when yield signals stop.
func processEntrySeq(
	ctx context.Context,
	f os.DirEntry,
	p string,
	opt *options,
	yield func(string, error) bool,
) bool {
	isDir := f.IsDir()
	isSymlink := f.Type()&os.ModeSymlink != 0

	// Resolve symlinks only when explicitly requested via FollowSymlinks.
	if isSymlink && opt.symlinks {
		info, err := os.Stat(p)
		if err != nil {
			return yield("", err)
		}

		isDir = info.IsDir()
	}

	if opt.isSearchedType(isDir) && opt.match(p, f.Name()) {
		found := formatFound(f, p, opt)

		if !yield(found, nil) {
			return false
		}

		if opt.max != -1 {
			opt.max--
		}
	}

	// Do not recurse if max has already been reached.
	if opt.rec && isDir && opt.max != 0 {
		rp := p

		if isSymlink && opt.symlinks {
			// Resolve the symlink so findSeq receives an absolute real path.
			var err error

			rp, err = filepath.EvalSymlinks(p)
			if err != nil {
				return yield("", err)
			}
		}

		return findSeq(ctx, rp, opt, yield)
	}

	return true
}

// formatFound returns the display form of path p according to opt flags.
func formatFound(f os.DirEntry, p string, opt *options) string {
	switch {
	case opt.name:
		return f.Name()
	case opt.relative:
		return strings.Replace(p, opt.resOrig, opt.orig, 1)
	default:
		return p
	}
}

// Find collects matches into a slice before return.
func Find(
	ctx context.Context, where, pattern string, opts ...Option,
) ([]string, error) {
	result := make([]string, 0, 10)

	for found, err := range FindSeq(ctx, where, pattern, opts...) {
		if err != nil {
			return nil, err
		}

		result = append(result, found)
	}

	return result, nil
}
