package configengine

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
)

// validateCommandConfig validates raw parsed configuration data against the
// Command AST and populates dest with resolved key-value strings. Unknown keys
// or sections trigger strict configError failures.
func validateCommandConfig(prefix string, rawMap map[string]any, cmd *spec.Command, dest map[string]string, filePath string) error {
	for key, value := range rawMap {
		if err := validateSingleCommandEntry(prefix, key, value, cmd, dest, filePath); err != nil {
			return err
		}
	}

	return nil
}

func validateSingleCommandEntry(prefix, key string, value any, cmd *spec.Command, dest map[string]string, filePath string) error {
	if nestedMap, isMap := value.(map[string]any); isMap {
		return handleNestedSection(prefix, key, nestedMap, cmd, dest, filePath)
	}

	if flag := findFlag(cmd, key); flag != nil {
		if flag.ConfigKey == "-" {
			return &configError{
				path:  filePath,
				msg:   fmt.Sprintf("config file %q: field %q is excluded from config files in command %q", filePath, key, cmd.Name),
				cause: fmt.Errorf("%w: field %q not allowed in config", sentinel.ErrConfigUnknownField, key),
			}
		}

		storeFlagValue(prefix, flag, value, dest)

		return nil
	}

	return &configError{
		path:  filePath,
		msg:   fmt.Sprintf("config file %q: unknown field %q in command %q%s", filePath, key, cmd.Name, suggestionHint(cmd, key)),
		cause: fmt.Errorf("%w: unknown field %q in command %q", sentinel.ErrConfigUnknownField, key, cmd.Name),
	}
}

func handleNestedSection(prefix, key string, nestedMap map[string]any, cmd *spec.Command, dest map[string]string, filePath string) error {
	if subcommand := findSubcommand(cmd, key); subcommand != nil {
		subPrefix := cond.If(prefix != "", prefix+"."+subcommand.Name, subcommand.Name)

		return validateCommandConfig(subPrefix, nestedMap, subcommand, dest, filePath)
	}

	if flag := findFlag(cmd, key); flag != nil && flag.IsMap {
		storeFlagValue(prefix, flag, nestedMap, dest)

		return nil
	}

	if hasFlagWithPrefix(cmd, key) {
		subPrefix := cond.If(prefix != "", prefix+"."+key, key)

		return validateNamespaceConfig(subPrefix, nestedMap, cmd, dest, filePath)
	}

	return &configError{
		path:  filePath,
		msg:   fmt.Sprintf("config file %q: unknown section %q in command %q%s", filePath, key, cmd.Name, suggestionHint(cmd, key)),
		cause: fmt.Errorf("%w: unknown section %q in command %q", sentinel.ErrConfigUnknownField, key, cmd.Name),
	}
}

func validateNamespaceConfig(prefix string, rawMap map[string]any, cmd *spec.Command, dest map[string]string, filePath string) error {
	for key, value := range rawMap {
		fullKey := prefix + "." + key

		if nestedMap, isMap := value.(map[string]any); isMap {
			if err := validateNamespaceConfig(fullKey, nestedMap, cmd, dest, filePath); err != nil {
				return err
			}

			continue
		}

		flag := findFlagByConfigKeyOrName(cmd, fullKey, key)
		if flag != nil {
			if flag.ConfigKey == "-" {
				return &configError{
					path:  filePath,
					msg:   fmt.Sprintf("config file %q: field %q is excluded from config files in command %q", filePath, fullKey, cmd.Name),
					cause: fmt.Errorf("%w: field %q not allowed in config", sentinel.ErrConfigUnknownField, fullKey),
				}
			}

			storeFlagValue(prefix, flag, value, dest)
			dest[fullKey] = formatConfigValue(value)

			continue
		}

		return &configError{
			path:  filePath,
			msg:   fmt.Sprintf("config file %q: unknown field %q in namespace %q%s", filePath, key, prefix, suggestionHint(cmd, key)),
			cause: fmt.Errorf("%w: unknown field %q in namespace %q", sentinel.ErrConfigUnknownField, key, prefix),
		}
	}

	return nil
}

func hasFlagWithPrefix(cmd *spec.Command, prefix string) bool {
	normPrefix := normalizeConfigKey(prefix)

	for _, flag := range cmd.Flags {
		if flag.ConfigKey != "" && flag.ConfigKey != "-" {
			if first, _, hasDot := strings.Cut(flag.ConfigKey, "."); hasDot {
				if matchesConfigName(first, prefix, normPrefix) {
					return true
				}
			}
		}
	}

	return false
}

func findFlagByConfigKeyOrName(cmd *spec.Command, fullKey, subKey string) *spec.Flag {
	normFull := normalizeConfigKey(fullKey)
	normSub := normalizeConfigKey(subKey)

	for _, flag := range cmd.Flags {
		if flag.IsConfigFile {
			continue
		}

		if flag.ConfigKey == "-" {
			if matchesConfigName(flag.Name, subKey, normSub) || slices.ContainsFunc(flag.Aliases, func(alias string) bool {
				return matchesConfigName(alias, subKey, normSub)
			}) {
				return flag
			}

			continue
		}

		if flag.ConfigKey != "" && matchesConfigName(flag.ConfigKey, fullKey, normFull) {
			return flag
		}

		if matchesConfigName(flag.Name, subKey, normSub) {
			return flag
		}

		if slices.ContainsFunc(flag.Aliases, func(alias string) bool {
			return matchesConfigName(alias, subKey, normSub)
		}) {
			return flag
		}
	}

	return nil
}

func storeFlagValue(prefix string, flag *spec.Flag, value any, dest map[string]string) {
	canonicalKey := cond.If(prefix != "", prefix+"."+flag.Name, flag.Name)
	formatted := formatConfigValue(value)

	// Store by explicit ConfigKey if defined. For subcommand flags the key is
	// namespaced under the command path so it cannot collide with an ancestor
	// flag that shares the same config key (e.g. root "ports" and "network.ports").
	if flag.ConfigKey != "" && flag.ConfigKey != "-" {
		configKey := flag.ConfigKey
		if prefix != "" && !strings.HasPrefix(flag.ConfigKey, prefix+".") {
			configKey = prefix + "." + flag.ConfigKey
		}

		dest[configKey] = formatted
	}

	// Store by canonical path (e.g. "port" or "cluster.region")
	dest[canonicalKey] = formatted

	// Also store by direct flag name for root
	if prefix == "" {
		dest[flag.Name] = formatted
	}
}

func formatConfigValue(value any) string {
	switch typedValue := value.(type) {
	case []any:
		return strings.Join(slicex.Map(typedValue, stringx.Stringify), ",")
	case []string:
		return strings.Join(typedValue, ",")
	case map[string]any:
		pairs := make([]string, 0, len(typedValue))
		for key, item := range typedValue {
			pairs = append(pairs, key+"="+stringx.Stringify(item))
		}

		slices.Sort(pairs)

		return strings.Join(pairs, ",")
	default:
		return stringx.Stringify(value)
	}
}

func findFlag(cmd *spec.Command, key string) *spec.Flag {
	return findFlagByConfigKeyOrName(cmd, key, key)
}

func findSubcommand(cmd *spec.Command, key string) *spec.Command {
	normalizedKey := normalizeConfigKey(key)

	for _, subcommand := range cmd.Subcommands {
		if matchesConfigName(subcommand.Name, key, normalizedKey) {
			return subcommand
		}

		if slices.ContainsFunc(subcommand.Aliases, func(alias string) bool {
			return matchesConfigName(alias, key, normalizedKey)
		}) {
			return subcommand
		}
	}

	return nil
}

func matchesConfigName(candidate, rawKey, normKey string) bool {
	return candidate == rawKey || normalizeConfigKey(candidate) == normKey
}

func normalizeConfigKey(key string) string {
	for i := range len(key) {
		character := key[i]
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return normalizeConfigKeySlow(key)
		}
	}

	return key
}

func normalizeConfigKeySlow(key string) string {
	var builder strings.Builder

	for _, character := range key {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(unicode.ToLower(character))
		}
	}

	return builder.String()
}

func findClosestSuggestion(cmd *spec.Command, key string) string {
	var candidates []string

	for _, flag := range cmd.Flags {
		if flag.ConfigKey != "-" {
			candidates = append(candidates, flag.Name, casing.Snake(flag.Name))
			if flag.ConfigKey != "" {
				candidates = append(candidates, flag.ConfigKey)
			}
		}
	}

	for _, subcommand := range cmd.Subcommands {
		candidates = append(candidates, subcommand.Name, casing.Snake(subcommand.Name))
	}

	return levenshtein.Suggest(key, candidates, 4)
}

func suggestionHint(cmd *spec.Command, key string) string {
	suggestion := findClosestSuggestion(cmd, key)
	if suggestion == "" {
		return ""
	}

	return fmt.Sprintf(" (did you mean %q?)", suggestion)
}
