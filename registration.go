package goyze

import (
	errs "github.com/gomatic/go-error"
	"golang.org/x/tools/go/analysis"
)

// Registration validation errors.
const (
	// ErrMissingName reports a Registration with no analyzer name.
	ErrMissingName errs.Const = "registration is missing a name"
	// ErrMissingAnalyzer reports a Registration with no underlying analyzer.
	ErrMissingAnalyzer errs.Const = "registration is missing an analyzer"
)

// Category is a many-to-many semantic tag carried as metadata. An analyzer may
// belong to several categories; categories drive filtering and documentation.
type Category string

// Registration declares one analyzer's identity and taxonomy to the framework.
type Registration struct {
	Analyzer   *analysis.Analyzer
	Name       string
	URL        string
	Categories []Category
}

// RuleID returns the stable rule identifier "yze/<name>" carried by every
// Diagnostic the analyzer emits.
func (r Registration) RuleID() string {
	return "yze/" + r.Name
}

// Validate reports the first way a Registration is not well-formed.
func (r Registration) Validate() error {
	if r.Name == "" {
		return ErrMissingName
	}
	if r.Analyzer == nil {
		return ErrMissingAnalyzer
	}
	return nil
}
