// Package featureenabled pins the clean path of the feature opt-in guard: the
// anchored app enables features, so the conga.Features container and its
// feature:"..." tags are accepted.
package featureenabled

import (
	"context"

	"github.com/conga-sh/conga"
)

type enabledFeatures struct {
	HTTP3 bool
}

// CLI declares features and a gated flag with features enabled.
type CLI struct {
	Features conga.Features[enabledFeatures] `flag:"feature-gates"`

	HTTP3 conga.OptionalFlag[bool] `feature:"http3" help:"Experimental listener"`
}

// Run executes the CLI.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func run() {
	app := conga.New[CLI]().EnableFeatures()
	app.Run()
}
