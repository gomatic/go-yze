package goyze

import (
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

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
// requires.
func defaultLoad(patterns []Pattern) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{Mode: packages.LoadAllSyntax}, patternStrings(patterns)...)
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
	analyzers, byAnalyzer := indexAnalyzers(regs)
	graph, err := analyze(analyzers, pkgs)
	if err != nil {
		return nil, nil, err
	}
	return fsetOf(pkgs), rootResults(graph, byAnalyzer), nil
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
