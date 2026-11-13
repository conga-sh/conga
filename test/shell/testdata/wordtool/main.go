// Command wordtool is a shell-completion fixture for word handling: a file
// flag for paths containing spaces and an enum whose values need quoting,
// including an ASCII and a Unicode (U+2019) apostrophe.
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

// CLI declares the flags the word-handling tests complete.
type CLI struct {
	Kubeconfig conga.OptionalFlag[conga.File] `help:"Kubeconfig file"`
	Env        conga.OptionalFlag[string]     `enum:"my env,a!b,prod,it's,it’s"  help:"Target environment"`
	Completion conga.CompletionCommand
}

func (c *CLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("wordtool").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
