package goyze_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	goyze "github.com/gomatic/go-yze"
)

func sampleRegistration() goyze.Registration {
	return goyze.Registration{
		Name:       "errconst",
		Categories: []goyze.Category{"errors"},
		URL:        "https://docs.gomatic.dev/yze/errconst",
		Analyzer:   &analysis.Analyzer{Name: "errconst", Doc: "checks sentinel error constants"},
	}
}

func TestRegistrationRuleID(t *testing.T) {
	assert.Equal(t, "yze/errconst", sampleRegistration().RuleID())
}

func TestRegistrationValidateAcceptsCompleteRegistration(t *testing.T) {
	require.NoError(t, sampleRegistration().Validate())
}

func TestRegistrationValidateRejectsMissingName(t *testing.T) {
	reg := sampleRegistration()
	reg.Name = ""

	err := reg.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrMissingName))
}

func TestRegistrationValidateRejectsMissingAnalyzer(t *testing.T) {
	reg := sampleRegistration()
	reg.Analyzer = nil

	err := reg.Validate()

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrMissingAnalyzer))
}

// TestWithTestScopeCopiesRatherThanMutates pins that a catalog can declare the
// test-file policy centrally without altering the analyzer package's own
// registration value.
func TestWithTestScopeCopiesRatherThanMutates(t *testing.T) {
	t.Parallel()

	original := goyze.Registration{Name: "thing", Analyzer: &analysis.Analyzer{Name: "thing"}}

	scoped := original.WithTestScope(goyze.TestScopeSourceOnly)

	assert.Equal(t, goyze.TestScopeSourceOnly, scoped.TestScope)
	assert.Equal(t, goyze.TestScopeAll, original.TestScope, "the original is unchanged")
	assert.Equal(t, original.Name, scoped.Name)
}
