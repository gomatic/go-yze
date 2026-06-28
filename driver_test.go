package goyze_test

import (
	"go/token"
	"testing"

	errs "github.com/gomatic/go-error"
	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

func fakeDriver(fset *token.FileSet, results []goyze.DriverResult, err error) goyze.Driver {
	return func(_ []goyze.Registration, _ []string) (*token.FileSet, []goyze.DriverResult, error) {
		return fset, results, err
	}
}

func TestRunCollectsDiagnosticsFromAllResults(t *testing.T) {
	fset, f := newFile(t)
	results := []goyze.DriverResult{
		{Registration: sampleRegistration(), Diagnostics: []analysis.Diagnostic{{Pos: f.Pos(0), Message: "boom"}}},
		{Registration: sampleRegistration(), Diagnostics: nil},
	}

	report, err := goyze.Run(fakeDriver(fset, results, nil), []goyze.Registration{sampleRegistration()}, []string{"./..."})

	require.NoError(t, err)
	require.Len(t, report.Diagnostics, 1)
	assert.Equal(t, "yze/go/errconst", report.Diagnostics[0].Rule)
	assert.Equal(t, "boom", report.Diagnostics[0].Message)
}

func TestRunValidatesRegistrationsBeforeDriving(t *testing.T) {
	bad := sampleRegistration()
	bad.Name = ""
	called := false
	driver := func(_ []goyze.Registration, _ []string) (*token.FileSet, []goyze.DriverResult, error) {
		called = true
		return nil, nil, nil
	}

	_, err := goyze.Run(driver, []goyze.Registration{bad}, nil)

	require.ErrorIs(t, err, goyze.ErrMissingName)
	assert.False(t, called, "driver must not run when a registration is invalid")
}

func TestRunWrapsDriverFailure(t *testing.T) {
	boom := errs.Const("kaboom")

	_, err := goyze.Run(fakeDriver(nil, nil, boom), []goyze.Registration{sampleRegistration()}, nil)

	require.ErrorIs(t, err, goyze.ErrDriver)
	require.ErrorIs(t, err, boom)
}
