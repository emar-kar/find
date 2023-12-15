// Package find allows to search for files/folders with options.
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

// Templater defines type constraint for generic Find function.
type Templater interface {
	~string | ~[]string
}

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

		if _, err := find(ctx, resPath, ts, opt); err != nil {
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

	return find(ctx, resPath, ts, opt)
}

func find(
	ctx context.Context,
	where string,
	ts Templates,
	opt *options,
) ([]string, error) {
	resPath, data, err := readAndResolve(where)
	if err != nil {
		lErr := opt.logError(err)

		return nil, lErr
	}

	res := make([]string, 0)

	for _, f := range data {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if opt.max == 0 {
				return res, nil
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
					return nil, err
				}

				if opt.iter {
					opt.iterCh <- found
				} else {
					res = append(res, found)
				}

				if opt.max != -1 {
					opt.max--
				}
			}

			if opt.rec && f.IsDir() {
				recData, err := find(ctx, p, ts, opt)
				if err != nil {
					return nil, err
				}

				res = append(res, recData...)
			}
		}
	}

	return res, nil
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

func newTemplates[T Templater](t T, fn caseFunc) (Templates, error) {
	var ts Templates

	switch any(t).(type) {
	case string:
		ts = Templates{NewTemplate(fn(any(t).(string)))}
	case []string:
		sl := make([]string, 0, len(any(t).([]string)))

		for _, str := range any(t).([]string) {
			sl = append(sl, fn(str))
		}

		ts = NewTemplates(sl)
	default:
		return nil, fmt.Errorf("%w: %v", ErrTemplateType, t)
	}

	return ts, nil
}
