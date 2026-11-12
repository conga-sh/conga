// Package featureimplicit pins the implicit-command feature guard: a feature
// tag on an implicit command is rejected, while gating the parent of an
// implicit command or a flag declared on it is accepted.
package featureimplicit

import (
	"context"

	"github.com/conga-sh/conga"
)

type gates struct {
	Runner bool
}

type runCmd struct {
	Fast conga.OptionalFlag[bool] `feature:"runner" help:"Gated flag on an implicit command"`

	Script conga.RequiredArg[string] `help:"Script to run"`
}

// Run executes the run command.
func (c *runCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type buildCmd struct{}

// Run executes the build command.
func (c *buildCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type toolsCmd struct {
	Exec conga.Command[runCmd] `implicit:"true" summary:"Run a script"`
}

// App gates an implicit command, which is rejected, and gates the parent of
// another implicit command, which is accepted.
type App struct {
	Features conga.Features[gates] `flag:"feature-gates"`

	Run   conga.Command[runCmd]   `feature:"runner" implicit:"true" summary:"Run a script"` // want `struct App field Run: schema error: tag "feature" cannot be combined with implicit:"true" \(an implicit command is entered without being named, so it cannot be gated; gate the parent, or make the command explicit\)`
	Build conga.Command[buildCmd] `summary:"Build"`
	Tools conga.Command[toolsCmd] `feature:"runner" summary:"Gated parent of an implicit command"`
}

func run() {
	app := conga.New[App]().EnableFeatures()
	app.Run()
}
