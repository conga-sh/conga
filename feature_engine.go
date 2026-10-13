package conga

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/tokenizer"
)

// prunedFlag is a flag removed from the schema by feature pruning, carrying the
// owning command's dotted path (empty for root flags) and the feature expression
// whose inactivity removed it.
type prunedFlag struct {
	flag        *spec.Flag
	prefix      string
	featureExpr string
}

// configKeys lists the configuration document keys that could supply the pruned
// flag, mirroring lookupConfig's read tiers: the direct flag name, an explicit
// config key (bare and namespaced under the owning command path), and the
// command-scoped canonical path.
func (p prunedFlag) configKeys() []string {
	if p.flag.ConfigKey == "-" {
		return nil
	}

	var keys []string

	if p.prefix == "" {
		// The bare flag name is a shared config tier (lookupConfig tier 4):
		// every flag of that name reads it regardless of owning command. Only a
		// root-owned pruned flag may claim it, otherwise a pruned subcommand
		// flag would reject a top-level entry a sibling's same-named flag
		// legitimately consumes.
		keys = append(keys, p.flag.Name)
	}

	if p.flag.ConfigKey != "" {
		keys = append(keys, p.flag.ConfigKey)

		if p.prefix != "" && !strings.HasPrefix(p.flag.ConfigKey, p.prefix+".") {
			keys = append(keys, p.prefix+"."+p.flag.ConfigKey)
		}
	}

	if p.prefix != "" {
		keys = append(keys, p.prefix+"."+p.flag.Name)

		if lastDot := strings.LastIndex(p.prefix, "."); lastDot != -1 {
			leafName := p.prefix[lastDot+1:]
			keys = append(keys, leafName+"."+p.flag.Name)
		}
	}

	return keys
}

// featureDefaults seeds the canonical feature value map from the declared field
// defaults, falling back to the zero value of each feature's kind.
func featureDefaults(cmd *spec.Command) map[string]string {
	values := make(map[string]string, len(cmd.FeatureFields))

	for _, field := range cmd.FeatureFields {
		switch {
		case field.Default != "":
			values[field.Name] = field.Default
		case field.IsInt:
			values[field.Name] = "0"
		default:
			values[field.Name] = "false"
		}
	}

	return values
}

// featureGates derives the boolean gate map consulted by feature expressions
// from the canonical value map: a boolean feature is on when its canonical
// value is "true", an integer feature when its canonical level is non-zero.
func featureGates(cmd *spec.Command, values map[string]string) map[string]bool {
	gates := make(map[string]bool, len(cmd.FeatureFields))

	for _, feature := range cmd.FeatureFields {
		value := values[feature.Name]

		if feature.IsInt {
			gates[feature.Name] = value != "0"

			continue
		}

		gates[feature.Name] = value == "true"
	}

	return gates
}

// featureValues overlays the source-provided feature map on the field defaults,
// recording the canonical values and the accepted keys, and returns the first
// diagnostic in sorted key order.
func featureValues(cmd *spec.Command, cmdChain []*spec.Command, provided map[string]string, source ValueSource) (map[string]string, map[string]string, error) {
	var firstErr error

	values, accepted := collectFeatureValues(cmd, provided, func(name string, raw string, feature *spec.FeatureField, cause error) {
		if firstErr != nil {
			return
		}

		if feature == nil {
			firstErr = unknownFeatureError(cmdChain, cmd, name, source)

			return
		}

		firstErr = invalidFeatureValueError(cmdChain, feature, raw, cause, source)
	})

	return values, accepted, firstErr
}

// featureValuesLenient overlays the provided feature map on the field defaults
// without building diagnostics: unknown or invalid keys fall back to their
// declared defaults. It backs tolerant paths (--why, completion binding),
// where a rejected key must render its defaulted value rather than an error.
func featureValuesLenient(cmd *spec.Command, provided map[string]string) (map[string]string, map[string]string) {
	return collectFeatureValues(cmd, provided, nil)
}

// collectFeatureValues is the shared overlay loop: known keys are canonicalized
// and recorded, and unknown or invalid keys invoke reject when it is non-nil.
// Keys are processed in sorted order for deterministic diagnostics.
func collectFeatureValues(cmd *spec.Command, provided map[string]string, reject func(name string, raw string, feature *spec.FeatureField, cause error)) (map[string]string, map[string]string) {
	values := featureDefaults(cmd)
	accepted := make(map[string]string, len(provided))

	names := slices.Sorted(maps.Keys(provided))

	for _, name := range names {
		raw := provided[name]

		feature := cmd.FindFeature(casing.Kebab(name))
		if feature == nil {
			if reject != nil {
				reject(name, raw, nil, nil)
			}

			continue
		}

		canonical, err := feature.Canonicalize(raw)
		if err != nil {
			if reject != nil {
				reject(name, raw, feature, err)
			}

			continue
		}

		values[feature.Name] = canonical
		accepted[feature.Name] = canonical
	}

	return values, accepted
}

// bindFeatureValues binds the synthesized feature map flag, overlays the
// provided feature map onto the defaults, and materializes the typed feature
// struct.
func bindFeatureValues(binding flagState, target any, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) (map[string]bool, error) {
	provided, err := bindFeatureMap(binding, cmd, cmdChain, flagValues, state)
	if err != nil && !state.TolerateFeatureErrors {
		return nil, err
	}

	values, _, err := featureValues(cmd, cmdChain, provided, binding.Source())
	if err != nil && !state.TolerateFeatureErrors {
		return nil, err
	}

	materializeFeatureValues(target, cmd.FeatureFields, values)

	return featureGates(cmd, values), nil
}

// bindFeatureMap applies the synthesized feature map flag from the tier
// cascade and returns the raw provided map, or nil when the flag stays
// unbound.
func bindFeatureMap(binding flagState, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) (map[string]string, error) {
	if cmd.FeatureFlag != nil {
		if err := applyFlagValue(cmd, cmdChain, cmd.FeatureFlag, flagValues, state); err != nil {
			return nil, err
		}
	}

	var provided map[string]string
	if rawProvided, ok := binding.getValue().(map[string]string); ok {
		provided = rawProvided
	}

	return provided, nil
}

// bindFeatureValuesLenient is the tolerant binding path used by completion
// pruning: a malformed feature map leaves the flag unbound, and resolution
// proceeds from the declared defaults without materializing diagnostics.
func bindFeatureValuesLenient(binding flagState, target any, cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) map[string]bool {
	provided, err := bindFeatureMap(binding, cmd, []*spec.Command{cmd}, flagValues, state)

	values := featureDefaults(cmd)
	if err == nil {
		values, _ = featureValuesLenient(cmd, provided)
	}

	materializeFeatureValues(target, cmd.FeatureFields, values)

	return featureGates(cmd, values)
}

// resolveFeatureGates binds the feature map flag, materializes the typed
// feature struct, enforces subcommand gates, prunes inactive flags and
// subcommands, and enforces CLI flag restrictions.
func resolveFeatureGates(binding flagState, target any, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) ([]prunedFlag, error) {
	gates, err := bindFeatureValues(binding, target, cmd, cmdChain, flagValues, state)
	if err != nil {
		return nil, err
	}

	if err := validateSubcommandFeatures(cmdChain, gates); err != nil {
		return nil, err
	}

	pruned := pruneInactiveFeatures(cmd, "", gates)

	if err := checkFeatureFlagCLIRestrictions(flagValues, gates, cmdChain); err != nil {
		return nil, err
	}

	return pruned, nil
}

// pruneFeaturesForCompletion binds the feature map flag for a completion
// invocation and prunes inactive features so suggestions never leak gated
// commands or flags. Binding is best-effort: a malformed feature map degrades
// to the declared defaults instead of aborting the completion.
func pruneFeaturesForCompletion(binding flagState, target any, cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) {
	gates := bindFeatureValuesLenient(binding, target, cmd, flagValues, state)

	pruneInactiveFeatures(cmd, "", gates)
}

func unknownFeatureError(cmdChain []*spec.Command, cmd *spec.Command, name string, source ValueSource) error {
	var hint string

	if suggestion := levenshtein.Suggest(casing.Kebab(name), cmd.FeatureNames(), 4); suggestion != "" {
		hint = fmt.Sprintf(" (did you mean %q?)", suggestion)
	}

	if source == SourceConfig {
		return fmt.Errorf("%w: unknown feature %q%s", ErrConfig, name, hint)
	}

	return &parseError{
		msg:      fmt.Sprintf("unknown feature %q%s", name, hint),
		cmdChain: cmdChain,
		cause:    ErrParse,
	}
}

func invalidFeatureValueError(cmdChain []*spec.Command, feature *spec.FeatureField, raw string, cause error, source ValueSource) error {
	message := fmt.Sprintf("invalid value %q for feature %q: %v", raw, feature.Name, cause)

	if source == SourceConfig {
		return fmt.Errorf("%w: %s", ErrConfig, message)
	}

	return &validationError{
		msg:         message,
		cmdChain:    cmdChain,
		cause:       errInvalidValue,
		valueShaped: true,
	}
}

// materializeFeatureValues reflects the resolved canonical value map into the
// inner feature struct so application code reads it through Features.Get.
// Boolean fields take their canonical "true"/"false" value; integer fields take
// their canonical decimal level, whose parse is guaranteed by bind-time
// canonicalization, so a failure here is an engine invariant.
func materializeFeatureValues(target any, fields []*spec.FeatureField, values map[string]string) {
	inner := reflect.ValueOf(target).Elem()

	for _, feature := range fields {
		value, ok := values[feature.Name]
		if !ok {
			continue
		}

		field := inner.Field(feature.FieldIndex)

		if !feature.IsInt {
			field.SetBool(value == "true")

			continue
		}

		if feature.IsUnsigned {
			parsed, err := strconv.ParseUint(value, 10, field.Type().Bits())
			if err != nil {
				panic(fmt.Sprintf("conga: invariant violation: feature %q value %q does not fit %s", feature.Name, value, field.Type()))
			}

			field.SetUint(parsed)

			continue
		}

		parsed, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			panic(fmt.Sprintf("conga: invariant violation: feature %q value %q does not fit %s", feature.Name, value, field.Type()))
		}

		field.SetInt(parsed)
	}
}

// joinSubcommandPrefix extends a dotted command prefix with a subcommand name,
// returning the bare name for root-level commands.
func joinSubcommandPrefix(prefix string, name string) string {
	if prefix == "" {
		return name
	}

	return prefix + "." + name
}

func pruneInactiveFeatures(cmd *spec.Command, prefix string, activeFeatures map[string]bool) []prunedFlag {
	var pruned []prunedFlag

	if cmd.ConfigFileFlag != nil && cmd.ConfigFileFlag.Feature != "" && !spec.MatchFeature(cmd.ConfigFileFlag.Feature, activeFeatures) {
		// The flag stays in cmd.Flags, so the filter below records the pruned
		// entry; clearing the pointer only detaches the dedicated accessor.
		cmd.ConfigFileFlag = nil
	}

	cmd.Flags = slicex.Filter(cmd.Flags, func(flagSpec *spec.Flag) bool {
		if flagSpec.Feature != "" && !spec.MatchFeature(flagSpec.Feature, activeFeatures) {
			pruned = append(pruned, prunedFlag{flag: flagSpec, prefix: prefix, featureExpr: flagSpec.Feature})

			return false
		}

		return true
	})

	cmd.Subcommands = slicex.Filter(cmd.Subcommands, func(subcommand *spec.Command) bool {
		subcommandPrefix := joinSubcommandPrefix(prefix, subcommand.Name)

		if subcommand.Feature != "" && !spec.MatchFeature(subcommand.Feature, activeFeatures) {
			pruned = append(pruned, collectPrunedSubtree(subcommand, subcommandPrefix, subcommand.Feature)...)

			return false
		}

		pruned = append(pruned, pruneInactiveFeatures(subcommand, subcommandPrefix, activeFeatures)...)

		return true
	})

	return pruned
}

// collectPrunedSubtree records every flag of a subcommand whose own gate is
// inactive, so configuration values targeting the pruned subtree are rejected
// rather than silently ignored. Every flag is attributed to the subtree's
// gating expression: that gate is off, which is why nothing in the subtree can
// be reached.
func collectPrunedSubtree(cmd *spec.Command, prefix string, featureExpr string) []prunedFlag {
	pruned := make([]prunedFlag, 0, len(cmd.Flags))

	for _, flagSpec := range cmd.Flags {
		pruned = append(pruned, prunedFlag{flag: flagSpec, prefix: prefix, featureExpr: featureExpr})
	}

	for _, subcommand := range cmd.Subcommands {
		subcommandPrefix := joinSubcommandPrefix(prefix, subcommand.Name)

		pruned = append(pruned, collectPrunedSubtree(subcommand, subcommandPrefix, featureExpr)...)
	}

	return pruned
}

func validateSubcommandFeatures(cmdChain []*spec.Command, activeFeatures map[string]bool) error {
	for _, subcommand := range cmdChain[1:] {
		if subcommand.Feature != "" && !spec.MatchFeature(subcommand.Feature, activeFeatures) {
			return &parseError{
				msg:      fmt.Sprintf("command %q requires feature %q", subcommand.Name, subcommand.Feature),
				cmdChain: cmdChain,
				cause:    ErrParse,
			}
		}
	}

	return nil
}

// checkFeatureFlagCLIRestrictions rejects command-line invocations of a flag
// whose gating feature expression is inactive. It runs before builtin rendering
// so --help on a gated flag fails symmetrically with a gated subcommand.
func checkFeatureFlagCLIRestrictions(flagValues map[*spec.Flag][]string, activeFeatures map[string]bool, cmdChain []*spec.Command) error {
	for flagSpec := range flagValues {
		if flagSpec.Feature != "" && !spec.MatchFeature(flagSpec.Feature, activeFeatures) {
			return &parseError{
				msg:      fmt.Sprintf("flag '--%s' requires feature %q", flagSpec.Name, flagSpec.Feature),
				cmdChain: cmdChain,
				cause:    ErrParse,
			}
		}
	}

	return nil
}

// checkFeatureConfigRestrictions rejects configuration entries that target a
// flag pruned by feature gating, so an inactive flag's value is never silently
// ignored. It runs only on real executions (never on a tolerated builtin
// request whose configuration may legitimately be unresolved).
func checkFeatureConfigRestrictions(pruned []prunedFlag, configData map[string]string) error {
	for _, prunedMember := range pruned {
		for _, key := range prunedMember.configKeys() {
			if _, exists := configData[key]; exists {
				return fmt.Errorf("%w: config file declares flag '--%s' which requires feature %q", ErrConfig, prunedMember.flag.Name, prunedMember.featureExpr)
			}
		}
	}

	return nil
}

// cliFlagValues extracts the command-line values supplied for the given target
// flags from completion words, so completion feature pruning observes the CLI
// tier of the feature cascade and an explicit --config-file supplied on the
// completion line. NoCLI targets (such as an unnamed synthesized feature flag)
// can never appear on the completion line and are skipped. Extraction is
// deliberately tolerant: unknown tokens, dangling flags, and partial values are
// ignored rather than failing the invocation.
func cliFlagValues(cmd *spec.Command, args []string, targets ...*spec.Flag) map[*spec.Flag][]string {
	wanted := make(map[*spec.Flag]bool, len(targets))
	for _, target := range targets {
		if target != nil && !target.NoCLI {
			wanted[target] = true
		}
	}

	if len(wanted) == 0 {
		return nil
	}

	index := newFlagIndex(cmd)
	tokens := tokenizer.Tokenize(args)
	values := make(map[*spec.Flag][]string)

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		if tok.Type == tokenizer.TokenTypeTerminator {
			break
		}

		if found := index.find(tok); found != nil && wanted[found] {
			if tok.HasAttachedValue {
				values[found] = append(values[found], tok.Value)

				continue
			}

			if i+1 < len(tokens) {
				values[found] = append(values[found], tokens[i+1].Raw)
				i++
			}

			continue
		}

		if skipped, next := skipFlagValue(tokens, i, index); skipped {
			i = next
		}
	}

	if len(values) == 0 {
		return nil
	}

	return values
}
