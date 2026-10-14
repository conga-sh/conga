// Package featuredisabled pins the lint mirror of the runtime feature opt-in
// guard: a root struct declaring conga.Features and a feature:"..." tag under
// an app that never calls EnableFeatures() is rejected at schema build.
package featuredisabled

import (
	"context"

	"github.com/conga-sh/conga"
)

type disabledFeatures struct {
	HTTP3 bool
}

// CLI declares features and a gated flag without enabling features.
type CLI struct {
	Features conga.Features[disabledFeatures] // want `conga\.Features declared on root command requires app\.EnableFeatures\(\)`

	HTTP3 conga.OptionalFlag[bool] `feature:"http3" help:"Experimental listener"` // want `feature struct tag requires app\.EnableFeatures\(\)`
}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func run() {
	app := conga.New[CLI]()
	app.Run()
}
