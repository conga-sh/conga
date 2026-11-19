// Package enumtargetclean holds the fields next to the value-type tag checks
// that stay valid: glob on conga.File, domain tags on *conga.File, enum with a non-path predictor or a
// predictable domain type, named string map keys, a self-parsing named map, and
// a method that only resembles a filterable one.
package enumtargetclean

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/predict"
)

// Key is a named string map key.
type Key string

// Labels is a named map that parses itself, so it is one value.
type Labels map[string]string

func (l *Labels) UnmarshalText(text []byte) error { return nil }

// NotFilter has a MatchFile method with the wrong signature.
type NotFilter string

func (NotFilter) MatchFile(path string) bool { return true }

func (f *NotFilter) UnmarshalText(text []byte) error { *f = NotFilter(text); return nil }

type Clean struct {
	Config    conga.OptionalFlag[conga.File]         `glob:"*.yaml"`
	Pointer   conga.OptionalFlag[*conga.File]        `existing:"true"`
	Pointers  conga.OptionalFlag[[]*conga.File]      `glob:"*.yaml"`
	None      conga.OptionalFlag[string]             `enum:"a,b" predictor:"none"`
	Off       conga.OptionalFlag[string]             `enum:"a,b" predictor:"off"`
	Envs      conga.OptionalFlag[string]             `enum:"a,b" predictor:"envs"`
	Method    conga.OptionalFlag[network.HTTPMethod] `enum:"GET,POST"`
	Keyed     conga.OptionalFlag[map[Key]string]
	Labels    conga.OptionalFlag[Labels]    `enum:"a"`
	NotFilter conga.OptionalFlag[NotFilter] `enum:"a"`
}

func (c *Clean) Run(ctx context.Context, session *conga.Session) error { return nil }

var _ = conga.New[Clean]().Predictor("envs", predict.Values("a", "b"))
