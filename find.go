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

// Find parses given string or slice of strings into templates and
// searches for matching paths in where with given opts.
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
		ts = Templates{NewTemplate(opt.caseFunc(any(t).(string)))}
	case []string:
		sl := make([]string, 0, len(any(t).([]string)))

		for _, str := range any(t).([]string) {
			sl = append(sl, opt.caseFunc(str))
		}

		ts = NewTemplates(sl)
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

	res := make([]string, 0)

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

			// Check if current path matches searched object and if it does,
			// use the match func to process it with the match function.
			if (opt.fType == Both ||
				(opt.fType == File && !f.IsDir()) ||
				(opt.fType == Folder && f.IsDir())) &&
				((opt.full && opt.matchFunc(ts, opt.caseFunc(p))) ||
					(!opt.full && opt.matchFunc(ts, opt.caseFunc(f.Name())))) {
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

				res = append(res, found)

				if opt.max != -1 && len(res) >= opt.max {
					return res, nil
				}
			}

			if opt.rec && f.IsDir() {
				recData, err := find(ctx, p, ts, opt)
				if err != nil {
					return nil, err
				}

				if opt.max != -1 && len(res)+len(recData) >= opt.max {
					res = append(res, recData[:opt.max-len(res)]...)

					return res, nil
				} else {
					res = append(res, recData...)
				}
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
