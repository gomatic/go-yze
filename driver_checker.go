package goyze

import (
	"go/token"

	errs "github.com/gomatic/go-error"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// ErrLoadPackages reports that the package loader produced no usable packages:
// a non-empty pattern list that matched nothing, or packages carrying load,
// parse, or type errors. Without this gate the checker silently skips errored
// packages and the run degrades to a false pass with zero diagnostics — e.g.
// under an active go.work workspace that does not include the target module,
// packages.Load returns one placeholder package whose only content is a
// "directory prefix . does not contain modules listed in go.work" list error.
const ErrLoadPackages errs.Const = "failed to load packages"

// ErrAnalyzer reports an analyzer whose Run returned an error. The checker
// records a failed Run on its action (Action.Err) rather than failing
// Analyze — whose own error covers only setup — so without this gate a failed
// analyzer silently contributes zero diagnostics and the run degrades to a
// false pass, the same failure mode ErrLoadPackages guards against.
const ErrAnalyzer errs.Const = "analyzer failed"

// Injected collaborators behind CheckerDriver, so its error and mapping paths are
// testable without loading real packages.
type (
	packageLoader func(patterns []Pattern) ([]*packages.Package, error)
	graphAnalyzer func(analyzers []*analysis.Analyzer, pkgs []*packages.Package) (*checker.Graph, error)
)

// CheckerDriver is the default Driver: it loads the patterns' packages and runs
// the registered analyzers through the go/analysis checker.
func CheckerDriver(regs []Registration, patterns []Pattern) (*token.FileSet, []DriverResult, error) {
	return driveWith(defaultLoad, defaultAnalyze, regs, patterns)
}

// defaultLoad loads packages with the full syntax/type information the checker
// requires, plus module identity (NeedModule) so analyzers can distinguish the
// analyzed module's own types from foreign ones (ptrparam's foreign-convention
// rule reads pass.Module).
//
// Tests:true is REQUIRED, not an optimization. Without it the loader presents
// only each package's non-test files, so every analyzer that inspects _test.go
// files — errtest, testfile, errtested, invariant — sees an empty test surface
// and can never report anything. They shipped as gating analyzers that were
// silently incapable of failing; verified by a fixture whose banned
// error-expectation shape the standalone analyzer reports and the suite driver
// did not. It is also what makes deduplication in collect necessary: a
// package's non-test files appear in BOTH the plain package and its test
// variant, so an analyzer reporting on them fires once per variant.
func defaultLoad(patterns []Pattern) ([]*packages.Package, error) {
	return packages.Load(
		&packages.Config{Mode: packages.LoadAllSyntax | packages.NeedModule, Tests: true},
		patternStrings(patterns)...,
	)
}

// patternStrings projects domain patterns onto the plain strings packages.Load
// expects.
func patternStrings(patterns []Pattern) []string {
	out := make([]string, len(patterns))
	for i, p := range patterns {
		out[i] = string(p)
	}
	return out
}

// defaultAnalyze runs the analyzers over the loaded packages.
func defaultAnalyze(analyzers []*analysis.Analyzer, pkgs []*packages.Package) (*checker.Graph, error) {
	return checker.Analyze(analyzers, pkgs, nil)
}

// driveWith is the testable core of CheckerDriver: load, analyze, then map root
// actions back to their registrations.
func driveWith(
	load packageLoader,
	analyze graphAnalyzer,
	regs []Registration,
	patterns []Pattern,
) (*token.FileSet, []DriverResult, error) {
	pkgs, err := load(patterns)
	if err != nil {
		return nil, nil, err
	}
	if err = validateLoad(patterns, pkgs); err != nil {
		return nil, nil, err
	}
	analyzers, byAnalyzer := indexAnalyzers(regs)
	graph, err := analyze(analyzers, pkgs)
	if err != nil {
		return nil, nil, err
	}
	if err := analyzerError(graph); err != nil {
		return nil, nil, err
	}
	return fsetOf(pkgs), rootResults(graph, byAnalyzer), nil
}

// analyzerError reports the first root action whose analysis failed. A root
// carrying only the checker's synthetic "failed prerequisites" error is
// drilled down to the dependency action whose own Run produced the failure,
// so the analyzer named and the cause wrapped are the real ones.
func analyzerError(graph *checker.Graph) error {
	for _, act := range graph.Roots {
		if act.Err != nil {
			fail := failedAction(act)
			return ErrAnalyzer.With(fail.Err, "analyzer", fail.Analyzer.Name)
		}
	}
	return nil
}

// failedAction walks a failed action's dependencies to the deepest failed
// action — the one whose own Run returned the error rather than inheriting a
// prerequisite's failure.
func failedAction(act *checker.Action) *checker.Action {
	for _, dep := range act.Deps {
		if dep.Err != nil {
			return failedAction(dep)
		}
	}
	return act
}

// indexAnalyzers extracts the analyzers to run and a reverse lookup from each
// analyzer to the registration that owns it.
func indexAnalyzers(regs []Registration) ([]*analysis.Analyzer, map[*analysis.Analyzer]Registration) {
	analyzers := make([]*analysis.Analyzer, 0, len(regs))
	byAnalyzer := make(map[*analysis.Analyzer]Registration, len(regs))
	for _, r := range regs {
		analyzers = append(analyzers, r.Analyzer)
		byAnalyzer[r.Analyzer] = r
	}
	return analyzers, byAnalyzer
}

// rootResults gathers each root action's diagnostics under its registration,
// skipping any action whose analyzer was not registered.
func rootResults(graph *checker.Graph, byAnalyzer map[*analysis.Analyzer]Registration) []DriverResult {
	results := make([]DriverResult, 0, len(graph.Roots))
	for _, act := range graph.Roots {
		reg, ok := byAnalyzer[act.Analyzer]
		if !ok {
			continue
		}
		results = append(results, DriverResult{Registration: reg, Diagnostics: act.Diagnostics})
	}
	return results
}

// fsetOf returns the FileSet shared by the loaded packages, or a fresh one when no
// packages were loaded.
func fsetOf(pkgs []*packages.Package) *token.FileSet {
	if len(pkgs) == 0 {
		return token.NewFileSet()
	}
	return pkgs[0].Fset
}
