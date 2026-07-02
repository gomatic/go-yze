package goyze

import (
	"go/format"
	"sort"

	errs "github.com/gomatic/go-error"
)

// File-fix errors.
const (
	// ErrReadFile reports a source file that could not be read for fixing.
	ErrReadFile errs.Const = "cannot read file for fixing"
	// ErrFormat reports a fixed file that could not be reformatted.
	ErrFormat errs.Const = "cannot format fixed file"
	// ErrWriteFile reports a fixed file that could not be written back.
	ErrWriteFile errs.Const = "cannot write fixed file"
)

// Injected filesystem and formatting collaborators, so ApplyFixes is driven
// without touching disk in tests.
type (
	// FileReader returns the current bytes of a file.
	FileReader func(path string) ([]byte, error)
	// FileWriter persists the rewritten bytes of a file.
	FileWriter func(path string, data []byte) error
	// Formatter canonicalizes a file's bytes after edits are applied.
	Formatter func(src []byte) ([]byte, error)
)

// FixResult summarizes what ApplyFixes changed.
type FixResult struct {
	FilesChanged int
	EditsApplied int
}

// GoFormat is the default Formatter: gofmt-canonical Go source.
func GoFormat(src []byte) ([]byte, error) {
	return format.Source(src)
}

// ApplyFixes applies every fix's edits to disk, one file at a time: it merges all
// edits targeting a file, rewrites the file's bytes via ApplyEdits, reformats the
// result, and writes it back. Files are processed in sorted path order for
// determinism. Any read, overlap, format, or write failure aborts with that error.
func ApplyFixes(read FileReader, write FileWriter, format Formatter, fixes []Fix) (FixResult, error) {
	grouped := groupEdits(fixes)
	result := FixResult{}
	for _, path := range sortedPaths(grouped) {
		edits := grouped[path]
		if err := applyFileFixes(read, write, format, pathParam(path), edits); err != nil {
			return FixResult{}, err
		}
		result.FilesChanged++
		result.EditsApplied += len(edits)
	}
	return result, nil
}

// groupEdits collects every fix's edits into one slice per target file. A
// FileEdit carrying no edits is skipped so a no-op fix never creates a file entry
// (which would otherwise be read, reformatted, and rewritten for no reason).
func groupEdits(fixes []Fix) map[string][]TextEdit {
	grouped := map[string][]TextEdit{}
	for _, fix := range fixes {
		for _, fe := range fix.Files {
			if len(fe.Edits) == 0 {
				continue
			}
			grouped[fe.Path] = append(grouped[fe.Path], fe.Edits...)
		}
	}
	return grouped
}

// pathParam names the path parameter of applyFileFixes; rename it to the real domain concept.
type pathParam string

// applyFileFixes rewrites a single file with its merged edits.
func applyFileFixes(read FileReader, write FileWriter, format Formatter, path pathParam, edits []TextEdit) error {
	content, err := read(string(path))
	if err != nil {
		return ErrReadFile.With(err, "path", string(path))
	}
	edited, err := ApplyEdits(content, edits)
	if err != nil {
		return err
	}
	formatted, err := format(edited)
	if err != nil {
		return ErrFormat.With(err, "path", string(path))
	}
	if err := write(string(path), formatted); err != nil {
		return ErrWriteFile.With(err, "path", string(path))
	}
	return nil
}

// sortedPaths returns the grouped file paths in deterministic order.
func sortedPaths(grouped map[string][]TextEdit) []string {
	paths := make([]string, 0, len(grouped))
	for path := range grouped {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
