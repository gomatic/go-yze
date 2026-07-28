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

// TestScope declares which files an analyzer's findings apply to.
//
// Test code is a different kind of code, and most rules about production design
// are wrong when applied to it: a table-driven test's anonymous struct is the
// idiom, not a defect; a `want` field is a fine boolean name; a test double may
// construct an ad-hoc error. Rules of that shape declare TestScopeSourceOnly and
// their findings in _test.go files are dropped.
//
// The zero value is TestScopeAll, so an analyzer that says nothing keeps
// reporting everywhere — a scope is opted INTO, never inherited by accident.
type TestScope string

// The available test scopes.
const (
	// TestScopeAll reports findings in every file. The default.
	TestScopeAll TestScope = ""
	// TestScopeSourceOnly drops findings located in _test.go files.
	TestScopeSourceOnly TestScope = "source-only"
)

// Registration declares one analyzer's identity and taxonomy to the framework.
type Registration struct {
	Analyzer   *analysis.Analyzer
	Name       AnalyzerName
	URL        HelpURL
	TestScope  TestScope
	Categories []Category
}

// WithTestScope returns a copy of the registration carrying the given scope, so
// a catalog can declare the policy centrally without every analyzer repository
// restating it.
func (r Registration) WithTestScope(scope TestScope) Registration {
	r.TestScope = scope
	return r
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
