// Command rawtool is a shell-completion fixture: an implicit leaf whose RawArgs
// predictor applies once a flag declared only by that leaf selects it.
package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// RunCmd forwards every token after its flags to the executed script.
type RunCmd struct {
	DryRun conga.OptionalFlag[bool] `help:"Print what would run" short:"n"`
	Args   conga.RawArgs            `predictor:"forwarded"`
}

func (c *RunCmd) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// InstallCmd is an explicit sibling of the implicit run command.
type InstallCmd struct{}

func (c *InstallCmd) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

// CLI routes unmatched operands and run-only flags into the implicit run command.
type CLI struct {
	Exec       conga.Command[RunCmd]     `implicit:"true"                name:"run" summary:"Run a script"`
	Install    conga.Command[InstallCmd] `summary:"Install dependencies"`
	Completion conga.CompletionCommand
}

func main() {
	app := conga.New[CLI]().
		Name("rawtool").
		EnableCompletion().
		Predictor("forwarded", predict.Values("--script-watch", "script.ts"))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
