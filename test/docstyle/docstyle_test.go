package docstyle_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// publicPackages lists every package whose doc comments render on pkg.go.dev.
// The guard rejects markdown backticks in these comments because pkg.go.dev
// renders them literally; indented code blocks carry the same text without the
// stray backtick characters.
//
//nolint:gochecknoglobals // static package policy list shared by the guard
var publicPackages = []string{
	".",
	"decoder/hcl",
	"decoder/json",
	"decoder/toml",
	"decoder/yaml",
	"network",
	"predict",
	"system",
	"timezone",
}

func TestPublicPackages_HaveNoBackticksInComments(t *testing.T) {
	t.Parallel()

	for _, packagePath := range publicPackages {
		t.Run(packagePath, func(t *testing.T) {
			t.Parallel()

			violations := findBacktickedComments(t, filepath.Join("..", "..", packagePath))

			assert.True(t, len(violations) == 0, "markdown backticks in comments: "+strings.Join(violations, "; "))
		})
	}
}

// findBacktickedComments parses every non-test Go file in dir and reports each
// comment line that contains a backtick outside an indented code block. A
// comment line is a code block when its text after the comment marker begins
// with a tab, matching the gofmt convention for godoc preformatted blocks.
func findBacktickedComments(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", dir, err)
	}

	var violations []string

	fileSet := token.NewFileSet()

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, name)

		parsed, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", filePath, err)
		}

		for _, commentGroup := range parsed.Comments {
			for _, comment := range commentGroup.List {
				text := strings.TrimPrefix(comment.Text, "//")
				if strings.HasPrefix(text, "\t") || !strings.Contains(text, "`") {
					continue
				}

				position := fileSet.Position(comment.Pos())

				violations = append(violations, fmt.Sprintf("%s:%d: %s", name, position.Line, strings.TrimSpace(text)))
			}
		}
	}

	return violations
}
