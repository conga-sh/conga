package spec

import (
	"strings"

	"github.com/conga-sh/conga/internal/casing"
)

const (
	// KeyCaseKebab spells derived keys as the names themselves (api-endpoint).
	KeyCaseKebab KeyCase = iota
	// KeyCaseSnake spells derived keys in snake_case (api_endpoint).
	KeyCaseSnake
	// KeyCaseCamel spells derived keys in camelCase (apiEndpoint).
	KeyCaseCamel
)

// KeyCase selects how config keys derived from flag, subcommand, and feature
// names are spelled. Every derived key has exactly one spelling; explicit
// config tags are always used as written.
type KeyCase uint8

// Apply spells a kebab-case flag, subcommand, or feature name in the key case.
func (k KeyCase) Apply(name string) string {
	switch k {
	case KeyCaseSnake:
		return casing.Snake(name)
	case KeyCaseCamel:
		return casing.Camel(name)
	case KeyCaseKebab:
		return name
	default:
		return name
	}
}

// ConfigKeyPath returns the one config key path that sets flag, spelled as a
// config file writes it: the owning commands' section names in the app's key
// case, then the flag's config tag as written, or its name in the key case
// when it has no tag. A tag that already starts with the owning command path
// names that path itself and is not prefixed twice. chain runs from the root
// command, which carries the key case, to the command declaring flag.
func ConfigKeyPath(flag *Flag, chain []*Command) string {
	if len(chain) == 0 {
		return flagConfigKey(flag, KeyCaseKebab)
	}

	keyCase := chain[0].ConfigSettings.KeyCase

	canonical := make([]string, 0, len(chain)-1)
	sections := make([]string, 0, len(chain))

	for _, cmd := range chain[1:] {
		canonical = append(canonical, cmd.Name)
		sections = append(sections, keyCase.Apply(cmd.Name))
	}

	key := flagConfigKey(flag, keyCase)

	if prefix := strings.Join(canonical, "."); prefix != "" && flag.ConfigKey != "" {
		key = strings.TrimPrefix(key, prefix+".")
	}

	return strings.Join(append(sections, key), ".")
}

func flagConfigKey(flag *Flag, keyCase KeyCase) string {
	if flag.ConfigKey != "" && flag.ConfigKey != "-" {
		return flag.ConfigKey
	}

	return keyCase.Apply(flag.Name)
}
