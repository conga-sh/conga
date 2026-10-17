// Package why renders the neutral value-provenance report produced by the
// --why builtin: one row per flag and configuration-file container with its
// display value (already redacted or formatted by the caller), the source
// tier, and the env/config channels that can set the value. The package owns
// layout only; value formatting and redaction policy live in package conga,
// mirroring the internal/help split.
package why

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/jsonwire"
)

// Redacted is the fixed-width placeholder shown for every value whose field is
// not tagged safe:"true". The width is constant so the output leaks nothing
// about the underlying value's length.
const Redacted = "██████"

// FooterHint is the point-of-use self-documentation rendered after any
// redacted row, advertising the safe struct tag.
const FooterHint = `values are redacted — tag fields safe:"true" to display`

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
	ansiRed      = "\033[31m"
	ansiGreen    = "\033[32m"
	ansiYellow   = "\033[33m"
	ansiCyan     = "\033[36m"
	ansiDarkGray = "\033[90m"
)

// Row is one neutral report line for a single flag or config-file container.
type Row struct {
	// Label is the flag display label (e.g. "--port").
	Label string
	// Value is the display value, redacted or formatted by the caller; empty
	// for unset values (the source column says "unset"). Not rendered for a
	// row carrying per-key entries: the sub-rows state each key's value.
	Value string
	// Source is the bare source-tier word ("cli", "env", "config", "default",
	// "unset"). For a merged map flag this is the highest contributing tier.
	// Not rendered for a row carrying per-key entries: the sub-rows state each
	// key's tier.
	Source string
	// Detail optionally annotates the source with the concrete origin, e.g.
	// the configuration file that supplied the value. Rendered in parentheses
	// after the source word; empty renders the bare word.
	Detail string
	// Env lists the environment variables that can set the flag, or "-" when
	// none apply.
	Env string
	// ConfigPath is the config-document key that can set the flag, or "-" when
	// the flag is excluded from configuration files.
	ConfigPath string
	// Error optionally carries a diagnostic message (coercion failure,
	// config-file read failure) rendered as an "error: ..." row diagnostic.
	Error string
	// Hidden marks flags hidden from help and completion.
	Hidden bool
	// Entries lists the per-key provenance of a merged map flag value,
	// rendered as sub-rows beneath the flag row. Empty for scalar and slice
	// flags and for error rows.
	Entries []MapEntry
}

// MapEntry is one key of a merged map flag value with the source tier that
// last wrote it.
type MapEntry struct {
	// Key is the map key.
	Key string
	// Value is the key's display value, redacted or formatted by the caller.
	Value string
	// Source is the bare source-tier word for the tier that last wrote the key.
	Source string
	// Detail optionally annotates the source with the concrete origin, e.g.
	// the configuration file that contributed the key. Rendered in
	// parentheses after the source word; empty renders the bare word.
	Detail string
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
	// the configuration file that contributed the toggle.
	Detail string
}

// Report is the complete --why report: the invocation's command path, the
// rows for the active command's flags, the rows for inherited ancestor flags,
// the candidate configuration paths probed during resolution, the registered
// configuration decoder extensions driving those probes, example
// configuration file locations suggested when nothing was loaded, and whether
// any value was redacted (driving the footer hint).
type Report struct {
	CommandPath string
	Rows        []Row
	GlobalRows  []Row
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
	Redacted       bool
	// Color enables ANSI color styling in Format(). When false, plain text is
	// rendered.
	Color bool
}

// Format renders the report as plain text following internal/help table
// conventions: a titled line, the aligned flag/value/source/env/config-path
// table for the active command, an inherited "Global Flags" table when
// ancestors contribute flags, per-row diagnostics, and the redaction footer
// hint.
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

		sectionTitle := "Config paths checked:"
		if renderCtx.useIndices {
			sectionTitle = "Config files:"
		}

		builder.WriteString(styleText(sectionTitle, ansiBold, renderCtx.color))
		builder.WriteString("\n")
		builder.WriteString(renderConfigPaths(r.ConfigPaths, renderCtx))

		if len(r.ConfigExamples) > 0 {
			builder.WriteString("\nCreate a config file at one of:\n")
			builder.WriteString(renderConfigExamples(r.ConfigExamples))
		}
	}

	if r.Redacted {
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

	buffer = append(buffer, `{"command":`...)

	path := r.CommandPath
	if path == "" {
		path = "app"
	}

	buffer = jsonwire.AppendString(buffer, path)

	buffer = append(buffer, `,"flags":[`...)

	for i, row := range r.Rows {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendRowJSON(buffer, row)
	}

	buffer = append(buffer, `],"globalFlags":[`...)

	for i, row := range r.GlobalRows {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendRowJSON(buffer, row)
	}

	buffer = append(buffer, `],"features":[`...)

	for i, feat := range r.Features {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendFeatureJSON(buffer, feat)
	}

	buffer = append(buffer, `],"config":{"decoders":[`...)

	for i, dec := range r.ConfigDecoders {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = jsonwire.AppendString(buffer, dec)
	}

	buffer = append(buffer, `],"paths":[`...)

	for i, configPath := range r.ConfigPaths {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendConfigPathJSON(buffer, configPath)
	}

	buffer = append(buffer, `],"examples":[`...)

	for i, ex := range r.ConfigExamples {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = jsonwire.AppendString(buffer, ex)
	}

	buffer = append(buffer, `]},"redacted":`...)
	if r.Redacted {
		buffer = append(buffer, "true}"...)
	} else {
		buffer = append(buffer, "false}"...)
	}

	return string(buffer)
}

// ConfigPath is one candidate configuration file path probed during config
// resolution, annotated with the outcome of the probe.
type ConfigPath struct {
	// Path is the expanded candidate location (a search-path tier, an
	// environment-derived path, or an explicit --config-file location).
	Path string
	// Status is the probe outcome: "loaded: <file>" when the candidate
	// resolved to a configuration file, "directory does not exist" when the
	// candidate location is absent, "no config file" when the candidate is
	// an existing directory holding no matching file, or "error" when
	// reading the candidate failed.
	Status string
	// ProbedFiles lists the concrete configuration file locations the
	// resolution probed for a search-path tier that found nothing, as full
	// paths (e.g. "<tier>/config.json", "<tier>.yaml"). One report line is
	// rendered per entry. Empty for whole-path candidates and for tiers
	// that loaded a file or failed.
	ProbedFiles []string
}

// IsColorEnabled reports whether styled color output should be enabled for writer,
// honoring CONGA_COLOR, NO_COLOR, CLICOLOR, TERM=dumb, and TTY detection.
func IsColorEnabled(writer io.Writer) bool {
	if val, ok := os.LookupEnv("CONGA_COLOR"); ok && val != "" {
		return val != "0"
	}

	if val, ok := os.LookupEnv("NO_COLOR"); ok && val != "" {
		return false
	}

	if os.Getenv("CLICOLOR") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return isTerminalWriter(writer)
}

// renderContext carries terminal capabilities and origin indexing state for
// layout formatting.
type renderContext struct {
	color         bool
	originIndices map[string]int
	useIndices    bool
}

func newRenderContext(report Report) renderContext {
	renderCtx := renderContext{
		color:         report.Color,
		originIndices: make(map[string]int),
	}

	nextIndex := 1

	for _, configPath := range report.ConfigPaths {
		if strings.HasPrefix(configPath.Status, "loaded") {
			loadedFile := configPath.Path
			if file, ok := strings.CutPrefix(configPath.Status, "loaded: "); ok {
				loadedFile = file
			}

			if _, exists := renderCtx.originIndices[loadedFile]; !exists {
				renderCtx.originIndices[loadedFile] = nextIndex
				nextIndex++
			}

			renderCtx.originIndices[configPath.Path] = renderCtx.originIndices[loadedFile]
		}
	}

	collectConfigDetail := func(source, detail string) {
		if source == "config" && detail != "" {
			if _, exists := renderCtx.originIndices[detail]; !exists {
				renderCtx.originIndices[detail] = nextIndex
				nextIndex++
			}
		}
	}

	for _, row := range report.Rows {
		collectConfigDetail(row.Source, row.Detail)

		for _, entry := range row.Entries {
			collectConfigDetail(entry.Source, entry.Detail)
		}
	}

	for _, row := range report.GlobalRows {
		collectConfigDetail(row.Source, row.Detail)

		for _, entry := range row.Entries {
			collectConfigDetail(entry.Source, entry.Detail)
		}
	}

	for _, feat := range report.Features {
		collectConfigDetail(feat.Source, feat.Detail)
	}

	distinctCount := 0
	seen := make(map[int]bool)

	for _, idx := range renderCtx.originIndices {
		if !seen[idx] {
			seen[idx] = true
			distinctCount++
		}
	}

	renderCtx.useIndices = distinctCount >= 1

	return renderCtx
}

func (rc renderContext) sourceText(source, detail string) (string, string) {
	if source == "config" && rc.useIndices && detail != "" {
		if idx, ok := rc.originIndices[detail]; ok {
			badge := "[" + strconv.Itoa(idx) + "]"

			base := "config " + badge
			if rc.color {
				plain := "● " + base
				styled := styleText("●", ansiYellow, true) + " " + styleText(base, ansiBold, true)

				return plain, styled
			}

			return base, base
		}
	}

	text := source
	if detail != "" {
		text = source + " (" + detail + ")"
	}

	if !rc.color {
		return text, text
	}

	switch source {
	case "cli":
		return "● " + text, styleText("●", ansiCyan, true) + " " + styleText(text, ansiBold, true)
	case "env":
		return "● " + text, styleText("●", ansiGreen, true) + " " + styleText(text, ansiBold, true)
	case "config":
		return "● " + text, styleText("●", ansiYellow, true) + " " + styleText(text, ansiBold, true)
	case "default":
		return "· " + text, styleText("·", ansiDim, true) + " " + styleText(text, ansiDim, true)
	default:
		return "○ " + text, styleText("○", ansiDarkGray, true) + " " + styleText(text, ansiDarkGray, true)
	}
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

		val := "(" + strconv.Itoa(count) + " " + keyLabel + ")"
		srcPlain, srcStyled := rc.sourceText(row.Source, row.Detail)

		return []string{lbl, val, srcPlain, row.Env, row.ConfigPath},
			[]string{styledLbl, val, srcStyled, row.Env, row.ConfigPath}
	}

	val := row.Value

	styledVal := val
	if rc.color && val == Redacted {
		styledVal = styleText(val, ansiDarkGray, true)
	}

	srcPlain, srcStyled := rc.sourceText(row.Source, row.Detail)
	if row.Error != "" {
		srcPlain = ""
		srcStyled = ""
	}

	return []string{lbl, val, srcPlain, row.Env, row.ConfigPath},
		[]string{styledLbl, styledVal, srcStyled, row.Env, row.ConfigPath}
}

type renderedRow struct {
	cells        []string
	styledCells  []string
	row          Row
	errorMessage string
}

type renderedPath struct {
	badge       string
	badgeStyled string
	bullet      string
	path        string
	status      string
	probedFiles []string
}

// renderRows lays out the aligned flag · value · source · env · config-path
// table beneath a header row. Error rows replace the source column with an
// "error: ..." diagnostic and stop there, so long messages never push the
// later columns out of alignment for the remaining rows.
func renderRows(rows []Row, optionalRC ...renderContext) string {
	var renderCtx renderContext
	if len(optionalRC) > 0 {
		renderCtx = optionalRC[0]
	}

	header := []string{headerLabel, headerValue, headerSource, headerEnv, headerConfigPath}

	lines := make([]renderedRow, 0, len(rows))
	measurement := make([][]string, 0, len(rows)+1)
	measurement = append(measurement, header)

	for _, row := range rows {
		plainCells, styledCells := renderCtx.rowCells(row)
		measurement = append(measurement, plainCells)

		lines = append(lines, renderedRow{
			cells:        plainCells,
			styledCells:  styledCells,
			row:          row,
			errorMessage: row.Error,
		})
	}

	widths := measureColumnWidths(measurement)

	var builder strings.Builder

	if len(rows) > 0 {
		builder.WriteString("\n")
	}

	styledHeader := make([]string, len(header))
	for i, h := range header {
		styledHeader[i] = styleText(h, ansiBold, renderCtx.color)
	}

	writeGridRowWithStyled(&builder, header, styledHeader, widths)

	for _, line := range lines {
		if line.errorMessage != "" {
			builder.WriteString(strings.Repeat(" ", rowIndent))
			writeTableCellsWithStyled(&builder, line.cells[:2], line.styledCells[:2], widths)
			builder.WriteString(styleText("error: ", ansiRed+ansiBold, renderCtx.color))
			builder.WriteString(styleText(line.errorMessage, ansiRed, renderCtx.color))
			builder.WriteString("\n")

			continue
		}

		writeGridRowWithStyled(&builder, line.cells, line.styledCells, widths)

		if len(line.row.Entries) > 0 {
			renderMapSubRows(&builder, line.row.Entries, widths, renderCtx)
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
		_, srcStyled := renderCtx.sourceText(entry.Source, entry.Detail)

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

// renderFeatureRows lays out the aligned feature-name · value · source table
// beneath a header row.
func renderFeatureRows(rows []FeatureRow, optionalRC ...renderContext) string {
	var renderCtx renderContext
	if len(optionalRC) > 0 {
		renderCtx = optionalRC[0]
	}

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
		srcPlain, srcStyled := renderCtx.sourceText(row.Source, row.Detail)
		plainCells := []string{row.Name, row.Value, srcPlain}
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

func renderLoadedIndexedConfigPath(configPath ConfigPath, renderCtx renderContext) renderedPath {
	item := renderedPath{
		path:        cleanAndAbsPath(configPath.Path),
		probedFiles: configPath.ProbedFiles,
	}

	loadedFile := configPath.Path
	if file, ok := strings.CutPrefix(configPath.Status, "loaded: "); ok {
		loadedFile = file
	}

	idx := renderCtx.originIndices[configPath.Path]
	if idx == 0 {
		idx = renderCtx.originIndices[loadedFile]
	}

	item.badge = "[" + strconv.Itoa(idx) + "]"
	item.badgeStyled = styleText(item.badge, ansiDim, renderCtx.color)

	if renderCtx.color {
		item.bullet = styleText("●", ansiGreen, true) + " "
	}

	if cleanAndAbsPath(loadedFile) == item.path {
		item.status = styleText("(loaded)", ansiGreen, renderCtx.color)
	} else {
		item.status = styleText("(loaded: "+cleanAndAbsPath(loadedFile)+")", ansiGreen, renderCtx.color)
	}

	return item
}

func renderIndexedConfigPath(configPath ConfigPath, renderCtx renderContext) renderedPath {
	if strings.HasPrefix(configPath.Status, "loaded") {
		return renderLoadedIndexedConfigPath(configPath, renderCtx)
	}

	item := renderedPath{
		path:        cleanAndAbsPath(configPath.Path),
		probedFiles: configPath.ProbedFiles,
	}

	if configPath.Status == "error" {
		item.badge = "[!]"
		item.badgeStyled = styleText("[!]", ansiRed, renderCtx.color)

		if renderCtx.color {
			item.bullet = styleText("✖", ansiRed, true) + " "
		}

		item.status = styleText("(error)", ansiRed, renderCtx.color)

		return item
	}

	item.badge = "[—]"
	item.badgeStyled = styleText("[—]", ansiDim, renderCtx.color)

	if renderCtx.color {
		item.bullet = styleText("○", ansiDarkGray, true) + " "
	}

	item.status = styleText("("+configPath.Status+")", ansiDarkGray, renderCtx.color)

	return item
}

func renderPlainConfigPath(configPath ConfigPath, renderCtx renderContext) renderedPath {
	item := renderedPath{
		path:        cleanAndAbsPath(configPath.Path),
		probedFiles: configPath.ProbedFiles,
	}

	isLoaded := strings.HasPrefix(configPath.Status, "loaded")
	isError := configPath.Status == "error"

	if isLoaded {
		loadedFile := configPath.Path
		if file, ok := strings.CutPrefix(configPath.Status, "loaded: "); ok {
			loadedFile = file
		}

		if renderCtx.color {
			item.bullet = styleText("●", ansiGreen, true) + " "
		}

		if cleanAndAbsPath(loadedFile) == item.path {
			item.status = styleText("(loaded)", ansiGreen, renderCtx.color)
		} else {
			item.status = styleText("(loaded: "+cleanAndAbsPath(loadedFile)+")", ansiGreen, renderCtx.color)
		}

		return item
	}

	if isError {
		item.status = styleText("(error)", ansiRed, renderCtx.color)

		if renderCtx.color {
			item.bullet = styleText("✖", ansiRed, true) + " "
		}

		return item
	}

	item.status = styleText("("+configPath.Status+")", ansiDarkGray, renderCtx.color)

	if renderCtx.color {
		item.bullet = styleText("○", ansiDarkGray, true) + " "
	}

	return item
}

// renderConfigPaths lays out the candidate-path list beneath its header,
// annotating each path with its probe outcome.
func renderConfigPaths(paths []ConfigPath, optionalRC ...renderContext) string {
	var renderCtx renderContext
	if len(optionalRC) > 0 {
		renderCtx = optionalRC[0]
	}

	const (
		probeIndent = 4
		columnGap   = 3
	)

	rendered := make([]renderedPath, 0, len(paths))
	maxPathWidth := 0

	for _, configPath := range paths {
		var item renderedPath
		if renderCtx.useIndices {
			item = renderIndexedConfigPath(configPath, renderCtx)
		} else {
			item = renderPlainConfigPath(configPath, renderCtx)
		}

		maxPathWidth = max(maxPathWidth, utf8.RuneCountInString(item.path))
		rendered = append(rendered, item)
	}

	var builder strings.Builder

	for _, item := range rendered {
		builder.WriteString(strings.Repeat(" ", rowIndent))
		builder.WriteString(item.bullet)

		if item.badge != "" {
			builder.WriteString(item.badgeStyled)
			builder.WriteString(" ")
		}

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

func appendRowJSON(buffer []byte, row Row) []byte {
	buffer = append(buffer, `{"label":`...)
	buffer = jsonwire.AppendString(buffer, row.Label)
	buffer = append(buffer, `,"value":`...)
	buffer = jsonwire.AppendString(buffer, row.Value)
	buffer = append(buffer, `,"source":`...)
	buffer = jsonwire.AppendString(buffer, row.Source)
	buffer = append(buffer, `,"detail":`...)
	buffer = jsonwire.AppendString(buffer, row.Detail)
	buffer = append(buffer, `,"env":`...)
	buffer = jsonwire.AppendString(buffer, row.Env)
	buffer = append(buffer, `,"configPath":`...)
	buffer = jsonwire.AppendString(buffer, row.ConfigPath)
	buffer = append(buffer, `,"error":`...)
	buffer = jsonwire.AppendString(buffer, row.Error)

	buffer = append(buffer, `,"hidden":`...)
	if row.Hidden {
		buffer = append(buffer, "true"...)
	} else {
		buffer = append(buffer, "false"...)
	}

	buffer = append(buffer, `,"entries":[`...)

	for i, entry := range row.Entries {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = appendMapEntryJSON(buffer, entry)
	}

	buffer = append(buffer, "]}"...)

	return buffer
}

func appendMapEntryJSON(buffer []byte, entry MapEntry) []byte {
	buffer = append(buffer, `{"key":`...)
	buffer = jsonwire.AppendString(buffer, entry.Key)
	buffer = append(buffer, `,"value":`...)
	buffer = jsonwire.AppendString(buffer, entry.Value)
	buffer = append(buffer, `,"source":`...)
	buffer = jsonwire.AppendString(buffer, entry.Source)
	buffer = append(buffer, `,"detail":`...)
	buffer = jsonwire.AppendString(buffer, entry.Detail)
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
	buffer = append(buffer, `,"detail":`...)
	buffer = jsonwire.AppendString(buffer, feat.Detail)
	buffer = append(buffer, '}')

	return buffer
}

func appendConfigPathJSON(buffer []byte, configPath ConfigPath) []byte {
	buffer = append(buffer, `{"path":`...)
	buffer = jsonwire.AppendString(buffer, configPath.Path)
	buffer = append(buffer, `,"status":`...)
	buffer = jsonwire.AppendString(buffer, configPath.Status)
	buffer = append(buffer, `,"probedFiles":[`...)

	for i, probed := range configPath.ProbedFiles {
		if i > 0 {
			buffer = append(buffer, ',')
		}

		buffer = jsonwire.AppendString(buffer, probed)
	}

	buffer = append(buffer, "]}"...)

	return buffer
}

func cleanAndAbsPath(rawPath string) string {
	if rawPath == "" {
		return ""
	}

	suffix := ""
	basePath := rawPath

	if idx := strings.Index(rawPath, " ("); idx != -1 && strings.HasSuffix(rawPath, ")") {
		basePath = rawPath[:idx]
		suffix = rawPath[idx:]
	}

	if !strings.ContainsAny(basePath, `/\.`) {
		if _, err := os.Stat(basePath); err != nil {
			return rawPath
		}
	}

	cleaned := filepath.Clean(basePath)
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs + suffix
	}

	return cleaned + suffix
}
