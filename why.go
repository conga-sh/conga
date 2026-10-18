package conga

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
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
	"github.com/conga-sh/conga/internal/why"
)

// whyJSONFormatterFunc renders the --why report as a machine-readable JSON object.
// It is installed on App only when WhyJSON() is passed to EnableWhy.
type whyJSONFormatterFunc func(report *why.Report) string

type whyHooks struct {
	render              func(session *Session, rawFormat string) error
	renderString        func(session *Session, format string) string
	bindSession         func(session *Session)
	validateFormat      func(cmdChain []*spec.Command, rawFormat string) error
	recordMapProvenance func(session *Session, flag *spec.Flag, sources map[string]ValueSource, origins map[string]string)
	jsonFormatter       whyJSONFormatterFunc
}

// mapFlagProvenance is the per-key provenance of a merged map flag recorded
// during binding. It lives in a run-scoped table on Session rather than on the
// flag containers: OptionalFlag[T] and RequiredFlag[T] are instantiated once
// per flag value type, so per-container provenance fields would enlarge every
// instantiation in the binary.
type mapFlagProvenance struct {
	sources map[string]ValueSource
	origins map[string]string
}

func (s *Session) recordMapProvenance(flag *spec.Flag, sources map[string]ValueSource, origins map[string]string) {
	if s == nil {
		return
	}

	if s.mapProvenance == nil {
		s.mapProvenance = make(map[*spec.Flag]mapFlagProvenance)
	}

	s.mapProvenance[flag] = mapFlagProvenance{sources: sources, origins: origins}
}

func (s *Session) lookupMapProvenance(flag *spec.Flag) (map[string]ValueSource, map[string]string) {
	if s == nil {
		return nil, nil
	}

	provenance := s.mapProvenance[flag]

	return provenance.sources, provenance.origins
}

func (a *App[T]) installWhyRenderer(settings whySettings) {
	a.whyHooks = &whyHooks{
		render: func(session *Session, rawFormat string) error {
			return a.renderWhy(session, rawFormat)
		},
		renderString: func(session *Session, format string) string {
			return a.renderWhyString(session, format)
		},
		bindSession: func(session *Session) {
			a.bindWhySession(session)
		},
		validateFormat: func(cmdChain []*spec.Command, rawFormat string) error {
			return a.validateWhyFormat(cmdChain, rawFormat)
		},
		recordMapProvenance: (*Session).recordMapProvenance,
		jsonFormatter:       settings.jsonFormatter,
	}
}

func (a *App[T]) resolveWhyFormat(cmdChain []*spec.Command, rawFormat string) (string, error) {
	switch rawFormat {
	case "", "true", "text":
		return "text", nil
	case "json":
		if a.whyHooks == nil || a.whyHooks.jsonFormatter == nil {
			return "", &validationError{
				msg:         `invalid value "json" for --why: JSON output format is not enabled on this application (enable with app.EnableWhy(conga.WhyJSON()))`,
				cmdChain:    cmdChain,
				cause:       errInvalidValue,
				valueShaped: true,
			}
		}

		return "json", nil
	default:
		return "", &validationError{
			msg:         fmt.Sprintf("invalid value %q (must be one of: text, json) for --why", truncateForDisplay(rawFormat)),
			cmdChain:    cmdChain,
			cause:       errInvalidValue,
			valueShaped: true,
		}
	}
}

func (a *App[T]) validateWhyFormat(cmdChain []*spec.Command, rawFormat string) error {
	_, err := a.resolveWhyFormat(cmdChain, rawFormat)

	return err
}

func (a *App[T]) renderWhy(session *Session, rawFormat string) error {
	format, err := a.resolveWhyFormat(session.cmdChain, rawFormat)
	if err != nil {
		return err
	}

	fmt.Fprintln(session.Stdout(), a.renderWhyString(session, format))

	return nil
}

func (a *App[T]) renderWhyString(session *Session, format string) string {
	report := a.buildWhyReport(session)
	report.Color = why.IsColorEnabled(session.Stdout())

	if format == "json" {
		if a.whyHooks != nil && a.whyHooks.jsonFormatter != nil {
			return a.whyHooks.jsonFormatter(&report)
		}

		return ""
	}

	return report.Format()
}

func (a *App[T]) buildWhyReport(session *Session) why.Report {
	cmdChain := session.cmdChain
	root := cmdChain[0]
	leaf := cmdChain[len(cmdChain)-1]

	var (
		rows       []why.Row
		globalRows []why.Row
	)

	for cmdIndex, cmd := range cmdChain {
		isLeaf := cmd == leaf

		for _, flagSpec := range cmd.Flags {
			if flagSpec.Handle == nil {
				// Built-in command flags (for example version --json) carry no
				// container and therefore no provenance.
				continue
			}

			if flagSpec == cmd.FeatureFlag && flagSpec.Hidden {
				continue
			}

			row := a.buildWhyFlagRow(session, cmd, cmdChain, cmdIndex, flagSpec)
			if isLeaf {
				rows = append(rows, row)
			} else {
				globalRows = append(globalRows, row)
			}
		}
	}

	var features []why.FeatureRow
	if root.FeatureFields != nil && root.FeatureFlag != nil {
		features = a.buildWhyFeatureRows(root)
	}

	var configDecoders []string
	if a.configRegistry != nil {
		configDecoders = a.configRegistry.Extensions()
	}

	resolver := a.configurationView(root)
	expandedTiers := slicex.Map(resolver.EffectiveSearchPaths(), pathx.Expand)
	configProbed := make(map[string][]string)

	for _, candidate := range session.candidateFiles {
		if slices.Contains(expandedTiers, candidate) {
			configProbed[candidate] = resolver.ProbedFiles(candidate, configDecoders)
		}
	}

	var configExamples []string
	if len(session.loadedFiles) == 0 {
		configExamples = resolver.ConfigExamples(session.candidateFiles, expandedTiers, configDecoders)
	}

	configPaths := a.buildWhyConfigPaths(session, configProbed)

	return why.Report{
		CommandPath:    session.Path(),
		Version:        session.appVersion,
		Rows:           rows,
		GlobalRows:     globalRows,
		Features:       features,
		ConfigPaths:    configPaths,
		ConfigDecoders: configDecoders,
		ConfigExamples: configExamples,
		RedactedRows:   hasRedactedRows(rows, globalRows),
	}
}

func (a *App[T]) buildWhyFlagRow(session *Session, cmd *spec.Command, cmdChain []*spec.Command, cmdIndex int, flagSpec *spec.Flag) why.Row {
	target := flagStateOf(flagSpec.Handle)
	source := target.Source()

	detail := ""
	if source == SourceConfig && !flagSpec.IsConfigFile && !flagSpec.IsMap {
		detail = target.SourceDetail()
	}

	row := why.Row{
		Name:       flagSpec.Name,
		Label:      "--" + flagSpec.Name,
		Type:       whyFlagType(flagSpec),
		Hidden:     flagSpec.Hidden,
		Env:        whyEnvColumn(flagSpec, cmd, session.envPrefix),
		ConfigPath: whyConfigPath(flagSpec, cmdChain, cmdIndex),
		Source:     whySourceWord(source),
		Detail:     detail,
	}

	if source == SourceNone {
		return row
	}

	value := target.getValue()
	row.RawValue = value

	if flagSpec.IsConfigFile {
		return buildWhyConfigFileRow(flagSpec, value, row)
	}

	if flagSpec.IsMap && flagSpec != cmd.FeatureFlag {
		sources, origins := session.lookupMapProvenance(flagSpec)

		return a.buildWhyMapRow(flagSpec, value, sources, origins, row)
	}

	if !flagSpec.Safe && source != SourceDefault {
		row.Redacted = true
		row.Value = why.Redacted

		return row
	}

	row.Value = whyDisplayValue(flagSpec, value)

	return row
}

func (a *App[T]) buildWhyMapRow(flagSpec *spec.Flag, value any, sources map[string]ValueSource, origins map[string]string, row why.Row) why.Row {
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
		if sources[key] != SourceDefault {
			hasNonDefault = true

			break
		}
	}

	if !flagSpec.Safe && hasNonDefault {
		row.Redacted = true
	} else {
		row.Value = whyDisplayValue(flagSpec, value)
	}

	entries := make([]why.MapEntry, len(keys))
	for i, key := range keys {
		rawEntryValue := entryValues[key]
		keySource := sources[key]

		entry := why.MapEntry{
			Key:      key,
			Source:   whySourceWord(keySource),
			Detail:   origins[key],
			RawValue: rawEntryValue,
		}

		if !flagSpec.Safe && keySource != SourceDefault {
			entry.Redacted = true
			entry.Value = why.Redacted
		} else {
			entry.Value = stringx.Stringify(rawEntryValue)
		}

		entries[i] = entry
	}

	row.Entries = entries

	return row
}

func (a *App[T]) buildWhyFeatureRows(root *spec.Command) []why.FeatureRow {
	target := featureStateOfCommand(root)
	if target == nil {
		return nil
	}

	sources, origins := target.mapProvenance()
	values := target.resolvedValues()

	rows := make([]why.FeatureRow, 0, len(root.FeatureFields))
	for _, feature := range root.FeatureFields {
		fieldName := feature.Name
		keySource := sources[fieldName]

		if keySource == SourceNone && feature.Default != "" {
			keySource = SourceDefault
		}

		rows = append(rows, why.FeatureRow{
			Name:   feature.Name,
			Value:  values[fieldName],
			Source: whySourceWord(keySource),
			Detail: origins[fieldName],
		})
	}

	return rows
}

func (a *App[T]) buildWhyConfigPaths(session *Session, configProbed map[string][]string) []why.ConfigPath {
	candidates := session.candidateFiles
	if len(candidates) == 0 {
		return nil
	}

	paths := make([]why.ConfigPath, 0, len(candidates))
	for _, candidate := range candidates {
		if loadedFile := whyLoadedConfigFile(candidate, session.loadedFiles); loadedFile != "" {
			paths = append(paths, why.ConfigPath{
				Path:   candidate,
				Status: "loaded: " + loadedFile,
			})
		} else {
			probed := configProbed[candidate]
			paths = append(paths, whyAbsentConfigPath(candidate, probed))
		}
	}

	return paths
}

func (a *App[T]) bindWhySession(session *Session) {
	session.whyRenderer = a.renderWhyString
}

func buildWhyConfigFileRow(flagSpec *spec.Flag, value any, row why.Row) why.Row {
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
		row.SubRows = make([]why.SubRow, len(paths))

		for i, p := range paths {
			row.SubRows[i] = why.SubRow{
				Value: p,
			}
		}

		return row
	}

	row.Value = paths[0]

	return row
}

func whyAbsentConfigPath(candidate string, probedFiles []string) why.ConfigPath {
	configPath := why.ConfigPath{Path: candidate, ProbedFiles: probedFiles}

	if _, err := os.Stat(candidate); err != nil {
		configPath.Status = "directory does not exist"
	} else {
		configPath.Status = "no config file"
	}

	return configPath
}

func whyLoadedConfigFile(candidate string, loadedPaths []string) string {
	for _, loaded := range loadedPaths {
		if loaded == candidate || filepath.Dir(loaded) == candidate || strings.HasPrefix(loaded, candidate+".") {
			return loaded
		}
	}

	return ""
}

func hasRedactedRows(rows, globalRows []why.Row) bool {
	for _, r := range rows {
		if r.Redacted {
			return true
		}
	}

	for _, r := range globalRows {
		if r.Redacted {
			return true
		}
	}

	return false
}

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

func whyEnvColumn(flagSpec *spec.Flag, cmd *spec.Command, envPrefix string) string {
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

func whyJoinSep(flagSpec *spec.Flag) string {
	if flagSpec.Sep == "" || flagSpec.Sep == "none" {
		return " "
	}

	return flagSpec.Sep
}

func whyScalarKindType(kind reflect.Kind) string {
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

func whyFlagType(flagSpec *spec.Flag) string {
	if flagSpec.TargetType != nil && whyStringType(flagSpec.TargetType) {
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
		if whyStringType(flagSpec.TargetType.Elem()) {
			return "string[]"
		}

		return whyScalarKindType(flagSpec.TargetType.Elem().Kind()) + "[]"
	}

	if flagSpec.TargetType.Kind() == reflect.Map {
		return "map"
	}

	return whyScalarKindType(flagSpec.TargetType.Kind())
}

// whyStringType reports whether values of targetType render through their own
// string form (fmt.Stringer or encoding.TextMarshaler), which the JSON
// serializer mirrors. Such values are described as "string" in the report.
func whyStringType(targetType reflect.Type) bool {
	return targetType.Implements(reflect.TypeFor[fmt.Stringer]()) ||
		targetType.Implements(reflect.TypeFor[encoding.TextMarshaler]())
}

func formatWhyJSON(report *why.Report) string {
	enrichWhyJSONReport(report)

	return report.FormatJSON()
}

func enrichWhyJSONReport(report *why.Report) {
	report.Timestamp = time.Now().UTC().Format(time.RFC3339)

	if workingDirectory, err := os.Getwd(); err == nil {
		report.WorkingDirectory = workingDirectory
	}
}
