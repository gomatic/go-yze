// Generated-file exclusion: the gomatic standards commit generated trees
// (protobuf stubs, ANTLR parsers) but exempt them from every gate by the
// standard marker. Diagnostics in generated files are dropped, mirroring
// golangci-lint's default and `go vet`'s community convention: generated code
// is a build artifact — change the generator input, not the output.
package goyze

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// generatedMarker is the Go convention for generated files
// (https://go.dev/s/generatedcode): a whole-line comment before the package
// clause reading `// Code generated <how> DO NOT EDIT.`.
var generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// readHead reads a file's opening bytes; injected so tests drive the decision
// without a real tree.
type readHead func(path string) ([]byte, error)

// osReadHead reads up to the first 4KiB of a file — the marker precedes the
// package clause, comfortably within the head.
func osReadHead(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, 4096)
	n, err := bufio.NewReader(f).Read(head)
	if n > 0 {
		return head[:n], nil
	}
	return nil, err
}

// generatedFiles memoizes per-path generated-marker checks across one report.
type generatedFiles struct {
	read readHead
	seen map[string]bool
}

// isGenerated reports whether the file at path carries the generated marker.
// An unreadable file is not generated (the diagnostic survives; better a
// spurious finding than a silently dropped one).
func (g generatedFiles) isGenerated(path string) bool {
	if verdict, ok := g.seen[path]; ok {
		return verdict
	}
	verdict := g.check(path)
	g.seen[path] = verdict
	return verdict
}

func (g generatedFiles) check(path string) bool {
	head, err := g.read(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(head), "\n") {
		if strings.HasPrefix(line, "package ") {
			return false
		}
		if generatedMarker.MatchString(strings.TrimRight(line, "\r")) {
			return true
		}
	}
	return false
}
