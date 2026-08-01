package goyze

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Validating a package load before any analysis runs. The checker SKIPS
// packages carrying load or type errors, so a module that does not compile
// would otherwise produce zero diagnostics and a clean exit — a gate that
// asserted nothing. Everything here exists to make that outcome impossible.

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
