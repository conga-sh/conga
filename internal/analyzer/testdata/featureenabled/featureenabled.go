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

type stagedFeatures struct {
	HTTP3  bool `help:"Experimental listener" stage:"alpha"`
	Syntax int  `enum:"1,2" default:"2" help:"Syntax version" stage:"beta"`
}

// StagedCLI declares a fully staged feature set with help variables and an
// enumhint opt-out: every construct here is valid.
type StagedCLI struct {
	Features conga.Features[stagedFeatures] `flag:"feature-gates" env:"APP_GATES" help:"Feature gates ({{enum}}) via {{env}}"`

	Format conga.OptionalFlag[string] `enum:"json,yaml" enumhint:"-" help:"Output format"`
	Verb   conga.RequiredArg[string]  `enum:"get,post" enumhint:"-" help:"HTTP verb"`
}

// Run executes the CLI.
func (c *StagedCLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func runStaged() {
	app := conga.New[StagedCLI]().EnableFeatures()
	app.Run()
}
