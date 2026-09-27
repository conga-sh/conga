// Package cascading covers the static cascading⟹multi guard for a
// single-anchor package: a root config-file flag without multi:"true" cannot
// report every merged file when the app declares Cascading(). Ambiguous
// builder evidence must degrade conservatively to no diagnostic.
package cascading

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
)

// CascadingSingleFlag declares Cascading() with a single-value config-file
// flag: the flag cannot report every merged file.
type CascadingSingleFlag struct { // want `does not implement conga\.Runner`
	Config conga.ConfigFile `name:"config"` // want `Cascading\(\) merges multiple configuration files; the root config-file flag "config" must declare multi:"true"`
}

var _ = conga.New[CascadingSingleFlag]().ConfigOptions(config.Cascading())
