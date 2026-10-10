package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// AdvancedCmd groups the required-flag, positional-argument, and provenance
// demonstrations.
type AdvancedCmd struct {
	Required   conga.Command[RequiredCmd]   `summary:"Required flag with positional arguments and trailing args"`
	Provenance conga.Command[ProvenanceCmd] `summary:"Programmatic value provenance report"`
}

// RequiredCmd demonstrates RequiredFlag, positional arguments with custom
// placeholders, and DashArgs forwarding.
type RequiredCmd struct {
	APIKey      conga.RequiredFlag[string]  `help:"API key (required)" short:"k"`
	Name        conga.RequiredArg[string]   `help:"Resource name"`
	Message     conga.OptionalArg[string]   `name:"[message]"`
	Extra       conga.OptionalArg[[]string] `name:"[extras...]"`
	Passthrough conga.DashArgs
}

func (c *RequiredCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	fmt.Fprintf(session.Stdout(), "Advanced: name=%s message=%q extras=%d passthrough=%v\n",
		c.Name.Get(), c.Message.Get(), len(c.Extra.Get()), c.Passthrough.Get())

	return nil
}

// ProvenanceCmd demonstrates the programmatic provenance API: Changed(),
// Source(), and SourceDetail() on the root containers.
type ProvenanceCmd struct{}

func (c *ProvenanceCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[KitchenSinkCLI]())

	root := session.Root[KitchenSinkCLI]()
	stdout := session.Stdout()

	fmt.Fprintln(stdout, "=== Root Value Provenance ===")
	fmt.Fprintf(stdout, "verbosity:  changed=%t source=%s\n", root.Verbosity.Changed(), root.Verbosity.Source())
	fmt.Fprintf(stdout, "format:     changed=%t source=%s detail=%s\n", root.Format.Changed(), root.Format.Source(), root.Format.SourceDetail())
	fmt.Fprintf(stdout, "cache:      changed=%t source=%s\n", root.Cache.Changed(), root.Cache.Source())
	fmt.Fprintf(stdout, "secret-key: changed=%t source=%s detail=%s\n", root.SecretKey.Changed(), root.SecretKey.Source(), root.SecretKey.SourceDetail())
	fmt.Fprintf(stdout, "config:     changed=%t source=%s detail=%s\n", root.Config.Changed(), root.Config.Source(), root.Config.SourceDetail())

	return nil
}
