package goyze

import (
	"sort"

	errs "github.com/gomatic/go-error"
)

// Sentinel errors emitted by the fix engine.
const (
	// ErrOverlappingEdits reports two text edits whose byte ranges intersect.
	ErrOverlappingEdits errs.Const = "overlapping text edits"
	// ErrEditOutOfBounds reports a text edit whose range falls outside the
	// content, or whose start is greater than its end.
	ErrEditOutOfBounds errs.Const = "text edit out of bounds"
)

// TextEdit is a byte-range replacement within a single file's content. Start is
// inclusive and End is exclusive (both byte offsets); an edit with Start == End
// is a pure insertion of NewText.
type TextEdit struct {
	NewText string `json:"new_text"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

// ApplyEdits applies edits to content and returns the rewritten bytes. Edits may
// be supplied in any order; they are applied as one atomic batch. It reports
// ErrEditOutOfBounds for a range outside content (or an inverted range) and
// ErrOverlappingEdits when two ranges intersect. content is never mutated.
func ApplyEdits(content []byte, edits []TextEdit) ([]byte, error) {
	if len(edits) == 0 {
		return content, nil
	}
	sorted := make([]TextEdit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool { return editLess(sorted[i], sorted[j]) })
	if err := validateEdits(content, sorted); err != nil {
		return nil, err
	}
	return spliceEdits(content, sorted), nil
}

// editLess is the total order edits are sorted by: ascending Start, then
// ascending End, then NewText. A total order (never just Start) makes the
// overlap verdict and the splice result deterministic when two edits share a
// Start, where an unstable Start-only sort would otherwise pick either order.
func editLess(a, b TextEdit) bool {
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	if a.End != b.End {
		return a.End < b.End
	}
	return a.NewText < b.NewText
}

// validateEdits checks each edit's bounds and that no two ranges overlap. sorted
// must be ordered by ascending Start.
func validateEdits(content []byte, sorted []TextEdit) error {
	prevEnd := 0
	for i, e := range sorted {
		if err := boundsCheck(content, e); err != nil {
			return err
		}
		if i > 0 && e.Start < prevEnd {
			return ErrOverlappingEdits
		}
		prevEnd = e.End
	}
	return nil
}

// boundsCheck verifies a single edit's range lies within content.
func boundsCheck(content []byte, e TextEdit) error {
	if e.Start < 0 || e.End > len(content) || e.Start > e.End {
		return ErrEditOutOfBounds
	}
	return nil
}

// spliceEdits rewrites content by applying sorted edits right-to-left so that
// earlier offsets stay valid as later ranges are replaced.
func spliceEdits(content []byte, sorted []TextEdit) []byte {
	result := content
	for i := len(sorted) - 1; i >= 0; i-- {
		e := sorted[i]
		next := make([]byte, 0, len(result)-(e.End-e.Start)+len(e.NewText))
		next = append(next, result[:e.Start]...)
		next = append(next, e.NewText...)
		next = append(next, result[e.End:]...)
		result = next
	}
	return result
}
