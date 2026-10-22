package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
)

var errMetricsRequireTLS = errors.New("metrics require strict TLS: enable strict-tls or disable metrics")

// FeatureStateCmd reports the resolved feature state and per-key provenance,
// and demonstrates the validation and post-run lifecycle hooks on a subcommand.
type FeatureStateCmd struct{}

// Validate rejects the unobservable combination of an exposed metrics endpoint
// with strict TLS disabled: the endpoint would serve plaintext.
func (c *FeatureStateCmd) Validate(ctx context.Context, session *conga.Session) error {
	features := session.Root[KitchenSinkCLI]().Features.Get()

	if features.Metrics && !features.StrictTLS {
		return errMetricsRequireTLS
	}

	return nil
}

// Run reports the resolved feature state after the inspector prints every
// bound flag and its origin.
func (c *FeatureStateCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[KitchenSinkCLI]()
	inspector.Print(session, root, c)

	features := root.Features.Get()

	fmt.Fprintf(session.Stdout(), "features: http3=%t strict-tls=%t metrics=%t compression=%d\n",
		features.HTTP3, features.StrictTLS, features.Metrics, features.Compression)

	return nil
}

// PostRun runs on the success path, after Run returns without error.
func (c *FeatureStateCmd) PostRun(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] features PostRun: flushing feature report")

	return nil
}

// Finally runs unconditionally after the command completes, error or not.
func (c *FeatureStateCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	fmt.Fprintln(session.Stdout(), "[HOOK] features Finally: closing feature audit log")

	return nil
}

// ExperimentalCmd is gated behind the http3 feature. It is invisible and
// unusable until that feature is active.
type ExperimentalCmd struct {
	Probe conga.OptionalFlag[bool] `help:"Send a QUIC probe" short:"p"`
}

// Run reports the experimental listener state.
func (c *ExperimentalCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[KitchenSinkCLI]()
	inspector.Print(session, root, c)

	fmt.Fprintf(session.Stdout(), "experimental: QUIC listener on :%d probe=%t\n",
		root.HTTP3Port.Get(), c.Probe.Get())

	return nil
}

// kitchenFeatures declares the application's feature toggles. Each exported
// field is one feature: the field name (kebab-cased) is the feature name, a
// bool is an on/off toggle, and an integer is a leveled toggle where 0 means
// off and non-zero levels must be listed in the enum tag.
//
// Environment binding is derived from the container's envprefix tag as
// KITCHEN_SINK_FEATURE_<UPPER_SNAKE(feature name)>. A field may override that
// name with its own env tag (StrictTLS) or opt out entirely with env:"-"
// (Metrics). The stage tag annotates maturity in the help screen's Features
// section and is all-or-nothing across the set, so every field declares one.
type kitchenFeatures struct {
	HTTP3       bool `help:"Experimental QUIC/HTTP3 listener" stage:"alpha"`
	StrictTLS   bool `default:"true"                          env:"KITCHEN_SINK_STRICT" help:"Require TLS and reject weak ciphers"  stage:"beta"`
	Metrics     bool `default:"true"                          env:"-"                       help:"Expose the metrics endpoint"          stage:"beta"`
	Compression int  `default:"1"                             enum:"1,2"                    help:"Response compression level (0 = off)" stage:"beta"`
}
