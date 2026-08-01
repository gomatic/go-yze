package goyze_test

import (
	"errors"
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goyze "github.com/gomatic/go-yze"
)

// memFS is an in-memory filesystem used to drive ApplyFixes without touching disk.
type memFS struct {
	files   map[string]string
	written map[string]string
}

func newMemFS(files map[string]string) *memFS {
	return &memFS{files: files, written: map[string]string{}}
}

func (m *memFS) read(path string) ([]byte, error) {
	content, ok := m.files[path]
	if !ok {
		return nil, errors.New("no such file")
	}
	return []byte(content), nil
}

func (m *memFS) write(path string, data []byte) error {
	m.written[path] = string(data)
	return nil
}

func identityFormat(src []byte) ([]byte, error) { return src, nil }

func fix(path string, edits ...goyze.TextEdit) goyze.Fix {
	return goyze.Fix{Files: []goyze.FileEdit{{Path: path, Edits: edits}}}
}

func TestApplyFixesRewritesAndFormatsSingleFile(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "hello world"})

	res, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 6, End: 11, NewText: "gophers"}),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, res.FilesChanged)
	assert.Equal(t, 1, res.EditsApplied)
	assert.Equal(t, "hello gophers", fs.written["a.go"])
}

func TestApplyFixesMergesEditsFromMultipleFixesIntoOneFile(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "the quick brown fox"})

	res, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 4, End: 9, NewText: "slow"}),
		fix("a.go", goyze.TextEdit{Start: 16, End: 19, NewText: "dog"}),
	})

	require.NoError(t, err)
	assert.Equal(t, 1, res.FilesChanged)
	assert.Equal(t, 2, res.EditsApplied)
	assert.Equal(t, "the slow brown dog", fs.written["a.go"])
}

func TestApplyFixesAcrossMultipleFiles(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "aaa", "b.go": "bbb"})

	res, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 1, NewText: "X"}),
		fix("b.go", goyze.TextEdit{Start: 2, End: 3, NewText: "Y"}),
	})

	require.NoError(t, err)
	assert.Equal(t, 2, res.FilesChanged)
	assert.Equal(t, "Xaa", fs.written["a.go"])
	assert.Equal(t, "bbY", fs.written["b.go"])
}

func TestApplyFixesDedupesIdenticalEditsAcrossFixes(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "hello world"})

	res, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 6, End: 11, NewText: "gophers"}),
		fix("a.go", goyze.TextEdit{Start: 6, End: 11, NewText: "gophers"}),
	})

	require.NoError(t, err, "two fixes proposing the identical edit must not abort as overlapping")
	assert.Equal(t, 1, res.FilesChanged)
	assert.Equal(t, 1, res.EditsApplied, "a deduplicated edit counts once")
	assert.Equal(t, "hello gophers", fs.written["a.go"])
}

func TestApplyFixesReportsReadError(t *testing.T) {
	fs := newMemFS(map[string]string{})

	_, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("missing.go", goyze.TextEdit{Start: 0, End: 0, NewText: "x"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrReadFile))
}

func TestApplyFixesPropagatesOverlapError(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "hello world"})

	_, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 5, NewText: "x"}),
		fix("a.go", goyze.TextEdit{Start: 3, End: 8, NewText: "y"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrOverlappingEdits))
}

func TestApplyFixesReportsFormatError(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "abc"})
	boom := errs.Const("boom")
	failFormat := func(_ []byte) ([]byte, error) { return nil, boom }

	_, err := goyze.ApplyFixes(fs.read, fs.write, failFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 1, NewText: "X"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrFormat))
}

func TestApplyFixesReportsWriteError(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "abc"})
	boom := errs.Const("disk full")
	failWrite := func(_ string, _ []byte) error { return boom }

	_, err := goyze.ApplyFixes(fs.read, failWrite, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 1, NewText: "X"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrWriteFile))
}

func TestApplyFixesSkipsEmptyFileEditWithoutTouchingFile(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "abc"})
	// A format that errors if ever invoked proves the empty edit short-circuits
	// before the file is read/reformatted/written.
	boom := errs.Const("format must not run for an empty FileEdit")
	failFormat := func(_ []byte) ([]byte, error) { return nil, boom }

	res, err := goyze.ApplyFixes(fs.read, fs.write, failFormat, []goyze.Fix{
		{Description: "no-op", Files: []goyze.FileEdit{{Path: "a.go"}}},
	})

	require.NoError(t, err)
	assert.Zero(t, res.FilesChanged)
	assert.Zero(t, res.EditsApplied)
	assert.Empty(t, fs.written)
	assert.Equal(t, "abc", fs.files["a.go"], "the file must remain byte-identical")
}

func TestApplyFixesWithNoFixesChangesNothing(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "abc"})

	res, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, nil)

	require.NoError(t, err)
	assert.Zero(t, res.FilesChanged)
	assert.Empty(t, fs.written)
}

func TestGoFormatFormatsValidGoAndRejectsInvalid(t *testing.T) {
	formatted, err := goyze.GoFormat([]byte("package p\nvar  x   =1"))
	require.NoError(t, err)
	assert.Contains(t, string(formatted), "var x = 1")

	_, err = goyze.GoFormat([]byte("package ???"))
	require.Error(t, err)
}

// TestGroupEditsDropsDuplicatesAndSkipsEmptyFileEdits names groupEdits' two
// claims, both of which are about what must NOT happen to a user's source.
//
// Independent analyzers regularly propose the identical edit — two rules that
// both want the same import added, say. Without deduplication those identical
// edits either abort the whole file as overlapping (so every real fix in it is
// lost) or, for a pure insertion, both apply and insert the text twice.
//
// The empty-FileEdit case is quieter and worse in a different way: a fix
// carrying no edits for a file would still create an entry, and that file would
// then be read, REFORMATTED and rewritten. A rule that found nothing would show
// up in a diff as a whole-file reformat.
func TestGroupEditsDropsDuplicatesAndSkipsEmptyFileEdits(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	fs := newMemFS(map[string]string{"a.go": "package a\n", "untouched.go": "package  a\n"})
	same := goyze.TextEdit{Start: 9, End: 9, NewText: "b"}

	result, err := goyze.ApplyFixes(fs.read, fs.write, identityFormat, []goyze.Fix{
		fix("a.go", same),
		fix("a.go", same),
		{Files: []goyze.FileEdit{{Path: "untouched.go"}}},
	})

	must.NoError(err, "two fixes proposing the identical edit must not abort as overlapping")
	want.Equal("package ab\n", fs.written["a.go"], "the identical edit applies once, not twice")
	want.NotContains(fs.written, "untouched.go",
		"a fix with no edits must not cause the file to be rewritten")
	want.Equal(1, result.EditsApplied, "the applied-edit count must not be inflated by the duplicate")
}

// TestDedupeEditsPreservesFirstSeenOrder names dedupeEdits' ordering claim.
// Deduplication that reordered would change which edit survives when two carry
// the same range, and the surviving replacement text is what lands in the
// user's file — so order here is a correctness property, not a cosmetic one.
func TestDedupeEditsPreservesFirstSeenOrder(t *testing.T) {
	t.Parallel()
	want, must := assert.New(t), require.New(t)

	first := goyze.TextEdit{Start: 0, End: 0, NewText: "1"}
	second := goyze.TextEdit{Start: 5, End: 5, NewText: "2"}

	got, err := goyze.ApplyEdits([]byte("hello world"), []goyze.TextEdit{first, second, first, second})

	must.NoError(err)
	want.Equal("1hello2 world", string(got), "each distinct edit applies exactly once, in position order")
}

// TestEditLessIsATotalOrder names editLess' claim. Sorting on Start alone
// leaves two edits sharing a Start in an order the sort may pick either way, so
// the overlap verdict and the spliced result would differ between runs on
// identical input — a fix tool whose output is not reproducible. Ordering by
// Start, then End, then text makes the answer the same every time.
func TestEditLessIsATotalOrder(t *testing.T) {
	t.Parallel()
	must := require.New(t)

	// Two pure insertions at the same offset: only the secondary keys separate
	// them, and their relative order decides the spliced text.
	edits := []goyze.TextEdit{
		{Start: 5, End: 5, NewText: "B"},
		{Start: 5, End: 5, NewText: "A"},
	}

	first, err := goyze.ApplyEdits([]byte("hello world"), edits)
	must.NoError(err)

	for range 20 {
		again, err := goyze.ApplyEdits([]byte("hello world"), []goyze.TextEdit{edits[1], edits[0]})
		must.NoError(err)
		must.Equal(string(first), string(again),
			"the result must not depend on the order the edits arrived in")
	}
	must.Equal("helloAB world", string(first), "ties break on NewText, ascending")
}
