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

	err := goyze.ApplyConfig([]goyze.Registration{reg}, goyze.Settings{
		"dummy": {"allow": "pkg.Foo,pkg.Bar"},
	})

	require.NoError(t, err)
	assert.Equal(t, "pkg.Foo,pkg.Bar", *allow)
}

func TestApplyConfigIgnoresUnknownAnalyzer(t *testing.T) {
	reg, _ := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, goyze.Settings{
		"other": {"allow": "x"},
	})

	require.NoError(t, err)
}

func TestApplyConfigRejectsUnknownSetting(t *testing.T) {
	reg, _ := dummyReg()

	err := goyze.ApplyConfig([]goyze.Registration{reg}, goyze.Settings{
		"dummy": {"nope": "x"},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrUnknownSetting))
	// An unknown setting name is NOT an invalid value: the sentinels are distinct.
	assert.False(t, errors.Is(err, goyze.ErrInvalidSettingValue))
}

func TestApplyConfigRejectsInvalidValue(t *testing.T) {
	reg, _ := dummyReg()

	// "count" IS a supported setting; only its value is bad. The error must name
	// the value defect, not pretend the setting is unknown.
	err := goyze.ApplyConfig([]goyze.Registration{reg}, goyze.Settings{
		"dummy": {"count": "not-a-number"},
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrInvalidSettingValue))
	// A bad value for a known setting is NOT an unknown setting.
	assert.False(t, errors.Is(err, goyze.ErrUnknownSetting))
}
