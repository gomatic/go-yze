package goyze_test

import (
	"encoding/json"
	"errors"
	"testing"

	goyze "github.com/gomatic/go-yze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleReport() goyze.Report {
	return goyze.Report{
		Diagnostics: []goyze.Diagnostic{
			{
				Tool:     "yze",
				Rule:     "yze/go/errconst",
				Path:     "pkg/foo/foo.go",
				Line:     12,
				Col:      5,
				EndLine:  12,
				EndCol:   20,
				Severity: goyze.SeverityError,
				Message:  "use a sentinel Error constant",
				URL:      "https://docs.gomatic.dev/yze/go/errconst",
				Fixes: []goyze.Fix{
					{
						Description: "convert to errs.Const",
						Files: []goyze.FileEdit{
							{
								Path:  "pkg/foo/foo.go",
								Edits: []goyze.TextEdit{{Start: 100, End: 130, NewText: "const ErrX errs.Const = \"x\""}},
							},
						},
					},
				},
			},
		},
	}
}

func TestReportJSONRoundTrip(t *testing.T) {
	original := sampleReport()

	data, err := goyze.MarshalReport(original)
	require.NoError(t, err)

	got, err := goyze.UnmarshalReport(data)
	require.NoError(t, err)

	assert.Equal(t, original, got)
}

func TestMarshalReportUsesDiagnosticsEnvelopeAndSnakeCase(t *testing.T) {
	data, err := goyze.MarshalReport(sampleReport())
	require.NoError(t, err)

	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &envelope))
	_, hasDiagnostics := envelope["diagnostics"]
	assert.True(t, hasDiagnostics, "top-level object must carry a diagnostics array")
	assert.Contains(t, string(data), `"end_line"`)
	assert.Contains(t, string(data), `"new_text"`)
}

func TestUnmarshalReportRejectsInvalidJSON(t *testing.T) {
	_, err := goyze.UnmarshalReport([]byte("{ not json"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, goyze.ErrInvalidReport))
}

func TestSeverityValues(t *testing.T) {
	assert.Equal(t, goyze.Severity("error"), goyze.SeverityError)
	assert.Equal(t, goyze.Severity("warning"), goyze.SeverityWarning)
	assert.Equal(t, goyze.Severity("info"), goyze.SeverityInfo)
}
