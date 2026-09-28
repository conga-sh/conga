package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// CLI is a flat CLI without subcommands. With EnableCompletion(), CONGA
// provides the --completion flag (Task / ripgrep style) without generating an
// unnecessary Commands: section in the help display.
type CLI struct {
	Level  conga.OptionalFlag[int]    `default:"1"                 help:"Minification level (1-9)" safe:"true" short:"l"`
	Output conga.OptionalFlag[string] `help:"Output file path"     safe:"true"                     short:"o"`
	Input  conga.RequiredArg[string]  `help:"Input file to minify"`
}

// Run executes the minification command.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)
	fmt.Fprintf(session.Stdout(), "Minifying %s (level=%d, output=%s)\n",
		c.Input.Get(), c.Level.Get(), c.Output.GetOr("<stdout>"))

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("minify").
		Description("Asset minifier (flat CLI with default --completion flag)").
		EnableCompletion()

	app.FatalIfError(app.Run())
}
