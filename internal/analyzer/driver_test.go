package analyzer_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDriverDiagnostics runs the shipped conga-lint binary the way the go
// tool does (singlechecker over a module whose dependencies load from export
// data), mirroring the production invocation. analysistest type-checks every
// dependency from source and therefore cannot detect loading-mode regressions
// such as resolution silently failing under the analysis driver.
func TestDriverDiagnostics(t *testing.T) {
	t.Parallel()

	binary := filepath.Join(t.TempDir(), "conga-lint")

	buildCommand := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "github.com/conga-sh/conga/cmd/conga-lint")
	if output, err := buildCommand.CombinedOutput(); err != nil {
		t.Fatalf("building conga-lint: %v\n%s", err, output)
	}

	testCases := []struct {
		name        string
		arguments   []string
		wantExit    int
		wantMessage string
	}{
		{name: "invalid fixture reports diagnostics", arguments: []string{"./invalid"}, wantExit: 3, wantMessage: "does not implement conga.Runner"},
		{name: "invalid fixture reports throwaway roots", arguments: []string{"./invalid"}, wantExit: 3, wantMessage: "conga.New root instance is not referenced outside the app"},
		{name: "invalid fixture reports unknown tags by default", arguments: []string{"./invalid"}, wantExit: 3, wantMessage: "unknown struct tag"},
		{name: "valid fixture is clean", arguments: []string{"./valid"}, wantExit: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			command := exec.CommandContext(t.Context(), binary, testCase.arguments...)
			command.Dir = testdataDirectory(t)

			command.Env = append(os.Environ(), "GOWORK=off")

			output, err := command.CombinedOutput()

			exitCode := 0
			if exitError, hasExitError := errors.AsType[*exec.ExitError](err); hasExitError {
				exitCode = exitError.ExitCode()
			} else if err != nil {
				t.Fatalf("running conga-lint: %v\n%s", err, output)
			}

			if exitCode != testCase.wantExit {
				t.Fatalf("exit code = %d, want %d\n%s", exitCode, testCase.wantExit, output)
			}

			if testCase.wantMessage != "" && !strings.Contains(string(output), testCase.wantMessage) {
				t.Fatalf("output missing %q:\n%s", testCase.wantMessage, output)
			}
		})
	}
}

// testdataDirectory resolves the testdata module directory for the analysis
// driver subprocess.
func testdataDirectory(t *testing.T) string {
	t.Helper()

	directory, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("resolving testdata directory: %v", err)
	}

	return directory
}
