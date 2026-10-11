package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI contrasts CONGA's automatic constraint hints with {{...}} help
// variables. The automatic form appends the declared bound as a bracket hint;
// the alternative inlines the same value into the prose and suppresses the
// hint, so the constraint is stated exactly once.
type CLI struct {
	// Default rendering: the engine appends [1..65535] after the help text.
	Automatic conga.OptionalFlag[int] `default:"8080" help:"Port using the automatic hint" range:"1..65535"`

	// Alternative rendering: {{range}} states the bound in prose and suppresses
	// the automatic hint.
	Inlined conga.OptionalFlag[int] `default:"8080" help:"Port stated once ({{range}})" range:"1..65535"`

	// {{enum}} inlines the allowed values.
	Environment conga.OptionalFlag[string] `enum:"dev,staging,prod" help:"Environment ({{enum}})"`

	// {{items}} inlines the element-count bound.
	Zones conga.OptionalFlag[[]string] `help:"Availability zones ({{items}} items)" items:"2..3"`

	// {{default}} moves the default out of the value slot and into the prose.
	Profile conga.OptionalFlag[string] `default:"dev" help:"Build profile (default {{default}})"`

	// {{env}} inlines the bound environment variable name.
	Region conga.OptionalFlag[string] `env:"REGION" help:"Region (from {{env}})"`

	// A literal {{ is written by doubling the opening braces.
	Literal conga.OptionalFlag[string] `help:"Literal braces: {{{{not-a-variable}}"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("helpvars").
		Description("Help-variable interpolation: state a constraint once").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
