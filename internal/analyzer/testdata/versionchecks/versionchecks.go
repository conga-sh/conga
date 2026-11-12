// Package versionchecks exercises the built-in version command's --json
// ancestor shadowing contract.
package versionchecks

import (
	"context"

	"github.com/conga-sh/conga"
)

// versionShadowApp declares the VersionCommand field before the colliding
// root flag, so the diagnostic must not depend on field order.
type versionShadowApp struct {
	Version conga.VersionCommand // want `command "version": schema error: duplicate flag: --json \(already declared by ancestor command "version-shadow-app"; the built-in version command owns --json: rename the ancestor flag or remove the VersionCommand field\)`

	JSON conga.OptionalFlag[bool] `help:"Output JSON"`
}

func (a *versionShadowApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[versionShadowApp]()

// namedVersionShadowApp checks that the diagnostic names the built-in command
// by its declared name.
type namedVersionShadowApp struct {
	Version conga.VersionCommand `name:"v"` // want `command "v": schema error: duplicate flag: --json \(already declared by ancestor command "named-version-shadow-app"; the built-in version command owns --json: rename the ancestor flag or remove the VersionCommand field\)`

	JSON conga.OptionalFlag[bool] `help:"Output JSON"`
}

func (a *namedVersionShadowApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[namedVersionShadowApp]()

// versionOnlyApp is the clean path: the version command's --json selector has
// no ancestor collision.
type versionOnlyApp struct {
	Port conga.OptionalFlag[int] `help:"Listen port"`

	Version conga.VersionCommand
}

func (a *versionOnlyApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[versionOnlyApp]()
