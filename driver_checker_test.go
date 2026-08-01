package goyze

import (
	"fmt"
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

func TestDriveWithAllowsEmptyPatternsWithNoPackages(t *testing.T) {
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		return &checker.Graph{}, nil
	}

	_, results, err := driveWith(loadOf(), analyze, nil, nil)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDriveWithFailsWhenRootActionErrs(t *testing.T) {
	runErr := errs.Const("probe exploded")
	a := &analysis.Analyzer{Name: "probe"}
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		return &checker.Graph{Roots: []*checker.Action{{Analyzer: a, Err: runErr}}}, nil
	}

	_, _, err := driveWith(
		loadOf(&packages.Package{Fset: token.NewFileSet()}), analyze, []Registration{regWith(a)}, []Pattern{"./..."},
	)

	require.ErrorIs(t, err, ErrAnalyzer)
	require.ErrorIs(t, err, runErr, "the analyzer's own Run error must stay matchable through the sentinel")
	assert.Contains(t, err.Error(), "probe", "the failing analyzer must be named")
}

func TestDriveWithFailsWhenDependencyActionErrs(t *testing.T) {
	depErr := errs.Const("inspector exploded")
	healthy := &checker.Action{Analyzer: &analysis.Analyzer{Name: "healthy"}}
	dep := &checker.Action{Analyzer: &analysis.Analyzer{Name: "inspect"}, Err: depErr}
	root := &checker.Action{
		Analyzer: &analysis.Analyzer{Name: "parent"},
		// The checker stamps a synthetic "failed prerequisites" error on the
		// root; the real cause lives on the failed dependency action.
		Err:  errs.Const("failed prerequisites: inspect@p"),
		Deps: []*checker.Action{healthy, dep},
	}
	analyze := func(_ []*analysis.Analyzer, _ []*packages.Package) (*checker.Graph, error) {
		return &checker.Graph{Roots: []*checker.Action{root}}, nil
	}

	_, _, err := driveWith(
		loadOf(&packages.Package{Fset: token.NewFileSet()}),
		analyze, []Registration{regWith(root.Analyzer)}, []Pattern{"./..."},
	)

	require.ErrorIs(t, err, ErrAnalyzer)
	require.ErrorIs(t, err, depErr, "the failed dependency's own error is the cause")
	assert.Contains(t, err.Error(), "inspect", "the analyzer blamed is the one whose Run actually failed")
}

func TestCheckerDriverFailsWhenAnalyzerRunErrs(t *testing.T) {
	runErr := errs.Const("probe run failed")
	probe := &analysis.Analyzer{
		Name: "proberr",
		Doc:  "always fails",
		Run:  func(*analysis.Pass) (any, error) { return nil, runErr },
	}

	_, _, err := CheckerDriver([]Registration{regWith(probe)}, []Pattern{"."})

	require.ErrorIs(t, err, ErrAnalyzer, "a failed Run must never degrade to a clean pass")
	require.ErrorIs(t, err, runErr)
	assert.Contains(t, err.Error(), "proberr")
}

func TestCheckerDriverFailsWhenRequiredAnalyzerRunErrs(t *testing.T) {
	depErr := errs.Const("required analyzer failed")
	child := &analysis.Analyzer{
		Name: "childerr",
		Doc:  "always fails",
		Run:  func(*analysis.Pass) (any, error) { return nil, depErr },
	}
	parent := &analysis.Analyzer{
		Name:     "parenterr",
		Doc:      "requires childerr",
		Requires: []*analysis.Analyzer{child},
		Run:      func(*analysis.Pass) (any, error) { return nil, nil },
	}

	_, _, err := CheckerDriver([]Registration{regWith(parent)}, []Pattern{"."})

	// Verified against x/tools v0.47.0 (checker.go execOnce): a failed
	// dependency IS reflected on the root action as a synthetic "failed
	// prerequisites" error, so scanning graph.Roots suffices to fail the run;
	// the dependency walk recovers the real cause and analyzer name.
	require.ErrorIs(t, err, ErrAnalyzer)
	require.ErrorIs(t, err, depErr, "the required analyzer's own error is the cause")
	assert.Contains(t, err.Error(), "childerr", "the dependency whose Run failed must be named")
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

	// The loader runs with Tests:true, so one pattern yields several package
	// variants — the plain package, its internal-test variant, the external
	// test package, and the generated test main. Every variant is analyzed,
	// which is exactly what makes test-file analyzers able to fire at all.
	require.Greater(t, len(results), 1, "each package variant is analyzed")
	for _, result := range results {
		assert.NotEmpty(t, result.Diagnostics, "every variant reports")
	}

	// collect collapses the duplicates those variants produce, so the reported
	// finding appears once per distinct position rather than once per variant.
	report := collect(fset, results)
	positions := map[string]int{}
	for _, d := range report.Diagnostics {
		positions[fmt.Sprintf("%s:%d:%d", d.Path, d.Line, d.Col)]++
	}
	for at, count := range positions {
		assert.Equal(t, 1, count, "duplicate diagnostic at %s", at)
	}
}
