package completion_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/internal/completion/wire"
)

// driverEnvVars lists every environment variable read by the completion drivers,
// internal/envx, and internal/style so TestMain and mode-based conformance
// tests start from a neutral environment. The root corpus test keeps the same
// list; TestDriverEnvVars_CoverWireConstants in each package pins it to the
// wire constants.
//
//nolint:gochecknoglobals // shared list of driver environment variables for test isolation
var driverEnvVars = []string{
	wire.EnvCompLine,
	wire.EnvCompPoint,
	wire.EnvCompType,
	wire.EnvCompWordbreaks,
	wire.EnvColor,
	wire.EnvNoColor,
	wire.EnvCLIColor,
	wire.EnvDescriptionLength,
	wire.EnvMergeFlags,
	wire.EnvShell,
	wire.EnvTooltip,
	wire.EnvZshListSeparator,
	wire.EnvZshQuote,
	wire.EnvZshBanghist,
	wire.EnvZshStyleLimit,
	"LS_COLORS",
}

// TestMain neutralizes the ambient driver and colour environment so the
// package's tests observe documented defaults rather than the caller's shell.
func TestMain(m *testing.M) {
	for _, key := range driverEnvVars {
		os.Unsetenv(key)
	}

	os.Exit(m.Run())
}

// TestDriverEnvVars_CoverWireConstants pins driverEnvVars to the Env*
// constants declared in internal/completion/wire, so a new driver variable
// cannot escape the neutral test environment. LS_COLORS is read by
// internal/style and has no wire constant.
func TestDriverEnvVars_CoverWireConstants(t *testing.T) {
	t.Parallel()

	envxValues := envConstantValues(t, "../envx/envx.go", nil)
	wireValues := envConstantValues(t, "wire/env.go", envxValues)

	expected := append(slices.Collect(maps.Values(wireValues)), "LS_COLORS")
	slices.Sort(expected)

	assert.Equal(t, expected, slices.Sorted(slices.Values(driverEnvVars)))
}

// envConstantValues parses the Go file at path and returns the string value of
// every package-level constant whose name starts with Env, keyed by name. A
// constant defined as envx.<Name> resolves through imported.
func envConstantValues(t *testing.T, path string, imported map[string]string) map[string]string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	assert.NoError(t, err)

	values := map[string]string{}

	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.CONST {
			continue
		}

		for _, item := range general.Specs {
			valueSpec := item.(*ast.ValueSpec)

			for i, name := range valueSpec.Names {
				if !strings.HasPrefix(name.Name, "Env") {
					continue
				}

				switch expression := valueSpec.Values[i].(type) {
				case *ast.BasicLit:
					unquoted, err := strconv.Unquote(expression.Value)
					assert.NoError(t, err)

					values[name.Name] = unquoted
				case *ast.SelectorExpr:
					value, ok := imported[expression.Sel.Name]
					assert.True(t, ok, "constant %s refers to unknown %s", name.Name, expression.Sel.Name)

					values[name.Name] = value
				default:
					assertx.Unreachable(t, "constant "+name.Name+" has an unsupported definition")
				}
			}
		}
	}

	return values
}
