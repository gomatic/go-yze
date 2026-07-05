package goyze_test

import (
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	goyze "github.com/gomatic/go-yze"
)

const convSrc = "package p\n\nfunc f() {}\n"

func newFile(t *testing.T) (*token.FileSet, *token.File) {
	t.Helper()
	fset := token.NewFileSet()
	f := fset.AddFile("foo.go", fset.Base(), len(convSrc))
	f.SetLinesForContent([]byte(convSrc))
	return fset, f
}

func TestToDiagnosticMapsPositionAndMetadata(t *testing.T) {
	fset, f := newFile(t)
	reg := sampleRegistration()

	got := goyze.ToDiagnostic(fset, reg, analysis.Diagnostic{
		Pos:     f.Pos(0),
		Message: "boom",
	})

	assert.Equal(t, "yze", got.Tool)
	assert.Equal(t, "yze/errconst", got.Rule)
	assert.Equal(t, "foo.go", got.Path)
	assert.Equal(t, 1, got.Line)
	assert.Equal(t, 1, got.Col)
	assert.Equal(t, goyze.SeverityError, got.Severity)
	assert.Equal(t, "boom", got.Message)
	assert.Equal(t, string(reg.URL), got.URL)
	assert.Nil(t, got.Fixes)
	assert.Zero(t, got.EndLine)
}

func TestToDiagnosticSetsEndWhenValid(t *testing.T) {
	fset, f := newFile(t)

	got := goyze.ToDiagnostic(fset, sampleRegistration(), analysis.Diagnostic{
		Pos:     f.Pos(0),
		End:     f.Pos(4),
		Message: "boom",
	})

	assert.Equal(t, 1, got.EndLine)
	assert.Equal(t, 5, got.EndCol)
}

func TestToDiagnosticInsertionEditWithInvalidEnd(t *testing.T) {
	fset, f := newFile(t)

	got := goyze.ToDiagnostic(fset, sampleRegistration(), analysis.Diagnostic{
		Pos:     f.Pos(0),
		Message: "boom",
		SuggestedFixes: []analysis.SuggestedFix{{
			Message:   "insert",
			TextEdits: []analysis.TextEdit{{Pos: f.Pos(5), End: token.NoPos, NewText: []byte("x")}},
		}},
	})

	require.Len(t, got.Fixes, 1)
	require.Len(t, got.Fixes[0].Files, 1)
	// analysis.TextEdit permits End == token.NoPos for a pure insertion ("End
	// can either be set to Pos or token.NoPos"); an invalid End means End =
	// Pos, never offset 0 (which would invert the range and abort the fix).
	assert.Equal(t, []goyze.TextEdit{{Start: 5, End: 5, NewText: "x"}}, got.Fixes[0].Files[0].Edits)
}

func TestToDiagnosticConvertsSuggestedFixesGroupedByFile(t *testing.T) {
	fset, f := newFile(t)

	got := goyze.ToDiagnostic(fset, sampleRegistration(), analysis.Diagnostic{
		Pos:     f.Pos(0),
		Message: "boom",
		SuggestedFixes: []analysis.SuggestedFix{
			{
				Message: "rewrite",
				TextEdits: []analysis.TextEdit{
					{Pos: f.Pos(0), End: f.Pos(7), NewText: []byte("package")},
					{Pos: f.Pos(8), End: f.Pos(9), NewText: []byte("q")},
				},
			},
		},
	})

	require.Len(t, got.Fixes, 1)
	assert.Equal(t, "rewrite", got.Fixes[0].Description)
	require.Len(t, got.Fixes[0].Files, 1)
	fe := got.Fixes[0].Files[0]
	assert.Equal(t, "foo.go", fe.Path)
	assert.Equal(t, []goyze.TextEdit{
		{Start: 0, End: 7, NewText: "package"},
		{Start: 8, End: 9, NewText: "q"},
	}, fe.Edits)
}
