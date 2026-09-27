// Package config declares the configuration-file authoring surface for CONGA
// applications: the [Option] constructors ([Paths], [Explicit], and
// [Cascading]) that control how configuration files are discovered and merged,
// and the [Decoder] type for custom configuration formats. Options are passed
// to App.ConfigOptions and snapshotted by [Apply] into engine-facing
// [Settings]. The resolution engine behind these declarations is internal to
// CONGA.
package config

import (
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/configengine"
)

// Option declares one slot of the application's configuration-file resolution
// settings for app.ConfigOptions. Options compose and are independent of call
// order; repeating an option replaces its slot. Option is a sealed interface:
// values are produced exclusively by this package's constructors (Paths,
// Explicit, Cascading) and cannot be implemented by other packages.
type Option interface {
	applyTo(settings *Settings)
}

// Settings is the resolved snapshot of an application's configuration-file
// resolution options. It is engine-facing: produced by Apply and consumed by
// conga.App.ConfigOptions through the engine's own settings representation.
// Constructing a Settings value directly has no effect: App.ConfigOptions
// consumes Option values only, and the fields below are populated by Apply.
type Settings struct {
	// Explicit names one concrete configuration file with fatal-if-missing
	// semantics. It outranks the configuration environment variable and the
	// search tier.
	Explicit string

	// Tier lists the author-declared search paths probed in precedence order.
	// Declaring a tier replaces the built-in conventional XDG tier.
	Tier []string

	// Cascading merges every existing tier file in precedence order (lowest to
	// highest) instead of stopping at the first existing file.
	Cascading bool
}

// Decoder parses raw configuration bytes into a generic key-value map for AST
// validation.
type Decoder = configengine.Decoder

// Paths declares the author configuration search tier for app.ConfigOptions: the
// paths are probed in precedence order and the first existing configuration
// file wins unless Cascading is also declared. Declaring paths replaces the
// built-in conventional configuration tier
// (${XDG_CONFIG_HOME:-~/.config}/<app-name>). Panics on a zero-argument call
// or on empty path entries — both are developer defects: a zero-argument call
// would silently leave the built-in tier in place, and a blank tier entry is
// never a meaningful search path.
func Paths(paths ...string) Option {
	if len(paths) == 0 {
		panic("conga: app.ConfigOptions(config.Paths(...)): at least one path is required")
	}

	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			panic("conga: app.ConfigOptions(config.Paths(...)): path entries must be non-empty")
		}
	}

	tier := slices.Clone(paths)

	return optionFunc(func(settings *Settings) {
		settings.Tier = slices.Clone(tier)
	})
}

// Cascading merges every existing configuration file found in the search tier
// (author-declared via Paths, otherwise the built-in conventional tier) in
// precedence order, lowest to highest, instead of stopping at the first
// existing file. A declared root ConfigFile flag must accept multiple values
// (multi:"true"); the schema rejects the combination otherwise.
func Cascading() Option {
	return optionFunc(func(settings *Settings) {
		settings.Cascading = true
	})
}

// Explicit names one concrete configuration file with fatal-if-missing
// semantics for app.ConfigOptions: the file must exist and be readable. It outranks
// the configuration environment variable (<PREFIX>_CONFIG) and the search
// tier, and bypasses the built-in conventional tier.
func Explicit(path string) Option {
	if strings.TrimSpace(path) == "" {
		panic("conga: app.ConfigOptions(config.Explicit(...)): the explicit path cannot be empty")
	}

	return optionFunc(func(settings *Settings) {
		settings.Explicit = path
	})
}

// Apply resolves options into a Settings snapshot in call order. Options
// declare independent slots, so call order never changes the outcome, and a
// repeated option keeps its last declaration. It is engine-facing: called by
// conga.App.ConfigOptions to derive the application's resolution settings.
func Apply(options ...Option) Settings {
	var settings Settings

	for _, option := range options {
		option.applyTo(&settings)
	}

	return settings
}

type optionFunc func(*Settings)

func (f optionFunc) applyTo(settings *Settings) {
	f(settings)
}
