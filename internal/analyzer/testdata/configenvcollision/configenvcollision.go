// Package configenvcollision exercises the conga-lint mirror of the config env
// collision guard against a declared root ConfigFile: flags, subcommand flags,
// feature fields, and the feature map bound to the ConfigFile's variable.
package configenvcollision

import (
	"context"

	"github.com/conga-sh/conga"
)

// A root flag bound to the ConfigFile's variable.
type flagApp struct {
	ConfigFile conga.ConfigFile        `env:"APP_CONFIG" help:"Config file"`
	Port       conga.OptionalFlag[int] `env:"APP_CONFIG" help:"Port"` // want `struct flagApp field Port: environment variable "APP_CONFIG" is bound by both flag --config-file and flag --port`
}

func (a *flagApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[flagApp]()

// A subcommand flag bound to the root ConfigFile's variable.
type deployCommand struct {
	Region conga.OptionalFlag[string] `env:"APP_CONFIG" help:"Region"` // want `struct deployCommand field Region: environment variable "APP_CONFIG" is bound by both flag --settings and flag --region`
}

func (c *deployCommand) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type subcommandApp struct {
	ConfigFile conga.ConfigFile             `env:"APP_CONFIG" help:"Config file" name:"settings"`
	Deploy     conga.Command[deployCommand] `summary:"Deploy"`
}

var _ = conga.New[subcommandApp]()

// A feature field bound to the ConfigFile's variable.
type featureSet struct {
	Config bool `help:"Config"`
}

type featureApp struct {
	ConfigFile conga.ConfigFile           `env:"PC_CONFIG"  help:"Config file"`
	Features   conga.Features[featureSet] `envprefix:"PC_" flag:"feature-gates"` // want `struct featureApp field Features: environment variable "PC_CONFIG" is bound by both flag --config-file and feature "config"`
}

func (a *featureApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureApp]().EnableFeatures()

// The feature map variable, with and without a feature map flag.
type gateSet struct {
	Debug bool `help:"Debug"`
}

type mapFlagApp struct {
	ConfigFile conga.ConfigFile        `env:"APP_CONFIG" help:"Config file"`
	Features   conga.Features[gateSet] `env:"APP_CONFIG" flag:"feature-gates"` // want `struct mapFlagApp field Features: environment variable "APP_CONFIG" is bound by both flag --config-file and the feature map flag --feature-gates`
}

func (a *mapFlagApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[mapFlagApp]().EnableFeatures()

type mapNoFlagApp struct {
	ConfigFile conga.ConfigFile        `env:"APP_CONFIG" help:"Config file"`
	Features   conga.Features[gateSet] `env:"APP_CONFIG"` // want `struct mapNoFlagApp field Features: environment variable "APP_CONFIG" is bound by both flag --config-file and the Features container`
}

func (a *mapNoFlagApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[mapNoFlagApp]().EnableFeatures()
