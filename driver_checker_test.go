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

// noAnalyze is a graphAnalyzer that fails the test if invoked; load validation
// must reject the run before analysis starts.
func noAnalyze(t *testing.T) graphAnalyzer {
	t.Helper()
	return func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		t.Fatal("analyze must not run after a load validation failure")
		return nil, nil
	}
}

// loadOf is a packageLoader returning fixed packages.
func loadOf(pkgs ...*packages.Package) packageLoader {
	return func(_ []Pattern) ([]*packages.Package, error) { return pkgs, nil }
}

// erroredPkg is a package carrying the given load errors, mimicking the
// placeholder package packages.Load returns for an unmatchable pattern.
func erroredPkg(errors ...packages.Error) *packages.Package {
	return &packages.Package{ID: "./...", Errors: errors}
}

// goWorkErr is the exact list error packages.Load attaches when an active
// go.work workspace does not include the target module.
var goWorkErr = packages.Error{
	Kind: packages.ListError,
	Msg:  "pattern ./...: directory prefix . does not contain modules listed in go.work or their selected dependencies",
}

func TestDriveWithFailsWhenPatternsMatchNoPackages(t *testing.T) {
	_, _, err := driveWith(loadOf(), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "no packages matched patterns: ./...")
}

func TestDriveWithAllowsEmptyPatternsWithNoPackages(t *testing.T) {
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		return &checker.Graph{}, nil
	}

	_, results, err := driveWith(loadOf(), analyze, nil, nil)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDriveWithFailsOnGoWorkExcludedModule(t *testing.T) {
	_, _, err := driveWith(loadOf(erroredPkg(goWorkErr)), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), goWorkErr.Msg, "the loader's own mismatch text must reach the caller")
	assert.Contains(t, err.Error(), goWorkHint)
}

func TestDriveWithLoadErrorIncludesPosition(t *testing.T) {
	_, _, err := driveWith(loadOf(erroredPkg(
		packages.Error{Pos: "a.go:3:7", Msg: "undefined: b", Kind: packages.TypeError},
	)), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "a.go:3:7: undefined: b")
	assert.NotContains(t, err.Error(), goWorkHint, "no workspace hint without a go.work mismatch")
}

func TestDriveWithLoadErrorOmitsDashPosition(t *testing.T) {
	_, _, err := driveWith(loadOf(erroredPkg(
		packages.Error{Pos: "-", Msg: "positionless failure", Kind: packages.ListError},
	)), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "positionless failure")
	assert.NotContains(t, err.Error(), "-: positionless failure")
}

func TestDriveWithLoadErrorsTruncateToFirstFew(t *testing.T) {
	_, _, err := driveWith(loadOf(erroredPkg(
		packages.Error{Pos: "a.go:1:1", Msg: "one", Kind: packages.TypeError},
		packages.Error{Pos: "a.go:2:1", Msg: "two", Kind: packages.TypeError},
		packages.Error{Pos: "a.go:3:1", Msg: "three", Kind: packages.TypeError},
		packages.Error{Pos: "a.go:4:1", Msg: "four", Kind: packages.TypeError},
		packages.Error{Pos: "a.go:5:1", Msg: "five", Kind: packages.TypeError},
	)), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "a.go:3:1: three")
	assert.NotContains(t, err.Error(), "four")
	assert.Contains(t, err.Error(), "... and 2 more error(s)")
}

func TestDriveWithFailsOnDependencyLoadErrors(t *testing.T) {
	dep := &packages.Package{
		ID:     "example.com/dep",
		Errors: []packages.Error{{Pos: "dep.go:1:1", Msg: "broken dep", Kind: packages.TypeError}},
	}
	root := &packages.Package{
		ID:      "example.com/root",
		Fset:    token.NewFileSet(),
		Imports: map[string]*packages.Package{"example.com/dep": dep},
	}

	_, _, err := driveWith(loadOf(root), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "dep.go:1:1: broken dep")
}

func TestLoadErrorsVisitsSharedDependencyOnce(t *testing.T) {
	dep := &packages.Package{ID: "d", Errors: []packages.Error{{Msg: "broken dep"}}}
	a := &packages.Package{ID: "a", Imports: map[string]*packages.Package{"d": dep}}
	b := &packages.Package{ID: "b", Imports: map[string]*packages.Package{"d": dep}}

	assert.Len(t, loadErrors([]*packages.Package{a, b, dep}), 1, "a diamond dependency's errors must not repeat")
}

func TestLoadErrorsWalksImportsInSortedPathOrder(t *testing.T) {
	root := &packages.Package{ID: "root", Imports: map[string]*packages.Package{
		"z": {ID: "z", Errors: []packages.Error{{Msg: "zed"}}},
		"a": {ID: "a", Errors: []packages.Error{{Msg: "aye"}}},
	}}

	got := loadErrors([]*packages.Package{root})

	require.Len(t, got, 2)
	assert.Equal(t, "aye", got[0].Msg)
	assert.Equal(t, "zed", got[1].Msg)
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
