package goyze_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	goyze "github.com/gomatic/go-yze"
)

func dummyReg() (goyze.Registration, *string) {
	a := &analysis.Analyzer{Name: "dummy"}
	var allow string
	var count int
	a.Flags.StringVar(&allow, "allow", "", "")
	a.Flags.IntVar(&count, "count", 0, "")
	return goyze.Registration{Name: "dummy", Analyzer: a}, &allow
}

func TestApplyConfigSetsKnownSettings(t *testing.T) {
	reg, allow := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, map[string]map[string]string{
		"dummy": {"allow": "pkg.Foo,pkg.Bar"},
	})

	require.NoError(t, err)
	assert.Equal(t, "pkg.Foo,pkg.Bar", *allow)
}

func TestApplyConfigIgnoresUnknownAnalyzer(t *testing.T) {
	reg, _ := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, map[string]map[string]string{
		"other": {"allow": "x"},
	})

	require.NoError(t, err)
}

func TestApplyConfigRejectsUnknownSetting(t *testing.T) {
	reg, _ := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, map[string]map[string]string{
		"dummy": {"nope": "x"},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrUnknownSetting))
}

func TestApplyConfigRejectsInvalidValue(t *testing.T) {
	reg, _ := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, map[string]map[string]string{
		"dummy": {"count": "not-a-number"},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrUnknownSetting))
}
