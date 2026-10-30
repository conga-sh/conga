package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

// CLI demonstrates exactly-one flag groups. Supplying neither source fails,
// supplying both fails, and supplying exactly one runs the command.
type CLI struct {
	InputFile conga.OptionalFlag[string] `help:"Read input from a file" name:"input-file" oneof:"source" placeholder:"PATH"`
	InputURL  conga.OptionalFlag[string] `help:"Read input from a URL"  name:"input-url"  oneof:"source" placeholder:"URL"`
}

// Run prints the selected source.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	source := "file:" + c.InputFile.Get()
	if c.InputURL.IsSet() {
		source = "url:" + c.InputURL.Get()
	}

	fmt.Fprintf(session.Stdout(), "source=%s\n", source)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("oneof-demo").
		Description("Exactly-one flag groups with oneof").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
