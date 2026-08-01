package goyze

import (
	"go/token"
	"strings"
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

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

func TestDriveWithFailsWhenPatternsMatchNoPackages(t *testing.T) {
	_, _, err := driveWith(loadOf(), noAnalyze(t), nil, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrLoadPackages)
	assert.Contains(t, err.Error(), "no packages matched patterns: ./...")
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

// TestDefaultLoadPresentsTestFilesToAnalyzers names defaultLoad's claim, and it
// is the most consequential regression guard in this package. Tests:true is
// REQUIRED: without it the loader hands analyzers only each package's non-test
// files, so every rule whose whole subject is _test.go — errtest, testfile,
// errtested, invariant — sees an empty test surface and can never report
// anything. Those shipped as GATING analyzers that were silently incapable of
// failing, which is worse than not having them: a green gate asserted a
// property nothing had checked.
//
// The assertion is on the loaded syntax rather than on a diagnostic, so it
// fails for the actual cause — the loader configuration — rather than for
// whichever analyzer happened to be wired up.
func TestDefaultLoadPresentsTestFilesToAnalyzers(t *testing.T) {
	pkgs, err := defaultLoad([]Pattern{"."})
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)

	var sawTestFile, sawSourceFile bool
	for _, pkg := range pkgs {
		for _, path := range pkg.CompiledGoFiles {
			if strings.HasSuffix(path, "_test.go") {
				sawTestFile = true
			} else if strings.HasSuffix(path, ".go") {
				sawSourceFile = true
			}
		}
	}

	assert.True(t, sawSourceFile, "the loader must present the package's source files")
	assert.True(t, sawTestFile,
		"the loader must present _test.go files, or every test-scoped analyzer is silently inert")
}

// TestDefaultLoadPresentsNonTestFilesInBothVariants names the consequence
// defaultLoad's comment calls out: with Tests:true a package's non-test files
// appear in BOTH the plain package and its test variant. That is not a defect,
// it is why collect must deduplicate — and asserting it here means the
// deduplication test below is guarding a real condition rather than a
// hypothetical one.
func TestDefaultLoadPresentsNonTestFilesInBothVariants(t *testing.T) {
	pkgs, err := defaultLoad([]Pattern{"."})
	require.NoError(t, err)

	counts := map[string]int{}
	for _, pkg := range pkgs {
		for _, path := range pkg.CompiledGoFiles {
			if !strings.HasSuffix(path, "_test.go") {
				counts[path]++
			}
		}
	}

	var duplicated int
	for _, n := range counts {
		if n > 1 {
			duplicated++
		}
	}
	assert.Positive(t, duplicated,
		"a non-test file must appear in more than one variant, which is what makes collect's dedup necessary")
}

// TestValidateLoadRejectsALoadThatCannotSupportAnalysis names validateLoad's
// claim. The checker SKIPS packages carrying load or type errors, so a run over
// a module that does not compile would report zero diagnostics and exit clean —
// a green gate that analyzed nothing. Both ways that happens are refused here.
func TestValidateLoadRejectsALoadThatCannotSupportAnalysis(t *testing.T) {
	t.Run("patterns that matched nothing", func(t *testing.T) {
		err := validateLoad([]Pattern{"./does-not-exist/..."}, nil)
		assert.ErrorIs(t, err, ErrLoadPackages)
	})

	t.Run("a package carrying load errors", func(t *testing.T) {
		pkgs := []*packages.Package{{
			PkgPath: "example.test/broken",
			Errors:  []packages.Error{{Msg: "undefined: Thing", Pos: "x.go:3:1"}},
		}}
		err := validateLoad([]Pattern{"./..."}, pkgs)
		require.ErrorIs(t, err, ErrLoadPackages)
		assert.Contains(t, err.Error(), "undefined: Thing", "the cause must reach the operator")
	})

	t.Run("a clean load is accepted", func(t *testing.T) {
		assert.NoError(t, validateLoad([]Pattern{"./..."}, []*packages.Package{{PkgPath: "ok"}}))
	})

	t.Run("no patterns and no packages is not an error", func(t *testing.T) {
		assert.NoError(t, validateLoad(nil, nil))
	})
}
