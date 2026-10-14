package conga

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
	"github.com/conga-sh/conga/internal/why"
)

// configPathError is implemented by configuration errors carrying the failing
// path, which the --why report surfaces on the offending candidate row.
type configPathError interface {
	error
	ConfigPath() string
}

func (a *App[T]) renderWhy(cmdChain []*spec.Command, cliValues map[*spec.Flag][]string) {
	report := a.buildWhyReport(cmdChain, cliValues)

	fmt.Fprintln(a.stdout, report.Format())
}

// buildWhyReport replays the flag-tier cascade (CLI -> env -> config ->
// default) for every flag and config-file container on the command chain,
// capturing per-field outcomes as report rows. The active command's flags
// form the main table; inherited ancestor flags render under Global Flags.
func (a *App[T]) buildWhyReport(cmdChain []*spec.Command, cliValues map[*spec.Flag][]string) why.Report {
	if cliValues == nil {
		cliValues = make(map[*spec.Flag][]string)
	}

	// Tolerant config loading: resolution and read failures surface as a
	// diagnostic row on the config-file container instead of aborting. The
	// resolution is memoized on the App, so a feature-capable run that already
	// resolved configuration for gating does not probe and decode a second time.
	resolution := a.resolveConfig(cmdChain[0], cmdChain, cliValues)

	loadedPaths := resolution.loadedPaths
	loadedSource := resolution.loadedSource
	loadErr := resolution.err

	resolver := a.configurationView(cmdChain[0])
	candidatePaths := resolution.candidates

	state := &bindingState{
		EnvPrefix:   a.envPrefix,
		ConfigData:  a.configData,
		TypeParsers: a.collectTypeParsers(),
		Stdin:       a.stdin,
		Stdout:      a.stdout,
	}

	leaf := cmdChain[len(cmdChain)-1]
	root := cmdChain[0]

	report := why.Report{CommandPath: commandChainPath(cmdChain)}

	// Resolve the feature-map fold once and share it between the feature flag
	// row and the Features table, so the config/env/CLI cascade is parsed a
	// single time per report.
	featureResult, featureErr := foldMapTiers(root, cmdChain, root.FeatureFlag, cliValues, state, true)

	for cmdIndex, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Handle == nil {
				// Built-in flags carry no provenance.
				continue
			}

			// A synthesized feature map flag with NoCLI (no flag tag) is engine-owned
			// and not a CLI flag. It is excluded from the flag table; its toggles
			// are displayed in the dedicated Features section.
			if flagSpec == root.FeatureFlag && flagSpec.NoCLI {
				continue
			}

			var row why.Row

			switch {
			case flagSpec.IsConfigFile:
				row = a.whyConfigFileRow(cmd, cmdChain, flagSpec, cliValues, state, loadedPaths, loadedSource, loadErr)
				row.ConfigPath = "-"
			case flagSpec == root.FeatureFlag:
				row = whyMapFoldRow(flagSpec, featureResult, featureErr, why.Row{Label: "--" + flagSpec.Name, Hidden: flagSpec.Hidden}, false)
				row.ConfigPath = whyConfigPath(flagSpec, cmdChain, cmdIndex)
			default:
				row = a.whyFlagRow(cmd, cmdChain, flagSpec, cliValues, state)
				row.ConfigPath = whyConfigPath(flagSpec, cmdChain, cmdIndex)
			}

			row.Env = whyEnvColumn(flagSpec, cmd, state)

			if cmd == leaf {
				report.Rows = append(report.Rows, row)
			} else {
				report.GlobalRows = append(report.GlobalRows, row)
			}
		}
	}

	report.Redacted = whyReportRedacted(report)
	report.Features = whyFeatureRows(root, featureResult)
	extensions := a.configRegistry.Extensions()
	report.ConfigDecoders = extensions
	report.ConfigPaths = whyConfigPaths(resolver, candidatePaths, loadedPaths, loadErr, extensions)

	if len(loadedPaths) == 0 {
		report.ConfigExamples = resolver.ConfigExamples(candidatePaths, slicex.Map(resolver.EffectiveSearchPaths(), pathx.Expand), extensions)
	}

	return report
}

// whyFlagRow resolves one ordinary flag through the tier cascade, capturing
// the outcome as a report row.
func (a *App[T]) whyFlagRow(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState) why.Row {
	row := why.Row{
		Label:  "--" + flagSpec.Name,
		Hidden: flagSpec.Hidden,
	}

	// Map flags merge every tier key-by-key, so their row replays the merged
	// cascade and records one provenance entry per key.
	if flagSpec.IsMap {
		return a.whyMapRow(cmd, cmdChain, flagSpec, cliValues, state, row)
	}

	// Tier 1: CLI Flags
	if values, ok := cliValues[flagSpec]; ok && len(values) > 0 {
		return a.whyCoercedRow(cmdChain, flagSpec, values, SourceCLI, "--"+flagSpec.Name, state, row, !flagSpec.Safe)
	}

	// Tier 2: Environment Variables
	if envValue, envName, ok := lookupEnv(flagSpec, cmd, state); ok {
		return a.whyCoercedRow(cmdChain, flagSpec, []string{envValue}, SourceEnv, envName, state, row, !flagSpec.Safe)
	}

	// Tier 3: Configuration Files
	if configValue, configKey, ok := lookupConfig(flagSpec, cmd, cmdChain, state); ok {
		return a.whyCoercedRow(cmdChain, flagSpec, []string{configValue}, SourceConfig, configKey, state, row, !flagSpec.Safe)
	}

	// Tier 4: Default Values: defaults are always displayed, matching --help.
	if flagSpec.Default != "" {
		return a.whyCoercedRow(cmdChain, flagSpec, []string{flagSpec.Default}, SourceDefault, flagSpec.Default, state, row, false)
	}

	row.Source = whySourceWord(SourceNone)

	return row
}

// whyMapRow replays the per-key merge cascade for a map flag, mirroring
// applyMapFlagValue: every tier is coerced and folded key-by-key in ascending
// precedence, and each key records the tier that last wrote it. The row value
// shows the merged map, and each key renders as a sub-row with its own source
// tier.
func (a *App[T]) whyMapRow(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState, row why.Row) why.Row {
	result, err := foldMapTiers(cmd, cmdChain, flagSpec, cliValues, state, true)

	return whyMapFoldRow(flagSpec, result, err, row, true)
}

// whyConfigFileRow renders the config-file container row. Config-file paths
// are always displayed: the provenance of the input location is the row's
// purpose. On failure it renders an error diagnostic with the failing path.
func (a *App[T]) whyConfigFileRow(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState, loadedPaths []string, loadedSource ValueSource, loadErr error) why.Row {
	row := why.Row{
		Label:  "--" + flagSpec.Name,
		Hidden: flagSpec.Hidden,
	}

	if loadErr != nil {
		row.Value = whyFailedConfigPath(loadErr)
		row.Error = loadErr.Error()

		return row
	}

	if len(loadedPaths) == 0 {
		// Nothing resolved: fall back to the ordinary tier cascade so unset
		// state still renders.
		return a.whyFlagRow(cmd, cmdChain, flagSpec, cliValues, state)
	}

	resolvedPath := loadedPaths[len(loadedPaths)-1]
	if flagSpec.IsSlice {
		resolvedPath = strings.Join(loadedPaths, ", ")
	}

	var displayValue any = resolvedPath
	if flagSpec.IsSlice {
		displayValue = loadedPaths
	}

	row.Value = whyDisplayValue(flagSpec, displayValue)
	row.Source = whySourceWord(loadedSource)

	return row
}

// whyCoercedRow coerces the raw tier values for one flag, turning coercion
// failures into error rows instead of aborting the report. The redact flag
// selects between the fixed-width redaction block and the formatted value.
func (a *App[T]) whyCoercedRow(cmdChain []*spec.Command, flagSpec *spec.Flag, rawValues []string, source ValueSource, sourceName string, state *bindingState, row why.Row, redact bool) why.Row {
	row.Source = whySourceWord(source)

	value, err := whyCoerceValue(flagSpec, rawValues, state)
	if err != nil {
		row.Error = newFlagCoercionError(cmdChain, flagSpec, source, sourceName, whyRawForError(flagSpec, rawValues), err).Error()
		row.Value = cond.If(redact, why.Redacted, strings.Join(rawValues, whyJoinSep(flagSpec)))

		return row
	}

	row.Value = cond.If(redact, why.Redacted, whyDisplayValue(flagSpec, value))

	return row
}

// whyMapFoldRow renders one map flag row from a resolved fold. A zero fold
// renders unset; a coercion failure renders an error row naming the failing
// tier; otherwise the row shows the merged map and one sub-row per key with
// the tier that last wrote it (when includeEntries is set). Redaction applies
// per key: keys written by a non-default tier of an unsafe flag are redacted,
// while default-tier keys stay visible, matching the report's always-display-defaults
// rule.
func whyMapFoldRow(flagSpec *spec.Flag, result mapMergeResult, err error, row why.Row, includeEntries bool) why.Row {
	if len(result.tiers) == 0 {
		row.Source = whySourceWord(SourceNone)

		return row
	}

	if err != nil {
		row.Error = err.Error()
		if !flagSpec.Safe {
			row.Value = why.Redacted
		} else if result.failedTier != nil {
			row.Value = strings.Join(result.failedTier.rawValues, whyJoinSep(flagSpec))
		}

		return row
	}

	highest := highestContributingTier(result.tiers, result.provenance)
	if highest == nil {
		highest = &result.tiers[len(result.tiers)-1]
	}

	row.Source = whySourceWord(highest.source)

	mergedValue := result.merged
	mergedReflect := reflect.ValueOf(mergedValue)
	iter := mergedReflect.MapRange()

	var entries []why.MapEntry
	if includeEntries {
		entries = make([]why.MapEntry, 0, mergedReflect.Len())
	}

	hasNonDefaultKey := false

	for iter.Next() {
		key := stringx.Stringify(iter.Key().Interface())
		keySource := result.provenance[key]

		if keySource != SourceDefault {
			hasNonDefaultKey = true
		}

		if includeEntries {
			entry := why.MapEntry{Key: key, Source: whySourceWord(keySource)}
			if !flagSpec.Safe && keySource != SourceDefault {
				entry.Value = why.Redacted
			} else {
				entry.Value = stringx.Stringify(iter.Value().Interface())
			}

			entries = append(entries, entry)
		}
	}

	if includeEntries {
		slices.SortFunc(entries, func(first, second why.MapEntry) int {
			return strings.Compare(first.Key, second.Key)
		})

		row.Entries = entries
	}

	if !flagSpec.Safe && hasNonDefaultKey {
		row.Value = why.Redacted
	} else {
		row.Value = whyDisplayValue(flagSpec, mergedValue)
	}

	return row
}

// whyFeatureRows renders one provenance row per declared feature. The feature
// map merges every tier key-by-key, so each feature reports the tier that
// last wrote its key; keys no tier set fall back to their field default (or
// unset).
func whyFeatureRows(root *spec.Command, result mapMergeResult) []why.FeatureRow {
	if root.FeatureFlag == nil {
		return nil
	}

	var provided map[string]string
	if merged, ok := result.merged.(map[string]string); ok {
		provided = merged
	}

	values, accepted := featureValuesLenient(root, provided)

	rows := make([]why.FeatureRow, 0, len(root.FeatureFields))

	for _, feature := range root.FeatureFields {
		source := whySourceWord(SourceNone)

		switch {
		case accepted[feature.Name] != "":
			source = whySourceWord(result.provenance[feature.Name])
		case feature.Default != "":
			source = whySourceWord(SourceDefault)
		}

		rows = append(rows, why.FeatureRow{
			Name:   feature.Name,
			Value:  values[feature.Name],
			Source: source,
		})
	}

	return rows
}

// whyConfigPaths annotates every candidate configuration path probed during
// resolution with its probe outcome: the loaded file, the absence of the
// candidate location, the absence of a configuration file, or a read
// failure. Candidates are listed in declared search-path order, mirroring
// the CandidateConfigFiles context accessor. Search-path tiers where nothing
// was found additionally list the concrete file locations the resolution
// probed; explicit and env-derived candidates are resolved as whole paths,
// so they never carry a probed-file list.
func whyConfigPaths(resolver configView, candidates []string, loadedPaths []string, loadErr error, extensions []string) []why.ConfigPath {
	if len(candidates) == 0 {
		return nil
	}

	failedPath := whyFailedConfigPath(loadErr)

	expandedTiers := slicex.Map(resolver.EffectiveSearchPaths(), pathx.Expand)

	paths := make([]why.ConfigPath, 0, len(candidates))

	for _, candidate := range candidates {
		configPath := why.ConfigPath{Path: candidate}

		if loadedFile := whyLoadedConfigFile(candidate, loadedPaths); loadedFile != "" {
			configPath.Status = "loaded: " + loadedFile
		} else if failedPath != "" && whyPathsReferToSameLocation(candidate, failedPath) {
			configPath.Status = "error"
		} else {
			configPath = whyAbsentConfigPath(resolver, candidate, expandedTiers, extensions)
		}

		paths = append(paths, configPath)
	}

	return paths
}

// whyAbsentConfigPath builds the report entry for a candidate path where
// nothing was found: an absent candidate location reports "directory does
// not exist", otherwise "no config file". Search-path tiers the resolution
// actually probes additionally carry the concrete probed file locations;
// explicit and environment-derived candidates do not.
func whyAbsentConfigPath(resolver configView, candidate string, expandedTiers []string, extensions []string) why.ConfigPath {
	configPath := why.ConfigPath{Path: candidate}

	if _, err := os.Stat(candidate); err != nil {
		configPath.Status = "directory does not exist"
	} else {
		configPath.Status = "no config file"
	}

	if slices.Contains(expandedTiers, candidate) {
		configPath.ProbedFiles = resolver.ProbedFiles(candidate, extensions)
	}

	return configPath
}

// whyReportRedacted reports whether any main or global row carries a redacted
// value, driving the footer hint.
func whyReportRedacted(report why.Report) bool {
	for _, rows := range [][]why.Row{report.Rows, report.GlobalRows} {
		for _, row := range rows {
			if row.Value == why.Redacted {
				return true
			}
		}
	}

	return false
}

// whyCoerceValue mirrors applyFlagCoercion's type dispatch but returns the
// coerced value instead of binding it into the flag container, so coercion
// failures surface as report rows rather than aborting the pipeline.
func whyCoerceValue(flagSpec *spec.Flag, rawValues []string, state *bindingState) (any, error) {
	if flagSpec.IsCounter {
		totalCount := 0

		for _, raw := range rawValues {
			parsedCount, err := strconv.Atoi(raw)
			if err != nil {
				return nil, err
			}

			totalCount += parsedCount
		}

		return totalCount, nil
	}

	if flagSpec.IsMap {
		return parseMapTokens(rawValues, flagSpec.TargetType, flagSpec.Sep, state.TypeParsers)
	}

	decodeContext := state.decodeContext(flagSpec.FieldTags)

	if flagSpec.IsSlice {
		return coerce.SliceWith(rawValues, flagSpec.TargetType, flagSpec.Sep, state.TypeParsers, decodeContext)
	}

	return coerce.ScalarWith(rawValues[len(rawValues)-1], flagSpec.TargetType, state.TypeParsers, decodeContext)
}

// whyDisplayValue formats a resolved flag value for the report: strings
// direct, slices joined per Sep, maps rendered with sorted keys for
// deterministic output, and scalars via their typed string form. A nil value
// renders as an empty cell.
func whyDisplayValue(flagSpec *spec.Flag, value any) string {
	if value == nil {
		return ""
	}

	reflected := reflect.ValueOf(value)

	switch reflected.Kind() {
	case reflect.Slice:
		items := make([]string, 0, reflected.Len())

		for i := range reflected.Len() {
			items = append(items, stringx.Stringify(reflected.Index(i).Interface()))
		}

		return strings.Join(items, whyJoinSep(flagSpec))

	case reflect.Map:
		pairs := make([]string, 0, reflected.Len())

		iter := reflected.MapRange()
		for iter.Next() {
			pairs = append(pairs, stringx.Stringify(iter.Key().Interface())+"="+stringx.Stringify(iter.Value().Interface()))
		}

		slices.Sort(pairs)

		return strings.Join(pairs, ", ")

	default:
		return stringx.Stringify(value)
	}
}

// whyJoinSep returns the join separator for a flag's multi-value display;
// sep:"none" means space-separated.
func whyJoinSep(flagSpec *spec.Flag) string {
	if flagSpec.Sep == "" || flagSpec.Sep == "none" {
		return " "
	}

	return flagSpec.Sep
}

// whyRawForError selects the offending raw value for a coercion error,
// mirroring applyFlagCoercion's message inputs.
func whyRawForError(flagSpec *spec.Flag, rawValues []string) string {
	if flagSpec.IsSlice || flagSpec.IsMap {
		return strings.Join(rawValues, flagSpec.Sep)
	}

	return rawValues[len(rawValues)-1]
}

// whyFailedConfigPath extracts the failing path from a config load error, or
// "" when the error carries none.
func whyFailedConfigPath(loadErr error) string {
	if configErr, ok := errors.AsType[configPathError](loadErr); ok {
		return configErr.ConfigPath()
	}

	return ""
}

// whySourceWord renders the bare source-tier word for a report row.
func whySourceWord(source ValueSource) string {
	switch source {
	case SourceCLI:
		return "cli"
	case SourceEnv:
		return "env"
	case SourceConfig:
		return "config"
	case SourceDefault:
		return "default"
	default:
		return "unset"
	}
}

// whyEnvColumn derives the env cell for a report row: the declared env tags,
// or the prefix-derived variable mirroring lookupEnv's derivation, or "-"
// when the flag is excluded from environment variables.
func whyEnvColumn(flagSpec *spec.Flag, cmd *spec.Command, state *bindingState) string {
	if flagSpec.EnvExcluded {
		return "-"
	}

	if len(flagSpec.EnvVars) > 0 {
		return strings.Join(flagSpec.EnvVars, ", ")
	}

	prefix := cmd.EnvPrefix
	if prefix == "" && state != nil {
		prefix = state.EnvPrefix
	}

	if prefix == "" {
		return "-"
	}

	return prefix + casing.UpperSnake(flagSpec.Name)
}

// whyConfigPath derives the config-document key that can set this flag,
// mirroring storeFlagValue's canonical write-side keys: the explicit config
// tag namespaced under the owning command's path, or the flag's canonical
// dotted path. The prefix covers the chain elements up to and including the
// command that owns the flag; root-owned flags carry no prefix regardless
// of which subcommand is active.
func whyConfigPath(flagSpec *spec.Flag, cmdChain []*spec.Command, cmdIndex int) string {
	if flagSpec.ConfigKey == "-" {
		return "-"
	}

	prefix := strings.Join(slicex.Map(cmdChain[1:cmdIndex+1], func(cmd *spec.Command) string { return cmd.Name }), ".")

	if flagSpec.ConfigKey != "" {
		if prefix != "" && !strings.HasPrefix(flagSpec.ConfigKey, prefix+".") {
			return prefix + "." + flagSpec.ConfigKey
		}

		return flagSpec.ConfigKey
	}

	if prefix != "" {
		return prefix + "." + flagSpec.Name
	}

	return flagSpec.Name
}

// whyLoadedConfigFile returns the loaded configuration file belonging to a
// candidate search-path tier, or "" when none does. Directory tiers load
// entries joined directly beneath them, file tiers load themselves, and stem
// tiers load the stem extended by a decoder extension.
func whyLoadedConfigFile(candidate string, loadedPaths []string) string {
	for _, loaded := range loadedPaths {
		if loaded == candidate || filepath.Dir(loaded) == candidate || strings.HasPrefix(loaded, candidate+".") {
			return loaded
		}
	}

	return ""
}

// whyPathsReferToSameLocation reports whether a candidate path refers to the
// same location as a reported failure path, which may still be in raw,
// pre-expansion form (e.g. "~/.config/app.yaml" or "${VAR:-fallback}").
func whyPathsReferToSameLocation(candidate string, failedPath string) bool {
	return candidate == failedPath || candidate == pathx.Expand(failedPath)
}
