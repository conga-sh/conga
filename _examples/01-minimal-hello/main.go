package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/conga-sh/conga"
)

// CLI represents a minimal single-command application.
type CLI struct {
	Name conga.OptionalFlag[string] `default:"World"                    help:"Name of the person to greet" short:"n"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase" short:"l"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI")

	app.FatalIfError(app.Run())
}
