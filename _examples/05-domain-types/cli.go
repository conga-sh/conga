package main

import (
	"context"
	"fmt"
	"io"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates strongly-typed domain objects with built-in filesystem and format validation.
type CLI struct {
	InputFile  conga.OptionalFlag[conga.File]     `existing:"true"                         short:"i"`
	OutputDir  conga.OptionalFlag[conga.Dir]      `existing:"true"                         short:"o"`
	MaxMemory  conga.OptionalFlag[conga.ByteSize] `default:"512MiB"                        short:"m"`
	DeployHook conga.OptionalFlag[conga.File]     `short:"l"                               symlink:"true"`
	Ports      conga.OptionalFlag[PortRange]      `help:"Port range (e.g. 8000-8080)"      short:"p"`
	Headers    conga.OptionalFlag[HeaderMap]      `help:"HTTP request headers (key=value)" short:"H"`
	Payload    conga.OptionalFlag[conga.FileContent]
	Output     conga.OptionalFlag[conga.File] `stdout:"true"`
	Query      conga.OptionalFlag[conga.File] `short:"q"           stdin:"true"`
	Templates  conga.OptionalFlag[conga.File] `glob:"*.yaml,*.yml"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	memory := c.MaxMemory.Get()
	fmt.Fprintf(session.Stdout(), "Validated Domain Types: max_memory_bytes=%d\n", memory.Bytes())

	if payload := c.Payload.Get(); payload != "" {
		fmt.Fprintf(session.Stdout(), "payload_bytes=%d\n", len(payload.Bytes()))
	}

	// A File with stdin:"true" streams lazily through Reader(): the stream is
	// consumed here, unlike FileContent which is loaded eagerly at decode time.
	if query := c.Query.Get(); query.String() != "" {
		reader, err := query.Reader()
		if err != nil {
			return fmt.Errorf("open query file: %w", err)
		}

		defer func() {
			_ = reader.Close() //nolint:errcheck // stdio readers are non-closing wrappers; close is best-effort.
		}()

		count, err := io.Copy(io.Discard, reader)
		if err != nil {
			return fmt.Errorf("read query file: %w", err)
		}

		fmt.Fprintf(session.Stdout(), "query_bytes=%d\n", count)
	}

	// A stdio output sink is not a file: skip filesystem side effects and keep
	// diagnostics off the payload stream.
	if output := c.Output.Get(); output.IsStdio() {
		if payload := c.Payload.Get(); payload != "" {
			fmt.Fprint(session.Stdout(), string(payload))
		}

		fmt.Fprintln(session.Stderr(), "output stream detected; diagnostics stay on stderr")
	}

	return nil
}
