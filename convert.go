package goyze

import (
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// toolName is the Tool value stamped on every Diagnostic the yze analyzers emit.
const toolName = "yze"

// ToDiagnostic normalizes a go/analysis diagnostic into the lean Diagnostic
// schema, resolving token positions through fset and stamping the registration's
// rule id and help URL. Analyzer findings are always reported at error severity.
func ToDiagnostic(fset *token.FileSet, reg Registration, d analysis.Diagnostic) Diagnostic {
	start := fset.Position(d.Pos)
	out := Diagnostic{
		Tool:     toolName,
		Rule:     reg.RuleID(),
		Path:     start.Filename,
		Line:     start.Line,
		Col:      start.Column,
		Severity: SeverityError,
		Message:  d.Message,
		URL:      string(reg.URL),
		Fixes:    convertFixes(fset, d.SuggestedFixes),
	}
	if d.End.IsValid() {
		end := fset.Position(d.End)
		out.EndLine = end.Line
		out.EndCol = end.Column
	}
	return out
}

// convertFixes maps go/analysis suggested fixes into the schema's Fix list.
func convertFixes(fset *token.FileSet, fixes []analysis.SuggestedFix) []Fix {
	if len(fixes) == 0 {
		return nil
	}
	out := make([]Fix, 0, len(fixes))
	for _, f := range fixes {
		out = append(out, Fix{
			Description: f.Message,
			Files:       convertEdits(fset, f.TextEdits),
		})
	}
	return out
}

// convertEdits resolves token-based edits into byte-offset edits grouped by file,
// preserving first-seen file order.
func convertEdits(fset *token.FileSet, edits []analysis.TextEdit) []FileEdit {
	byPath := map[string][]TextEdit{}
	order := []string{}
	for _, e := range edits {
		start := fset.Position(e.Pos)
		if _, seen := byPath[start.Filename]; !seen {
			order = append(order, start.Filename)
		}
		byPath[start.Filename] = append(byPath[start.Filename], TextEdit{
			Start:   start.Offset,
			End:     fset.Position(e.End).Offset,
			NewText: string(e.NewText),
		})
	}
	out := make([]FileEdit, 0, len(order))
	for _, path := range order {
		out = append(out, FileEdit{Path: path, Edits: byPath[path]})
	}
	return out
}
