// Package configconflict tests the static rejection of combining
// conga.ConfigExplicit() with conga.ConfigPaths() or conga.ConfigCascading().
package configconflict

import (
	"context"

	"github.com/conga-sh/conga"
)

type ConflictApp struct {
	Port conga.OptionalFlag[int] `name:"port"`
}

func (c *ConflictApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ConflictApp]().
	ConfigOptions(conga.ConfigExplicit("app.json"), conga.ConfigPaths("search.json")) // want `conga\.ConfigExplicit\(\) specifies an absolute configuration file and cannot be combined with conga\.ConfigPaths\(\) or conga\.ConfigCascading\(\)`
