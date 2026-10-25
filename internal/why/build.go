package why

import (
	"encoding"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
)

// FlagInput carries the engine-extracted provenance for one flag row. The
// caller supplies value and source state as plain data; this package never
// observes engine container handles.
type FlagInput struct {
	// Command is the command that declares the flag.
	Command *spec.Command
	// CommandChain is the active command path from the root to the leaf.
	CommandChain []*spec.Command
	// CommandIndex is the position of Command within CommandChain.
	CommandIndex int
	// EnvPrefix is the application-level environment prefix used when the
	// command declares none.
	EnvPrefix string
	// Flag is the flag's schema metadata.
	Flag *spec.Flag
	// Source is the tier that supplied the value.
	Source container.Source
	// Value is the coerced native value, or nil when Source is SourceNone.
	Value any
	// MapSources records the per-key source tier of a merged map value.
	MapSources map[string]container.Source
	// MapOrigins records the per-key configuration origin of a merged map
	// value.
	MapOrigins map[string]configdata.Value
	// ConfigOrigin is the pure configuration origin for a scalar or slice
	// config value.
	ConfigOrigin string
	// ConfigScope is the optional region within ConfigOrigin.
	ConfigScope string
	// EnvOrigin is the .env file that supplied an environment-sourced scalar
	// or slice value, or "" for host environment variables.
	EnvOrigin string
	// RowType classifies the flag's schema type for the JSON report. It is nil
	// when JSON output is not enabled, keeping the reflection machinery
	// unreferenced.
	RowType func(*spec.Flag) string
}

// FeatureInput carries the engine-extracted provenance for the feature-toggle
// table.
type FeatureInput struct {
	// Fields lists the declared feature toggles in declaration order.
	Fields []*spec.FeatureField
	// Sources records the per-key source tier of the merged feature map.
	Sources map[string]container.Source
	// Origins records the per-key configuration origin of the merged feature
	// map.
	Origins map[string]configdata.Value
	// Values holds the canonical resolved value of each feature toggle.
	Values map[string]string
}

// BuildFlagRow assembles one report row for a flag from prepared provenance
// inputs, applying the display formatting and redaction policy.
func BuildFlagRow(input FlagInput) Row {
	source := input.Source
	includeJSON := input.RowType != nil

	row := Row{
		Label:      "--" + input.Flag.Name,
		Hidden:     input.Flag.Hidden,
		Env:        envColumn(input.Flag, input.Command, input.EnvPrefix),
		ConfigPath: configPath(input.Flag, input.CommandChain, input.CommandIndex),
		Source:     sourceWord(source),
	}

	if includeJSON {
		row.Name = input.Flag.Name
		row.Type = input.RowType(input.Flag)
	}

	row.Detail, row.Scope = flagRowDetail(input, source)

	if source == container.SourceNone {
		return row
	}

	value := input.Value
	if includeJSON {
		row.RawValue = value
	}

	if input.Flag.IsConfigFile {
		return buildConfigFileRow(input.Flag, value, row)
	}

	if input.Flag.IsMap && input.Flag != input.Command.FeatureFlag {
		return buildMapRow(input.Flag, value, input.MapSources, input.MapOrigins, row, includeJSON)
	}

	if !input.Flag.Safe && source != container.SourceDefault {
		row.Redacted = true
		row.Value = Redacted

		return row
	}

	if input.Flag.IsSlice {
		row = buildSliceRow(input.Flag, value, row)
	} else {
		row.Value = displayValue(input.Flag, value)
	}

	if includeJSON && row.Type == "string" {
		row.RawValue = row.Value
	}

	return row
}

// BuildFeatureRows assembles the feature-toggle table from prepared provenance
// inputs.
func BuildFeatureRows(input FeatureInput) []FeatureRow {
	rows := make([]FeatureRow, 0, len(input.Fields))

	for _, feature := range input.Fields {
		fieldName := feature.Name
		keySource := input.Sources[fieldName]

		if keySource == container.SourceNone && feature.Default != "" {
			keySource = container.SourceDefault
		}

		rows = append(rows, FeatureRow{
			Name:   feature.Name,
			Value:  input.Values[fieldName],
			Source: sourceWord(keySource),
			Detail: input.Origins[fieldName].Origin,
			Scope:  input.Origins[fieldName].Scope,
		})
	}

	return rows
}

// BuildConfigPaths assembles the candidate configuration path table from the
// session's candidate and loaded paths and the per-tier probe results.
func BuildConfigPaths(candidates, loaded []string, configProbed map[string][]string) []ConfigPath {
	if len(candidates) == 0 {
		return nil
	}

	paths := make([]ConfigPath, 0, len(candidates))

	for _, candidate := range candidates {
		if loadedFile := loadedConfigFile(candidate, loaded); loadedFile != "" {
			configPath := ConfigPath{Path: candidate, Status: "loaded"}
			if loadedFile != candidate {
				configPath.Loaded = loadedFile
			}

			paths = append(paths, configPath)
		} else {
			paths = append(paths, absentConfigPath(candidate, configProbed[candidate]))
		}
	}

	return paths
}

// HasRedactedRows reports whether any row in either table carries a redacted
// value.
func HasRedactedRows(rows, globalRows []Row) bool {
	for _, row := range rows {
		if row.Redacted {
			return true
		}
	}

	for _, row := range globalRows {
		if row.Redacted {
			return true
		}
	}

	return false
}

// FlagType classifies a flag's schema type for the JSON report.
func FlagType(flagSpec *spec.Flag) string {
	if flagSpec.TargetType != nil && stringType(flagSpec.TargetType) {
		return "string"
	}

	if flagSpec.IsMap {
		return "map"
	}

	if flagSpec.IsBool {
		return "bool"
	}

	if flagSpec.IsCounter {
		return "int"
	}

	if flagSpec.TargetType == nil {
		if flagSpec.IsSlice {
			return "string[]"
		}

		return "string"
	}

	if flagSpec.TargetType.Kind() == reflect.Slice {
		if stringType(flagSpec.TargetType.Elem()) {
			return "string[]"
		}

		return scalarKindType(flagSpec.TargetType.Elem().Kind()) + "[]"
	}

	if flagSpec.TargetType.Kind() == reflect.Map {
		return "map"
	}

	return scalarKindType(flagSpec.TargetType.Kind())
}

// FormatJSONReport enriches the report with JSON-only metadata and renders it
// as a machine-readable JSON document.
func FormatJSONReport(report *Report) string {
	enrichJSONReport(report)

	return report.FormatJSON()
}

// flagRowDetail resolves a flag row's detail and scope: the configuration
// origin for a config-sourced value, or the .env file for an env-sourced value
// a dotenv file supplied.
func flagRowDetail(input FlagInput, source container.Source) (string, string) {
	if source == container.SourceConfig && !input.Flag.IsConfigFile {
		return input.ConfigOrigin, input.ConfigScope
	}

	if source == container.SourceEnv && input.EnvOrigin != "" {
		return input.EnvOrigin, ""
	}

	return "", ""
}

func buildMapRow(flagSpec *spec.Flag, value any, sources map[string]container.Source, origins map[string]configdata.Value, row Row, includeJSON bool) Row {
	if value == nil {
		return row
	}

	valueMap := reflect.ValueOf(value)
	if valueMap.Kind() != reflect.Map {
		return row
	}

	keys := make([]string, 0, valueMap.Len())
	entryValues := make(map[string]any, valueMap.Len())

	iter := valueMap.MapRange()
	for iter.Next() {
		key := stringx.Stringify(iter.Key().Interface())
		keys = append(keys, key)
		entryValues[key] = iter.Value().Interface()
	}

	slices.Sort(keys)

	hasNonDefault := false

	for _, key := range keys {
		if sources[key] != container.SourceDefault {
			hasNonDefault = true

			break
		}
	}

	if !flagSpec.Safe && hasNonDefault {
		row.Redacted = true
	} else {
		row.Value = displayValue(flagSpec, value)
	}

	entries := make([]MapEntry, len(keys))
	for i, key := range keys {
		rawEntryValue := entryValues[key]
		keySource := sources[key]

		entry := MapEntry{
			Key:    key,
			Source: sourceWord(keySource),
			Detail: origins[key].Origin,
			Scope:  origins[key].Scope,
		}

		if includeJSON {
			entry.RawValue = rawEntryValue
		}

		if !flagSpec.Safe && keySource != container.SourceDefault {
			entry.Redacted = true
			entry.Value = Redacted
		} else {
			entry.Value = stringx.Stringify(rawEntryValue)
		}

		entries[i] = entry
	}

	row.Entries = entries

	return row
}

func buildSliceRow(flagSpec *spec.Flag, value any, row Row) Row {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Slice || reflected.Len() <= 1 {
		row.Value = displayValue(flagSpec, value)

		return row
	}

	items := make([]string, reflected.Len())
	for index := range reflected.Len() {
		items[index] = stringx.Stringify(reflected.Index(index).Interface())
	}

	row.Value = fmt.Sprintf("(%d values)", len(items))
	row.SubRows = make([]SubRow, len(items))

	for index, item := range items {
		row.SubRows[index] = SubRow{Value: item}
	}

	return row
}

func buildConfigFileRow(flagSpec *spec.Flag, value any, row Row) Row {
	var paths []string

	switch typed := value.(type) {
	case []string:
		paths = typed
	case string:
		if typed != "" {
			paths = []string{typed}
		}
	}

	if len(paths) == 0 {
		return row
	}

	if flagSpec.IsSlice && len(paths) > 1 {
		row.Value = fmt.Sprintf("(%d files)", len(paths))
		row.SubRows = make([]SubRow, len(paths))

		for i, path := range paths {
			row.SubRows[i] = SubRow{
				Value: path,
			}
		}

		return row
	}

	row.Value = paths[0]

	return row
}

func absentConfigPath(candidate string, probedFiles []string) ConfigPath {
	configPath := ConfigPath{Path: candidate, ProbedFiles: probedFiles}

	if _, err := os.Stat(candidate); err != nil {
		configPath.Status = "directory does not exist"
	} else {
		configPath.Status = "no config file"
	}

	return configPath
}

func loadedConfigFile(candidate string, loadedPaths []string) string {
	for _, loaded := range loadedPaths {
		if loaded == candidate || filepath.Dir(loaded) == candidate || strings.HasPrefix(loaded, candidate+".") {
			return loaded
		}
	}

	return ""
}

func sourceWord(source container.Source) string {
	switch source {
	case container.SourceCLI:
		return "cli"
	case container.SourceEnv:
		return "env"
	case container.SourceConfig:
		return "config"
	case container.SourceDefault:
		return "default"
	default:
		return "unset"
	}
}

func envColumn(flagSpec *spec.Flag, cmd *spec.Command, envPrefix string) string {
	if flagSpec.EnvExcluded {
		return "-"
	}

	if len(flagSpec.EnvVars) > 0 {
		return strings.Join(flagSpec.EnvVars, ", ")
	}

	prefix := cmd.EnvPrefix
	if prefix == "" {
		prefix = envPrefix
	}

	if prefix == "" {
		return "-"
	}

	return prefix + casing.UpperSnake(flagSpec.Name)
}

func configPath(flagSpec *spec.Flag, cmdChain []*spec.Command, cmdIndex int) string {
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

func displayValue(flagSpec *spec.Flag, value any) string {
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

		return strings.Join(items, joinSep(flagSpec))

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

func joinSep(flagSpec *spec.Flag) string {
	if flagSpec.Sep == "" || flagSpec.Sep == "none" {
		return " "
	}

	return flagSpec.Sep
}

func scalarKindType(kind reflect.Kind) string {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Bool:
		return "bool"
	default:
		return "string"
	}
}

func stringType(targetType reflect.Type) bool {
	return targetType.Implements(reflect.TypeFor[fmt.Stringer]()) ||
		targetType.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}

func enrichJSONReport(report *Report) {
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)

	if workingDirectory, err := os.Getwd(); err == nil {
		report.WorkingDirectory = workingDirectory
	}
}
