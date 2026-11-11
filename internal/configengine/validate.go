package configengine

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

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
// source label. scope optionally names a region within origin.
func ValidateDocument(rawMap map[string]any, cmd *spec.Command, dest *configdata.Data, origin, scope string) error {
	return newDocumentValidator(cmd, dest, origin, scope).command("", rawMap, cmd)
}

// documentValidator validates one decoded document (a config file or one
// programmatic document entry) against the Command AST.
//
// Every flag has exactly one config key, matched byte for byte: its config
// tag as written, or its name in the app's key case when it has none.
// Subcommand sections and feature keys follow the key case too. Flag and subcommand aliases
// never match, and neither does another spelling or letter case. A dotted tag
// (`db.host`) is matched only as nested sections (`host` inside `db`), so a
// literal "db.host" key is unknown. Each flag is therefore reachable through
// one path, and a decoded map cannot set it twice. Keys are visited sorted so
// the first reported error is stable when a document holds several bad keys.
type documentValidator struct {
	dest    *configdata.Data
	origin  string
	scope   string
	keyCase spec.KeyCase
}

// newDocumentValidator builds a validator for one document; root carries the
// app-level key case.
func newDocumentValidator(root *spec.Command, dest *configdata.Data, origin, scope string) *documentValidator {
	if origin == "" {
		scope = ""
	}

	return &documentValidator{dest: dest, origin: origin, scope: scope, keyCase: root.ConfigSettings.KeyCase}
}

// command validates one command's section; prefix is the command's section
// path.
func (v *documentValidator) command(prefix string, rawMap map[string]any, cmd *spec.Command) error {
	for _, key := range slices.Sorted(maps.Keys(rawMap)) {
		if err := v.commandEntry(prefix, key, rawMap[key], cmd); err != nil {
			return err
		}
	}

	return nil
}

func (v *documentValidator) commandEntry(prefix, key string, value any, cmd *spec.Command) error {
	if nestedMap, isMap := value.(map[string]any); isMap {
		return v.nestedSection(prefix, key, nestedMap, cmd)
	}

	if flag := findFlag(cmd, prefix, []string{key}, v.keyCase); flag != nil {
		if flag.ConfigKey == "-" {
			return newExcludedConfigFieldError(v.origin, v.scope, key, cmd.Name)
		}

		return v.store(prefix, cmd, flag, value)
	}

	return newFieldConfigError(
		v.origin,
		v.scope,
		fmt.Sprintf("unknown field %q in command %q%s", key, cmd.Name, suggestionHint(cmd, prefix, nil, key, v.keyCase)),
		fmt.Sprintf("unknown field %q in command %q", key, cmd.Name),
	)
}

func (v *documentValidator) nestedSection(prefix, key string, nestedMap map[string]any, cmd *spec.Command) error {
	if subcommand := findSubcommand(cmd, key, v.keyCase); subcommand != nil {
		return v.command(joinConfigPath(prefix, subcommand.Name), nestedMap, subcommand)
	}

	flag := findFlag(cmd, prefix, []string{key}, v.keyCase)

	if flag != nil && flag.IsMap {
		if flag.ConfigKey == "-" {
			return newExcludedConfigFieldError(v.origin, v.scope, key, cmd.Name)
		}

		return v.store(prefix, cmd, flag, nestedMap)
	}

	section := []string{key}

	if flag != nil && !hasFlagUnder(cmd, prefix, section) {
		if flag.ConfigKey == "-" {
			return newExcludedConfigFieldError(v.origin, v.scope, key, cmd.Name)
		}

		return newFieldConfigError(
			v.origin,
			v.scope,
			fmt.Sprintf("field %q in command %q is not a map", key, cmd.Name),
			fmt.Sprintf("field %q in command %q is not a map", key, cmd.Name),
		)
	}

	if hasFlagUnder(cmd, prefix, section) {
		return v.namespace(prefix, section, nestedMap, cmd)
	}

	return newFieldConfigError(
		v.origin,
		v.scope,
		fmt.Sprintf("unknown section %q in command %q%s", key, cmd.Name, suggestionHint(cmd, prefix, nil, key, v.keyCase)),
		fmt.Sprintf("unknown section %q in command %q", key, cmd.Name),
	)
}

// namespace validates a section that groups dotted config tags (`db` for
// `config:"db.host"`). A key inside it matches only a flag whose tag is the
// section path plus that key, so `[db] port` cannot reach a root `--port`.
func (v *documentValidator) namespace(cmdPrefix string, section []string, rawMap map[string]any, cmd *spec.Command) error {
	displayNamespace := joinConfigPath(cmdPrefix, strings.Join(section, "."))

	for _, key := range slices.Sorted(maps.Keys(rawMap)) {
		value := rawMap[key]
		path := append(slices.Clone(section), key)

		flag := findFlag(cmd, cmdPrefix, path, v.keyCase)

		nestedMap, isMap := value.(map[string]any)

		// A map flag addressed at a dotted config tag consumes the nested
		// section itself, mirroring nestedSection's check order (map flag
		// before namespace recursion); every other nested map keeps
		// recursing into the namespace.
		if flag != nil && flag.IsMap && isMap {
			if err := v.store(cmdPrefix, cmd, flag, nestedMap); err != nil {
				return err
			}

			continue
		}

		if isMap {
			if err := v.namespace(cmdPrefix, path, nestedMap, cmd); err != nil {
				return err
			}

			continue
		}

		if flag != nil {
			if err := v.store(cmdPrefix, cmd, flag, value); err != nil {
				return err
			}

			continue
		}

		return newFieldConfigError(
			v.origin,
			v.scope,
			fmt.Sprintf("unknown field %q in namespace %q%s", key, displayNamespace, suggestionHint(cmd, cmdPrefix, section, key, v.keyCase)),
			fmt.Sprintf("unknown field %q in namespace %q", key, displayNamespace),
		)
	}

	return nil
}

// newExcludedConfigFieldError rejects a configuration entry addressed at a
// flag excluded from config files, the strict-validation counterpart of the
// binder's refusal to read such keys.
func newExcludedConfigFieldError(origin, scope, fieldKey, commandName string) error {
	return newFieldConfigError(
		origin,
		scope,
		fmt.Sprintf("field %q is excluded from config files in command %q", fieldKey, commandName),
		fmt.Sprintf("field %q not allowed in config", fieldKey),
	)
}

// validateCommandConfig validates raw parsed configuration data against the
// Command AST and populates dest with the resolved values. Unknown keys or
// sections trigger strict configError failures.
func validateCommandConfig(rawMap map[string]any, cmd *spec.Command, dest *configdata.Data, origin string) error {
	return newDocumentValidator(cmd, dest, origin, "").command("", rawMap, cmd)
}

// relativeConfigPath returns a flag's config tag, relative to the command's
// section path cmdPrefix, split into its section segments; nil when the flag
// has no tag or is excluded. A tag that already starts with the command path
// names it absolutely, matching the binder's storage keys in flagConfigKeys.
func relativeConfigPath(cmdPrefix string, flag *spec.Flag) []string {
	if flag.ConfigKey == "" || flag.ConfigKey == "-" {
		return nil
	}

	configKey := flag.ConfigKey
	if cmdPrefix != "" {
		configKey = strings.TrimPrefix(configKey, cmdPrefix+".")
	}

	return strings.Split(configKey, ".")
}

// hasFlagUnder reports whether a flag's config tag lies strictly below the
// section path.
func hasFlagUnder(cmd *spec.Command, cmdPrefix string, section []string) bool {
	return slices.ContainsFunc(cmd.Flags, func(flag *spec.Flag) bool {
		path := relativeConfigPath(cmdPrefix, flag)

		return len(path) > len(section) && slices.Equal(path[:len(section)], section)
	})
}

// findFlag resolves a key path within the command (one key at the command
// level, or a section path plus key inside a namespace) to the flag it sets.
// A flag with a config tag matches only that tag; a flag without one matches
// only its name in the key case, at the command level. Excluded flags
// (`config:"-"`) match their name so the caller can report the exclusion.
func findFlag(cmd *spec.Command, cmdPrefix string, path []string, keyCase spec.KeyCase) *spec.Flag {
	for _, flag := range cmd.Flags {
		if flag.IsConfigFile {
			continue
		}

		if configPath := relativeConfigPath(cmdPrefix, flag); configPath != nil {
			if slices.Equal(configPath, path) {
				return flag
			}

			continue
		}

		if len(path) == 1 && path[0] == keyCase.Apply(flag.Name) {
			return flag
		}
	}

	return nil
}

func findSubcommand(cmd *spec.Command, key string, keyCase spec.KeyCase) *spec.Command {
	for _, subcommand := range cmd.Subcommands {
		if keyCase.Apply(subcommand.Name) == key {
			return subcommand
		}
	}

	return nil
}

func joinConfigPath(prefix, key string) string {
	return cond.If(prefix != "", prefix+"."+key, key)
}

// store records one config value for a flag under every bindable config key.
// Scalar and slice values are stringified and replace whatever an earlier
// file stored; map flags contribute structured per-key entries so cascading
// files merge key by key at load time. cmd is the command declaring flag.
func (v *documentValidator) store(prefix string, cmd *spec.Command, flag *spec.Flag, value any) error {
	storageKeys := flagConfigKeys(prefix, flag)

	if flag.IsMap {
		entries := v.dest.EnsureMap(storageKeys)

		return v.storeMapEntries(cmd, flag, value, entries)
	}

	formatted := formatConfigValue(value)
	for _, storageKey := range storageKeys {
		v.dest.SetScalarScoped(storageKey, formatted, v.origin, v.scope)
	}

	return nil
}

// storeMapEntries records a map flag's entries from a nested map, a list of
// key=value tokens, or one separated string.
func (v *documentValidator) storeMapEntries(cmd *spec.Command, flag *spec.Flag, value any, entries map[string]configdata.Value) error {
	if nestedMap, isMap := value.(map[string]any); isMap {
		for _, key := range slices.Sorted(maps.Keys(nestedMap)) {
			if err := v.storeMapEntry(cmd, flag, key, stringx.Stringify(nestedMap[key]), entries); err != nil {
				return err
			}
		}

		return nil
	}

	if stringMap, isStringMap := value.(map[string]string); isStringMap {
		for _, key := range slices.Sorted(maps.Keys(stringMap)) {
			if err := v.storeMapEntry(cmd, flag, key, stringMap[key], entries); err != nil {
				return err
			}
		}

		return nil
	}

	var tokens []string

	switch typedValue := value.(type) {
	case []any:
		tokens = slicex.Map(typedValue, stringx.Stringify)
	case []string:
		tokens = typedValue
	default:
		tokens = strings.Split(formatConfigValue(value), cmp.Or(flag.Sep, ","))
	}

	for _, token := range tokens {
		if err := v.storeFlatMapToken(cmd, flag, token, entries); err != nil {
			return err
		}
	}

	return nil
}

// storeFlatMapToken parses one flat key=value token into entries, recording
// tokens without a separator or key as malformed pairs so binding surfaces
// them with config attribution. Each token is one pair: a value containing
// the flag's sep is never re-split.
func (v *documentValidator) storeFlatMapToken(cmd *spec.Command, flag *spec.Flag, token string, entries map[string]configdata.Value) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	key, value, ok := strings.Cut(token, "=")

	key = strings.TrimSpace(key)
	if !ok || key == "" {
		entries[""] = configdata.Value{Raw: token, Origin: v.origin, Scope: v.scope}

		return nil
	}

	return v.storeMapEntry(cmd, flag, key, strings.TrimSpace(value), entries)
}

// storeMapEntry records one map entry. In the features section a key must be
// a declared feature name spelled in the app's key case; another spelling of
// a declared feature is an unknown field naming the expected one, and the
// entry is stored under the canonical feature name the binder reads. Names
// that match no feature pass through for the binder to reject.
func (v *documentValidator) storeMapEntry(cmd *spec.Command, flag *spec.Flag, key, raw string, entries map[string]configdata.Value) error {
	if flag == cmd.FeatureFlag {
		if feature := cmd.FindFeature(casing.Kebab(key)); feature != nil {
			if expected := v.keyCase.Apply(feature.Name); key != expected {
				section := spec.ConfigKeyPath(flag, []*spec.Command{cmd})

				return newFieldConfigError(
					v.origin,
					v.scope,
					fmt.Sprintf("unknown field %q in section %q (did you mean %q?)", key, section, expected),
					fmt.Sprintf("unknown field %q in section %q", key, section),
				)
			}

			key = feature.Name
		}
	}

	entries[key] = configdata.Value{Raw: raw, Origin: v.origin, Scope: v.scope}

	return nil
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

// findClosestSuggestion offers the closest valid key for an unknown one.
// Inside a namespace (section non-empty) the candidates are the next segment
// of each config tag under it; at the command level they are each flag's
// first key segment (its name, or the first segment of its tag) and the
// subcommand names.
func findClosestSuggestion(cmd *spec.Command, cmdPrefix string, section []string, key string, keyCase spec.KeyCase) string {
	var candidates []string

	for _, flag := range cmd.Flags {
		if flag.ConfigKey == "-" || flag.IsConfigFile {
			continue
		}

		path := relativeConfigPath(cmdPrefix, flag)
		if path == nil {
			path = []string{keyCase.Apply(flag.Name)}
		}

		if len(path) > len(section) && slices.Equal(path[:len(section)], section) {
			candidates = append(candidates, path[len(section)])
		}
	}

	if len(section) == 0 {
		for _, subcommand := range cmd.Subcommands {
			candidates = append(candidates, keyCase.Apply(subcommand.Name))
		}
	}

	return levenshtein.Suggest(key, candidates, 4)
}

func suggestionHint(cmd *spec.Command, cmdPrefix string, section []string, key string, keyCase spec.KeyCase) string {
	suggestion := findClosestSuggestion(cmd, cmdPrefix, section, key, keyCase)
	if suggestion == "" {
		return ""
	}

	return fmt.Sprintf(" (did you mean %q?)", suggestion)
}
