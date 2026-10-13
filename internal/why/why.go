// Package why renders the neutral value-provenance report produced by the
// --why builtin: one row per flag and configuration-file container with its
// display value (already redacted or formatted by the caller), the source
// tier, and the env/config channels that can set the value. The package owns
// layout only; value formatting and redaction policy live in package conga,
// mirroring the internal/help split.
package why

import (
	"fmt"
	"strings"
	"unicode/utf8"
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

// Row is one neutral report line for a single flag or config-file container.
type Row struct {
	// Label is the flag display label (e.g. "--port").
	Label string
	// Value is the display value, redacted or formatted by the caller; empty
	// for unset values (the source column says "unset").
	Value string
	// Source is the bare source-tier word ("cli", "env", "config", "default",
	// "unset"). For a merged map flag this is the highest contributing tier.
	Source string
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
}

// Format renders the report as plain text following internal/help table
// conventions: a titled line, the aligned flag/value/source/env/config-path
// table for the active command, an inherited "Global Flags" table when
// ancestors contribute flags, per-row diagnostics, and the redaction footer
// hint.
func (r Report) Format() string {
	var builder strings.Builder

	path := r.CommandPath
	if path == "" {
		path = "app"
	}

	fmt.Fprintf(&builder, "Value provenance for %q:\n", path)

	builder.WriteString(renderRows(r.Rows))

	if len(r.GlobalRows) > 0 {
		builder.WriteString("\nGlobal Flags:\n")
		builder.WriteString(renderRows(r.GlobalRows))
	}

	if len(r.Features) > 0 {
		builder.WriteString("\nFeatures:\n")
		builder.WriteString(renderFeatureRows(r.Features))
	}

	if len(r.ConfigPaths) > 0 {
		if len(r.ConfigDecoders) > 0 {
			builder.WriteString("\nConfig decoders: ")
			builder.WriteString(strings.Join(r.ConfigDecoders, ", "))
			builder.WriteString("\n")
		}

		builder.WriteString("\nConfig paths checked:\n")
		builder.WriteString(renderConfigPaths(r.ConfigPaths))

		if len(r.ConfigExamples) > 0 {
			builder.WriteString("\nCreate a config file at one of:\n")
			builder.WriteString(renderConfigExamples(r.ConfigExamples))
		}
	}

	if r.Redacted {
		builder.WriteString("\n")
		builder.WriteString(FooterHint)
		builder.WriteString("\n")
	}

	return strings.TrimRight(builder.String(), "\n")
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

// renderRows lays out the aligned flag · value · source · env · config-path
// table beneath a header row. Error rows replace the source column with an
// "error: ..." diagnostic and stop there, so long messages never push the
// later columns out of alignment for the remaining rows.
func renderRows(rows []Row) string {
	header := []string{headerLabel, headerValue, headerSource, headerEnv, headerConfigPath}

	measurement := make([][]string, 0, len(rows)+1)
	measurement = append(measurement, header)

	for _, row := range rows {
		measurement = append(measurement, rowCells(row))

		for _, entry := range row.Entries {
			measurement = append(measurement, []string{"", entry.Key + "=" + entry.Value, entry.Source, "", ""})
		}
	}

	widths := measureColumnWidths(measurement)

	var builder strings.Builder

	if len(rows) > 0 {
		builder.WriteString("\n")
	}

	writeGridRow(&builder, header, widths)

	for _, row := range rows {
		cells := rowCells(row)

		if row.Error != "" {
			builder.WriteString(strings.Repeat(" ", rowIndent))
			writeTableCells(&builder, cells[:2], widths)
			builder.WriteString("error: ")
			builder.WriteString(row.Error)
			builder.WriteString("\n")

			continue
		}

		writeGridRow(&builder, cells, widths)

		for _, entry := range row.Entries {
			writeGridRow(&builder, []string{"", entry.Key + "=" + entry.Value, entry.Source}, widths)
		}
	}

	return builder.String()
}

// renderFeatureRows lays out the aligned feature-name · value · source table
// beneath a header row.
func renderFeatureRows(rows []FeatureRow) string {
	header := []string{"FEATURE", headerValue, headerSource}

	grid := make([][]string, 0, len(rows)+1)
	grid = append(grid, header)

	for _, row := range rows {
		grid = append(grid, []string{row.Name, row.Value, row.Source})
	}

	widths := measureColumnWidths(grid)

	var builder strings.Builder

	writeGridRow(&builder, header, widths)

	for index := range rows {
		writeGridRow(&builder, grid[index+1], widths)
	}

	return builder.String()
}

// writeGridRow writes one indented grid row: every cell but the last is padded
// to its measured width and gap-separated, and the final cell is written flush.
// Header and data rows share the layout.
func writeGridRow(builder *strings.Builder, cells []string, widths []int) {
	builder.WriteString(strings.Repeat(" ", rowIndent))
	writeTableCells(builder, cells[:len(cells)-1], widths)
	builder.WriteString(cells[len(cells)-1])
	builder.WriteString("\n")
}

// renderConfigPaths lays out the aligned candidate-path · status list beneath
// its header, appending one indented probed-file line per filename under
// tiers where nothing was found.
func renderConfigPaths(paths []ConfigPath) string {
	const (
		rowIndent   = 2
		probeIndent = 4
		columnGap   = 3
	)

	pathWidth := 0

	for _, configPath := range paths {
		pathWidth = max(pathWidth, utf8.RuneCountInString(configPath.Path))
	}

	var builder strings.Builder

	for _, configPath := range paths {
		builder.WriteString(strings.Repeat(" ", rowIndent))
		builder.WriteString(pad(configPath.Path, pathWidth))
		builder.WriteString(strings.Repeat(" ", columnGap))
		builder.WriteString(configPath.Status)
		builder.WriteString("\n")

		for _, probed := range configPath.ProbedFiles {
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
	const rowIndent = 2

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

// rowCells renders a row's table cells; an error row blanks the source column
// because the diagnostic replaces it.
func rowCells(row Row) []string {
	source := row.Source
	if row.Error != "" {
		source = ""
	}

	return []string{rowLabel(row), row.Value, source, row.Env, row.ConfigPath}
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

// writeTableCells writes each cell space-padded to its measured width and
// followed by the column gap. Callers write the final table column themselves,
// so trailing values never carry padding.
func writeTableCells(builder *strings.Builder, cells []string, widths []int) {
	for column, cell := range cells {
		builder.WriteString(pad(cell, widths[column]))
		builder.WriteString(strings.Repeat(" ", 3))
	}
}

func pad(cell string, width int) string {
	padding := width - utf8.RuneCountInString(cell)
	if padding <= 0 {
		return cell
	}

	return cell + strings.Repeat(" ", padding)
}
