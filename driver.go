package goyze

import (
	"go/token"
	"strings"

	errs "github.com/gomatic/go-error"
	"golang.org/x/tools/go/analysis"
)

// ErrDriver reports that the underlying analysis driver failed to run.
const ErrDriver errs.Const = "analysis driver failed"

// DriverResult is one analyzer's findings from a driver run, paired with the
// registration that produced them so positions and metadata can be normalized.
type DriverResult struct {
	Registration Registration
	Diagnostics  []analysis.Diagnostic
}

// Pattern is a package pattern (e.g. "./...") naming the packages an analyzer
// run targets.
type Pattern string

// Driver runs the registered analyzers over the given package patterns and
// returns the shared FileSet plus per-analyzer findings. It is the seam between
// the framework and a concrete analysis backend (the default is CheckerDriver).
type Driver func(regs []Registration, patterns []Pattern) (*token.FileSet, []DriverResult, error)

// Run validates the registrations, executes them through the driver, and
// normalizes every finding into a Report (the native stickler-json model).
func Run(driver Driver, regs []Registration, patterns []Pattern) (Report, error) {
	if err := validateAll(regs); err != nil {
		return Report{}, err
	}
	fset, results, err := driver(regs, patterns)
	if err != nil {
		return Report{}, ErrDriver.With(err)
	}
	return collect(fset, results), nil
}

// validateAll reports the first invalid registration.
func validateAll(regs []Registration) error {
	for _, r := range regs {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// outOfScope reports whether a finding falls outside its analyzer's declared
// test scope — a source-only rule reporting inside a _test.go file.
func outOfScope(reg Registration, diag Diagnostic) bool {
	return reg.TestScope == TestScopeSourceOnly && strings.HasSuffix(diag.Path, "_test.go")
}

// diagnosticKey identifies a finding for deduplication. The loader presents a
// package's non-test files in both the plain package and its test variant (see
// defaultLoad), so an analyzer reporting on those files produces the identical
// finding once per variant; without this the same line is reported twice.
type diagnosticKey struct {
	rule    string
	path    string
	message string
	line    int
	col     int
}

// keyOf projects a diagnostic onto its identity for deduplication.
func keyOf(d Diagnostic) diagnosticKey {
	return diagnosticKey{rule: d.Rule, path: d.Path, message: d.Message, line: d.Line, col: d.Col}
}

// collect normalizes every driver result's diagnostics into one Report,
// dropping findings in generated files (see generated.go) and collapsing the
// duplicates the test-variant load produces.
func collect(fset *token.FileSet, results []DriverResult) Report {
	report := Report{}
	generated := generatedFiles{read: osReadHead, seen: map[filePath]bool{}}
	seen := map[diagnosticKey]bool{}
	for _, res := range results {
		report.Diagnostics = append(report.Diagnostics, kept(fset, res, generated, seen)...)
	}
	return report
}

// kept normalizes one driver result's diagnostics, skipping generated files and
// findings already reported by another package variant.
func kept(fset *token.FileSet, res DriverResult, generated generatedFiles, seen map[diagnosticKey]bool) []Diagnostic {
	out := make([]Diagnostic, 0, len(res.Diagnostics))
	for _, d := range res.Diagnostics {
		diag := ToDiagnostic(fset, res.Registration, d)
		if outOfScope(res.Registration, diag) || generated.isGenerated(filePath(diag.Path)) || seen[keyOf(diag)] {
			continue
		}
		seen[keyOf(diag)] = true
		out = append(out, diag)
	}
	return out
}
