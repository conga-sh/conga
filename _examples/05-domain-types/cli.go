package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI demonstrates strongly-typed domain objects with built-in filesystem and format validation.
type CLI struct {
	InputFile  conga.OptionalFlag[conga.File]        `existing:"true"                                      help:"Path to existing input file"              short:"i"`
	OutputDir  conga.OptionalFlag[conga.Dir]         `existing:"true"                                      help:"Path to existing directory"               short:"o"`
	MaxMemory  conga.OptionalFlag[conga.ByteSize]    `default:"512MiB"                                     help:"Maximum memory limit (e.g. 500MiB, 2GiB)" short:"m"`
	DeployHook conga.OptionalFlag[conga.File]        `help:"Symlink to the deployment hook script"         short:"l"                                       symlink:"true"`
	Ports      conga.OptionalFlag[PortRange]         `help:"Port range (e.g. 8000-8080)"                   short:"p"`
	Headers    conga.OptionalFlag[HeaderMap]         `help:"HTTP request headers (key=value)"              short:"H"`
	Payload    conga.OptionalFlag[conga.FileContent] `help:"Payload file ('-' for stdin) read into memory"`
	Output     conga.OptionalFlag[conga.File]        `help:"Output file ('-' for stdout)"                  stdout:"true"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	memory := c.MaxMemory.Get()
	fmt.Fprintf(session.Stdout(), "Validated Domain Types: max_memory_bytes=%d\n", memory.Bytes())

	if payload := c.Payload.Get(); payload != "" {
		fmt.Fprintf(session.Stdout(), "payload_bytes=%d\n", len(payload.Bytes()))
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
