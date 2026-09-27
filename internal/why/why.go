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

// Row is one neutral report line for a single flag or config-file container.
type Row struct {
	// Label is the flag display label (e.g. "--port").
	Label string
	// Value is the display value, redacted or formatted by the caller; empty
	// for unset values (the source column says "unset").
	Value string
	// Source is the bare source-tier word ("cli", "env", "config", "default",
	// "unset").
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

	if len(r.ConfigPaths) > 0 {
		if len(r.ConfigDecoders) > 0 {
			builder.WriteString("\nConfig decoders: ")
			builder.WriteString(strings.Join(r.ConfigDecoders, ", "))
			builder.WriteString("\n")
		}

		builder.WriteString("\nConfig paths checked:\n")
		builder.WriteString(renderConfigPaths(r.ConfigPaths))

		if len(r.ConfigExamples) > 0 {
			builder.WriteString("\nCreate a configuration file at one of:\n")
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
	const (
		rowIndent = 2
		columnGap = 3
	)

	var (
		labelWidth      = utf8.RuneCountInString(headerLabel)
		valueWidth      = utf8.RuneCountInString(headerValue)
		sourceWidth     = utf8.RuneCountInString(headerSource)
		envWidth        = utf8.RuneCountInString(headerEnv)
		configPathWidth = utf8.RuneCountInString(headerConfigPath)
	)

	for _, row := range rows {
		label := rowLabel(row)

		labelWidth = max(labelWidth, utf8.RuneCountInString(label))
		valueWidth = max(valueWidth, utf8.RuneCountInString(row.Value))
		envWidth = max(envWidth, utf8.RuneCountInString(row.Env))
		configPathWidth = max(configPathWidth, utf8.RuneCountInString(row.ConfigPath))

		if row.Error == "" {
			sourceWidth = max(sourceWidth, utf8.RuneCountInString(row.Source))
		}
	}

	var builder strings.Builder

	if len(rows) > 0 {
		builder.WriteString("\n")
	}

	builder.WriteString(strings.Repeat(" ", rowIndent))
	writeCell(&builder, headerLabel, labelWidth)
	writeCell(&builder, headerValue, valueWidth)
	writeCell(&builder, headerSource, sourceWidth)
	writeCell(&builder, headerEnv, envWidth)
	builder.WriteString(headerConfigPath)
	builder.WriteString("\n")

	for _, row := range rows {
		builder.WriteString(strings.Repeat(" ", rowIndent))
		writeCell(&builder, rowLabel(row), labelWidth)
		writeCell(&builder, row.Value, valueWidth)

		if row.Error != "" {
			builder.WriteString("error: ")
			builder.WriteString(row.Error)
			builder.WriteString("\n")

			continue
		}

		writeCell(&builder, row.Source, sourceWidth)
		writeCell(&builder, row.Env, envWidth)
		builder.WriteString(row.ConfigPath)
		builder.WriteString("\n")
	}

	return builder.String()
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

// writeCell writes a space-padded table cell followed by the column gap.
func writeCell(builder *strings.Builder, cell string, width int) {
	builder.WriteString(pad(cell, width))
	builder.WriteString(strings.Repeat(" ", 3))
}

func pad(cell string, width int) string {
	padding := width - utf8.RuneCountInString(cell)
	if padding <= 0 {
		return cell
	}

	return cell + strings.Repeat(" ", padding)
}
