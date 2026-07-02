package goyze

import (
	"strings"

	errs "github.com/gomatic/go-error"
	"golang.org/x/tools/go/packages"
)

// ErrVerifyLoad reports that the packages could not be reloaded for post-fix
// verification.
const ErrVerifyLoad errs.Const = "cannot reload packages for verification"

// VerifyIssue is one parse or type error found when reloading the tree after
// fixes were applied. Pos is the loader's "file:line:col" position and may be
// empty (or "-") when the error carries no position.
type VerifyIssue struct {
	Pos string `json:"pos,omitempty"`
	Msg string `json:"msg"`
}

// String renders the issue as "file:line:col: message", or just the message
// when the issue carries no position.
func (i VerifyIssue) String() string {
	if file := issueFile(posParam(i.Pos)); file == "" {
		return i.Msg
	}
	return i.Pos + ": " + i.Msg
}

// VerifyResult is the outcome of reloading the tree after fixes were applied.
type VerifyResult struct {
	Issues []VerifyIssue
}

// Clean reports whether the reloaded tree carried no parse or type errors.
func (r VerifyResult) Clean() bool { return len(r.Issues) == 0 }

// Files counts the distinct files the issues point at. Issues without a
// position share one "unknown" bucket, so the count is never zero while issues
// remain.
func (r VerifyResult) Files() int {
	files := map[string]struct{}{}
	for _, issue := range r.Issues {
		files[issueFile(posParam(issue.Pos))] = struct{}{}
	}
	return len(files)
}

// posParam names the pos parameter of issueFile; rename it to the real domain concept.
type posParam string

// issueFile extracts the file part of a loader position ("file:line:col"),
// returning "" for a positionless issue.
func issueFile(pos posParam) string {
	if string(pos) == "-" {
		return ""
	}
	if i := strings.Index(string(pos), ":"); i >= 0 {
		return string(pos)[:i]
	}
	return string(pos)
}

// Verifier reloads the given package patterns — test files included — and
// returns every residual parse or type error. It is the seam between a fix
// applier and a concrete loader (the default is CheckerVerifier), so callers
// can verify a tree still compiles after edits without shelling out to a real
// build in their tests.
type Verifier func(patterns []Pattern) (VerifyResult, error)

// CheckerVerifier is the default Verifier: it reloads the patterns through
// packages.Load with Tests set, so _test.go files — which analysis drivers do
// not load — are type-checked too, and collects every package error.
func CheckerVerifier(patterns []Pattern) (VerifyResult, error) {
	return verifyWith(verifyLoad, patterns)
}

// verifyLoad loads the patterns with full syntax/type information plus their
// test files, which is what surfaces breakage in _test.go callers.
func verifyLoad(patterns []Pattern) ([]*packages.Package, error) {
	return packages.Load(&packages.Config{Mode: packages.LoadAllSyntax, Tests: true}, patternStrings(patterns)...)
}

// verifyWith is the testable core of CheckerVerifier: load, then collect every
// package's errors into one deduplicated result.
func verifyWith(load packageLoader, patterns []Pattern) (VerifyResult, error) {
	pkgs, err := load(patterns)
	if err != nil {
		return VerifyResult{}, ErrVerifyLoad.With(err)
	}
	return VerifyResult{Issues: collectIssues(pkgs)}, nil
}

// collectIssues gathers every loaded package's errors in encounter order,
// dropping duplicates — with Tests set the loader returns a package, its test
// variant, and its test binary, which all repeat the same underlying errors.
func collectIssues(pkgs []*packages.Package) []VerifyIssue {
	seen := map[VerifyIssue]struct{}{}
	var issues []VerifyIssue
	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			issue := VerifyIssue{Pos: e.Pos, Msg: e.Msg}
			if _, dup := seen[issue]; dup {
				continue
			}
			seen[issue] = struct{}{}
			issues = append(issues, issue)
		}
	}
	return issues
}
