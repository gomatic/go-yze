package goyze

import (
	"fmt"
	"go/token"
	"sort"
	"strings"

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
func defaultLoad(patterns []Pattern) ([]*packages.Package, error) {
	return packages.Load(
		&packages.Config{Mode: packages.LoadAllSyntax | packages.NeedModule},
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
	return fsetOf(pkgs), rootResults(graph, byAnalyzer), nil
}

// maxLoadErrors caps how many loader errors are echoed in the failure message;
// the remainder is summarized as a count.
const maxLoadErrors = 3

// goWorkHint is appended to the failure message when the loader blames an
// active Go workspace for the mismatch (a go.work file — named by the loader's
// own error text — that does not include the target module).
const goWorkHint = "hint: an active Go workspace (go.work) does not include the target module — " +
	"add the module to the workspace or run with GOWORK=off"

// validateLoad rejects a load that cannot support analysis: a non-empty pattern
// list that matched no packages, or any package (dependencies included) carrying
// load, parse, or type errors. The checker silently skips errored packages, so
// without this gate such a run would return zero diagnostics and exit clean.
func validateLoad(patterns []Pattern, pkgs []*packages.Package) error {
	if len(patterns) > 0 && len(pkgs) == 0 {
		return ErrLoadPackages.With(nil, "no packages matched patterns:", strings.Join(patternStrings(patterns), " "))
	}
	issues := loadErrors(pkgs)
	if len(issues) == 0 {
		return nil
	}
	return ErrLoadPackages.With(nil, formatLoadErrors(issues))
}

// loadErrors collects every package's load errors, dependencies included,
// visiting each package once in deterministic import order (what
// packages.Visit does for packages.PrintErrors, without the pointer-callback
// signature packages.Visit imposes).
func loadErrors(pkgs []*packages.Package) []packages.Error {
	seen := map[string]struct{}{}
	queue := append([]*packages.Package{}, pkgs...)
	var all []packages.Error
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		if _, dup := seen[pkg.ID]; dup {
			continue
		}
		seen[pkg.ID] = struct{}{}
		all = append(all, pkg.Errors...)
		queue = append(queue, importsOf(pkg.Imports)...)
	}
	return all
}

// importsOf returns a package's imports in deterministic (sorted path) order.
func importsOf(imports map[string]*packages.Package) []*packages.Package {
	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	deps := make([]*packages.Package, 0, len(paths))
	for _, path := range paths {
		deps = append(deps, imports[path])
	}
	return deps
}

// formatLoadErrors renders the first maxLoadErrors errors as file:line lines, a
// count of any remainder, and the go.work hint when an error names a workspace
// mismatch.
func formatLoadErrors(issues []packages.Error) string {
	shown := issues[:min(len(issues), maxLoadErrors)]
	parts := make([]string, 0, len(shown)+2)
	for _, e := range shown {
		parts = append(parts, errorLine(e))
	}
	if rest := len(issues) - len(shown); rest > 0 {
		parts = append(parts, fmt.Sprintf("... and %d more error(s)", rest))
	}
	if mentionsGoWork(issues) {
		parts = append(parts, goWorkHint)
	}
	return strings.Join(parts, "\n")
}

// errorLine renders one loader error as "file:line:col: message", or just the
// message when the error carries no position.
func errorLine(e packages.Error) string {
	if e.Pos == "" || e.Pos == "-" {
		return e.Msg
	}
	return e.Pos + ": " + e.Msg
}

// mentionsGoWork reports whether any loader error blames a go.work workspace.
func mentionsGoWork(issues []packages.Error) bool {
	for _, e := range issues {
		if strings.Contains(e.Msg, "go.work") {
			return true
		}
	}
	return false
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
