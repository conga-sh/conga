// Command ordertool is a shell-completion fixture: a KeepOrder predictor whose
// candidates share descriptions, which zsh groups and reorders by default.
package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// CLI declares one flag completed by the "levels" predictor.
type CLI struct {
	Level      conga.OptionalFlag[string] `help:"Log level" predictor:"levels"`
	Completion conga.CompletionCommand
}

func (c *CLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("ordertool").
		EnableCompletion().
		Predictor("levels", predict.Items(
			predict.NewItem("debug", "verbose"),
			predict.NewItem("info", "verbose"),
			predict.NewItem("warn", "alert"),
			predict.NewItem("error", "alert"),
		).KeepOrder())

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
