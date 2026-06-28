package goyze

import (
	"encoding/json"

	errs "github.com/gomatic/go-error"
)

// ErrInvalidReport reports a payload that is not a well-formed diagnostic report.
const ErrInvalidReport errs.Const = "invalid diagnostic report"

// Severity ranks a Diagnostic. It is the normalized severity shared across every
// tool stickler runs.
type Severity string

// The severity levels a Diagnostic may carry.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Diagnostic is the lean, normalized finding that every tool's output is mapped
// into. It is the single contract shared by the yze analyzers (producers) and the
// stickler runner (consumer).
type Diagnostic struct {
	Tool     string   `json:"tool"`
	Rule     string   `json:"rule"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Col      int      `json:"col"`
	EndLine  int      `json:"end_line,omitempty"`
	EndCol   int      `json:"end_col,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Fixes    []Fix    `json:"fixes,omitempty"`
	URL      string   `json:"url,omitempty"`
}

// Fix is a named, mechanically-applicable change attached to a Diagnostic. It is
// present only when the analyzer can offer a safe, deterministic edit.
type Fix struct {
	Description string     `json:"description"`
	Files       []FileEdit `json:"files"`
}

// FileEdit groups the TextEdits that apply to one file.
type FileEdit struct {
	Path  string     `json:"path"`
	Edits []TextEdit `json:"edits"`
}

// Report is the envelope serialized by the native stickler-json format: the set
// of diagnostics a tool produced in one run.
type Report struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// MarshalReport serializes a Report to the native stickler-json encoding.
func MarshalReport(r Report) ([]byte, error) {
	return json.Marshal(r)
}

// UnmarshalReport parses a native stickler-json payload, reporting ErrInvalidReport
// when the bytes are not a well-formed report.
func UnmarshalReport(data []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return Report{}, ErrInvalidReport.With(err)
	}
	return r, nil
}
