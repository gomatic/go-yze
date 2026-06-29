package goyze

import (
	"go/token"

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

// collect normalizes every driver result's diagnostics into one Report.
func collect(fset *token.FileSet, results []DriverResult) Report {
	report := Report{}
	for _, res := range results {
		for _, d := range res.Diagnostics {
			report.Diagnostics = append(report.Diagnostics, ToDiagnostic(fset, res.Registration, d))
		}
	}
	return report
}
