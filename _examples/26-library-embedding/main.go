// This example demonstrates embedding CONGA as a library inside a host
// process: the host supplies the argument vector, injects its own output
// streams, intercepts termination via a custom exit handler, and consumes
// typed errors programmatically instead of letting the framework take over
// the process.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/conga-sh/conga"
)

// DeployJobCLI is the command schema of the embedded runner.
type DeployJobCLI struct {
	Job      conga.RequiredArg[string] `help:"Batch job identifier"`
	Priority conga.OptionalFlag[int]   `default:"5"                 help:"Scheduling priority (1-9)"`
}

func (d *DeployJobCLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "Running job %s (priority %d)\n", d.Job.Get(), d.Priority.Get())

	payload, err := io.ReadAll(session.Stdin())
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	fmt.Fprintf(session.Stdout(), "stdin payload: %q\n", payload)

	return nil
}

func main() {
	// The host owns the streams: CONGA reads standard input from the injected
	// reader and writes help, version, business output, and errors into these
	// buffers, and the host decides where they go.
	var stdoutBuffer, stderrBuffer bytes.Buffer

	// The host owns termination: the exit handler records the requested code
	// instead of letting CONGA call os.Exit.
	capturedCode := 0

	app := conga.New[DeployJobCLI]().
		Name("batchrun").
		Description("Batch job runner with a fully hosted lifecycle").
		Version("1.0.0").
		// The host renders its own help/version UX; why and shell completion
		// are opt-in and stay off.
		DisableHelpFlag().
		DisableVersionFlag().
		// Errors stay clean; the host renders usage itself if it wants to.
		UsageOnError(conga.UsageSilence).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		Stdin(strings.NewReader("host-provided stdin\n")).
		Args(os.Args[1:]).
		ExitFunc(func(code int) { capturedCode = code })

	// RunContext accepts a host-provided context (cancellation, deadlines,
	// and values all flow into the command chain).
	err := app.RunContext(context.Background())

	// Typed errors let the host branch programmatically.
	if err != nil {
		fmt.Printf("[host] run error: %v\n", err)
		fmt.Printf("[host] usage-classified: %t\n", conga.IsUsageError(err))

		if usageErr, ok := errors.AsType[conga.UsageError](err); ok {
			fmt.Printf("[host] usage error at command path: %s\n", usageErr.CommandPath())
		}

		if errors.Is(err, conga.ErrCoerce) {
			fmt.Printf("[host] coercion failure: %v\n", err)
		}

		if errors.Is(err, conga.ErrParse) {
			fmt.Printf("[host] parse failure: %v\n", err)
		}
	}

	// FatalIfError renders the friendly error message into the injected
	// stderr and invokes the custom exit handler instead of os.Exit.
	app.FatalIfError(err)

	// The host flushes the captured streams wherever it wants (a logger, a
	// socket, a test buffer); here it just forwards them.
	fmt.Fprint(os.Stdout, stdoutBuffer.String())
	fmt.Fprint(os.Stderr, stderrBuffer.String())

	fmt.Printf("[host] captured exit code: %d\n", capturedCode)

	// The host performs the actual termination with its own policy.
	if capturedCode > 0 {
		os.Exit(capturedCode)
	}
}
