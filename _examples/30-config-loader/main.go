package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

// errUnknownProfile reports a --profile naming no stored profile.
var errUnknownProfile = errors.New("unknown profile")

// profileStore simulates the application's external configuration source: the
// document for a run is assembled in code, not loaded from files.
var profileStore = map[string]profile{
	"dev": {
		host:    "dev.example.com",
		port:    7100,
		headers: map[string]string{"X-Region": "eu"},
	},
	"prod": {
		host:     "prod.example.com",
		port:     7200,
		headers:  map[string]string{"X-Force": "on", "X-Region": "us"},
		forceAll: true,
	},
}

// CLI demonstrates conga.ConfigLoader: every run's configuration tier is built
// in code instead of loaded from files. The document keeps the config rank
// (CLI > environment > document > default), so --host and DEPLOY_HOST still
// outrank it, and the prod profile additionally carries the force-all gate.
type CLI struct {
	Profile conga.OptionalFlag[string] `default:"dev" help:"Stored profile to load" safe:"true"`

	Features conga.Features[experiments] `config:"experiments" env:"-"`

	Host     conga.OptionalFlag[string]            `default:"localhost"          help:"API host"                              safe:"true"`
	Port     conga.OptionalFlag[int]               `default:"8000"               help:"API port"                              safe:"true"`
	Headers  conga.OptionalFlag[map[string]string] `help:"Extra request headers"`
	ForceAll conga.OptionalFlag[bool]              `feature:"force-all"          help:"Skip confirmation prompts (prod only)"`
}

// Run reports the resolved configuration.
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, c)

	fmt.Fprintf(session.Stdout(), "connect %s:%d headers=%d force-all=%v\n",
		c.Host.Get(), c.Port.Get(), len(c.Headers.Get()), c.Features.Get().ForceAll)

	return nil
}

// profile is one stored deployment profile, mirroring what an embedded
// filesystem, a remote store, or a secrets manager would hand back.
type profile struct {
	host     string
	port     int
	headers  map[string]string
	forceAll bool
}

// experiments declares the feature toggles carried by the document.
type experiments struct {
	ForceAll bool `help:"Skip confirmation prompts" name:"force-all"`
}

// buildDocument is the conga.ConfigLoader: it runs after the command line is
// tokenized and before flags bind, so --profile selects which stored profile
// becomes the run's configuration document. The document is validated exactly
// like a decoded config file, and the origins it records surface in --why.
func buildDocument(ctx conga.ConfigContext) (*conga.ConfigDocument, error) {
	profileName, ok := ctx.FlagValue("profile")
	if !ok {
		profileName = "dev"
	}

	stored, ok := profileStore[profileName]
	if !ok {
		return nil, fmt.Errorf("%w %q", errUnknownProfile, profileName)
	}

	origin := "profile " + profileName

	document := &conga.ConfigDocument{}
	document.Set("host", stored.host, origin)
	document.Set("port", stored.port, origin)
	document.Set("headers", stored.headers, origin)

	if stored.forceAll {
		// The document is the only gate channel: the Features container
		// declares env:"-", so the prod profile's experiment cannot be set
		// from the environment.
		document.Set("experiments", map[string]string{"force-all": "true"}, origin)
	}

	return document, nil
}

func main() {
	app := conga.New[CLI]().
		Name("deploy").
		Description("Deployment CLI whose config tier is built in code").
		Version("1.0.0").
		EnableFeatures().
		EnableWhy().
		EnvPrefix("DEPLOY").
		ConfigOptions(conga.ConfigLoader(buildDocument))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
