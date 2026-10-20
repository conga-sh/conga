package conga

import (
	"cmp"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
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
// flag, mirroring lookupConfig's read tiers. Only command-scoped keys are
// claimed for subcommand-owned flags: the bare flag name and the bare explicit
// config key are shared fallback tiers that any flag, root or subcommand, may
// read, so a pruned subcommand flag must not claim them or a valid top-level
// entry an active flag legitimately consumes would be rejected.
//
//	root-owned:  Name, ConfigKey
//	subcommand:  prefix.ConfigKey (or ConfigKey when already qualified),
//	             prefix.Name, leafName.Name
func (p prunedFlag) configKeys() []string {
	if p.flag.ConfigKey == "-" {
		return nil
	}

	if p.prefix == "" {
		keys := []string{p.flag.Name}

		if p.flag.ConfigKey != "" {
			keys = append(keys, p.flag.ConfigKey)
		}

		return keys
	}

	var keys []string

	if p.flag.ConfigKey != "" {
		if strings.HasPrefix(p.flag.ConfigKey, p.prefix+".") {
			keys = append(keys, p.flag.ConfigKey)
		} else {
			keys = append(keys, p.prefix+"."+p.flag.ConfigKey)
		}
	}

	keys = append(keys, p.prefix+"."+p.flag.Name)

	if lastDot := strings.LastIndex(p.prefix, "."); lastDot != -1 {
		leafName := p.prefix[lastDot+1:]
		keys = append(keys, leafName+"."+p.flag.Name)
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
// recording the canonical values, and returns the first diagnostic in sorted
// key order. Diagnostics attribute each key to the tier that supplied it via
// provenance, falling back to the container's recorded source when a key has no
// recorded tier.
func featureValues(cmd *spec.Command, cmdChain []*spec.Command, provided map[string]string, provenance map[string]ValueSource, envNames map[string]string, fallback ValueSource) (map[string]string, error) {
	var firstErr error

	values := collectFeatureValues(cmd, provided, func(name string, raw string, feature *spec.FeatureField, cause error) {
		if firstErr != nil {
			return
		}

		source := fallback
		if provenanceSource, ok := provenance[name]; ok {
			source = provenanceSource
		}

		if feature == nil {
			firstErr = unknownFeatureError(cmdChain, cmd, name, source)

			return
		}

		firstErr = invalidFeatureValueError(cmdChain, feature, raw, cause, source, envNames[name])
	})

	return values, firstErr
}

// featureValuesLenient overlays the provided feature map on the field defaults
// without building diagnostics: unknown or invalid keys fall back to their
// declared defaults. It backs completion binding, where a rejected key must
// resolve its defaulted value rather than aborting the completion.
func featureValuesLenient(cmd *spec.Command, provided map[string]string) map[string]string {
	return collectFeatureValues(cmd, provided, nil)
}

// collectFeatureValues is the shared overlay loop: known keys are canonicalized
// and recorded, and unknown or invalid keys invoke reject when it is non-nil.
// Keys are processed in sorted order for deterministic diagnostics.
func collectFeatureValues(cmd *spec.Command, provided map[string]string, reject func(name string, raw string, feature *spec.FeatureField, cause error)) map[string]string {
	values := featureDefaults(cmd)

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
	}

	return values
}

// bindFeatureValues binds the synthesized feature map flag, overlays the
// provided feature map onto the defaults, and materializes the typed feature
// struct.
func bindFeatureValues(binding featureState, target any, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) (map[string]bool, error) {
	provided, envNames, err := bindFeatureMap(cmd, cmdChain, flagValues, state)
	if err != nil && !state.TolerateFeatureErrors {
		return nil, err
	}

	sources, _ := binding.mapProvenance()

	values, err := featureValues(cmd, cmdChain, provided, sources, envNames, binding.Source())
	if err != nil && !state.TolerateFeatureErrors {
		return nil, err
	}

	binding.setResolvedValues(values)
	materializeFeatureValues(target, cmd.FeatureFields, values)

	return featureGates(cmd, values), nil
}

// bindFeatureMap applies the synthesized feature map flag from the merged
// tier cascade, overlays per-feature environment variables, and returns the
// resolved provided map (canonical feature-name keys), the key-to-variable
// attribution map backing error messages for combined-map and per-feature env
// keys, and a nil error when the flag binds.
func bindFeatureMap(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) (map[string]string, map[string]string, error) {
	if cmd.FeatureFlag == nil {
		return make(map[string]string), nil, nil
	}

	var result mapMergeResult

	result.wantCLIKeys = state == nil || !state.WantMapProvenance

	if err := foldMapTiers(cmd, cmdChain, cmd.FeatureFlag, flagValues, state, &result); err != nil {
		return nil, nil, err
	}

	var provided map[string]string
	if rawProvided, ok := result.merged.(map[string]string); ok {
		provided = rawProvided
	}

	if provided == nil {
		provided = make(map[string]string)
	}

	envNames := overlayFeatureFieldEnv(cmd, state, provided, &result)

	// Feature binding always records per-key provenance, so env-sourced keys
	// can be attributed to the combined-map variable that supplied them.
	for name, source := range result.provenance {
		if source != SourceEnv || result.envName == "" {
			continue
		}

		if _, ok := envNames[name]; ok {
			continue
		}

		if envNames == nil {
			envNames = make(map[string]string, len(result.provenance))
		}

		envNames[name] = result.envName
	}

	if target := featureStateOfCommand(cmd); target != nil {
		if result.hasValues {
			target.setValue(result.merged, result.highestTier, result.sourceDetail())
		}

		target.setMapProvenance(result.provenance, result.origins)
	}

	return provided, envNames, nil
}

// overlayFeatureFieldEnv resolves each declared feature field's environment
// variables onto the merged feature map. Per-feature variables sit between
// the CLI tier and the configuration tier of the resolution ladder: a key
// supplied on the command line wins, an env-supplied key overrides config
// and defaults, and keys the environment does not name keep their
// lower-tier values. Raw values are recorded as-is; collectFeatureValues
// canonicalizes every key uniformly, so invalid env values surface through
// the shared diagnostics. Each contributed key records SourceEnv provenance
// with a zeroed origin, keeping the --why report identical to combined-map
// env keys, and envNames maps each contributed key to its winning variable
// for error attribution.
func overlayFeatureFieldEnv(cmd *spec.Command, state *bindingState, provided map[string]string, result *mapMergeResult) map[string]string {
	var (
		envNames        map[string]string
		contributedVars []string
	)

	lookup := envLookupFunc(state)

	// foldMapTiers allocates the provenance maps only when a tier contributed;
	// a feature map supplied solely by per-feature variables needs them too.
	if state != nil && state.WantMapProvenance && result.provenance == nil {
		result.provenance = make(map[string]ValueSource)
		result.origins = make(map[string]configdata.Value)
	}

	for _, feature := range cmd.FeatureFields {
		rawValue, envName, found := resolveFeatureEnvValue(feature, result, lookup)
		if !found {
			continue
		}

		provided[feature.Name] = rawValue

		if result.provenance != nil {
			result.provenance[feature.Name] = SourceEnv
		}

		if result.origins != nil {
			result.origins[feature.Name] = configdata.Value{}
		}

		if envNames == nil {
			envNames = make(map[string]string)
		}

		envNames[feature.Name] = envName
		contributedVars = append(contributedVars, envName)
	}

	recordFeatureEnvContribution(contributedVars, provided, result)

	return envNames
}

// resolveFeatureEnvValue looks up one feature field's environment variables in
// declaration order: the first set variable wins, an empty value is unset, and
// a key the command line already supplied keeps its CLI value. CLI presence
// reads full provenance when it was recorded, else the lightweight key set
// collected for callers that do not request provenance.
func resolveFeatureEnvValue(feature *spec.FeatureField, result *mapMergeResult, lookup func(string) (string, bool)) (string, string, bool) {
	if feature.EnvExcluded || len(feature.EnvVars) == 0 {
		return "", "", false
	}

	if result.provenance[feature.Name] == SourceCLI {
		return "", "", false
	}

	if _, ok := result.cliKeys[feature.Name]; ok {
		return "", "", false
	}

	for _, envVar := range feature.EnvVars {
		if value, ok := lookup(envVar); ok && value != "" {
			return value, envVar, true
		}
	}

	return "", "", false
}

// recordFeatureEnvContribution publishes a completed per-feature env overlay
// into the merge result: contributed keys mark the map as env-sourced, the
// highest tier moves to the environment unless the command line supplied a
// key, and an env-only map becomes the merged value.
func recordFeatureEnvContribution(contributedVars []string, provided map[string]string, result *mapMergeResult) {
	if len(contributedVars) == 0 {
		return
	}

	result.hasValues = true

	if result.highestTier != SourceCLI {
		result.highestTier = SourceEnv
		result.highestName = strings.Join(contributedVars, ", ")
		result.highestOrigins = nil
	}

	if result.merged == nil {
		result.merged = provided
	}
}

// completionFeatureGates resolves the feature gate map for a completion
// invocation from the tier cascade without mutating the application's feature
// container: completion is an observation, and App permits a real execution
// after a completion invocation, so binding state here would leak into that
// run. A malformed feature map degrades to the declared defaults rather than
// aborting the completion.
func completionFeatureGates(cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) map[string]bool {
	values := featureValuesLenient(cmd, completionProvidedMap(cmd, flagValues, state))

	return featureGates(cmd, values)
}

// completionProvidedMap resolves the synthesized feature flag's raw provided
// map from the merged tier cascade plus the per-feature environment overlay,
// without coercing into the container.
func completionProvidedMap(cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) map[string]string {
	if cmd.FeatureFlag == nil {
		return nil
	}

	var result mapMergeResult

	result.wantCLIKeys = state == nil || !state.WantMapProvenance

	if err := foldMapTiers(cmd, []*spec.Command{cmd}, cmd.FeatureFlag, flagValues, state, &result); err != nil {
		return nil
	}

	provided, ok := result.merged.(map[string]string)
	if !ok {
		provided = make(map[string]string)
	}

	overlayFeatureFieldEnv(cmd, state, provided, &result)

	return provided
}

// pruneFeaturesForCompletion prunes inactive features for a completion
// invocation so suggestions never leak gated commands or flags.
func pruneFeaturesForCompletion(cmd *spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) {
	pruneInactiveFeatures(cmd, "", completionFeatureGates(cmd, flagValues, state))
}

// resolveFeatureGates binds the feature map flag, materializes the typed
// feature struct, enforces subcommand gates, prunes inactive flags and
// subcommands, and enforces CLI flag restrictions.
func resolveFeatureGates(binding featureState, target any, cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string, state *bindingState) ([]prunedFlag, error) {
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

func invalidFeatureValueError(cmdChain []*spec.Command, feature *spec.FeatureField, raw string, cause error, source ValueSource, envName string) error {
	location := ""
	if source == SourceEnv && envName != "" {
		location = fmt.Sprintf(" (from environment variable %s)", envName)
	}

	message := fmt.Sprintf("invalid value %q for feature %q%s: %v", raw, feature.Name, location, cause)

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
// Targets are inspected in flag-name order so the first diagnostic is
// deterministic when several gated flags are supplied.
func checkFeatureFlagCLIRestrictions(flagValues map[*spec.Flag][]string, activeFeatures map[string]bool, cmdChain []*spec.Command) error {
	targets := slices.SortedFunc(maps.Keys(flagValues), func(first *spec.Flag, second *spec.Flag) int {
		return cmp.Compare(first.Name, second.Name)
	})

	for _, flagSpec := range targets {
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
func checkFeatureConfigRestrictions(pruned []prunedFlag, configData *configdata.Data) error {
	for _, prunedMember := range pruned {
		if slices.ContainsFunc(prunedMember.configKeys(), configData.HasKey) {
			return fmt.Errorf("%w: config file declares flag '--%s' which requires feature %q", ErrConfig, prunedMember.flag.Name, prunedMember.featureExpr)
		}
	}

	return nil
}
