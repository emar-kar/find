// Package find allows to search for files/folders with options.
package find

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// FindWithIterator acts the same way as [Find] but returns channels instead.
// String channel will return every match. Error channel returns first occured
// error during search or if [WithErrorsSkip] was set, first critical error.
// As soon as search is over or interrupted, both channels will be closed.
// For example:
//
//	outCh, errCh := FindWithIterator(ctx, where, ts, opts...)
//	for f := range outCh {
//		// do something here...
//	}
//	if err := <-errCh {
//		// process error...
//	}
//
// NOTE: output channel should be cosumed by the caller.
// Overwise it can cause a deadlock.
func FindWithIterator[T Templater](
	ctx context.Context,
	where string,
	t T,
	opts ...optFunc,
) (chan string, chan error) {
	opt := defaultOptionsWithCustom(opts...)

	opt.iterCh = make(chan string, opt.maxIter)
	opt.errCh = make(chan error, 1)
	opt.iter = true

	go func() {
		defer func() {
			close(opt.iterCh)
			close(opt.errCh)
		}()

		resPath, err := resolvePath(where)
		if err != nil {
			opt.errCh <- err
			return
		}

		opt.orig = where
		opt.resOrig = resPath

		ts, err := newTemplates(t, opt.caseFunc)
		if err != nil {
			opt.errCh <- err
			return
		}

		if err := find(ctx, resPath, ts, opt, nil); err != nil {
			opt.errCh <- err
		}
	}()

	return opt.iterCh, opt.errCh
}

// Find searches for matches with the given templates in where.
func Find[T Templater](
	ctx context.Context,
	where string,
	t T,
	opts ...optFunc,
) ([]string, error) {
	// Primary path resolution, even if `skip` flag was set,
	// this error is critical and should not be omitted.
	resPath, err := resolvePath(where)
	if err != nil {
		return nil, err
	}

	opt := defaultOptionsWithCustom(opts...)

	// Pre-save location file and its resolved path, for further
	// usage if relative paths will be needed.
	opt.orig = where
	opt.resOrig = resPath

	ts, err := newTemplates(t, opt.caseFunc)
	if err != nil {
		return nil, err
	}

	var result []string
	if opt.max > 0 {
		result = make([]string, 0, opt.max)
	} else {
		result = make([]string, 0, 10)
	}

	err = find(ctx, resPath, ts, opt, &result)

	return result, err
}

func find(
	ctx context.Context,
	where string,
	ts Templates,
	opt *options,
	result *[]string,
) error {
	resPath, entries, err := readAndResolve(where)
	if err != nil {
		return opt.logError(err)
	}

	for _, f := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if opt.max == 0 {
				return nil
			}

			p := filepath.Join(resPath, f.Name())

			var found string

			if opt.isSearchedType(f.IsDir()) && opt.match(ts, p) {
				switch {
				case opt.name:
					found = f.Name()
				case opt.relative:
					found = strings.ReplaceAll(p, opt.resOrig, opt.orig)
				default:
					found = p
				}

				if err := opt.printOutput(found); err != nil {
					return opt.logError(err)
				}

				if opt.iter {
					opt.iterCh <- found
				} else {
					*result = append(*result, found)
				}

				if opt.max != -1 {
					opt.max--
				}
			}

			if opt.rec && f.IsDir() {
				if err := find(ctx, p, ts, opt, result); err != nil {
					return err
				}
			}
		}
	}

	return nil
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

func readAndResolve(p string) (string, []os.DirEntry, error) {
	resPath, err := resolvePath(p)
	if err != nil {
		return "", nil, err
	}

	data, err := os.ReadDir(resPath)

	return resPath, data, err
}
