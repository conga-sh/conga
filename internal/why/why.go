// Package why renders the neutral value-provenance report produced by the
// --why builtin: one row per flag and configuration-file container with its
// display value (already redacted or formatted by the caller), the source
// tier, and the env/config channels that can set the value. The package owns
// layout only; value formatting and redaction policy live in package conga,
// mirroring the internal/help split.
package why

import (
	"encoding"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/jsonwire"
)

// Redacted is the fixed-width placeholder shown for every value whose field is
// not tagged safe:"true". The width is constant so the output leaks nothing
// about the underlying value's length.
const Redacted = "██████"

// FooterHint is the point-of-use self-documentation rendered after any
// redacted row, advertising the safe struct tag.
const FooterHint = `values are redacted — tag fields safe:"true" to display`

// SchemaVersion is the current JSON schema version emitted by --why=json.
const SchemaVersion = 1

// SchemaURI is the canonical JSON schema URI for --why=json.
const SchemaURI = "https://conga.sh/schemas/why/v1.json"

// Table headers shared by the main table and the Global Flags section.
const (
	headerLabel      = "FLAG"
	headerValue      = "VALUE"
	headerSource     = "SOURCE"
	headerEnv        = "ENV"
	headerConfigPath = "CONFIG PATH"
)

// rowIndent is the leading indentation shared by every rendered table row.
const rowIndent = 2

// ANSI escape sequences for terminal styling.
const (
	ansiReset    = "\033[0m"
	ansiBold     = "\033[1m"
	ansiDim      = "\033[2m"
	ansiGreen    = "\033[32m"
	ansiYellow   = "\033[33m"
	ansiCyan     = "\033[36m"
	ansiDarkGray = "\033[90m"
)

// Row is one neutral report line for a single flag or config-file container.
type Row struct {
	// Name is the bare flag identifier (e.g. "port").
	Name string
	// Type is the schema type name ("int", "string", "string[]", "map", etc.).
	Type string
	// RawValue is the coerced native value for machine-readable export.
	RawValue any
	// Redacted marks this individual row's value as sensitive/redacted.
	Redacted bool
	// Label is the flag display label (e.g. "--port").
	Label string
	// Value is the display value, redacted or formatted by the caller; empty
	// for unset values (the source column says "unset"). Not rendered for a
	// row carrying per-key entries: the sub-rows state each key's value. For
	// a row carrying sub-rows it is the parent value cell, such as
	// "(2 files)" or "(3 values)".
	Value string
	// Source is the bare source-tier word ("cli", "env", "config", "default",
	// "unset"). For a merged map flag this is the highest contributing tier.
	// Not rendered for a row carrying per-key entries: the sub-rows state each
	// key's tier.
	Source string
	// Detail optionally annotates the source with the concrete origin, e.g.
	// the configuration file that supplied the value. Rendered in parentheses
	// after the source word; empty renders the bare word. Detail is always the
	// pure origin; Scope carries any region within it.
	Detail string
	// Scope optionally names a region within Detail. Rendered after Detail
	// inside the same parentheses; empty renders Detail alone.
	Scope string
	// Env lists the environment variables that can set the flag, or "-" when
	// none apply.
	Env string
	// ConfigPath is the config-document key that can set the flag, or "-" when
	// the flag is excluded from configuration files.
	ConfigPath string
	// Hidden marks flags hidden from help and completion.
	Hidden bool
	// Entries lists the per-key provenance of a merged map flag value,
	// rendered as sub-rows beneath the flag row. Empty for scalar and slice
	// flags.
	Entries []MapEntry
	// SubRows lists sub-items (such as multiple loaded config file paths)
	// rendered as tree branches beneath the flag row.
	SubRows []SubRow
}

// SubRow is a single sub-item beneath a row, such as one loaded file of a
// multi-file configuration container.
type SubRow struct {
	Value  string
	Source string
	Detail string
}

// MapEntry is one key of a merged map flag value with the source tier that
// last wrote it.
type MapEntry struct {
	// Key is the map key.
	Key string
	// Value is the key's display value, redacted or formatted by the caller.
	Value string
	// RawValue is the coerced native value for machine-readable export.
	RawValue any
	// Redacted marks this entry's value as sensitive/redacted.
	Redacted bool
	// Source is the bare source-tier word for the tier that last wrote the key.
	Source string
	// Detail optionally annotates the source with the concrete origin, e.g.
	// the configuration file that contributed the key. Rendered in
	// parentheses after the source word; empty renders the bare word. Detail
	// is always the pure origin; Scope carries any region within it.
	Detail string
	// Scope optionally names a region within Detail. Rendered after Detail
	// inside the same parentheses; empty renders Detail alone.
	Scope string
}

// FeatureRow is one feature-toggle line in the report.
type FeatureRow struct {
	// Name is the normalized kebab-case feature name.
	Name string
	// Value is the canonical resolved value ("true"/"false" or a decimal level).
	Value string
	// Source is the bare source-tier word ("cli", "env", "config", "default",
	// "unset").
	Source string
	// Detail optionally annotates the source with the concrete origin, e.g.
	// the configuration file that contributed the toggle. Detail is always the
	// pure origin; Scope carries any region within it.
	Detail string
	// Scope optionally names a region within Detail. Rendered after Detail
	// inside the same parentheses; empty renders Detail alone.
	Scope string
}

// ConfigPath is one candidate configuration file path probed during config
// resolution, annotated with the outcome of the probe.
type ConfigPath struct {
	// Path is the expanded candidate location (a search-path tier, an
	// environment-derived path, or an explicit --config-file location).
	Path string
	// Status is the probe outcome: "loaded" when the candidate resolved to a
	// configuration file, "directory does not exist" when the candidate
	// location is absent, or "no config file" when the candidate is an
	// existing directory holding no matching file.
	Status string
	// Loaded is the concrete file the candidate resolved to when it differs
	// from Path, such as a search-directory tier or a stem candidate. It is
	// empty when Path itself loaded.
	Loaded string
	// ProbedFiles lists the concrete configuration file locations the
	// resolution probed for a search-path tier that found nothing, as full
	// paths (e.g. "<tier>/config.json", "<tier>.yaml"). One report line is
	// rendered per entry. Empty for whole-path candidates and for tiers
	// that loaded a file or failed.
	ProbedFiles []string
}

// Report is the complete --why report: the invocation's command path, the
// rows for the active command's flags, the rows for inherited ancestor flags,
// the candidate configuration paths probed during resolution, the registered
// configuration decoder extensions driving those probes, example
// configuration file locations suggested when nothing was loaded, and whether
// any value was redacted (driving the footer hint).
type Report struct {
	CommandPath      string
	Version          string
	Timestamp        string
	WorkingDirectory string
	Rows             []Row
	GlobalRows       []Row
	// Features lists the declared feature toggles of the root command with
	// their resolved value and source tier.
	Features    []FeatureRow
	ConfigPaths []ConfigPath
	// ConfigDecoders lists the registered configuration decoder extensions
	// (e.g. ".json", ".toml") that drove the configuration file probes;
	// rendered as a standalone line above the config paths.
	ConfigDecoders []string
	// ConfigExamples lists example configuration file locations the user can
	// create for the application to see, rendered when no configuration file
	// was loaded anywhere.
	ConfigExamples []string
	RedactedRows   bool
	// Color enables ANSI color styling in Format(). When false, plain text is
	// rendered.
	Color bool
}

// Format renders the report as plain text following internal/help table
// conventions: a titled line, the aligned flag/value/source/env/config-path
// table for the active command, an inherited "Global Flags" table when
// ancestors contribute flags, the map-entry and sub-row trees, and the
// redaction footer hint.
func (r Report) Format() string {
	renderCtx := newRenderContext(r)

	var builder strings.Builder

	path := r.CommandPath
	if path == "" {
		path = "app"
	}

	fmt.Fprintf(&builder, "Value provenance for %q:\n", path)

	builder.WriteString(renderRows(r.Rows, renderCtx))

	if len(r.GlobalRows) > 0 {
		builder.WriteString("\n")
		builder.WriteString(styleText("Global Flags:", ansiBold, renderCtx.color))
		builder.WriteString("\n")
		builder.WriteString(renderRows(r.GlobalRows, renderCtx))
	}

	if len(r.Features) > 0 {
		builder.WriteString("\n")
		builder.WriteString(styleText("Features:", ansiBold, renderCtx.color))
		builder.WriteString("\n")
		builder.WriteString(renderFeatureRows(r.Features, renderCtx))
	}

	if len(r.ConfigPaths) > 0 {
		if len(r.ConfigDecoders) > 0 {
			builder.WriteString("\nConfig decoders: ")
			builder.WriteString(strings.Join(r.ConfigDecoders, ", "))
			builder.WriteString("\n")
		}

		builder.WriteString("\n")
		builder.WriteString(styleText("Config paths:", ansiBold, renderCtx.color))
		builder.WriteString("\n")
		builder.WriteString(renderConfigPaths(r.ConfigPaths, renderCtx))

		if len(r.ConfigExamples) > 0 {
			builder.WriteString("\nCreate a config file at one of:\n")
			builder.WriteString(renderConfigExamples(r.ConfigExamples))
		}
	}

	if r.RedactedRows {
		builder.WriteString("\n")
		builder.WriteString(styleText(FooterHint, ansiDim, renderCtx.color))
		builder.WriteString("\n")
	}

	return strings.TrimRight(builder.String(), "\n")
}

// FormatJSON renders the report as a machine-readable JSON object following RFC
// 8259, using internal/jsonwire to avoid linking encoding/json.
func (r Report) FormatJSON() string {
	buffer := make([]byte, 0, 1024)

	buffer = append(buffer, `{"$schema":"`...)
	buffer = append(buffer, SchemaURI...)
	buffer = append(buffer, `","schemaVersion":`...)
	buffer = strconv.AppendInt(buffer, SchemaVersion, 10)
	buffer = append(buffer, `,"command":`...)

	path := r.CommandPath
	if path == "" {
		path = "app"
	}

	buffer = jsonwire.AppendString(buffer, path)

	if r.Version != "" {
		buffer = append(buffer, `,"version":`...)
		buffer = jsonwire.AppendString(buffer, r.Version)
	}

	if r.Timestamp != "" {
		buffer = append(buffer, `,"timestamp":`...)
		buffer = jsonwire.AppendString(buffer, r.Timestamp)
	}

	if r.WorkingDirectory != "" {
		buffer = append(buffer, `,"workingDirectory":`...)
		buffer = jsonwire.AppendString(buffer, r.WorkingDirectory)
	}

	buffer = append(buffer, `,"redacted":`...)
	buffer = strconv.AppendBool(buffer, r.RedactedRows)

	buffer = appendRowArrayJSON(buffer, `,"flags":[`, r.Rows)

	if len(r.GlobalRows) > 0 {
		buffer = appendRowArrayJSON(buffer, `,"globalFlags":[`, r.GlobalRows)
	}

	if len(r.Features) > 0 {
		buffer = append(buffer, `,"features":[`...)

		for i, feat := range r.Features {
			if i > 0 {
				buffer = append(buffer, ',')
			}

			buffer = appendFeatureJSON(buffer, feat)
		}

		buffer = append(buffer, ']')
	}

	buffer = appendConfigJSON(buffer, r)

	return string(buffer)
}

// IsColorEnabled reports whether styled color output should be enabled for writer,
// honoring CONGA_COLOR, NO_COLOR, CLICOLOR, TERM=dumb, and TTY detection.
func IsColorEnabled(writer io.Writer) bool {
	if value, ok := os.LookupEnv("CONGA_COLOR"); ok && value != "" {
		return value != "0"
	}

	if value, ok := os.LookupEnv("NO_COLOR"); ok && value != "" {
		return false
	}

	if value, ok := os.LookupEnv("CLICOLOR"); ok && value == "0" {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return isTerminalWriter(writer)
}

// originShortener maps displayed provenance origins to concise forms. A
// unique basename is used when it is unambiguous; colliding basenames fall
// back to the shortest unique directory prefix followed by "...".
type originShortener struct {
	displays map[string]string
}

func newOriginShortener(report Report) originShortener {
	origins := collectReportOrigins(report)
	if len(origins) == 0 {
		return originShortener{}
	}

	groups := make(map[string][]string, len(origins))

	for origin := range origins {
		candidate := originCandidate(origin)
		groups[candidate] = append(groups[candidate], origin)
	}

	displays := make(map[string]string, len(origins))

	for candidate, group := range groups {
		if len(group) == 1 {
			if group[0] != candidate {
				displays[group[0]] = candidate
			}

			continue
		}

		for origin, display := range disambiguateOrigins(group) {
			if origin != display {
				displays[origin] = display
			}
		}
	}

	return originShortener{displays: displays}
}

func (s originShortener) display(origin string) string {
	if display, ok := s.displays[origin]; ok {
		return display
	}

	return origin
}

type renderContext struct {
	color   bool
	origins originShortener
}

func newRenderContext(report Report) renderContext {
	return renderContext{
		color:   report.Color,
		origins: newOriginShortener(report),
	}
}

func (rc renderContext) sourceText(source, detail, scope string) (string, string) {
	if detail == "" {
		return formatSourceTier(source, rc.color)
	}

	detail = configdata.JoinOriginScope(rc.origins.display(detail), scope)

	plainTier, styledTier := formatSourceTier(source, rc.color)
	plain := plainTier + " (" + detail + ")"
	styled := styledTier + " " + styleText("("+detail+")", ansiDim, rc.color)

	return plain, styled
}

func (rc renderContext) rowCells(row Row) ([]string, []string) {
	lbl := rowLabel(row)

	styledLbl := lbl
	if rc.color {
		styledLbl = styleText(lbl, ansiBold, true)
	}

	if len(row.Entries) > 0 {
		count := len(row.Entries)

		keyLabel := "keys"
		if count == 1 {
			keyLabel = "key"
		}

		value := "(" + strconv.Itoa(count) + " " + keyLabel + ")"
		srcPlain, srcStyled := rc.sourceText(row.Source, row.Detail, row.Scope)

		return []string{lbl, value, srcPlain, row.Env, row.ConfigPath},
			[]string{styledLbl, value, srcStyled, row.Env, row.ConfigPath}
	} else if len(row.SubRows) > 0 {
		value := subRowValue(row)
		srcPlain, srcStyled := rc.sourceText(row.Source, row.Detail, row.Scope)

		return []string{lbl, value, srcPlain, row.Env, row.ConfigPath},
			[]string{styledLbl, value, srcStyled, row.Env, row.ConfigPath}
	}

	srcPlain, srcStyled := rc.sourceText(row.Source, row.Detail, row.Scope)

	value := row.Value
	styledValue := value

	if rc.color && row.Redacted {
		styledValue = styleText(value, ansiDarkGray, true)
	}

	return []string{lbl, value, srcPlain, row.Env, row.ConfigPath},
		[]string{styledLbl, styledValue, srcStyled, row.Env, row.ConfigPath}
}

type tableLine struct {
	cells       []string
	styledCells []string
	row         Row
}

type renderedPath struct {
	path        string
	status      string
	bullet      string
	probedFiles []string
}

func formatDetail(detail, scope string) string {
	return configdata.JoinOriginScope(detail, scope)
}

func subRowValue(row Row) string {
	if row.Value != "" {
		return row.Value
	}

	count := len(row.SubRows)

	fileLabel := "files"
	if count == 1 {
		fileLabel = "file"
	}

	return "(" + strconv.Itoa(count) + " " + fileLabel + ")"
}

func collectReportOrigins(report Report) map[string]struct{} {
	origins := make(map[string]struct{})

	add := func(origin string) {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}

	addRows := func(rows []Row) {
		for _, row := range rows {
			add(row.Detail)

			for _, entry := range row.Entries {
				add(entry.Detail)
			}

			for _, subRow := range row.SubRows {
				add(subRow.Value)
				add(subRow.Detail)
			}
		}
	}

	addRows(report.Rows)
	addRows(report.GlobalRows)

	for _, feature := range report.Features {
		add(feature.Detail)
	}

	for _, configPath := range report.ConfigPaths {
		add(configPath.Path)
		add(configPath.Loaded)

		for _, probed := range configPath.ProbedFiles {
			add(probed)
		}
	}

	for _, example := range report.ConfigExamples {
		add(example)
	}

	return origins
}

func originCandidate(origin string) string {
	if !isPathLike(origin) {
		return origin
	}

	base := filepath.Base(origin)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return origin
	}

	return base
}

func disambiguateOrigins(origins []string) map[string]string {
	displays := make(map[string]string, len(origins))

	for _, origin := range origins {
		if display := disambiguateOrigin(origin, origins); display != origin {
			displays[origin] = display
		}
	}

	seen := make(map[string]string, len(displays))

	for origin, display := range displays {
		if previous, duplicate := seen[display]; duplicate {
			delete(displays, previous)
			delete(displays, origin)

			continue
		}

		seen[display] = origin
	}

	return displays
}

func disambiguateOrigin(origin string, group []string) string {
	if !isPathLike(origin) {
		return origin
	}

	base := filepath.Base(origin)
	if base == "." || base == ".." || base == string(filepath.Separator) {
		return origin
	}

	shared := 0

	for _, other := range group {
		if other != origin && isPathLike(other) {
			shared = max(shared, sharedPrefixLength(origin, other))
		}
	}

	if shared == 0 || shared >= len(origin) {
		return origin
	}

	remainder := origin[shared:]
	if index := strings.IndexByte(remainder, byte(filepath.Separator)); index > 0 {
		prefix := origin[:shared+index]
		if prefix == filepath.Dir(origin) {
			return origin
		}

		separator := string(filepath.Separator)

		return prefix + separator + "..." + separator + base
	}

	return origin
}

func sharedPrefixLength(first, second string) int {
	length := min(len(first), len(second))

	for index := range length {
		if first[index] != second[index] {
			return index
		}
	}

	return length
}

func isPathLike(value string) bool {
	return strings.ContainsAny(value, `/\`)
}

func formatSourceTier(source string, enableColor bool) (string, string) {
	if !enableColor {
		return source, source
	}

	switch source {
	case "cli":
		return "● " + source, styleText("●", ansiCyan, true) + " " + styleText(source, ansiCyan, true)
	case "env":
		return "● " + source, styleText("●", ansiYellow, true) + " " + styleText(source, ansiYellow, true)
	case "config":
		return "● " + source, styleText("●", ansiGreen, true) + " " + styleText(source, ansiGreen, true)
	case "default":
		return "○ " + source, styleText("○", ansiDarkGray, true) + " " + styleText(source, ansiDarkGray, true)
	default:
		return source, styleText(source, ansiDim, true)
	}
}

func renderRows(rows []Row, renderCtx renderContext) string {
	if len(rows) == 0 {
		return ""
	}

	header := []string{headerLabel, headerValue, headerSource, headerEnv, headerConfigPath}

	styledHeader := make([]string, len(header))
	for i, h := range header {
		styledHeader[i] = styleText(h, ansiBold, renderCtx.color)
	}

	grid := make([][]string, 0, len(rows)+1)
	grid = append(grid, header)

	styledGrid := make([][]string, 0, len(rows)+1)
	styledGrid = append(styledGrid, styledHeader)

	lines := make([]tableLine, 0, len(rows))

	for _, row := range rows {
		plainCells, styledCells := renderCtx.rowCells(row)
		grid = append(grid, plainCells)
		styledGrid = append(styledGrid, styledCells)

		lines = append(lines, tableLine{
			cells:       plainCells,
			styledCells: styledCells,
			row:         row,
		})
	}

	widths := measureColumnWidths(grid)

	return renderTableRows(grid, styledGrid, widths, lines, renderCtx)
}

func renderTableRows(grid [][]string, styledGrid [][]string, widths []int, lines []tableLine, renderCtx renderContext) string {
	var builder strings.Builder

	builder.WriteString("\n")
	writeGridRowWithStyled(&builder, grid[0], styledGrid[0], widths)

	for _, line := range lines {
		writeGridRowWithStyled(&builder, line.cells, line.styledCells, widths)

		if len(line.row.Entries) > 0 {
			renderMapSubRows(&builder, line.row.Entries, widths, renderCtx)
		} else if len(line.row.SubRows) > 0 {
			renderSubRows(&builder, line.row.SubRows, widths, renderCtx)
		}
	}

	return builder.String()
}

func renderMapSubRows(builder *strings.Builder, entries []MapEntry, widths []int, renderCtx renderContext) {
	maxTokenWidth := 0

	for _, entry := range entries {
		token := entry.Key + "=" + entry.Value
		maxTokenWidth = max(maxTokenWidth, utf8.RuneCountInString(token))
	}

	valColIndent := rowIndent + widths[0] + 3

	for entryIdx, entry := range entries {
		branch := "├─ "
		if entryIdx == len(entries)-1 {
			branch = "└─ "
		}

		token := entry.Key + "=" + entry.Value
		_, srcStyled := renderCtx.sourceText(entry.Source, entry.Detail, entry.Scope)

		builder.WriteString(strings.Repeat(" ", valColIndent))
		builder.WriteString(styleText(branch, ansiDim, renderCtx.color))

		styledToken := token
		if renderCtx.color && entry.Value == Redacted {
			styledToken = entry.Key + "=" + styleText(entry.Value, ansiDarkGray, true)
		}

		builder.WriteString(styledToken)
		builder.WriteString(strings.Repeat(" ", maxTokenWidth-utf8.RuneCountInString(token)))
		builder.WriteString(strings.Repeat(" ", 3))
		builder.WriteString(srcStyled)
		builder.WriteString("\n")
	}
}

func renderSubRows(builder *strings.Builder, subRows []SubRow, widths []int, renderCtx renderContext) {
	maxTokenWidth := 0

	for _, subRow := range subRows {
		maxTokenWidth = max(maxTokenWidth, utf8.RuneCountInString(subRow.Value))
	}

	valColIndent := rowIndent + widths[0] + 3

	for idx, subRow := range subRows {
		branch := "├─ "
		if idx == len(subRows)-1 {
			branch = "└─ "
		}

		builder.WriteString(strings.Repeat(" ", valColIndent))
		builder.WriteString(styleText(branch, ansiDim, renderCtx.color))
		builder.WriteString(subRow.Value)

		if subRow.Source != "" {
			_, srcStyled := renderCtx.sourceText(subRow.Source, subRow.Detail, "")
			builder.WriteString(strings.Repeat(" ", maxTokenWidth-utf8.RuneCountInString(subRow.Value)))
			builder.WriteString(strings.Repeat(" ", 3))
			builder.WriteString(srcStyled)
		}

		builder.WriteString("\n")
	}
}

// renderFeatureRows lays out the aligned feature-name · value · source table
// beneath a header row.
func renderFeatureRows(rows []FeatureRow, renderCtx renderContext) string {
	header := []string{"FEATURE", headerValue, headerSource}

	grid := make([][]string, 0, len(rows)+1)
	grid = append(grid, header)

	styledHeader := make([]string, len(header))
	for i, h := range header {
		styledHeader[i] = styleText(h, ansiBold, renderCtx.color)
	}

	styledGrid := make([][]string, 0, len(rows)+1)
	styledGrid = append(styledGrid, styledHeader)

	for _, row := range rows {
		_, srcStyled := renderCtx.sourceText(row.Source, row.Detail, row.Scope)
		plainCells := []string{row.Name, row.Value, row.Source}
		styledCells := []string{styleText(row.Name, ansiBold, renderCtx.color), row.Value, srcStyled}

		grid = append(grid, plainCells)
		styledGrid = append(styledGrid, styledCells)
	}

	widths := measureColumnWidths(grid)

	var builder strings.Builder

	writeGridRowWithStyled(&builder, header, styledHeader, widths)

	for index := range rows {
		writeGridRowWithStyled(&builder, grid[index+1], styledGrid[index+1], widths)
	}

	return builder.String()
}

func renderPlainConfigPath(configPath ConfigPath, renderCtx renderContext) renderedPath {
	status := configPath.Status
	if configPath.Loaded != "" && configPath.Loaded != configPath.Path {
		status += ": " + configPath.Loaded
	}

	item := renderedPath{
		path:        configPath.Path,
		status:      status,
		probedFiles: configPath.ProbedFiles,
	}

	if strings.HasPrefix(configPath.Status, "loaded") {
		if renderCtx.color {
			item.bullet = styleText("●", ansiGreen, true) + " "
		}

		item.status = styleText(item.status, ansiGreen, renderCtx.color)
	} else {
		if renderCtx.color {
			item.bullet = styleText("○", ansiDarkGray, true) + " "
		}

		item.status = styleText(item.status, ansiDarkGray, renderCtx.color)
	}

	return item
}

// renderConfigPaths lays out the candidate-path list beneath its header,
// annotating each path with its probe outcome.
func renderConfigPaths(paths []ConfigPath, renderCtx renderContext) string {
	const (
		probeIndent = 4
		columnGap   = 3
	)

	rendered := make([]renderedPath, 0, len(paths))
	maxPathWidth := 0

	for _, configPath := range paths {
		item := renderPlainConfigPath(configPath, renderCtx)
		maxPathWidth = max(maxPathWidth, utf8.RuneCountInString(item.path))
		rendered = append(rendered, item)
	}

	var builder strings.Builder

	for _, item := range rendered {
		builder.WriteString(strings.Repeat(" ", rowIndent))
		builder.WriteString(item.bullet)
		builder.WriteString(pad(item.path, maxPathWidth))
		builder.WriteString(strings.Repeat(" ", columnGap))
		builder.WriteString(item.status)
		builder.WriteString("\n")

		for _, probed := range item.probedFiles {
			builder.WriteString(strings.Repeat(" ", probeIndent))
			builder.WriteString("looked for: ")
			builder.WriteString(probed)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// renderConfigExamples lays out the indented example configuration locations
// beneath the creation hint.
func renderConfigExamples(examples []string) string {
	var builder strings.Builder

	for _, example := range examples {
		builder.WriteString(strings.Repeat(" ", rowIndent))
		builder.WriteString(example)
		builder.WriteString("\n")
	}

	return builder.String()
}

// rowLabel renders a row's flag label with the hidden marker applied.
func rowLabel(row Row) string {
	if row.Hidden {
		return row.Label + " (hidden)"
	}

	return row.Label
}

func writeGridRowWithStyled(builder *strings.Builder, plainCells, styledCells []string, widths []int) {
	builder.WriteString(strings.Repeat(" ", rowIndent))
	writeTableCellsWithStyled(builder, plainCells[:len(plainCells)-1], styledCells[:len(styledCells)-1], widths)
	builder.WriteString(styledCells[len(styledCells)-1])
	builder.WriteString("\n")
}

func writeTableCellsWithStyled(builder *strings.Builder, plainCells, styledCells []string, widths []int) {
	for column, plain := range plainCells {
		styled := styledCells[column]

		builder.WriteString(styled)

		padding := widths[column] - utf8.RuneCountInString(plain)
		if padding > 0 {
			builder.WriteString(strings.Repeat(" ", padding))
		}

		builder.WriteString(strings.Repeat(" ", 3))
	}
}

// measureColumnWidths measures the display width of every table column across
// the provided rows. Rows shorter than the widest row contribute no width for
// the columns they omit.
func measureColumnWidths(rows [][]string) []int {
	var widths []int

	for _, cells := range rows {
		for column, cell := range cells {
			if column == len(widths) {
				widths = append(widths, 0)
			}

			widths[column] = max(widths[column], utf8.RuneCountInString(cell))
		}
	}

	return widths
}

func pad(cell string, width int) string {
	padding := width - utf8.RuneCountInString(cell)
	if padding <= 0 {
		return cell
	}

	return cell + strings.Repeat(" ", padding)
}

func styleText(text, escape string, enableColor bool) string {
	if !enableColor || escape == "" || text == "" {
		return text
	}

	return escape + text + ansiReset
}

func isTerminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return false
	}

	return fileInfo.Mode()&os.ModeCharDevice != 0
}

func appendRowArrayJSON(buffer []byte, field string, rows []Row) []byte {
	buffer = append(buffer, field...)

	for i, row := range rows {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendRowJSON(buffer, row)
	}

	return append(buffer, ']')
}

func appendConfigJSON(buffer []byte, report Report) []byte {
	buffer = append(buffer, `,"config":{"decoders":[`...)

	for i, dec := range report.ConfigDecoders {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = jsonwire.AppendString(buffer, dec)
	}

	buffer = append(buffer, `],"paths":[`...)

	for i, configPath := range report.ConfigPaths {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendConfigPathJSON(buffer, configPath)
	}

	buffer = append(buffer, ']')

	if len(report.ConfigExamples) > 0 {
		buffer = append(buffer, `,"examples":[`...)

		for i, ex := range report.ConfigExamples {
			if i > 0 {
				buffer = append(buffer, ',')
			}

			buffer = jsonwire.AppendString(buffer, ex)
		}

		buffer = append(buffer, ']')
	}

	return append(buffer, `}}`...)
}

func appendConfigPathJSON(buffer []byte, configPath ConfigPath) []byte {
	buffer = append(buffer, '{')
	buffer = append(buffer, `"path":`...)
	buffer = jsonwire.AppendString(buffer, configPath.Path)
	buffer = append(buffer, `,"status":`...)
	buffer = jsonwire.AppendString(buffer, normalizeStatusEnum(configPath.Status))

	if len(configPath.ProbedFiles) > 0 {
		buffer = append(buffer, `,"probedFiles":[`...)

		for i, probed := range configPath.ProbedFiles {
			if i > 0 {
				buffer = append(buffer, ',')
			}

			buffer = jsonwire.AppendString(buffer, probed)
		}

		buffer = append(buffer, ']')
	}

	buffer = append(buffer, '}')

	return buffer
}

func normalizeStatusEnum(status string) string {
	switch {
	case strings.HasPrefix(status, "loaded"):
		return "loaded"
	case strings.Contains(status, "directory does not exist"):
		return "not_found"
	case strings.Contains(status, "no config file"):
		return "no_config_file"
	default:
		return status
	}
}

func appendRowJSON(buffer []byte, row Row) []byte {
	buffer = append(buffer, '{')
	buffer = append(buffer, `"name":`...)

	name := row.Name
	if name == "" {
		name = strings.TrimPrefix(row.Label, "--")
	}

	buffer = jsonwire.AppendString(buffer, name)

	if row.Type != "" {
		buffer = append(buffer, `,"type":`...)
		buffer = jsonwire.AppendString(buffer, row.Type)
	}

	if row.Redacted {
		buffer = append(buffer, `,"redacted":true`...)
		if row.Type != "map" {
			buffer = append(buffer, `,"value":null`...)
		}
	} else if len(row.Entries) == 0 {
		buffer = append(buffer, `,"value":`...)

		value := row.RawValue
		if value == nil && row.Value != "" {
			value = row.Value
		}

		buffer = appendValueJSON(buffer, value)
	}

	buffer = append(buffer, `,"source":`...)
	buffer = jsonwire.AppendString(buffer, row.Source)

	if detail := formatDetail(row.Detail, row.Scope); detail != "" {
		buffer = append(buffer, `,"detail":`...)
		buffer = jsonwire.AppendString(buffer, detail)
	}

	return appendRowMetadataJSON(buffer, row)
}

func appendRowMetadataJSON(buffer []byte, row Row) []byte {
	if row.Env != "" && row.Env != "-" {
		buffer = append(buffer, `,"env":`...)
		buffer = jsonwire.AppendString(buffer, row.Env)
	}

	if row.ConfigPath != "" && row.ConfigPath != "-" {
		buffer = append(buffer, `,"configPath":`...)
		buffer = jsonwire.AppendString(buffer, row.ConfigPath)
	}

	if row.Hidden {
		buffer = append(buffer, `,"hidden":true`...)
	}

	if len(row.Entries) > 0 {
		buffer = append(buffer, `,"entries":[`...)

		for i, entry := range row.Entries {
			if i > 0 {
				buffer = append(buffer, ',')
			}

			buffer = appendMapEntryJSON(buffer, entry)
		}

		buffer = append(buffer, ']')
	}

	return append(buffer, '}')
}

func appendMapEntryJSON(buffer []byte, entry MapEntry) []byte {
	buffer = append(buffer, `{"key":`...)
	buffer = jsonwire.AppendString(buffer, entry.Key)

	if entry.Redacted {
		buffer = append(buffer, `,"redacted":true,"value":null`...)
	} else {
		buffer = append(buffer, `,"value":`...)

		value := entry.RawValue
		if value == nil && entry.Value != "" {
			value = entry.Value
		}

		buffer = appendValueJSON(buffer, value)
	}

	buffer = append(buffer, `,"source":`...)
	buffer = jsonwire.AppendString(buffer, entry.Source)

	if detail := formatDetail(entry.Detail, entry.Scope); detail != "" {
		buffer = append(buffer, `,"detail":`...)
		buffer = jsonwire.AppendString(buffer, detail)
	}

	buffer = append(buffer, '}')

	return buffer
}

func appendFeatureJSON(buffer []byte, feat FeatureRow) []byte {
	buffer = append(buffer, `{"name":`...)
	buffer = jsonwire.AppendString(buffer, feat.Name)
	buffer = append(buffer, `,"value":`...)
	buffer = jsonwire.AppendString(buffer, feat.Value)
	buffer = append(buffer, `,"source":`...)
	buffer = jsonwire.AppendString(buffer, feat.Source)

	if detail := formatDetail(feat.Detail, feat.Scope); detail != "" {
		buffer = append(buffer, `,"detail":`...)
		buffer = jsonwire.AppendString(buffer, detail)
	}

	buffer = append(buffer, '}')

	return buffer
}

func appendValueJSON(buffer []byte, value any) []byte {
	if value == nil {
		return append(buffer, "null"...)
	}

	if text, ok := stringForm(value); ok {
		return jsonwire.AppendString(buffer, text)
	}

	return appendKindValueJSON(buffer, reflect.ValueOf(value))
}

// stringForm renders values that carry their own textual representation,
// matching whyDisplayValue's fmt.Stringer/encoding.TextMarshaler precedence.
// Pointer-receiver implementations are reached through an addressable copy,
// mirroring stringx.Stringify.
func stringForm(value any) (string, bool) {
	if stringer, ok := value.(fmt.Stringer); ok {
		if reflected := reflect.ValueOf(stringer); reflected.Kind() != reflect.Pointer || !reflected.IsNil() {
			return stringer.String(), true
		}
	}

	if marshaler, ok := value.(encoding.TextMarshaler); ok {
		if reflected := reflect.ValueOf(marshaler); reflected.Kind() != reflect.Pointer || !reflected.IsNil() {
			if text, err := marshaler.MarshalText(); err == nil {
				return string(text), true
			}
		}
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Struct {
		return "", false
	}

	addressable := reflect.New(reflected.Type())
	addressable.Elem().Set(reflected)

	if stringer, ok := reflect.TypeAssert[fmt.Stringer](addressable); ok {
		return stringer.String(), true
	}

	if marshaler, ok := reflect.TypeAssert[encoding.TextMarshaler](addressable); ok {
		if text, err := marshaler.MarshalText(); err == nil {
			return string(text), true
		}
	}

	return "", false
}

func appendKindValueJSON(buffer []byte, value reflect.Value) []byte {
	switch value.Kind() {
	case reflect.Bool:
		return strconv.AppendBool(buffer, value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(buffer, value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.AppendUint(buffer, value.Uint(), 10)
	case reflect.Float32:
		return strconv.AppendFloat(buffer, value.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.AppendFloat(buffer, value.Float(), 'f', -1, 64)
	case reflect.String:
		return jsonwire.AppendString(buffer, value.String())
	case reflect.Slice:
		return appendSliceValueJSON(buffer, value)
	case reflect.Map:
		return appendMapValueJSON(buffer, value)
	default:
		return jsonwire.AppendString(buffer, fmt.Sprint(value.Interface()))
	}
}

func appendSliceValueJSON(buffer []byte, value reflect.Value) []byte {
	buffer = append(buffer, '[')

	for i := range value.Len() {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendValueJSON(buffer, value.Index(i).Interface())
	}

	return append(buffer, ']')
}

func appendMapValueJSON(buffer []byte, value reflect.Value) []byte {
	type mapEntry struct {
		key   string
		value reflect.Value
	}

	entries := make([]mapEntry, 0, value.Len())

	iter := value.MapRange()
	for iter.Next() {
		entries = append(entries, mapEntry{key: fmt.Sprint(iter.Key().Interface()), value: iter.Value()})
	}

	slices.SortFunc(entries, func(a, b mapEntry) int {
		return strings.Compare(a.key, b.key)
	})

	buffer = append(buffer, '{')

	for i, entry := range entries {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = jsonwire.AppendString(buffer, entry.key)
		buffer = append(buffer, ':')
		buffer = appendValueJSON(buffer, entry.value.Interface())
	}

	return append(buffer, '}')
}
