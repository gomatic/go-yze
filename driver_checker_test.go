package goyze

import (
	"go/token"
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// These tests live in package goyze (white-box) to exercise the unexported
// driveWith seam with injected loader/analyzer collaborators.

func regWith(a *analysis.Analyzer) Registration {
	return Registration{Name: AnalyzerName(a.Name), Analyzer: a}
}

func TestDriveWithHappyPathReturnsResults(t *testing.T) {
	fset := token.NewFileSet()
	f := fset.AddFile("x.go", fset.Base(), len("package p\n"))
	f.SetLinesForContent([]byte("package p\n"))
	a := &analysis.Analyzer{Name: "triv"}
	reg := regWith(a)

	load := func(_ []Pattern) ([]*packages.Package, error) {
		return []*packages.Package{{Fset: fset}}, nil
	}
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		return &checker.Graph{Roots: []*checker.Action{
			{Analyzer: a, Diagnostics: []analysis.Diagnostic{{Pos: f.Pos(0), Message: "boom"}}},
		}}, nil
	}

	gotFset, results, err := driveWith(load, analyze, []Registration{reg}, []Pattern{"./..."})

	require.NoError(t, err)
	assert.Same(t, fset, gotFset)
	require.Len(t, results, 1)
	assert.Equal(t, reg, results[0].Registration)
	require.Len(t, results[0].Diagnostics, 1)
	assert.Equal(t, "boom", results[0].Diagnostics[0].Message)
}

func TestDriveWithReturnsLoadError(t *testing.T) {
	boom := errs.Const("load failed")
	load := func(_ []Pattern) ([]*packages.Package, error) { return nil, boom }
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		t.Fatal("analyze must not run after a load error")
		return nil, nil
	}

	_, _, err := driveWith(load, analyze, nil, nil)

	require.ErrorIs(t, err, boom)
}

func TestDriveWithReturnsAnalyzeError(t *testing.T) {
	boom := errs.Const("analyze failed")
	load := func(_ []Pattern) ([]*packages.Package, error) {
		return []*packages.Package{{Fset: token.NewFileSet()}}, nil
	}
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) { return nil, boom }

	_, _, err := driveWith(load, analyze, nil, nil)

	require.ErrorIs(t, err, boom)
}

func TestRootResultsSkipsUnregisteredAnalyzers(t *testing.T) {
	known := &analysis.Analyzer{Name: "known"}
	foreign := &analysis.Analyzer{Name: "foreign"}
	reg := regWith(known)
	graph := &checker.Graph{Roots: []*checker.Action{
		{Analyzer: known, Diagnostics: nil},
		{Analyzer: foreign, Diagnostics: nil},
	}}

	results := rootResults(graph, map[*analysis.Analyzer]Registration{known: reg})

	require.Len(t, results, 1)
	assert.Equal(t, reg, results[0].Registration)
}

func TestFsetOfReturnsFreshSetForNoPackages(t *testing.T) {
	assert.NotNil(t, fsetOf(nil))
}

func TestFsetOfReturnsFirstPackageFset(t *testing.T) {
	fset := token.NewFileSet()
	assert.Same(t, fset, fsetOf([]*packages.Package{{Fset: fset}}))
}

func TestCheckerDriverRunsRealAnalyzerOverThisPackage(t *testing.T) {
	triv := &analysis.Analyzer{
		Name: "triv",
		Doc:  "reports once per file",
		Run: func(pass *analysis.Pass) (any, error) {
			pass.Reportf(pass.Files[0].Pos(), "triv was here")
			return nil, nil
		},
	}
	reg := regWith(triv)

	fset, results, err := CheckerDriver([]Registration{reg}, []Pattern{"."})

	require.NoError(t, err)
	require.NotNil(t, fset)
	require.Len(t, results, 1)
	assert.NotEmpty(t, results[0].Diagnostics)
}
