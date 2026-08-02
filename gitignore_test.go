package goyze

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// pkgWithFiles builds a package carrying the given Go files.
func pkgWithFiles(id string, files ...string) *packages.Package {
	return &packages.Package{ID: id, GoFiles: files}
}

// TestFilterDropsFullyIgnoredPackagesOnly pins the core rule: a package whose
// every Go file is ignored is dropped; one with any tracked file is kept; a
// package with no Go files is kept (nothing to drop).
func TestFilterDropsFullyIgnoredPackagesOnly(t *testing.T) {
	t.Parallel()

	pkgs := []*packages.Package{
		pkgWithFiles("owned", "/repo/a.go", "/repo/b.go"),
		pkgWithFiles("vendored", "/repo/node_modules/x.go", "/repo/node_modules/y.go"),
		pkgWithFiles("mixed", "/repo/c.go", "/repo/node_modules/z.go"),
		pkgWithFiles("nofiles"),
	}
	check := func([]string) (map[string]bool, error) {
		return map[string]bool{
			"/repo/node_modules/x.go": true,
			"/repo/node_modules/y.go": true,
			"/repo/node_modules/z.go": true,
		}, nil
	}

	kept := filterIgnoredPackages(check, pkgs)

	ids := make([]string, 0, len(kept))
	for _, p := range kept {
		ids = append(ids, p.ID)
	}
	assert.Equal(t, []string{"owned", "mixed", "nofiles"}, ids,
		"only the fully-ignored package is dropped")
}

// TestFilterFailsOpen pins the safety contract: a check error, and a check
// reporting nothing ignored, both return the packages unchanged — the filter
// never drops real source on uncertainty.
func TestFilterFailsOpen(t *testing.T) {
	t.Parallel()

	pkgs := []*packages.Package{pkgWithFiles("owned", "/repo/a.go")}

	errored := filterIgnoredPackages(func([]string) (map[string]bool, error) {
		return nil, errors.New("git missing")
	}, pkgs)
	assert.Equal(t, pkgs, errored, "a check error keeps every package")

	none := filterIgnoredPackages(func([]string) (map[string]bool, error) {
		return map[string]bool{}, nil
	}, pkgs)
	assert.Equal(t, pkgs, none, "nothing ignored keeps every package")

	// An error alongside a non-empty set is a misbehaving check; the error
	// wins and every package is kept — the partial set is never trusted to
	// drop source. (gitCheckIgnore never returns both, but the guard must.)
	ignored := []*packages.Package{pkgWithFiles("owned", "/repo/a.go")}
	untrusted := filterIgnoredPackages(func([]string) (map[string]bool, error) {
		return map[string]bool{"/repo/a.go": true}, errors.New("partial failure")
	}, ignored)
	assert.Equal(t, ignored, untrusted, "an error with a partial set keeps every package")
}

// TestAllGoFilesUnionsInOrder pins the file collection the check receives.
func TestAllGoFilesUnionsInOrder(t *testing.T) {
	t.Parallel()

	files := allGoFiles([]*packages.Package{
		pkgWithFiles("a", "/x/1.go"),
		pkgWithFiles("b"),
		pkgWithFiles("c", "/y/2.go", "/y/3.go"),
	})

	assert.Equal(t, []string{"/x/1.go", "/y/2.go", "/y/3.go"}, files)
}

// TestGitCheckIgnoreEmptyInput pins that no files means no work and no error.
func TestGitCheckIgnoreEmptyInput(t *testing.T) {
	t.Parallel()

	ignored, err := gitCheckIgnore(nil)

	require.NoError(t, err)
	assert.Empty(t, ignored)
}

// TestGitCheckIgnoreAgainstARealRepo drives the default check over a real git
// repository: a file matched by .gitignore is reported, a tracked file is not.
func TestGitCheckIgnoreAgainstARealRepo(t *testing.T) {
	dir, symErr := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, symErr)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}
	run("init", "-q")
	require.NoError(t, os.WriteFile(dir+"/.gitignore", []byte("ignored.go\n"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/ignored.go", []byte("package x\n"), 0o644))
	require.NoError(t, os.WriteFile(dir+"/tracked.go", []byte("package x\n"), 0o644))

	// An out-of-tree file (build/module cache, another repo) is included: the
	// check must silently drop it rather than let git's fatal "outside
	// repository" error fail the whole invocation.
	outside := t.TempDir() + "/elsewhere.go"
	ignored, err := gitCheckIgnoreIn(workDir(dir), []string{dir + "/ignored.go", dir + "/tracked.go", outside})

	require.NoError(t, err)
	assert.True(t, ignored[dir+"/ignored.go"], "the .gitignore-matched file is ignored")
	assert.False(t, ignored[dir+"/tracked.go"], "the tracked file is not ignored")
	assert.False(t, ignored[outside], "an out-of-tree file is dropped from the query, never ignored")

	// Querying only tracked files: git check-ignore exits 1 (nothing ignored),
	// which is a clean empty answer, not a failure.
	none, noneErr := gitCheckIgnoreIn(workDir(dir), []string{dir + "/tracked.go"})
	require.NoError(t, noneErr)
	assert.Empty(t, none, "an exit-1 'nothing ignored' is an empty set, not an error")

	// A path that is lexically under the tree but resolves outside it (via ..)
	// passes the prefix filter yet makes git check-ignore fail fatally; the
	// error is surfaced so the caller fails open rather than trusting a partial
	// answer.
	_, escapeErr := gitCheckIgnoreIn(workDir(dir), []string{dir + "/../escape.go"})
	assert.Error(t, escapeErr, "a check-ignore fatal error is surfaced, not swallowed")
}

// TestGitCheckIgnoreOutsideARepoFailsOpen pins that a path in no git
// repository yields an error, which the caller turns into fail-open.
func TestGitCheckIgnoreOutsideARepoFailsOpen(t *testing.T) {
	t.Parallel()

	// A path under the system temp root, with no repository above the check's
	// own working directory set to that isolated dir.
	dir := t.TempDir()
	_, err := gitCheckIgnoreIn(workDir(dir), []string{dir + "/x.go"})

	assert.Error(t, err, "check-ignore outside a repo is a real failure, not 'nothing ignored'")
}

// TestFilesUnderKeepsOnlyInTreePaths pins the in-tree filter that keeps git
// check-ignore from failing on the build- and module-cache paths packages.Load
// mixes in.
func TestFilesUnderKeepsOnlyInTreePaths(t *testing.T) {
	t.Parallel()

	kept := filesUnder("/repo", []string{
		"/repo/a.go",
		"/repo/sub/b.go",
		"/cache/go-build/x-d",
		"/reponot/c.go",
	})

	assert.Equal(t, []string{"/repo/a.go", "/repo/sub/b.go"}, kept,
		"only paths under the root's directory tree survive; a sibling prefix does not")
}

// TestGitCheckIgnoreOnlyQueriesInTreeFiles pins that an input list of only
// out-of-tree files yields an empty answer without a git failure — the
// synthetic test-main packages must not fail the filter open.
func TestGitCheckIgnoreOnlyQueriesInTreeFiles(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	ignored, err := gitCheckIgnoreIn(workDir(dir), []string{t.TempDir() + "/cache.go"})

	require.NoError(t, err, "an all-out-of-tree query is empty, not a failure")
	assert.Empty(t, ignored)
}

// TestExitCodeClassifiesErrors pins the exit-code extraction: nil is 0, an
// ExitError yields its code, and a non-exit error (a start failure) is not a
// meaningful code.
func TestExitCodeClassifiesErrors(t *testing.T) {
	t.Parallel()

	code, ok := exitCode(nil)
	assert.True(t, ok)
	assert.Equal(t, 0, code)

	err := exec.Command("false").Run()
	code, ok = exitCode(err)
	assert.True(t, ok, "a non-zero exit is a meaningful code")
	assert.Equal(t, 1, code)

	_, ok = exitCode(errors.New("not an exit error"))
	assert.False(t, ok, "a start failure carries no exit code")
}
