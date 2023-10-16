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

// Type of the searched object.
const (
	File uint8 = iota
	Folder
	Both
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
	opt *opts,
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

			if (opt.fType == Both ||
				(opt.fType == File && !f.IsDir()) ||
				(opt.fType == Folder && f.IsDir())) &&
				(opt.matchFunc(ts, f.Name()) ||
					(opt.tree && opt.matchFunc(ts, p))) {
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
