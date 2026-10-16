package configengine

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
)

// ValidateDocument validates one programmatic configuration document entry
// against the Command AST and populates dest with the resolved values, exactly
// as a decoded file entry is validated. rawMap holds one top-level document
// key in the decoder output shape (a scalar, a slice, or a nested map for
// sections and map flags); origin labels the value's provenance in stored
// values and validation errors, and may be any free-form application-supplied
// source label.
func ValidateDocument(rawMap map[string]any, cmd *spec.Command, dest *configdata.Data, origin string) error {
	return validateCommandConfig("", rawMap, cmd, dest, origin)
}

// validateCommandConfig validates raw parsed configuration data against the
// Command AST and populates dest with the resolved values. Unknown keys or
// sections trigger strict configError failures.
func validateCommandConfig(prefix string, rawMap map[string]any, cmd *spec.Command, dest *configdata.Data, filePath string) error {
	for key, value := range rawMap {
		if err := validateSingleCommandEntry(prefix, key, value, cmd, dest, filePath); err != nil {
			return err
		}
	}

	return nil
}

func validateSingleCommandEntry(prefix, key string, value any, cmd *spec.Command, dest *configdata.Data, filePath string) error {
	if nestedMap, isMap := value.(map[string]any); isMap {
		return handleNestedSection(prefix, key, nestedMap, cmd, dest, filePath)
	}

	if flag := findFlag(cmd, key); flag != nil {
		if flag.ConfigKey == "-" {
			return newExcludedConfigFieldError(filePath, key, cmd.Name)
		}

		storeFlagValue(prefix, flag, value, dest, filePath)

		return nil
	}

	return newFieldConfigError(
		filePath,
		fmt.Sprintf("unknown field %q in command %q%s", key, cmd.Name, suggestionHint(cmd, key)),
		fmt.Sprintf("unknown field %q in command %q", key, cmd.Name),
	)
}

func handleNestedSection(prefix, key string, nestedMap map[string]any, cmd *spec.Command, dest *configdata.Data, filePath string) error {
	if subcommand := findSubcommand(cmd, key); subcommand != nil {
		subPrefix := cond.If(prefix != "", prefix+"."+subcommand.Name, subcommand.Name)

		return validateCommandConfig(subPrefix, nestedMap, subcommand, dest, filePath)
	}

	if flag := findFlag(cmd, key); flag != nil && flag.IsMap {
		if flag.ConfigKey == "-" {
			return newExcludedConfigFieldError(filePath, key, cmd.Name)
		}

		storeFlagValue(prefix, flag, nestedMap, dest, filePath)

		return nil
	}

	if hasFlagWithPrefix(cmd, key) {
		return validateNamespaceConfig(prefix, key, nestedMap, cmd, dest, filePath)
	}

	return newFieldConfigError(
		filePath,
		fmt.Sprintf("unknown section %q in command %q%s", key, cmd.Name, suggestionHint(cmd, key)),
		fmt.Sprintf("unknown section %q in command %q", key, cmd.Name),
	)
}

// newExcludedConfigFieldError rejects a configuration entry addressed at a
// flag excluded from config files, the strict-validation counterpart of the
// binder's refusal to read such keys.
func newExcludedConfigFieldError(filePath, fieldKey, commandName string) error {
	return newFieldConfigError(
		filePath,
		fmt.Sprintf("field %q is excluded from config files in command %q", fieldKey, commandName),
		fmt.Sprintf("field %q not allowed in config", fieldKey),
	)
}

func validateNamespaceConfig(cmdPrefix, nsPrefix string, rawMap map[string]any, cmd *spec.Command, dest *configdata.Data, filePath string) error {
	displayNamespace := cond.If(cmdPrefix != "", cmdPrefix+"."+nsPrefix, nsPrefix)

	for key, value := range rawMap {
		fullKey := nsPrefix + "." + key

		displayKey := displayNamespace + "." + key

		flag := findFlagByConfigKeyOrName(cmd, fullKey, key)

		nestedMap, isMap := value.(map[string]any)

		// A map flag addressed at a dotted config key consumes the nested
		// section itself, mirroring handleNestedSection's check order (map
		// flag before namespace recursion); every other nested map keeps
		// recursing into the namespace.
		if flag != nil && flag.IsMap && isMap {
			if flag.ConfigKey == "-" {
				return newExcludedConfigFieldError(filePath, displayKey, cmd.Name)
			}

			storeFlagValue(cmdPrefix, flag, nestedMap, dest, filePath)

			continue
		}

		if isMap {
			if err := validateNamespaceConfig(cmdPrefix, fullKey, nestedMap, cmd, dest, filePath); err != nil {
				return err
			}

			continue
		}

		if flag != nil {
			if flag.ConfigKey == "-" {
				return newExcludedConfigFieldError(filePath, displayKey, cmd.Name)
			}

			storeFlagValue(cmdPrefix, flag, value, dest, filePath)

			continue
		}

		return newFieldConfigError(
			filePath,
			fmt.Sprintf("unknown field %q in namespace %q%s", key, displayNamespace, suggestionHint(cmd, key)),
			fmt.Sprintf("unknown field %q in namespace %q", key, displayNamespace),
		)
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

// storeFlagValue records one config file's raw value for a flag under every
// bindable config key. Scalar and slice values are stringified and replace
// whatever an earlier file stored; map flags contribute structured per-key
// entries so cascading files merge key by key at load time.
func storeFlagValue(prefix string, flag *spec.Flag, value any, dest *configdata.Data, filePath string) {
	aliasKeys := flagConfigKeys(prefix, flag)

	if flag.IsMap {
		entries := dest.EnsureMap(aliasKeys)
		storeMapEntries(flag, value, entries, filePath)

		return
	}

	formatted := formatConfigValue(value)
	for _, aliasKey := range aliasKeys {
		dest.SetScalar(aliasKey, formatted, filePath)
	}
}

func storeMapEntries(flag *spec.Flag, value any, entries map[string]configdata.Value, origin string) {
	if nestedMap, isMap := value.(map[string]any); isMap {
		for key, item := range nestedMap {
			if flag.KeyCanonicalizer != nil {
				key = flag.KeyCanonicalizer(key)
			}

			entries[key] = configdata.Value{Raw: stringx.Stringify(item), Origin: origin}
		}

		return
	}

	if stringMap, isStringMap := value.(map[string]string); isStringMap {
		for key, item := range stringMap {
			if flag.KeyCanonicalizer != nil {
				key = flag.KeyCanonicalizer(key)
			}

			entries[key] = configdata.Value{Raw: item, Origin: origin}
		}

		return
	}

	switch typedValue := value.(type) {
	case []any:
		for _, item := range typedValue {
			storeFlatMapToken(flag, stringx.Stringify(item), entries, origin)
		}

	case []string:
		for _, item := range typedValue {
			storeFlatMapToken(flag, item, entries, origin)
		}

	default:
		sep := cmp.Or(flag.Sep, ",")
		for item := range strings.SplitSeq(formatConfigValue(value), sep) {
			storeFlatMapToken(flag, item, entries, origin)
		}
	}
}

// storeFlatMapToken parses one flat key=value token into entries, recording
// tokens without a separator or key as malformed pairs so binding surfaces
// them with config attribution. Each token is one pair: a value containing
// the flag's sep is never re-split.
func storeFlatMapToken(flag *spec.Flag, token string, entries map[string]configdata.Value, origin string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}

	key, val, ok := strings.Cut(token, "=")

	key = strings.TrimSpace(key)
	if !ok || key == "" {
		entries[""] = configdata.Value{Raw: token, Origin: origin}

		return
	}

	if flag.KeyCanonicalizer != nil {
		key = flag.KeyCanonicalizer(key)
	}

	entries[key] = configdata.Value{Raw: strings.TrimSpace(val), Origin: origin}
}

// flagConfigKeys returns the bindable config keys a flag's stored value is
// written to: the explicit config tag (namespaced under the command path for
// subcommand flags so it cannot collide with an ancestor flag sharing the
// same config key), the flag's canonical dotted path, and the bare flag name
// for root-owned flags. Keys are deduplicated so a value or contribution is
// stored once per distinct key.
func flagConfigKeys(prefix string, flag *spec.Flag) []string {
	var keys []string

	addKey := func(key string) {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}

	if flag.ConfigKey != "" && flag.ConfigKey != "-" {
		configKey := flag.ConfigKey
		if prefix != "" && !strings.HasPrefix(flag.ConfigKey, prefix+".") {
			configKey = prefix + "." + flag.ConfigKey
		}

		addKey(configKey)
	}

	canonicalKey := cond.If(prefix != "", prefix+"."+flag.Name, flag.Name)
	addKey(canonicalKey)

	return keys
}

func formatConfigValue(value any) string {
	switch typedValue := value.(type) {
	case []any:
		return strings.Join(slicex.Map(typedValue, stringx.Stringify), ",")
	case []string:
		return strings.Join(typedValue, ",")
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
