# find

A lightweight Go package for searching files and folders by pattern. Zero
dependencies, context-aware, and built on Go 1.23+ iterators.

## Install

```bash
go get github.com/emar-kar/find/v2
```

## Quick start

```go
// Stream results with FindSeq (recommended).
for path, err := range find.FindSeq(ctx, "/src", "*.go", find.Recursive) {
    if err != nil {
        log.Println(err)
        continue
    }
    fmt.Println(path)
}
```

`FindSeq` returns an `iter.Seq2[string, error]` — use it with `for range`,
`break` out whenever you want, and handle errors inline.

## Pattern syntax

Patterns are parsed by `ParseTemplate`. A single string can express
wildcards, negation, boolean operators, and grouping:

| Pattern    | Meaning                               |
|------------|---------------------------------------|
| `*str*`    | path contains *str*                   |
| `str`      | path equals *str* (segment boundary)  |
| `*str`     | path ends with *str*                  |
| `str*`     | path starts with *str*                |
| `!pattern` | negation — path must **not** match    |
| `a&b`      | AND — both must match                 |
| `a\|b`     | OR — either must match                |
| `(a\|b)&c` | parentheses override precedence       |

`&` binds tighter than `|`, so `*go*&!*test*|*.md` is parsed as
`(*go* AND !*test*) OR *.md`.

The only universal wildcard is `*` (matches everything). `**` is normalised
to `*`.

### Examples

```
*.go                   — all Go files
*go*&!*test*           — Go files, excluding tests
(*go*|*.md)&!*vendor*  — Go or Markdown, excluding vendor
!(*.log|*.tmp)         — anything except logs and temp files
```

## API

### FindSeq *(recommended)*

```go
func FindSeq(ctx context.Context, where, pattern string, opts ...Option) iter.Seq2[string, error]
```

Iterator-based search. Yields `(path, nil)` for each match and `("", err)`
for errors. Setup errors (bad path, malformed pattern) appear on the first
iteration. Use `break` to stop early.

### Find

```go
func Find(ctx context.Context, where string, pattern string, opts ...Option) ([]string, error)
```

Finds all matches into a slice and returns them. Kept for backward
compatibility — prefer `FindSeq` for streaming results without allocating a
result slice. Accepts `string` or `[]string` patterns.

### ParseTemplate

```go
func ParseTemplate(str string) (*Template, error)
```

Parses a pattern string into a reusable `*Template`. Returns
`ErrMalformedPattern` for invalid input (empty segments, unbalanced
parentheses). Useful when you need to match paths independently of a
directory walk.

## Options

Options follow the standard Go functional-options pattern. Build reusable
option slices with `[]find.Option{...}`.

| Option            | Description                                            |
|-------------------|--------------------------------------------------------|
| `Only(t)`         | Filter by type: `File`, `Folder`, or `Both` (default). |
| `Recursive`       | Search subdirectories recursively.                     |
| `FollowSymlinks`  | Resolve and follow symlinks during traversal.          |
| `NamesOnly`       | Return entry names only, not full paths.               |
| `MatchFullPath`   | Match against the full path, not just the entry name.  |
| `RelativePaths`   | Keep paths relative to *where*.                        |
| `SkipErrors`      | Silently ignore non-critical errors (`Find` only).     |
| `Max(n)`          | Limit the number of results.                           |
| `CaseInsensitive` | Case-insensitive pattern matching.                     |

## Usage examples

### Recursive search with max results

```go
for path, err := range find.FindSeq(ctx, ".", "*.go",
    find.Recursive,
    find.Max(10),
) {
    if err != nil {
        log.Println(err)
        continue
    }
    fmt.Println(path)
}
```

### Case-insensitive search for folders

```go
for path, err := range find.FindSeq(ctx, "/var", "logs",
    find.Recursive,
    find.Only(find.Folder),
    find.CaseInsensitive,
) {
    if err != nil {
        log.Println(err)
        continue
    }
    fmt.Println(path)
}
```

### Reusable option sets

```go
opts := []find.Option{
    find.Recursive,
    find.CaseInsensitive,
    find.Only(find.File),
}

for path, err := range find.FindSeq(ctx, root, "*.go", opts...) {
    // ...
}
```

### Using ParseTemplate directly

```go
tmpl, err := find.ParseTemplate("*go*&!*test*")
if err != nil {
    log.Fatal(err)
}

if tmpl.Match("cmd/server/main.go") {
    fmt.Println("matched")
}
```
