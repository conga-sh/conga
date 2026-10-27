package conga

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/why"
)

// whyJSONFormatterFunc renders the --why report as a machine-readable JSON object.
// It is installed on App only when WhyOptionJSON() is passed to EnableWhy.
type whyJSONFormatterFunc func(report *why.Report) string

// whyRowTypeFunc classifies a flag's schema type for the JSON report. It is
// installed on App only when WhyOptionJSON() is passed to EnableWhy so text-only
// reports keep the reflection machinery unreferenced.
type whyRowTypeFunc func(flagSpec *spec.Flag) string

type whyHooks struct {
	render              func(session *Session, rawFormat string) error
	bindSession         func(session *Session)
	validateFormat      func(cmdChain []*spec.Command, rawFormat string) error
	recordMapProvenance func(session *Session, flag *spec.Flag, sources map[string]ValueSource, origins map[string]configdata.Value)
	jsonFormatter       whyJSONFormatterFunc
	rowType             whyRowTypeFunc
}

// mapFlagProvenance is the per-key provenance of a merged map flag recorded
// during binding. It lives in a run-scoped table on Session rather than on the
// flag containers: OptionalFlag[T] and RequiredFlag[T] are instantiated once
// per flag value type, so per-container provenance fields would enlarge every
// instantiation in the binary.
type mapFlagProvenance struct {
	sources map[string]ValueSource
	origins map[string]configdata.Value
}

func (a *App[T]) installWhyRenderer(settings whySettings) {
	a.whyHooks = &whyHooks{
		render: func(session *Session, rawFormat string) error {
			return a.renderWhy(session, rawFormat)
		},
		bindSession: func(session *Session) {
			a.bindWhySession(session)
		},
		validateFormat: func(cmdChain []*spec.Command, rawFormat string) error {
			return a.validateWhyFormat(cmdChain, rawFormat)
		},
		recordMapProvenance: (*Session).recordMapProvenance,
		jsonFormatter:       settings.jsonFormatter,
		rowType:             settings.rowType,
	}
}

func (a *App[T]) resolveWhyFormat(cmdChain []*spec.Command, rawFormat string) (string, error) {
	switch rawFormat {
	case "", "true", spec.WhyFormatText:
		return spec.WhyFormatText, nil
	case spec.WhyFormatJSON:
		if a.whyHooks == nil || a.whyHooks.jsonFormatter == nil {
			return "", &validationError{
				msg:         `invalid value "json" for --why: JSON output format is not enabled on this application (enable with app.EnableWhy(conga.WhyOptionJSON()))`,
				cmdChain:    cmdChain,
				cause:       errInvalidValue,
				valueShaped: true,
			}
		}

		return spec.WhyFormatJSON, nil
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

	if format == spec.WhyFormatJSON {
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

	var rowType whyRowTypeFunc
	if a.whyHooks != nil {
		rowType = a.whyHooks.rowType
	}

	for cmdIndex, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Handle == nil {
				// Built-in command flags (for example version --json) carry no
				// container and therefore no provenance.
				continue
			}

			if flagSpec == cmd.FeatureFlag && flagSpec.Hidden {
				continue
			}

			row := why.BuildFlagRow(a.whyFlagInput(session, cmd, cmdChain, cmdIndex, flagSpec, rowType))
			if cmd == leaf {
				rows = append(rows, row)
			} else {
				globalRows = append(globalRows, row)
			}
		}
	}

	var features []why.FeatureRow

	if root.FeatureFields != nil && root.FeatureFlag != nil {
		if featureTarget := featureStateOfCommand(root); featureTarget != nil {
			sources, origins := featureTarget.mapProvenance()
			features = why.BuildFeatureRows(why.FeatureInput{
				Fields:  root.FeatureFields,
				Sources: sources,
				Origins: origins,
				Values:  featureTarget.resolvedValues(),
			})
		}
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

	return why.Report{
		CommandPath:    session.Path(),
		Version:        session.appVersion,
		Rows:           rows,
		GlobalRows:     globalRows,
		Features:       features,
		ConfigPaths:    why.BuildConfigPaths(session.candidateFiles, session.loadedFiles, configProbed),
		ConfigDecoders: configDecoders,
		ConfigExamples: configExamples,
		RedactedRows:   why.HasRedactedRows(rows, globalRows),
	}
}

func (a *App[T]) whyFlagInput(session *Session, cmd *spec.Command, cmdChain []*spec.Command, cmdIndex int, flagSpec *spec.Flag, rowType whyRowTypeFunc) why.FlagInput {
	target := flagStateOf(flagSpec.Handle)
	source := target.Source()

	input := why.FlagInput{
		Command:      cmd,
		CommandChain: cmdChain,
		CommandIndex: cmdIndex,
		EnvPrefix:    session.envPrefix,
		Flag:         flagSpec,
		Source:       source,
		RowType:      rowType,
	}

	if source == SourceNone {
		return input
	}

	input.Value = target.getValue()

	if source == SourceConfig && !flagSpec.IsConfigFile {
		if flagSpec.IsMap {
			input.ConfigOrigin = whyMapConfigOrigin(featureStateOf(flagSpec.Handle), session, flagSpec)
		} else {
			input.ConfigOrigin, input.ConfigScope = whyConfigOrigin(flagSpec, cmd, cmdChain, a.configData)
		}
	}

	if source == SourceEnv {
		input.EnvOrigin = a.dotenvOrigins[target.SourceDetail()]
	}

	if flagSpec.IsMap && flagSpec != cmd.FeatureFlag {
		input.MapSources, input.MapOrigins = session.lookupMapProvenance(flagSpec)
	}

	return input
}

func (a *App[T]) bindWhySession(session *Session) {
	session.whyRenderer = a.renderWhyString
}

func (s *Session) recordMapProvenance(flag *spec.Flag, sources map[string]ValueSource, origins map[string]configdata.Value) {
	if s == nil {
		return
	}

	if s.mapProvenance == nil {
		s.mapProvenance = make(map[*spec.Flag]mapFlagProvenance)
	}

	s.mapProvenance[flag] = mapFlagProvenance{sources: sources, origins: origins}
}

func (s *Session) lookupMapProvenance(flag *spec.Flag) (map[string]ValueSource, map[string]configdata.Value) {
	if s == nil {
		return nil, nil
	}

	provenance := s.mapProvenance[flag]

	return provenance.sources, provenance.origins
}

// whyConfigOrigin recovers the pure origin and optional scope recorded for a
// config-sourced scalar or slice flag. Container details keep the display-ready
// joined form, so the structural pair comes from the resolved configuration
// data instead.
func whyConfigOrigin(flagSpec *spec.Flag, cmd *spec.Command, cmdChain []*spec.Command, data *configdata.Data) (string, string) {
	value, key, ok := lookupConfigValue(flagSpec, cmd, cmdChain, data)
	if !ok {
		return "", ""
	}

	return cmp.Or(value.Origin, key), value.Scope
}

// whyMapConfigOrigin returns the single configuration origin contributing to a
// merged map or feature row. Per-key sub-rows carry the individual origins when
// several files contribute, so the parent row stays unannotated instead of
// presenting a joined string the origin shortener would treat as one path.
func whyMapConfigOrigin(featureTarget featureState, session *Session, flagSpec *spec.Flag) string {
	if featureTarget != nil {
		sources, origins := featureTarget.mapProvenance()

		return singleConfigOrigin(sources, origins)
	}

	sources, origins := session.lookupMapProvenance(flagSpec)

	return singleConfigOrigin(sources, origins)
}

func singleConfigOrigin(sources map[string]ValueSource, origins map[string]configdata.Value) string {
	var origin string

	for key, source := range sources {
		if source != SourceConfig {
			continue
		}

		entryOrigin := origins[key].Origin
		if entryOrigin == "" {
			continue
		}

		if origin != "" && origin != entryOrigin {
			return ""
		}

		origin = entryOrigin
	}

	return origin
}
