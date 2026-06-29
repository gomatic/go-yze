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
