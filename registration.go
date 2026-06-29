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

// AnalyzerName is an analyzer's stable identifier, used as its rule-id suffix and
// as the key a Settings map targets.
type AnalyzerName string

// HelpURL is the documentation URL stamped onto every Diagnostic an analyzer emits.
type HelpURL string

// Registration declares one analyzer's identity and taxonomy to the framework.
type Registration struct {
	Analyzer   *analysis.Analyzer
	Name       AnalyzerName
	URL        HelpURL
	Categories []Category
}

// RuleID returns the stable rule identifier "yze/<name>" carried by every
// Diagnostic the analyzer emits.
func (r Registration) RuleID() string {
	return "yze/" + string(r.Name)
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
