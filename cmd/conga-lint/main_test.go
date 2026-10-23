package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"
)

// congaLintBinary is the path to the conga-lint binary built once in [TestMain]
// and exercised by the CLI contract tests below.
//
//nolint:gochecknoglobals // built once per test binary and shared read-only by the CLI contract tests
var congaLintBinary string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "conga-lint-cli")
	if err != nil {
		os.Exit(1)
	}

	congaLintBinary = filepath.Join(directory, "conga-lint")

	buildCommand := exec.CommandContext(context.Background(), "go", "build", "-o", congaLintBinary, ".")
	buildCommand.Dir = "."

	if output, err := buildCommand.CombinedOutput(); err != nil {
		os.Stderr.Write(output)
		os.RemoveAll(directory)

		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(directory)

	os.Exit(code)
}

func TestCLI_ValidPackageExitsZero(t *testing.T) {
	t.Parallel()

	exitCode, _ := runCLI(t, "./valid")

	assert.Equal(t, 0, exitCode)
}

func TestCLI_ViolationExitsThree(t *testing.T) {
	t.Parallel()

	exitCode, stderr := runCLI(t, "./invalid")

	assert.Equal(t, 3, exitCode)
	assert.NotEqual(t, "", stderr)
	assert.Contains(t, stderr, "does not implement conga.Runner")
}

func TestCLI_UnknownFlagExitsTwo(t *testing.T) {
	t.Parallel()

	exitCode, stderr := runCLI(t, "-nonexistent-flag")

	assert.Equal(t, 2, exitCode)
	assert.Contains(t, stderr, "flag provided but not defined")
}

// runCLI executes the built conga-lint binary against the analyzer fixture
// module and returns its exit code plus captured stderr.
func runCLI(t *testing.T, arguments ...string) (int, string) {
	t.Helper()

	var stderrBuffer bytes.Buffer

	command := exec.CommandContext(t.Context(), congaLintBinary, arguments...)
	command.Dir = fixtureDirectory(t)

	command.Env = append(os.Environ(), "GOWORK=off")
	command.Stdout = io.Discard
	command.Stderr = &stderrBuffer

	err := command.Run()

	exitCode := 0
	if exitError, hasExitError := errors.AsType[*exec.ExitError](err); hasExitError {
		exitCode = exitError.ExitCode()
	} else if err != nil {
		t.Fatalf("running conga-lint: %v\n%s", err, stderrBuffer.String())
	}

	return exitCode, stderrBuffer.String()
}

// fixtureDirectory resolves the nested analyzer fixture module directory that
// supplies the valid and invalid packages exercised above.
func fixtureDirectory(t *testing.T) string {
	t.Helper()

	directory, err := filepath.Abs(filepath.Join("..", "..", "internal", "analyzer", "testdata"))
	if err != nil {
		t.Fatalf("resolving fixture directory: %v", err)
	}

	return directory
}
