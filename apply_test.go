package goyze_test

import (
	"errors"
	"testing"

	errs "github.com/gomatic/go-error"
	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	failFormat := func(src []byte) ([]byte, error) { return nil, boom }

	_, err := goyze.ApplyFixes(fs.read, fs.write, failFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 1, NewText: "X"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrFormat))
}

func TestApplyFixesReportsWriteError(t *testing.T) {
	fs := newMemFS(map[string]string{"a.go": "abc"})
	boom := errs.Const("disk full")
	failWrite := func(path string, data []byte) error { return boom }

	_, err := goyze.ApplyFixes(fs.read, failWrite, identityFormat, []goyze.Fix{
		fix("a.go", goyze.TextEdit{Start: 0, End: 1, NewText: "X"}),
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrWriteFile))
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
