//go:build sizetest

package size_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// optInInitGuardPackages lists the packages that must stay free of dynamic
// package-level initializers: for opt-in subsystems a dynamic initializer emits
// a `..inittask` that keeps the package (and its subsystem) linked even when
// the feature is disabled, and internal/jsonwire is always-linked, so its
// initializer would run at every startup. Error sentinels for these packages
// live in the always-linked internal/sentinel leaf instead. Packages not yet
// converted (predict/network, predict/system, internal/filex, internal/signals,
// watch, decoder/*) are intentionally absent until their sentinels move there.
//
//nolint:gochecknoglobals // static package policy list shared by the guard subtests
var optInInitGuardPackages = []string{
	"config",
	"internal/completion",
	"internal/completion/bash",
	"internal/completion/elvish",
	"internal/completion/export",
	"internal/completion/fish",
	"internal/completion/lex",
	"internal/completion/nushell",
	"internal/completion/powershell",
	"internal/completion/wire",
	"internal/completion/zsh",
	"internal/configengine",
	"internal/dotenv",
	"internal/jsonwire",
	"internal/style",
	"plugin/network",
	"plugin/system",
	"plugin/timezone",
	"predict",
}

func TestOptInPackages_HaveNoDynamicInitializers(t *testing.T) {
	t.Parallel()

	for _, packagePath := range optInInitGuardPackages {
		t.Run(packagePath, func(t *testing.T) {
			t.Parallel()

			violations := findDynamicInitializers(t, filepath.Join("..", "..", packagePath))

			assert.True(t, len(violations) == 0, "dynamic package-level initializers: "+strings.Join(violations, "; "))
		})
	}
}

// findDynamicInitializers parses every non-test Go file in dir and reports the
// package-level declarations that require a dynamic initializer: calls, map
// literals, address-of-composite-literal values, and func init().
func findDynamicInitializers(t *testing.T, dir string) []string {
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

		parsed, parseErr := parser.ParseFile(fileSet, filePath, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", filePath, parseErr)
		}

		for _, declaration := range parsed.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Recv == nil && typed.Name.Name == "init" {
					violations = append(violations, name+": func init()")
				}
			case *ast.GenDecl:
				if typed.Tok != token.VAR {
					continue
				}

				for _, spec := range typed.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for _, value := range valueSpec.Values {
						if reason := dynamicValueReason(value); reason != "" {
							violations = append(violations, fmt.Sprintf("%s: %s: %s", name, valueSpec.Names[0].Name, reason))
						}
					}
				}
			}
		}
	}

	return violations
}

// dynamicValueReason reports why a package-level initializer requires dynamic
// initialization, or "" when the value is statically initialized.
func dynamicValueReason(expression ast.Expr) string {
	reason := ""

	ast.Inspect(expression, func(node ast.Node) bool {
		if reason != "" {
			return false
		}

		switch typed := node.(type) {
		case *ast.CallExpr:
			reason = "call expression"

			return false
		case *ast.CompositeLit:
			if _, isMap := typed.Type.(*ast.MapType); isMap {
				reason = "map literal"

				return false
			}
		case *ast.UnaryExpr:
			if typed.Op == token.AND {
				if _, isComposite := typed.X.(*ast.CompositeLit); isComposite {
					reason = "address of composite literal"

					return false
				}
			}
		}

		return true
	})

	return reason
}
