// Package shortsinnocent pins the conservative degradation of the built-in
// short guards for multi-anchor packages: package-wide HelpShortFlag and
// VersionShortFlag evidence from one anchor must not produce a diagnostic on
// an innocent sibling root. The analyzer skips both guards entirely for
// packages anchoring more than one conga.New root — it may only miss
// diagnostics, never invent them.
package shortsinnocent

import (
	"context"

	"github.com/conga-sh/conga"
)

// OptionOwner assigns both built-in runes and reuses one for a flag; with a
// single anchor this collision and the shared rune would be reported.
type OptionOwner struct {
	Host conga.OptionalFlag[string] `name:"host" short:"h" help:"Host"`
}

// Run executes the option owner.
func (o *OptionOwner) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// InnocentSibling declares the same rune without assigning it to a built-in:
// no diagnostic may ever fire on this root, regardless of sibling builder
// options in the same package.
type InnocentSibling struct {
	Host conga.OptionalFlag[string] `name:"host" short:"h" help:"Host"`
}

// Run executes the innocent sibling.
func (i *InnocentSibling) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[InnocentSibling]()

var _ = conga.New[OptionOwner]().HelpShortFlag('h').VersionShortFlag('h')
