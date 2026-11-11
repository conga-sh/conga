package conga

import (
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/spec"
)

// ConfigOption declares one slot of the application's configuration
// resolution settings for [App.ConfigOptions]. Options compose and are
// independent of call order; repeating an option replaces its slot.
// ConfigOption is a sealed interface: values are produced exclusively by the
// constructors [ConfigPaths], [ConfigExplicit], [ConfigCascading], and
// [ConfigKeyCase], and cannot be implemented by other packages.
type ConfigOption interface {
	applyTo(settings *spec.ConfigSettings)
}

// KeyCase names how config files spell the keys CONGA derives from flag,
// subcommand, and feature names, for [ConfigKeyCase].
type KeyCase uint8

const (
	// KebabCase spells derived keys as the names themselves: --api-endpoint
	// reads api-endpoint. It is the default.
	KebabCase KeyCase = iota
	// SnakeCase spells derived keys in snake_case: --api-endpoint reads
	// api_endpoint.
	SnakeCase
	// CamelCase spells derived keys in camelCase: --api-endpoint reads
	// apiEndpoint.
	CamelCase
)

// ConfigDecoder parses raw configuration bytes into a generic key-value map.
//
// Application code supplies decoders to [App.RegisterDecoder], which
// associates one with a file extension. A decoder unmarshals the file into a
// nested map with string keys, for example:
//
//	func Decode(data []byte) (map[string]any, error) {
//		var parsed map[string]any
//		if err := json.Unmarshal(data, &parsed); err != nil {
//			return nil, fmt.Errorf("decode json config: %w", err)
//		}
//
//		return parsed, nil
//	}
//
// Decoders for JSON, YAML, TOML, and HCL ship with CONGA in the
// decoder/json, decoder/yaml, decoder/toml, and decoder/hcl packages.
type ConfigDecoder = configengine.Decoder

// ConfigPaths declares the author configuration search tier for
// [App.ConfigOptions]: the paths are probed in precedence order and the first
// existing configuration file wins unless [ConfigCascading] is also declared.
// Declaring paths replaces the built-in conventional configuration tier:
//
//	${XDG_CONFIG_HOME:-~/.config}/<app-name>
//
// Panics:
//   - If called with no paths.
//   - If any path entry is empty or whitespace-only.
func ConfigPaths(paths ...string) ConfigOption {
	if len(paths) == 0 {
		panic("conga: app.ConfigOptions(conga.ConfigPaths(...)): at least one path is required")
	}

	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			panic("conga: app.ConfigOptions(conga.ConfigPaths(...)): path entries must be non-empty")
		}
	}

	tier := slices.Clone(paths)

	return optionFunc(func(settings *spec.ConfigSettings) {
		settings.Tier = slices.Clone(tier)
	})
}

// ConfigCascading merges every existing configuration file found in the search
// tier (author-declared via [ConfigPaths], otherwise the built-in conventional
// tier) instead of stopping at the first existing file. The first-listed tier
// path has the highest precedence on key conflicts; later files still supply
// keys the earlier ones omit. A declared root [ConfigFile] flag must accept
// multiple values (the multi tag); the schema rejects the combination
// otherwise.
func ConfigCascading() ConfigOption {
	return optionFunc(func(settings *spec.ConfigSettings) {
		settings.Cascading = true
	})
}

// ConfigExplicit names one concrete configuration file with fatal-if-missing
// semantics for [App.ConfigOptions]: the file must exist and be readable.
// It is the application's default file. A path named by the user outranks it:
// the root [ConfigFile] flag first, then the configuration environment
// variable (<PREFIX>_CONFIG). It replaces the built-in conventional search
// location, and cannot be combined with [ConfigPaths] or [ConfigCascading].
// Loaded values report the config source, as values from a search path do.
//
// Panics:
//   - If path is empty or whitespace-only.
func ConfigExplicit(path string) ConfigOption {
	if strings.TrimSpace(path) == "" {
		panic("conga: app.ConfigOptions(conga.ConfigExplicit(...)): the explicit path cannot be empty")
	}

	return optionFunc(func(settings *spec.ConfigSettings) {
		settings.Explicit = path
	})
}

// ConfigKeyCase sets how config files spell the keys derived from flag names,
// subcommand section names, and feature names in the features section, for
// [App.ConfigOptions]. Each key has exactly one spelling: under [SnakeCase],
// --api-endpoint reads api_endpoint, and api-endpoint is an unknown key.
// Explicit config tags are used as written in every case. The key case
// applies to config files and to [ConfigDocument] keys alike; the command
// line and the environment are unaffected. Without this option, keys use
// [KebabCase].
//
// Panics:
//   - If keyCase is not [KebabCase], [SnakeCase], or [CamelCase].
func ConfigKeyCase(keyCase KeyCase) ConfigOption {
	var internal spec.KeyCase

	switch keyCase {
	case KebabCase:
		internal = spec.KeyCaseKebab
	case SnakeCase:
		internal = spec.KeyCaseSnake
	case CamelCase:
		internal = spec.KeyCaseCamel
	default:
		panic("conga: app.ConfigOptions(conga.ConfigKeyCase(...)): unknown key case")
	}

	return optionFunc(func(settings *spec.ConfigSettings) {
		settings.KeyCase = internal
	})
}

type optionFunc func(*spec.ConfigSettings)

func (f optionFunc) applyTo(settings *spec.ConfigSettings) {
	f(settings)
}

// foldConfigOptions resolves options into an engine settings snapshot in call
// order. Options declare independent slots, so call order never changes the
// outcome, and a repeated option keeps its last declaration.
func foldConfigOptions(options []ConfigOption) spec.ConfigSettings {
	var settings spec.ConfigSettings

	for _, option := range options {
		option.applyTo(&settings)
	}

	return settings
}
