// Package envprefixambiguous pins the analyzer's degradation when the
// app-level environment prefix is not a literal: {{env}} stays available, so no
// diagnostic is invented for a flag that may bind environment variables at
// runtime.
package envprefixambiguous

import (
	"context"

	"github.com/conga-sh/conga"
)

// prefix is not a literal, so app.EnvPrefix leaves the analyzer without
// evidence about the effective environment prefix.
var prefix = "APP_"

// App is a leaf command implementing conga.Runner.
type App struct {
	Port conga.OptionalFlag[int] `name:"port" help:"Port from {{env}}"`
}

// Run executes the ambiguous-prefix fixture.
func (a *App) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[App]().EnvPrefix(prefix)
