package conga_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// panicPredictorCLI completes a flag through a predictor that panics.
type panicPredictorCLI struct {
	noopRunner

	Kind conga.OptionalFlag[string] `predictor:"boom"`
}

// failingWriter simulates an unwritable completion output stream.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("disk full")
}

// TestComplete_PanicRecovered_Exit2Contract verifies the wire-protocol
// internal-error contract (docs/completion-wire-protocol.md): a predictor
// panic during __complete is recovered, returns no stdout payload, and maps
// to exit code 2 with a diagnostic on stderr instead of corrupting the
// user's terminal line.
func TestComplete_PanicRecovered_Exit2Contract(t *testing.T) {
	t.Parallel()

	var (
		stdoutBuffer bytes.Buffer
		stderrBuffer bytes.Buffer
		exitCode     = -1
	)

	booming := predict.Callback(func(_ predict.Context) predict.Predictor {
		panic("boom")
	})

	app := conga.New[panicPredictorCLI]().
		EnableCompletion().
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code }).
		Predictor("boom", booming).
		Args([]string{"__complete", "bash", "--kind", ""})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal completion failure")

	app.Exit(err)

	assert.Equal(t, 2, exitCode)
	assert.Equal(t, "", stdoutBuffer.String())
	assert.Contains(t, stderrBuffer.String(), "internal completion failure")
	assert.Contains(t, stderrBuffer.String(), "boom")
}

// TestComplete_UnwritableStdout_Exit2Contract verifies that an unwritable
// completion stdout maps to the documented exit-2 contract with a stderr
// diagnostic instead of the generic runtime exit-1 path.
func TestComplete_UnwritableStdout_Exit2Contract(t *testing.T) {
	t.Parallel()

	var (
		buffer   bytes.Buffer
		exitCode = -1
	)

	app := conga.New[SimpleAppCLI]().
		EnableCompletion().
		Stdout(failingWriter{}).
		Stderr(&buffer).
		ExitFunc(func(code int) { exitCode = code }).
		Args([]string{"__complete", "bash", "--ca"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal completion failure")

	app.Exit(err)

	assert.Equal(t, 2, exitCode)
	assert.Contains(t, buffer.String(), "internal completion failure")
	assert.Contains(t, buffer.String(), "disk full")
}
