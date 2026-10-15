// Package configconflictsiblings pins per-call conflict attribution: option
// evidence must not leak across distinct apps in one package, so a
// ConfigExplicit app and a ConfigPaths sibling both stay diagnostic-free.
package configconflictsiblings

import (
	"context"

	"github.com/conga-sh/conga"
)

type ExplicitApp struct {
	Port conga.OptionalFlag[int] `name:"port"`
}

func (e *ExplicitApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ExplicitApp]().ConfigOptions(conga.ConfigExplicit("app.json"))

type PathsApp struct {
	Port conga.OptionalFlag[int] `name:"port"`
}

func (p *PathsApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[PathsApp]().ConfigOptions(conga.ConfigPaths("search.json"))
