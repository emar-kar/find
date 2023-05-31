# find
Simple find module for go applications

### Install:

```bash
go get github.com/emar-kar/find@latest
```

### Usage:

Find allows to search files and folders with the given template:

```go
ctx, cancel := context.WithDeadline(
  context.Background(),
  time.Now().Add(5*time.Minute),
)
defer cancel()

where := "path/to/the/source"

// Since recursive search does not active by default,
// Find will search for files and folders only in the
// root of where.
results, err := Find(ctx, where, "*template*")
if err != nil {
  log.Println(err)

  return
}

for _, r := range results {
  fmt.Println(r)
}
```

### Setup:

Find supports several options for search customization:

* `SearchFor` - allows to define what type should be the results: files, folders or both;
* `SearchRecursively` - activates recursive search;
* `SearchName` - if true, result will contain only names of the found elements, not their paths;
* `SearchStrict` - since Find supports passing several templates during search, by default path will be returned if it matchs any of the given template. This option switch this behavior to match all the templates.

```go
// defaultOptions default Find options.
func defaultOptions() *options {
	return &options{
		matchFunc: MatchAny,
		fType:     Both,
		rec:       false,
		name:      false,
	}
}
```

Find uses generic templates, which can be a simple `string` type or a slice of strings `[]string{}`.

String can contain the following setup:

* `str&str1` - means that searched path should be both str and str1
* `str|str1` - means that searched path should be str or str1
* `*str`     - means that searched path should ends with str
* `str*`     - means that searched path should starts with str
* `*str*`    - means that searched path should contain str
* `str`      - means that searched path should be str
* `!str`     - means that searched path should not be str
* `!*str`    - means that searched path should not end with str
* `!str*`    - means that searched path should not start with str
* `!*str*`   - means that searched path should not contain str
