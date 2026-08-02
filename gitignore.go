package goyze

// Dropping git-ignored packages before analysis. A file git does not track is
// not the repository's code — a vendored third-party tree under node_modules,
// a local build artifact — and no gate may judge it. `packages.Load("./...")`
// descends into such directories (Go has no notion of .gitignore), so a
// gitignored .go file would otherwise be held to the same standard as owned
// source and fail the build over code nobody here wrote.
//
// The filter fails OPEN: when git is unavailable, the files are not in a
// repository, or the check cannot be run, every package is kept. Silently
// dropping real source would turn a gate green over unanalyzed code — the one
// outcome worse than judging a stray vendored file.

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"golang.org/x/tools/go/packages"
)

// checkIgnore reports which of the given files git ignores. It is the seam a
// test replaces to drive the filter without a real repository.
type checkIgnore func(files []string) (map[string]bool, error)

// gitCheckIgnore is the default checkIgnore: one `git check-ignore` over all
// files at once, run from the process working directory — which during a lint
// pass is the repository root — reading the ignored subset from its
// NUL-delimited output.
func gitCheckIgnore(files []string) (map[string]bool, error) {
	return gitCheckIgnoreIn("", files)
}

// workDir is the directory git check-ignore runs from; empty means the process
// working directory, which during a lint pass is the repository root.
type workDir string

// gitCheckIgnoreIn runs the check from a specific working directory, so a test
// can drive it against an isolated repository. An empty dir uses the process
// working directory.
func gitCheckIgnoreIn(dir workDir, files []string) (map[string]bool, error) {
	if len(files) == 0 {
		return map[string]bool{}, nil
	}
	cmd := exec.Command("git", "check-ignore", "--stdin", "-z")
	cmd.Dir = string(dir)
	cmd.Stdin = strings.NewReader(strings.Join(files, "\x00"))
	var out bytes.Buffer
	cmd.Stdout = &out
	// git check-ignore exits 1 when NOTHING is ignored and 128 on a real
	// failure (not a repository, git missing). Only 0 and 1 are meaningful
	// answers; anything else is surfaced so the caller can fail open.
	err := cmd.Run()
	if code, ok := exitCode(err); ok && code == 1 {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	return ignoredSet(out.Bytes()), nil
}

// exitCode extracts a command's process exit code, reporting whether err is an
// ordinary non-zero exit (as opposed to a start failure like a missing binary).
func exitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), true
	}
	return 0, false
}

// ignoredSet parses git check-ignore -z output (a NUL-delimited list of the
// ignored paths) into a set.
func ignoredSet(out []byte) map[string]bool {
	ignored := map[string]bool{}
	for _, path := range strings.Split(string(out), "\x00") {
		if path != "" {
			ignored[path] = true
		}
	}
	return ignored
}

// filterIgnoredPackages drops every package all of whose Go files git ignores,
// keeping any package with at least one tracked Go file. A package with no Go
// files is kept (there is nothing to judge as ignored). The check runs once
// over the union of every package's files; on any error it fails open and
// returns the packages unchanged.
func filterIgnoredPackages(check checkIgnore, pkgs []*packages.Package) []*packages.Package {
	ignored, err := check(allGoFiles(pkgs))
	if err != nil || len(ignored) == 0 {
		return pkgs
	}
	kept := make([]*packages.Package, 0, len(pkgs))
	for _, pkg := range pkgs {
		if !allIgnored(ignored, pkg.GoFiles) {
			kept = append(kept, pkg)
		}
	}
	return kept
}

// allGoFiles is the union of every package's Go files, in encounter order.
func allGoFiles(pkgs []*packages.Package) []string {
	var files []string
	for _, pkg := range pkgs {
		files = append(files, pkg.GoFiles...)
	}
	return files
}

// allIgnored reports whether files is non-empty and every entry is ignored — a
// package the filter drops. An empty file list is not "all ignored": there is
// nothing to drop.
func allIgnored(ignored map[string]bool, files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !ignored[f] {
			return false
		}
	}
	return true
}
