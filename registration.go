package goyze

import (
	errs "github.com/gomatic/go-error"
	"golang.org/x/tools/go/analysis"
)

// Registration validation errors.
const (
	// ErrMissingName reports a Registration with no analyzer name.
	ErrMissingName errs.Const = "registration is missing a name"
	// ErrMissingGroup reports a Registration with no group.
	ErrMissingGroup errs.Const = "registration is missing a group"
	// ErrMissingAnalyzer reports a Registration with no underlying analyzer.
	ErrMissingAnalyzer errs.Const = "registration is missing an analyzer"
)

// Group is the single, stable name segment shared by every analyzer in a repo
// family — the <group> in a yze-<group>-<name> repo. It is the organizing axis
// embedded in the module path (the default axis is the language/target).
type Group string

// Category is a many-to-many semantic tag carried as metadata. An analyzer may
// belong to several categories; categories drive filtering and documentation and
// are deliberately decoupled from Group.
type Category string

// Registration declares one analyzer's identity and taxonomy to the framework.
type Registration struct {
	Name       string
	Group      Group
	Categories []Category
	URL        string
	Analyzer   *analysis.Analyzer
}

// RuleID returns the stable rule identifier "yze/<group>/<name>" carried by every
// Diagnostic the analyzer emits.
func (r Registration) RuleID() string {
	return "yze/" + string(r.Group) + "/" + r.Name
}

// Validate reports the first way a Registration is not well-formed.
func (r Registration) Validate() error {
	if r.Name == "" {
		return ErrMissingName
	}
	if r.Group == "" {
		return ErrMissingGroup
	}
	if r.Analyzer == nil {
		return ErrMissingAnalyzer
	}
	return nil
}
