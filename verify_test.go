package goyze

import (
	"testing"

	errs "github.com/gomatic/go-error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// These tests live in package goyze (white-box) to exercise the unexported
// verifyWith seam with an injected loader collaborator.

func pkgWithErrors(errors ...packages.Error) *packages.Package {
	return &packages.Package{Errors: errors}
}

func TestVerifyWithReturnsLoadError(t *testing.T) {
	boom := errs.Const("load failed")
	load := func(_ []Pattern) ([]*packages.Package, error) { return nil, boom }

	_, err := verifyWith(load, []Pattern{"./..."})

	require.ErrorIs(t, err, ErrVerifyLoad)
	require.ErrorIs(t, err, boom)
}

func TestVerifyWithCleanPackagesIsClean(t *testing.T) {
	load := func(_ []Pattern) ([]*packages.Package, error) {
		return []*packages.Package{pkgWithErrors()}, nil
	}

	result, err := verifyWith(load, []Pattern{"./..."})

	require.NoError(t, err)
	assert.True(t, result.Clean())
	assert.Zero(t, result.Files())
}

func TestVerifyWithCollectsIssuesInEncounterOrder(t *testing.T) {
	load := func(_ []Pattern) ([]*packages.Package, error) {
		return []*packages.Package{
			pkgWithErrors(packages.Error{Pos: "b.go:2:1", Msg: "undefined: gone"}),
			pkgWithErrors(packages.Error{Pos: "a_test.go:5:2", Msg: "too many arguments"}),
		}, nil
	}

	result, err := verifyWith(load, []Pattern{"./..."})

	require.NoError(t, err)
	assert.False(t, result.Clean())
	assert.Equal(t, []VerifyIssue{
		{Pos: "b.go:2:1", Msg: "undefined: gone"},
		{Pos: "a_test.go:5:2", Msg: "too many arguments"},
	}, result.Issues)
}

func TestVerifyWithDedupesRepeatedErrorsAcrossTestVariants(t *testing.T) {
	// With Tests set the loader returns a package, its test variant, and its test
	// binary, all repeating the same underlying errors.
	repeated := packages.Error{Pos: "a.go:1:1", Msg: "boom"}
	load := func(_ []Pattern) ([]*packages.Package, error) {
		return []*packages.Package{
			pkgWithErrors(repeated),
			pkgWithErrors(repeated, packages.Error{Pos: "a_test.go:3:4", Msg: "boom"}),
			pkgWithErrors(repeated),
		}, nil
	}

	result, err := verifyWith(load, []Pattern{"./..."})

	require.NoError(t, err)
	require.Len(t, result.Issues, 2)
	assert.Equal(t, VerifyIssue{Pos: "a.go:1:1", Msg: "boom"}, result.Issues[0])
	assert.Equal(t, VerifyIssue{Pos: "a_test.go:3:4", Msg: "boom"}, result.Issues[1])
}

func TestVerifyIssueStringRendersPositionThenMessage(t *testing.T) {
	assert.Equal(t, "a.go:1:2: boom", VerifyIssue{Pos: "a.go:1:2", Msg: "boom"}.String())
}

func TestVerifyIssueStringOmitsMissingPosition(t *testing.T) {
	assert.Equal(t, "boom", VerifyIssue{Msg: "boom"}.String())
	assert.Equal(t, "boom", VerifyIssue{Pos: "-", Msg: "boom"}.String())
}

func TestVerifyResultFilesCountsDistinctFiles(t *testing.T) {
	result := VerifyResult{Issues: []VerifyIssue{
		{Pos: "a.go:1:1", Msg: "one"},
		{Pos: "a.go:9:9", Msg: "two"},
		{Pos: "b_test.go:3:2", Msg: "three"},
	}}

	assert.Equal(t, 2, result.Files())
}

func TestVerifyResultFilesBucketsPositionlessIssuesOnce(t *testing.T) {
	result := VerifyResult{Issues: []VerifyIssue{
		{Pos: "-", Msg: "one"},
		{Msg: "two"},
		{Pos: "a.go", Msg: "position without line"},
	}}

	// The two positionless issues share one bucket; the colonless position is its
	// own file.
	assert.Equal(t, 2, result.Files())
}

func TestCheckerVerifierReportsCleanForThisPackage(t *testing.T) {
	result, err := CheckerVerifier([]Pattern{"."})

	require.NoError(t, err)
	assert.True(t, result.Clean(), "this package (tests included) must type-check: %v", result.Issues)
}
