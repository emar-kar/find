package find

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
)

// testTree creates a temporary directory structure for testing:
//
//	root/
//	  file.go
//	  file.txt
//	  file_test.go
//	  README.md
//	  sub/
//	    nested.go
//	    nested_test.go
//	    deep/
//	      deep.go
func testTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	dirs := []string{
		"sub",
		filepath.Join("sub", "deep"),
	}

	files := []string{
		"file.go",
		"file.txt",
		"file_test.go",
		"README.md",
		filepath.Join("sub", "nested.go"),
		filepath.Join("sub", "nested_test.go"),
		filepath.Join("sub", "deep", "deep.go"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

// names extracts basenames from a list of absolute paths.
func names(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}

	sort.Strings(out)

	return out
}

// --- ParseTemplate tests ---

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{"wildcard contains", "*go*", "file.go", true},
		{"wildcard contains negative", "*go*", "file.txt", false},
		{"wildcard suffix", "*.go", "file.go", true},
		{"wildcard suffix negative", "*.go", "file.txt", false},
		{"wildcard prefix", "file*", "file.go", true},
		{"wildcard prefix negative", "file*", "other.go", false},
		{"exact segment", "file.go", "file.go", true},
		{"exact segment negative", "file.go", "file.txt", false},
		{"exact in path", "sub", "foo/sub/bar", true},
		{"negation", "!*.txt", "file.go", true},
		{"negation negative", "!*.txt", "file.txt", false},
		{"and", "*go*&!*test*", "file.go", true},
		{"and negative", "*go*&!*test*", "file_test.go", false},
		{"or", "*.go|*.txt", "file.go", true},
		{"or second", "*.go|*.txt", "file.txt", true},
		{"or negative", "*.go|*.txt", "README.md", false},
		{"precedence and-or", "*go*&!*test*|*.md", "file_test.go", false},
		{"precedence and-or md", "*go*&!*test*|*.md", "README.md", true},
		{"parens override", "(*go*|*.md)&!*test*", "file.go", true},
		{"parens override negative", "(*go*|*.md)&!*test*", "file_test.go", false},
		{"negated group", "!(*.go|*.txt)", "README.md", true},
		{"negated group negative", "!(*.go|*.txt)", "file.go", false},
		{"universal wildcard", "*", "anything", true},
		{"double wildcard normalised", "**", "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := ParseTemplate(tt.pattern)
			if err != nil {
				t.Fatalf("ParseTemplate(%q) error: %v", tt.pattern, err)
			}

			if got := tmpl.Match(tt.input); got != tt.want {
				t.Errorf("ParseTemplate(%q).Match(%q) = %v, want %v",
					tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTemplateMalformed(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"trailing pipe", "a|"},
		{"leading pipe", "|b"},
		{"trailing ampersand", "a&"},
		{"empty string", ""},
		{"unmatched open paren", "(a|b"},
		{"unmatched close paren", "a|b)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTemplate(tt.pattern)
			if err == nil {
				t.Fatalf("ParseTemplate(%q) expected error, got nil", tt.pattern)
			}

			if !errors.Is(err, ErrMalformedPattern) {
				t.Errorf("ParseTemplate(%q) error = %v, want %v",
					tt.pattern, err, ErrMalformedPattern)
			}
		})
	}
}

// --- FindSeq tests ---

func TestFindSeqBasic(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*.go") {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	want := []string{"file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("FindSeq(*.go) names = %v, want %v", g, want)
	}
}

func TestFindSeqRecursive(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*.go", Recursive) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	want := []string{"deep.go", "file.go", "file_test.go", "nested.go", "nested_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("FindSeq recursive names = %v, want %v", g, want)
	}
}

func TestFindSeqCaseInsensitive(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*.GO", CaseInsensitive) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	want := []string{"file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("FindSeq case-insensitive names = %v, want %v", g, want)
	}
}

func TestFindSeqMax(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*", Max(2)) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	if len(got) != 2 {
		t.Errorf("FindSeq Max(2) returned %d results, want 2", len(got))
	}
}

func TestFindSeqNamesOnly(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*.go", NamesOnly) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	for _, p := range got {
		if filepath.Base(p) != p {
			t.Errorf("NamesOnly returned path %q, expected base name only", p)
		}
	}
}

func TestFindSeqRelativePaths(t *testing.T) {
	root := testTree(t)

	// Use a relative path as 'where' so RelativePaths produces relative output.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Chdir(orig) })

	var got []string
	for p, err := range FindSeq(context.Background(), ".", "*.go", RelativePaths) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	for _, p := range got {
		if filepath.IsAbs(p) {
			t.Errorf("RelativePaths returned absolute path %q", p)
		}
	}
}

func TestFindSeqOnlyFiles(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*", Only(File)) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	// "sub" is a directory and should not appear.
	for _, p := range got {
		if filepath.Base(p) == "sub" {
			t.Error("Only(File) returned directory 'sub'")
		}
	}
}

func TestFindSeqOnlyFolders(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*", Only(Folder)) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	want := []string{"sub"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("Only(Folder) names = %v, want %v", g, want)
	}
}

func TestFindSeqAndPattern(t *testing.T) {
	root := testTree(t)

	var got []string
	for p, err := range FindSeq(context.Background(), root, "*go*&!*test*", Recursive) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p)
	}

	want := []string{"deep.go", "file.go", "nested.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("AND pattern names = %v, want %v", g, want)
	}
}

func TestFindSeqMalformedPattern(t *testing.T) {
	var gotErr error
	for _, err := range FindSeq(context.Background(), ".", "a|") {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for malformed pattern")
	}

	if !errors.Is(gotErr, ErrMalformedPattern) {
		t.Errorf("error = %v, want %v", gotErr, ErrMalformedPattern)
	}
}

func TestFindSeqNonExistentDir(t *testing.T) {
	var gotErr error
	for _, err := range FindSeq(context.Background(), "/no/such/dir", "*.go") {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestFindSeqEarlyBreak(t *testing.T) {
	root := testTree(t)

	n := 0
	for _, err := range FindSeq(context.Background(), root, "*.go", Recursive) {
		if err != nil {
			t.Fatal(err)
		}
		n++
		if n == 1 {
			break
		}
	}

	if n != 1 {
		t.Errorf("early break: got %d iterations, want 1", n)
	}
}

func TestFindSeqContextCancelled(t *testing.T) {
	root := testTree(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var gotErr error
	for _, err := range FindSeq(ctx, root, "*.go") {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected context cancelled error")
	}
}

// --- Find tests ---

func TestFind(t *testing.T) {
	root := testTree(t)

	got, err := Find(context.Background(), root, "*.go")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("Find(*.go) names = %v, want %v", g, want)
	}
}

func TestFindRecursive(t *testing.T) {
	root := testTree(t)

	got, err := Find(context.Background(), root, "*.go", Recursive)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"deep.go", "file.go", "file_test.go", "nested.go", "nested_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("Find recursive names = %v, want %v", g, want)
	}
}

func TestFindSlicePattern(t *testing.T) {
	root := testTree(t)

	pattern := ParsePattern(false, "", "*.go", "*.md", "")
	got, err := Find(context.Background(), root, pattern)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"README.md", "file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("Find(ParsePattern) names = %v, want %v", g, want)
	}
}

func TestFindSlicePatternStrict(t *testing.T) {
	root := testTree(t)

	// Strict AND: must match *.go AND *file* — so file.go and file_test.go match.
	pattern := ParsePattern(true, "", "*.go", "*file*", "")
	got, err := Find(context.Background(), root, pattern)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("Find strict names = %v, want %v", g, want)
	}
}

func TestFindSkipErrors(t *testing.T) {
	// resolvePath pre-check bypasses SkipErrors for root-path errors.
	_, err := Find(context.Background(), "/no/such/dir", "*.go", SkipErrors)
	if err == nil {
		t.Error("expected error for non-existent root, even with SkipErrors")
	}
}

// --- FindWithIterator tests ---

func TestFindWithIterator(t *testing.T) {
	root := testTree(t)

	outCh, errCh := FindWithIterator(context.Background(), root, "*.go")

	var got []string
	for p := range outCh {
		got = append(got, p)
	}

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	want := []string{"file.go", "file_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("FindWithIterator names = %v, want %v", g, want)
	}
}

func TestFindWithIteratorRecursive(t *testing.T) {
	root := testTree(t)

	outCh, errCh := FindWithIterator(context.Background(), root, "*.go", Recursive)

	var got []string
	for p := range outCh {
		got = append(got, p)
	}

	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	want := []string{"deep.go", "file.go", "file_test.go", "nested.go", "nested_test.go"}
	if g := names(got); !slices.Equal(g, want) {
		t.Errorf("FindWithIterator recursive names = %v, want %v", g, want)
	}
}

// --- MatchAny / MatchAll tests ---

func TestMatchAny(t *testing.T) {
	ts := NewTemplates([]string{"*.go", "*.md"})

	if !MatchAny(ts, "file.go") {
		t.Error("MatchAny should match file.go")
	}

	if MatchAny(ts, "file.txt") {
		t.Error("MatchAny should not match file.txt")
	}
}

func TestMatchAll(t *testing.T) {
	ts := NewTemplates([]string{"*file*", "*.go"})

	if !MatchAll(ts, "file.go") {
		t.Error("MatchAll should match file.go")
	}

	if MatchAll(ts, "README.md") {
		t.Error("MatchAll should not match README.md")
	}
}

// benchTree creates a temporary directory tree with the given breadth and
// depth, placing one .go file and one .txt file in each directory.
// Returns the root path and total number of .go files created.
func benchTree(b *testing.B, breadth, depth int) (string, int) {
	b.Helper()

	root := b.TempDir()
	goFiles := 0

	var create func(dir string, level int)
	create = func(dir string, level int) {
		// Place files in every directory.
		if err := os.WriteFile(filepath.Join(dir, "file.go"), nil, 0o644); err != nil {
			b.Fatal(err)
		}
		goFiles++

		if err := os.WriteFile(filepath.Join(dir, "file.txt"), nil, 0o644); err != nil {
			b.Fatal(err)
		}

		if level >= depth {
			return
		}

		for i := range breadth {
			sub := filepath.Join(dir, "d"+string(rune('a'+i)))
			if err := os.Mkdir(sub, 0o755); err != nil {
				b.Fatal(err)
			}

			create(sub, level+1)
		}
	}

	create(root, 0)

	return root, goFiles
}

// --- Flat directory (no recursion) benchmarks ---

func BenchmarkFind_Flat(b *testing.B) {
	root := b.TempDir()
	for i := range 50 {
		name := "file" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + ".go"
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_, err := Find(ctx, root, "*.go")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindWithIterator_Flat(b *testing.B) {
	root := b.TempDir()
	for i := range 50 {
		name := "file" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + ".go"
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		outCh, errCh := FindWithIterator(ctx, root, "*.go")
		for range outCh {
		}

		if err := <-errCh; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindSeq_Flat(b *testing.B) {
	root := b.TempDir()
	for i := range 50 {
		name := "file" + string(rune('A'+i%26)) + string(rune('a'+i/26)) + ".go"
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			b.Fatal(err)
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		for _, err := range FindSeq(ctx, root, "*.go") {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// --- Recursive directory benchmarks ---

func BenchmarkFind_Recursive(b *testing.B) {
	root, _ := benchTree(b, 3, 3) // 3 subdirs x 3 levels = 40 directories
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_, err := Find(ctx, root, "*.go", Recursive)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindWithIterator_Recursive(b *testing.B) {
	root, _ := benchTree(b, 3, 3)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		outCh, errCh := FindWithIterator(ctx, root, "*.go", Recursive)
		for range outCh {
		}

		if err := <-errCh; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindSeq_Recursive(b *testing.B) {
	root, _ := benchTree(b, 3, 3)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		for _, err := range FindSeq(ctx, root, "*.go", Recursive) {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// --- Complex pattern benchmarks ---

func BenchmarkFind_ComplexPattern(b *testing.B) {
	root, _ := benchTree(b, 3, 3)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_, err := Find(ctx, root, "*go*&!*test*", Recursive)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindWithIterator_ComplexPattern(b *testing.B) {
	root, _ := benchTree(b, 3, 3)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		outCh, errCh := FindWithIterator(ctx, root, "*go*&!*test*", Recursive)
		for range outCh {
		}

		if err := <-errCh; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindSeq_ComplexPattern(b *testing.B) {
	root, _ := benchTree(b, 3, 3)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		for _, err := range FindSeq(ctx, root, "*go*&!*test*", Recursive) {
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

// --- Early termination benchmark (Max=1, large tree) ---

func BenchmarkFind_EarlyTermination(b *testing.B) {
	root, _ := benchTree(b, 3, 4)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_, err := Find(ctx, root, "*.go", Recursive, Max(1))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindWithIterator_EarlyTermination(b *testing.B) {
	root, _ := benchTree(b, 3, 4)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		outCh, errCh := FindWithIterator(ctx, root, "*.go", Recursive, Max(1))
		for range outCh {
		}

		if err := <-errCh; err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindSeq_EarlyTermination(b *testing.B) {
	root, _ := benchTree(b, 3, 4)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		n := 0
		for _, err := range FindSeq(ctx, root, "*.go", Recursive) {
			if err != nil {
				b.Fatal(err)
			}
			n++
			if n == 1 {
				break
			}
		}
	}
}

// --- Template construction benchmarks ---

func BenchmarkNewTemplate_Simple(b *testing.B) {
	for range b.N {
		NewTemplate("*.go")
	}
}

func BenchmarkParseTemplate_Simple(b *testing.B) {
	for range b.N {
		_, err := ParseTemplate("*.go")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewTemplate_Complex(b *testing.B) {
	for range b.N {
		NewTemplate("(*go*|*.md)&!*test*&!*vendor*")
	}
}

func BenchmarkParseTemplate_Complex(b *testing.B) {
	for range b.N {
		_, err := ParseTemplate("(*go*|*.md)&!*test*&!*vendor*")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewTemplate_Parens(b *testing.B) {
	for range b.N {
		NewTemplate("((*go*|*.md)&!*test*)|(*.txt&!*log*)")
	}
}

func BenchmarkParseTemplate_Parens(b *testing.B) {
	for range b.N {
		_, err := ParseTemplate("((*go*|*.md)&!*test*)|(*.txt&!*log*)")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewTemplates(b *testing.B) {
	patterns := []string{"*.go", "*.md", "*test*", "!*vendor*"}

	b.ResetTimer()
	for range b.N {
		NewTemplates(patterns)
	}
}

func BenchmarkTemplateMatch(b *testing.B) {
	tmpl, _ := ParseTemplate("*go*&!*test*")

	b.ResetTimer()
	for range b.N {
		tmpl.Match("some/path/to/file.go")
	}
}
