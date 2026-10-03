// Package config provides configuration-file discovery and merging options
// for CONGA applications.
//
// Pass options to [conga.App.ConfigOptions]:
//
//	// First existing file wins (search tier):
//	app.ConfigOptions(config.Paths("custom.yaml", "/etc/app/config.yaml"))
//
//	// Merge all existing files (override + base):
//	app.ConfigOptions(
//	    config.Paths("override.yaml", "base.yaml"),
//	    config.Cascading(),
//	)
//
//	// Required file (fatal if missing):
//	app.ConfigOptions(config.Explicit("/etc/app/mandatory.yaml"))
package config

import (
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/configengine"
)

// Option declares one slot of the application's configuration-file resolution
// settings for [conga.App.ConfigOptions]. Options compose and are independent of
// call order; repeating an option replaces its slot. Option is a sealed interface:
// values are produced exclusively by this package's constructors ([Paths],
// [Explicit], [Cascading]) and cannot be implemented by other packages.
type Option interface {
	applyTo(settings *Settings)
}

// Settings is the resolved snapshot of an application's configuration-file
// resolution options.
//
// Settings is engine-facing: [Apply] produces it and [conga.App.ConfigOptions]
// consumes it through the engine's own settings representation. Application code
// declares intent through [Option] values, never by populating Settings directly —
// a hand-built Settings has no effect on resolution, because ConfigOptions reads
// only the options it is passed.
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

// Decoder parses raw configuration bytes into a generic key-value map.
//
// Application code supplies decoders to [conga.App.RegisterDecoder], which
// associates one with a file extension. A decoder unmarshals the file into a
// nested map with string keys, for example:
//
//	func RawDecoder(data []byte) (map[string]any, error) {
//		var parsed map[string]any
//		if err := json.Unmarshal(data, &parsed); err != nil {
//			return nil, fmt.Errorf("decode json config: %w", err)
//		}
//
//		return parsed, nil
//	}
//
// Ready-made decoders ship as separate packages — decoder/json, decoder/yaml,
// decoder/toml, and decoder/hcl — so that unreferenced formats stay out of the
// linked binary.
type Decoder = configengine.Decoder

// Paths declares the author configuration search tier for [conga.App.ConfigOptions]:
// the paths are probed in precedence order and the first existing configuration
// file wins unless [Cascading] is also declared. Declaring paths replaces the
// built-in conventional configuration tier
// (${XDG_CONFIG_HOME:-~/.config}/<app-name>).
//
// Panics:
//   - If called with no paths.
//   - If any path entry is empty or whitespace-only.
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
// (author-declared via [Paths], otherwise the built-in conventional tier)
// instead of stopping at the first existing file. The first-listed tier path
// has the highest precedence on key conflicts; later files still supply keys
// the earlier ones omit. A declared root [conga.ConfigFile] flag must accept
// multiple values (`multi:"true"`); the schema rejects the combination otherwise.
func Cascading() Option {
	return optionFunc(func(settings *Settings) {
		settings.Cascading = true
	})
}

// Explicit names one concrete configuration file with fatal-if-missing
// semantics for [conga.App.ConfigOptions]: the file must exist and be readable.
// It outranks the configuration environment variable (<PREFIX>_CONFIG) and the
// search tier, and bypasses the built-in conventional tier.
//
// Panics:
//   - If path is empty or whitespace-only.
func Explicit(path string) Option {
	if strings.TrimSpace(path) == "" {
		panic("conga: app.ConfigOptions(config.Explicit(...)): the explicit path cannot be empty")
	}

	return optionFunc(func(settings *Settings) {
		settings.Explicit = path
	})
}

// Apply resolves options into a [Settings] snapshot in call order. Options
// declare independent slots, so call order never changes the outcome, and a
// repeated option keeps its last declaration. It is engine-facing: called by
// [conga.App.ConfigOptions] to derive the application's resolution settings.
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
