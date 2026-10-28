// Package help renders command help, usage, version, and completion
// instructions.
package help

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/appname"
	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/termwidth"
)

// Options carries optional feature renderers for feature-specific help
// screens. The zero value renders every screen without completion-specific
// sections: the completion subsystem is reachable only through renderers
// installed by the completion-enabled engine path.
type Options struct {
	// Width is the column width help text wraps at. Zero resolves through
	// termwidth: a COLUMNS override when set, else the default width.
	Width int
	// CompletionInstructions renders the per-shell enable instructions of a
	// completion command pinned to one shell (e.g. `--completion zsh --help`).
	CompletionInstructions func(shell string, useFlag bool, rootName string) string
	// CompletionFooter renders the supported-shells footer of the generic
	// completion screen.
	CompletionFooter func(cmdChain []*spec.Command, useFlag bool, width int) string
	// FeaturesSection renders the automatic Features section of the root help
	// screen. It is reached exclusively through the feature-enabled engine
	// path, so applications that never enable features keep the feature help
	// payload unreferenced.
	FeaturesSection func(root *spec.Command, width int) string
}

// RenderCompletionInstructions renders the enable instructions body of a
// completion screen pinned to one shell. It is the completion-enabled
// implementation behind Options.CompletionInstructions.
func RenderCompletionInstructions(shell string, useFlag bool, rootName string) string {
	opts := completion.EnableOptions{
		Homebrew: completion.DetectHomebrew(),
		UseFlag:  useFlag,
	}

	if current, ok := completion.DetectCurrentShell(); ok {
		opts.BashNeedsUpgrade = completion.ShouldWarnBash(current)
	}

	return completion.EnableInstructions(completion.Shell(shell), rootName, opts)
}

// RenderCompletionFooter renders the generic completion screen footer: the
// supported-shells list plus the dynamically generated current-shell enable
// hints. It is the completion-enabled implementation behind
// Options.CompletionFooter; detect resolves the caller's shell and is
// injectable for deterministic tests, and width carries the wrap width
// (zero resolves through termwidth). The per-shell instructions pointer is
// suppressed when the completion command's help flag is disabled, since the
// referenced invocation would be a usage error. An empty command chain renders
// nothing.
func RenderCompletionFooter(cmdChain []*spec.Command, useFlag bool, detect func() (completion.CurrentShell, bool), width int) string {
	if len(cmdChain) == 0 {
		return ""
	}

	width = resolveWidth(width)

	cmdName := invokedName(cmdChain)
	helpAvailable := !cmdChain[len(cmdChain)-1].DisableHelp

	var builder strings.Builder

	entries := slicex.Map(completion.ShellDescriptions(), func(info completion.ShellInfo) tableEntry {
		return tableEntry{label: info.Name, help: info.Description}
	})

	builder.WriteString(renderTable("Supported shells", entries, width))

	if current, ok := detect(); ok {
		builder.WriteString(completion.NowSection(current, cmdName, useFlag, helpAvailable))
	} else if helpAvailable {
		builder.WriteString("\nTo enable completions for your shell, run:\n  ")
		builder.WriteString(cmdName)

		if useFlag {
			builder.WriteString(" --completion <shell> --help\n")
		} else {
			builder.WriteString(" completion <shell> --help\n")
		}
	}

	return builder.String()
}

// RenderFeatures renders the automatic Features section of the root help
// screen: one aligned row per feature toggle, in declaration order, carrying
// the feature's help text followed by its declared stage, integer levels,
// default, and bound environment variables. GA and deprecated features are
// omitted — they are no longer experimental — and the section renders nothing
// when no feature remains. Width carries the wrap width (zero resolves through
// termwidth). It is the feature-enabled implementation behind
// Options.FeaturesSection.
func RenderFeatures(root *spec.Command, width int) string {
	width = resolveWidth(width)

	entries := make([]tableEntry, 0, len(root.FeatureFields))

	for _, feature := range root.FeatureFields {
		if !feature.ListedInHelp() {
			continue
		}

		helpText := feature.Help

		if feature.Stage != "" {
			helpText = appendNonEmpty(helpText, "["+feature.Stage+"]")
		}

		if feature.IsInt {
			helpText = appendNonEmpty(helpText, "[levels: "+strings.Join(feature.Enum, ", ")+"]")
		}

		if feature.Default != "" {
			helpText = appendNonEmpty(helpText, "[default: "+feature.Default+"]")
		}

		// One environment surface has one display location: per-feature
		// variables are rendered here, next to the feature that binds them,
		// rather than aggregated into the container's {{env}} substitution.
		// env:"-" and unbound features carry an empty EnvVars list and so no
		// hint.
		if len(feature.EnvVars) > 0 {
			helpText = appendNonEmpty(helpText, "[env: "+strings.Join(feature.EnvVars, ", ")+"]")
		}

		entries = append(entries, tableEntry{label: feature.Name, help: helpText})
	}

	return renderTable("Features", entries, width)
}

// Usage returns the standard 1-2 line usage syntax for the given command chain.
func Usage(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	leaf := cmdChain[len(cmdChain)-1]

	rootName := screenName(cmdChain)
	cmdPath := resolveCmdPath(cmdChain, rootName)

	if leaf.IsCompletionCmd && leaf.IsCompletionFlag {
		if leaf.Shell != "" {
			return "Usage:\n  " + rootName + " --completion " + leaf.Shell
		}

		return "Usage:\n  " + rootName + " --completion [shell]"
	}

	var parts []string

	parts = append(parts, strings.Join(cmdPath, " "))

	// The per-shell completion screen pins the already-supplied shell instead
	// of restating the <shell> placeholder.
	if leaf.IsCompletionCmd && leaf.Shell != "" {
		parts = append(parts, leaf.Shell)

		return "Usage:\n  " + strings.Join(parts, " ")
	}

	if leaf.HasUserSubcommands() {
		parts = append(parts, "<command>")
	}

	globalRequired, hasOptionalGlobal := globalUsageFlags(cmdChain)

	for _, flagSpec := range globalRequired {
		parts = append(parts, formatUsageFlag(flagSpec))
	}

	parts, hasOptionalFlags := appendUsageFlags(parts, leaf.Flags)

	for _, arg := range leaf.Args {
		if token := formatArgToken(arg); token != "" {
			parts = append(parts, token)
		}
	}

	if leaf.DashArgs != nil {
		parts = append(parts, leaf.DashArgs.Name)
	}

	if leaf.RawArgs != nil {
		parts = append(parts, leaf.RawArgs.Name)
	}

	if hasOptionalFlags || hasOptionalGlobal {
		parts = append(parts, "[flags...]")
	}

	return "Usage:\n  " + strings.Join(parts, " ")
}

// Hint returns the short hint pointing the user to --help.
func Hint(cmdChain []*spec.Command) string {
	if len(cmdChain) > 0 && cmdChain[len(cmdChain)-1].DisableHelp {
		return ""
	}

	cmdPath := resolveCmdPath(cmdChain, screenName(cmdChain))
	if len(cmdPath) == 0 {
		return ""
	}

	return fmt.Sprintf("Run '%s --help' for more information.", strings.Join(cmdPath, " "))
}

// Format returns the complete, formatted help screen for the given command chain.
func Format(cmdChain []*spec.Command) string {
	return FormatWith(cmdChain, Options{})
}

// FormatWith returns the complete, formatted help screen for the given command
// chain, delegating feature-specific sections to the optional renderers in
// opts. The completion subsystem is reachable only through renderers installed
// by the completion-enabled engine path.
func FormatWith(cmdChain []*spec.Command, opts Options) string {
	if len(cmdChain) == 0 {
		return ""
	}

	leaf := cmdChain[len(cmdChain)-1]

	var builder strings.Builder

	rootName := displayName(cmdChain)
	width := resolveWidth(opts.Width)

	writeDeprecationBanners(&builder, cmdChain, rootName)

	// 1. Description
	builder.WriteString(formatHelpDescription(leaf, width))

	// 2. Per-shell completion body: the enable instructions read as the
	// screen's documentation and precede usage (docker/cobra style).
	if leaf.IsCompletionCmd && leaf.Shell != "" && opts.CompletionInstructions != nil {
		builder.WriteString(opts.CompletionInstructions(leaf.Shell, leaf.IsCompletionFlag, invokedName(cmdChain)))
		builder.WriteString("\n")
	}

	// 3. Usage
	builder.WriteString(Usage(cmdChain))
	builder.WriteString("\n")

	// 4. Examples
	builder.WriteString(formatExamples(leaf.Examples))

	// 5. Subcommands
	builder.WriteString(formatSubcommands(leaf.Subcommands, width))

	// 6. Positional Arguments
	// The per-shell completion screen omits Arguments: the shell is already
	// supplied, so restating the <shell> placeholder adds noise.
	if (!leaf.IsCompletionCmd || leaf.Shell == "") && !leaf.IsCompletionFlag {
		builder.WriteString(formatArgs(leaf.Args, width))
	}

	// 7. Completion-specific footer: supported shells list and enable hints,
	// dynamically generated for the caller's current shell. Only rendered on
	// the generic screen; the per-shell body handled it above.
	if leaf.IsCompletionCmd && leaf.Shell == "" && opts.CompletionFooter != nil {
		builder.WriteString(opts.CompletionFooter(cmdChain, leaf.IsCompletionFlag, width))
	}

	// 8. Flags
	// The built-in --why flag is offered on every command (like --help); the
	// built-in --version flag is root-only: version information describes the
	// whole application, so subcommand help screens omit it.
	visibleHelp := !leaf.DisableHelp
	visibleVersion := leaf.VersionFlagVisible(len(cmdChain) == 1)
	visibleWhy := leaf.WhyFlagVisible()
	builder.WriteString(formatFlags(
		leaf.Flags,
		visibleHelp,
		visibleVersion,
		leaf.CompletionFlagVisible(len(cmdChain) == 1),
		visibleWhy,
		cmdChain[0].WhyFormats(),
		cmdChain[0].HelpShort,
		cmdChain[0].VersionShort,
		width,
	))

	// 8.5 Features (root screen only)
	// The automatic Features section renders the root command's feature
	// toggles with their stage, levels, and defaults; it lists every visible
	// feature by name, so the synthesized feature-gates flag suppresses its
	// own [enum: ...] hint. Features are declared on the root only.
	if len(cmdChain) == 1 && opts.FeaturesSection != nil {
		builder.WriteString(opts.FeaturesSection(cmdChain[0], width))
	}

	// 9. Global Flags (inherited from ancestor commands)
	builder.WriteString(formatGlobalFlags(cmdChain, width))

	// 10. Global help footer
	if footer := strings.TrimSpace(cmdChain[0].HelpFooter); footer != "" {
		builder.WriteString("\n")
		builder.WriteString(footer)
	}

	return strings.TrimRight(builder.String(), "\n")
}

type tableEntry struct {
	label string
	help  string
}

// tableSection is one titled slice of table rows within a sectioned table.
type tableSection struct {
	title   string
	entries []tableEntry
}

// sectionBuilder accumulates table entries into titled sections: ungrouped
// entries collect under the default title, grouped entries under one section
// per group, ordered by the group's first appearance. Declaration order is
// preserved within every section.
type sectionBuilder struct {
	defaultTitle string
	defaults     []tableEntry
	grouped      []tableSection
	index        map[string]int
}

func newSectionBuilder(defaultTitle string) *sectionBuilder {
	return &sectionBuilder{defaultTitle: defaultTitle, index: make(map[string]int)}
}

func (b *sectionBuilder) add(group string, entry tableEntry) {
	if group == "" {
		b.defaults = append(b.defaults, entry)

		return
	}

	if index, ok := b.index[group]; ok {
		b.grouped[index].entries = append(b.grouped[index].entries, entry)

		return
	}

	b.index[group] = len(b.grouped)
	b.grouped = append(b.grouped, tableSection{title: group, entries: []tableEntry{entry}})
}

// sections returns the ordered section list: the default (ungrouped) section
// first when non-empty, then the group sections in first-appearance order.
func (b *sectionBuilder) sections() []tableSection {
	sections := make([]tableSection, 0, len(b.grouped)+1)
	if len(b.defaults) > 0 {
		sections = append(sections, tableSection{title: b.defaultTitle, entries: b.defaults})
	}

	return append(sections, b.grouped...)
}

// hintSuppression records which automatic constraint hints the field's help tag
// already states, mirroring the help-variable references on the schema field.
type hintSuppression struct {
	rangeRef, lenRef, itemsRef bool
}

// appendUsageFlags appends long-form tokens for required flags to the usage
// synopsis and reports whether any optional flags remain to be collapsed.
// Only the shared engine definitions are skipped: a user-declared shadow of
// --help/--version is a regular flag and contributes to the synopsis.
func appendUsageFlags(parts []string, flags []*spec.Flag) ([]string, bool) {
	hasOptionalFlags := false

	for _, flag := range flags {
		if flag.Hidden {
			continue
		}

		if isEngineBuiltinFlag(flag) {
			continue
		}

		if flag.Required {
			parts = append(parts, formatUsageFlag(flag))
		} else {
			hasOptionalFlags = true
		}
	}

	return parts, hasOptionalFlags
}

// globalUsageFlags scans the ancestor commands of the given chain and returns
// their visible, non-built-in flags: required flags to render explicitly in
// the usage synopsis (in chain order, before the leaf's own required flags)
// plus whether optional flags remain to be collapsed into the [flags]
// placeholder. It mirrors the visibility rules of formatGlobalFlags: hidden
// flags and the shared built-in --help/--version definitions are skipped,
// while user-declared shadows stay regular flags. Built-in infrastructure
// leaves (completion, help, version) never surface ancestor flags because
// their operation never uses them.
func globalUsageFlags(cmdChain []*spec.Command) ([]*spec.Flag, bool) {
	if len(cmdChain) <= 1 {
		return nil, false
	}

	leaf := cmdChain[len(cmdChain)-1]

	if leaf.IsCompletionCmd || leaf.IsHelpCmd || leaf.IsVersionCmd {
		return nil, false
	}

	var required []*spec.Flag

	hasOptional := false

	for _, cmd := range cmdChain[:len(cmdChain)-1] {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Hidden {
				continue
			}

			if isEngineBuiltinFlag(flagSpec) {
				continue
			}

			if flagSpec.Required {
				required = append(required, flagSpec)
			} else {
				hasOptional = true
			}
		}
	}

	return required, hasOptional
}

// formatUsageFlag renders a required flag's long-form token for the usage
// synopsis, mirroring the flag table's value slot so the two stay consistent.
func formatUsageFlag(flagSpec *spec.Flag) string {
	name := flagSpec.Name

	if flagSpec.IsNegatable {
		name = "[no-]" + name
	}

	if flagSpec.IsBool || flagSpec.IsCounter {
		return "--" + name
	}

	value := flagValueSlot(flagSpec)

	if flagSpec.IsSlice || flagSpec.IsMap {
		value += ",..."
	}

	return "--" + name + "=" + value
}

// kindMetavar returns the Go-kind metavar for anonymous primitive types, so
// unnamed values never fall back to echoing the flag name.
func kindMetavar(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return "STRING"
	case reflect.Bool:
		return "BOOL"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return "INT"
	case reflect.Float32, reflect.Float64:
		return "FLOAT"
	default:
		return "VALUE"
	}
}

// resolveWidth normalizes a configured wrap width: zero (or negative) follows
// the termwidth resolution — a COLUMNS override when set, else the default.
func resolveWidth(width int) int {
	if width > 0 {
		return width
	}

	return termwidth.Resolve(nil)
}

// renderTable renders one titled, column-aligned table, wrapping help cells to
// the available width. Labels are measured in runes so non-ASCII identifiers
// keep the help column aligned, and rows whose help cell is empty end at the
// label so the output carries no trailing whitespace. The help column never
// grows past half the width; a label too wide for it renders on its own line
// above its help text.
func renderTable(title string, entries []tableEntry, width int) string {
	if len(entries) == 0 {
		return ""
	}

	var builder strings.Builder

	builder.WriteByte('\n')
	builder.WriteString(title)
	builder.WriteString(":\n")

	maxLength := 0

	for _, entry := range entries {
		if entryWidth := utf8.RuneCountInString(entry.label); entryWidth > maxLength {
			maxLength = entryWidth
		}
	}

	helpColumn := 2 + maxLength + 2
	if maxColumn := width / 2; helpColumn > maxColumn {
		helpColumn = maxColumn
	}

	continuation := strings.Repeat(" ", helpColumn)

	for _, entry := range entries {
		if entry.help == "" {
			fmt.Fprintf(&builder, "  %s\n", entry.label)

			continue
		}

		if 2+utf8.RuneCountInString(entry.label)+2 > helpColumn {
			fmt.Fprintf(&builder, "  %s\n", entry.label)
			writeWrappedHelp(&builder, entry.help, continuation, width-helpColumn)

			continue
		}

		labelPadding := helpColumn - 2 - utf8.RuneCountInString(entry.label)
		lines := wrapLine(entry.help, width-helpColumn)

		fmt.Fprintf(&builder, "  %s%s%s\n", entry.label, strings.Repeat(" ", labelPadding), lines[0])

		for _, line := range lines[1:] {
			builder.WriteString(continuation)
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

// writeWrappedHelp writes one cell's help text wrapped to width, indenting
// every line. Blank lines stay blank so the output carries no trailing spaces.
func writeWrappedHelp(builder *strings.Builder, help string, indent string, width int) {
	for _, line := range wrapLine(help, width) {
		if line == "" {
			builder.WriteByte('\n')

			continue
		}

		builder.WriteString(indent)
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
}

// wrapLine folds one logical line at space boundaries so no emitted line
// exceeds width runes. A line that already fits is returned untouched, and an
// atom longer than width occupies its own line unbroken.
func wrapLine(line string, width int) []string {
	if utf8.RuneCountInString(line) <= width {
		return []string{line}
	}

	atoms := splitWrapAtoms(line)
	if len(atoms) == 0 {
		return []string{""}
	}

	var (
		lines   []string
		current string
	)

	for _, atom := range atoms {
		switch {
		case current == "":
			current = atom
		case utf8.RuneCountInString(current)+1+utf8.RuneCountInString(atom) <= width:
			current += " " + atom
		default:
			lines = append(lines, current)

			current = atom
		}
	}

	return append(lines, current)
}

// splitWrapAtoms splits one line into wrap atoms at space boundaries, keeping a
// balanced bracketed group (e.g. "[env: NAME]", "[default: true]") whole so its
// interior space never becomes a break point and the closing bracket is never
// orphaned onto its own line. A line whose brackets do not balance — author
// prose carrying a stray "[" — wraps on plain whitespace, as does a bracketless
// line.
func splitWrapAtoms(line string) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return nil
	}

	if bracketDepth(line) != 0 {
		return words
	}

	atoms := make([]string, 0, len(words))

	var (
		current string
		depth   int
	)

	for _, word := range words {
		if current == "" {
			current = word
		} else {
			current += " " + word
		}

		depth += bracketDepth(word)

		if depth == 0 {
			atoms = append(atoms, current)

			current = ""
		}
	}

	if current != "" {
		atoms = append(atoms, current)
	}

	return atoms
}

// bracketDepth returns the net bracket nesting contributed by one word, so
// splitWrapAtoms can tell whether a bracketed group is still open. Nested
// brackets (a "[default: [a, b]]" hint) balance correctly.
func bracketDepth(word string) int {
	depth := 0

	for _, char := range word {
		switch char {
		case '[':
			depth++
		case ']':
			depth--
		}
	}

	return depth
}

// wrapText wraps every author line of text to width, preserving explicit line
// breaks and blank lines.
func wrapText(text string, width int) string {
	lines := strings.Split(text, "\n")

	for index, line := range lines {
		lines[index] = strings.Join(wrapLine(line, width), "\n")
	}

	return strings.Join(lines, "\n")
}

func formatSubcommands(subcommands []*spec.Command, width int) string {
	builder := newSectionBuilder("Commands")

	for _, subcommand := range subcommands {
		if subcommand.Hidden {
			continue
		}

		builder.add(subcommand.Group, tableEntry{
			label: subcommand.Name,
			help:  appendNonEmpty(subcommand.Summary, deprecationMarker(subcommand.Deprecated)),
		})
	}

	return renderSections(builder.sections(), width)
}

// renderSections renders every section as its own titled table. An empty
// section list renders nothing; each title carries renderTable's leading
// blank line so sections stay visually separated.
func renderSections(sections []tableSection, width int) string {
	var builder strings.Builder

	for _, section := range sections {
		builder.WriteString(renderTable(section.title, section.entries, width))
	}

	return builder.String()
}

// formatArgToken renders a positional argument's synopsis token, deriving
// repeatability and optionality from the schema so the usage line and the
// Arguments: section never diverge. Author-supplied brackets are resolved
// first, so a slice or map argument gains its "..." repetition marker inside
// them (and inside a trailing placeholder angle bracket when present), and an
// optional argument gains a single bracket pair unless its name already carries
// one. Author-written "..." is preserved.
func formatArgToken(arg *spec.Arg) string {
	token := arg.Name
	if token == "" {
		return ""
	}

	bracketed := isBracketed(token)
	if bracketed {
		token = strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
	}

	if (arg.IsSlice || arg.IsMap) && !strings.Contains(token, "...") {
		if trimmed, ok := strings.CutSuffix(token, ">"); ok {
			token = trimmed + "...>"
		} else {
			token += "..."
		}
	}

	if bracketed || !arg.Required {
		return "[" + token + "]"
	}

	return token
}

func isBracketed(token string) bool {
	return strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]")
}

func formatArgLabel(arg *spec.Arg) (string, string) {
	helpText := arg.Help

	if helpText == "" {
		helpText = typeHelpFor(arg.TargetType, arg.FieldTags)
	}

	suppression := hintSuppression{
		rangeRef: arg.HelpVars.Range,
		lenRef:   arg.HelpVars.Len,
		itemsRef: arg.HelpVars.Items,
	}

	if hint := formatConstraintHints(arg.Range, arg.Len, arg.Items, suppression); hint != "" {
		helpText = appendNonEmpty(helpText, hint)
	}

	if hint := formatEnumHint(arg.Enum); hint != "" && !arg.HelpVars.Enum && !arg.EnumHintHidden {
		helpText = appendNonEmpty(helpText, hint)
	}

	return formatArgToken(arg), helpText
}

func formatArgs(args []*spec.Arg, width int) string {
	if len(args) == 0 {
		return ""
	}

	var entries []tableEntry

	for _, arg := range args {
		label, helpText := formatArgLabel(arg)
		entries = append(entries, tableEntry{
			label: label,
			help:  helpText,
		})
	}

	return renderTable("Arguments", entries, width)
}

// formatFlagLabel renders the flag label column. The value slot follows the
// shared flagValueSlot precedence and multi-value flags (slices and maps) gain
// a ",..." tail to signal repetition.
func formatFlagLabel(flagSpec *spec.Flag) string {
	flagPrefix := cond.If(flagSpec.IsNegatable, "--[no-]", "--")
	label := cond.If(flagSpec.Short != 0,
		fmt.Sprintf("-%c, %s%s", flagSpec.Short, flagPrefix, flagSpec.Name),
		"    "+flagPrefix+flagSpec.Name,
	)

	if flagSpec.IsNegatable {
		return label
	}

	if flagSpec.IsBool || flagSpec.IsCounter {
		return label
	}

	value := flagValueSlot(flagSpec)

	if flagSpec.IsSlice || flagSpec.IsMap {
		value += ",..."
	}

	return label + "=" + value
}

// flagValueSlot returns the flag's value slot with the help table's
// precedence: an explicit placeholder tag wins, then the formatted default
// value (strings quoted, skipped when the help tag references {{default}}),
// then the IsConfigFile PATH placeholder, then the metavar of the resolved
// type. The usage synopsis and the flag table share it so the two never
// diverge.
func flagValueSlot(flagSpec *spec.Flag) string {
	return cond.SetOrDefaultFunc(flagSpec.Placeholder, func() string {
		return formatFlagValue(flagSpec)
	})
}

// formatFlagValue returns the placeholder text for a flag's value slot. The
// metavar is always derived from the flag's type, never its name: an explicit
// placeholder tag wins, then the formatted default value (skipped when the
// help tag references {{default}}), then the smart-cased name of the resolved
// type, then the Go kind for anonymous primitives.
func formatFlagValue(flagSpec *spec.Flag) string {
	if flagSpec.IsMap {
		return "KEY=VALUE"
	}

	if flagSpec.Default != "" && flagSpec.Handle != nil && !flagSpec.HelpVars.Default {
		if flagSpec.TargetType != nil && flagSpec.TargetType.Kind() == reflect.String {
			return strconv.Quote(flagSpec.Default)
		}

		return flagSpec.Default
	}

	if flagSpec.IsConfigFile {
		return "PATH"
	}

	return metavarForType(flagSpec.TargetType)
}

// metavarForType returns the metavar for a flag's resolved value type,
// unwrapping pointers and slices so the metavar describes the repeated value.
func metavarForType(targetType reflect.Type) string {
	if targetType == nil {
		return "VALUE"
	}

	for targetType.Kind() == reflect.Pointer || targetType.Kind() == reflect.Slice {
		targetType = targetType.Elem()
	}

	if name := targetType.Name(); name != "" {
		return casing.UppercaseWords(name)
	}

	return kindMetavar(targetType.Kind())
}

// typeHelpFor returns the default help text contributed by a flag's value
// type. Custom domain types may implement Help(FieldTags) to self-describe
// their accepted value format, tailored to the field's declared tags (e.g.
// ByteSize's suffix grammar, File's existing/stdin capabilities). Built-in
// stdlib types with a notable input grammar (time.Duration) are handled
// explicitly. Pointer and slice wrappers are unwrapped so the repeated
// value's help is used.
func typeHelpFor(targetType reflect.Type, tags spec.FieldTags) string {
	if targetType == nil {
		return ""
	}

	for targetType.Kind() == reflect.Pointer || targetType.Kind() == reflect.Slice {
		targetType = targetType.Elem()
	}

	if targetType == reflect.TypeFor[time.Duration]() {
		return "Duration (e.g., 300ms, 2m45s, 1h)"
	}

	return spec.DefaultTypeHelp(targetType, tags)
}

func formatFlagHelpText(flagSpec *spec.Flag) string {
	helpText := flagSpec.Help

	if helpText == "" {
		helpText = typeHelpFor(flagSpec.TargetType, flagSpec.FieldTags)
	}

	hasDefaultInLabel := flagSpec.Placeholder == "" && flagSpec.Default != "" && flagSpec.Handle != nil

	if flagSpec.Default != "" && !flagSpec.HelpVars.Default && showsDefaultHint(flagSpec, hasDefaultInLabel) {
		helpText = appendNonEmpty(helpText, fmt.Sprintf("[default: %s]", flagSpec.Default))
	}

	if len(flagSpec.EnvVars) > 0 && !flagSpec.HelpVars.Env {
		helpText = appendNonEmpty(helpText, fmt.Sprintf("[env: %s]", strings.Join(flagSpec.EnvVars, ", ")))
	}

	suppression := hintSuppression{
		rangeRef: flagSpec.HelpVars.Range,
		lenRef:   flagSpec.HelpVars.Len,
		itemsRef: flagSpec.HelpVars.Items,
	}

	if hint := formatConstraintHints(flagSpec.Range, flagSpec.Len, flagSpec.Items, suppression); hint != "" {
		helpText = appendNonEmpty(helpText, hint)
	}

	if hint := formatEnumHint(flagSpec.Enum); hint != "" && !flagSpec.HelpVars.Enum && !flagSpec.EnumHintHidden {
		helpText = appendNonEmpty(helpText, hint)
	}

	helpText = appendNonEmpty(helpText, deprecationMarker(flagSpec.Deprecated))

	return helpText
}

// showsDefaultHint reports whether a flag's default needs the help-column
// [default: ...] hint. A negatable flag has no value slot, so its default only
// lives in the help column. A non-bool flag needs the hint only when its value
// slot does not already carry the default. A plain bool needs it only when its
// declared default is true: the false zero value adds noise without
// information.
func showsDefaultHint(flagSpec *spec.Flag, hasDefaultInLabel bool) bool {
	if flagSpec.IsNegatable {
		return true
	}

	if !flagSpec.IsBool {
		return !hasDefaultInLabel
	}

	enabled, err := strconv.ParseBool(flagSpec.Default)

	return err == nil && enabled
}

// builtinFlagKind classifies a flag as a user-declared shadow of a built-in
// completion/help/version/why flag by name or alias, or "" for regular user flags.
func builtinFlagKind(flagSpec *spec.Flag) string {
	switch {
	case flagSpec.Name == "completion" || slices.Contains(flagSpec.Aliases, "completion"):
		return "completion"

	case flagSpec.Name == "help" || slices.Contains(flagSpec.Aliases, "help"):
		return "help"

	case flagSpec.Name == "version" || slices.Contains(flagSpec.Aliases, "version"):
		return "version"

	case flagSpec.Name == "why" || slices.Contains(flagSpec.Aliases, "why"):
		return "why"

	default:
		return ""
	}
}

// appendBuiltinFallback appends the shared builtin row when the builtin is
// offered and not shadowed by a user-declared flag of the same name.
func appendBuiltinFallback(entries []tableEntry, shadowed bool, offered bool, entry tableEntry) []tableEntry {
	if shadowed || !offered {
		return entries
	}

	return append(entries, entry)
}

// isEngineBuiltinFlag reports whether a flag row is one of the shared engine
// built-in definitions (--help/--version) rather than a user-declared flag
// that merely carries the same name or alias. Only engine rows are excluded
// from usage synopses and Global Flags sections: a user shadow — sanctioned
// after DisableHelpFlag/DisableVersionFlag — is a regular flag and stays
// visible.
func isEngineBuiltinFlag(flagSpec *spec.Flag) bool {
	return flagSpec == spec.BuiltinHelpFlag || flagSpec == spec.BuiltinVersionFlag
}

// withBuiltinShort returns flagSpec with the configured built-in short rune
// applied when it carries none, so a shared built-in flag row (the completion
// subcommand's --help, for example) renders the same short as the synthesized
// fallback rows. A user-declared shadow never inherits the rune: the binder
// registers it on the injected built-in alone, so advertising it on a shadow
// would promise a short the parser rejects.
func withBuiltinShort(flagSpec *spec.Flag, short rune) *spec.Flag {
	if short == 0 || flagSpec.Short != 0 || !inheritsBuiltinShort(flagSpec) {
		return flagSpec
	}

	withShort := *flagSpec
	withShort.Short = short

	return &withShort
}

// inheritsBuiltinShort reports whether a flag row may advertise the configured
// built-in rune: only engine-built-in flags — the shared --help/--version
// definitions, or another engine-synthesized flag.
func inheritsBuiltinShort(flagSpec *spec.Flag) bool {
	return flagSpec.EngineOwned || flagSpec == spec.BuiltinHelpFlag || flagSpec == spec.BuiltinVersionFlag
}

func formatFlags(flags []*spec.Flag, hasHelpInfo bool, hasVersionInfo bool, hasCompletionInfo bool, hasWhyInfo bool, whyFormats []string, helpShort rune, versionShort rune, width int) string {
	var (
		completionEntries []tableEntry
		helpEntries       []tableEntry
		versionEntries    []tableEntry
		whyEntries        []tableEntry
		hasHelp           bool
		hasVersion        bool
		hasCompletion     bool
		hasWhy            bool
	)

	builder := newSectionBuilder("Flags")

	for _, flagSpec := range flags {
		if flagSpec.Hidden {
			continue
		}

		switch builtinFlagKind(flagSpec) {
		case "completion":
			hasCompletion = true

			completionEntries = append(completionEntries, builderEntry(flagSpec))

		case "help":
			hasHelp = true

			helpEntries = append(helpEntries, builderEntry(withBuiltinShort(flagSpec, helpShort)))

		case "version":
			hasVersion = true

			versionEntries = append(versionEntries, builderEntry(withBuiltinShort(flagSpec, versionShort)))

		case "why":
			hasWhy = true

			whyEntries = append(whyEntries, builderEntry(flagSpec))

		default:
			builder.add(flagSpec.Group, builderEntry(flagSpec))
		}
	}

	whyEntries = appendBuiltinFallback(whyEntries, hasWhy, hasWhyInfo, builtinWhyEntry(whyFormats))
	completionEntries = appendBuiltinFallback(completionEntries, hasCompletion, hasCompletionInfo, tableEntry{
		label: formatFlagLabel(spec.BuiltinCompletionFlag),
		help:  spec.BuiltinCompletionFlag.Help,
	})
	helpEntries = appendBuiltinFallback(helpEntries, hasHelp, hasHelpInfo, tableEntry{
		label: formatFlagLabel(&spec.Flag{Name: "help", IsBool: true, Short: helpShort}),
		help:  spec.DefaultHelpFlagHelp,
	})
	versionEntries = appendBuiltinFallback(versionEntries, hasVersion, hasVersionInfo, tableEntry{
		label: formatFlagLabel(&spec.Flag{Name: "version", IsBool: true, Short: versionShort}),
		help:  spec.DefaultVersionFlagHelp,
	})

	builtinEntries := slices.Concat(whyEntries, completionEntries, helpEntries, versionEntries)

	// Built-in --why/--completion/--help/--version (or user-declared shadows
	// of them) close the ungrouped Flags section — regardless of declaration
	// order — so they never inherit a user group's heading. When every flag
	// carries a group, they close the screen under a trailing "Flags"
	// heading of their own.
	if len(builtinEntries) > 0 {
		if len(builder.defaults) > 0 || len(builder.grouped) == 0 {
			builder.defaults = append(builder.defaults, builtinEntries...)
		} else {
			builder.grouped = append(builder.grouped, tableSection{title: "Flags", entries: builtinEntries})
		}
	}

	return renderSections(builder.sections(), width)
}

// builtinWhyEntry renders the built-in --why flag row. The optional value
// slot and the format list are advertised only when more than the default
// text format is accepted (app.EnableWhy(conga.WhyOptionJSON())); otherwise --why
// presents as a plain toggle and the text format stays implicit.
func builtinWhyEntry(whyFormats []string) tableEntry {
	if len(whyFormats) < 2 {
		return builderEntry(spec.BuiltinWhyFlag)
	}

	return tableEntry{
		label: formatFlagLabel(spec.BuiltinWhyFlag) + "[=FORMAT]",
		help:  appendNonEmpty(spec.DefaultWhyFlagHelp, formatEnumHint(whyFormats)),
	}
}

// builderEntry renders one flag's table row.
func builderEntry(flagSpec *spec.Flag) tableEntry {
	return tableEntry{
		label: formatFlagLabel(flagSpec),
		help:  formatFlagHelpText(flagSpec),
	}
}

// formatGlobalFlags renders the "Global Flags" section listing non-hidden flags
// inherited from ancestor commands. It only appears on subcommand help screens
// (chains longer than one command), excludes the shared built-in --help/--version
// definitions (injected into the binder lookup map rather than cmd.Flags, while
// user-declared shadows stay regular flags), and is suppressed entirely for
// built-in infrastructure commands (completion, help, version), whose operation
// never uses the root flags.
func formatGlobalFlags(cmdChain []*spec.Command, width int) string {
	if len(cmdChain) <= 1 {
		return ""
	}

	leaf := cmdChain[len(cmdChain)-1]

	if leaf.IsCompletionCmd || leaf.IsHelpCmd || leaf.IsVersionCmd {
		return ""
	}

	builder := newSectionBuilder("Global Flags")

	for _, cmd := range cmdChain[:len(cmdChain)-1] {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Hidden {
				continue
			}

			if isEngineBuiltinFlag(flagSpec) {
				continue
			}

			builder.add(flagSpec.Group, builderEntry(flagSpec))
		}
	}

	return renderSections(builder.sections(), width)
}

func formatConstraintHints(valueRange, lengthRange, itemsRange *spec.Range, suppression hintSuppression) string {
	var hints []string

	if valueRange != nil && !suppression.rangeRef {
		if h := formatRangeHint(valueRange, ""); h != "" {
			hints = append(hints, h)
		}
	}

	if lengthRange != nil && !suppression.lenRef {
		if h := formatRangeHint(lengthRange, "len: "); h != "" {
			hints = append(hints, h)
		}
	}

	if itemsRange != nil && !suppression.itemsRef {
		if h := formatRangeHint(itemsRange, "items: "); h != "" {
			hints = append(hints, h)
		}
	}

	return strings.Join(hints, " ")
}

func formatRangeHint(rangeSpec *spec.Range, prefix string) string {
	text := rangeSpec.HintText()
	if text == "" {
		return ""
	}

	return "[" + prefix + text + "]"
}

// formatEnumHint renders a flag's or argument's allowed values as the
// "[enum: a, b]" hint, or "" when no values are declared.
func formatEnumHint(enumValues []string) string {
	if len(enumValues) == 0 {
		return ""
	}

	return "[enum: " + strings.Join(enumValues, ", ") + "]"
}

func appendNonEmpty(base, suffix string) string {
	if base == "" {
		return suffix
	}

	if suffix == "" {
		return base
	}

	return base + " " + suffix
}

// formatExamples renders the Examples: section, trimming leading/trailing blank
// lines, stripping the block's common source indentation, and indenting every
// remaining line 2 spaces. It returns "" when there are no examples (or only
// blank lines), so the section is omitted entirely.
func formatExamples(examples string) string {
	lines := strings.Split(examples, "\n")

	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return ""
	}

	indent := commonIndent(lines)

	var builder strings.Builder

	builder.WriteString("\nExamples:\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			builder.WriteString("\n")

			continue
		}

		builder.WriteString("  ")
		builder.WriteString(line[indent:])
		builder.WriteString("\n")
	}

	return builder.String()
}

// commonIndent returns the width of the longest common leading-whitespace prefix
// shared by every non-blank line, so author source-indentation is stripped
// before the renderer applies its own two-space indent. The width counts bytes,
// so a block mixing tabs and spaces keeps the remainder of a longer indent.
func commonIndent(lines []string) int {
	indent := -1

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		width := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == -1 || width < indent {
			indent = width
		}

		if indent == 0 {
			return 0
		}
	}

	if indent < 0 {
		return 0
	}

	return indent
}

// deprecationMarker returns the "[deprecated: <reason>]" suffix for a help
// column, or "" when the entity is not deprecated.
func deprecationMarker(reason string) string {
	if reason == "" {
		return ""
	}

	return "[deprecated: " + reason + "]"
}

// writeDeprecationBanners writes the step-0 deprecation warnings for the
// command chain: the root keeps its historic form, while a deprecated command
// carries the runtime warning's wording so the help screen and the execution
// warning agree.
func writeDeprecationBanners(builder *strings.Builder, cmdChain []*spec.Command, rootName string) {
	if cmdChain[0].Deprecated != "" {
		appendDeprecationBanner(builder, rootName, cmdChain[0].Deprecated)
	}

	for index, deprecatedCmd := range cmdChain[1:] {
		if deprecatedCmd.Deprecated == "" {
			continue
		}

		cmdPath := strings.Join(resolveCmdPath(cmdChain[:index+2], rootName), " ")

		appendDeprecationBanner(builder, "command '"+cmdPath+"'", deprecatedCmd.Deprecated)
	}
}

// appendDeprecationBanner writes one deprecation warning line followed by the
// blank separator that keeps banners above the screen content.
func appendDeprecationBanner(builder *strings.Builder, subject string, reason string) {
	builder.WriteString("WARNING: ")
	builder.WriteString(subject)
	builder.WriteString(" is deprecated: ")
	builder.WriteString(reason)
	builder.WriteString("\n\n")
}

// formatHelpDescription renders the command's description paragraph, wrapping
// overlong lines to width while preserving the author's explicit line breaks.
func formatHelpDescription(leaf *spec.Command, width int) string {
	description := strings.TrimSpace(leaf.Description)
	if description == "" {
		description = strings.TrimSpace(leaf.Summary)
	}

	if description == "" {
		return ""
	}

	if leaf.IsCompletionCmd && leaf.Shell != "" {
		description = "Generate the shell completion script for the " + leaf.Shell + " shell."
	}

	return wrapText(description, width) + "\n\n"
}

// displayName returns the root command's configured name: the application name
// set with app.Name, or the invoked binary name it defaulted to when no name
// was configured. Help, usage, hints, and the deprecation banner use it so an
// explicitly configured name always wins over the binary.
func displayName(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	return cmdChain[0].Name
}

// screenName returns the root name a screen renders: completion screens
// describe enabling completion for the concrete binary, so they keep the
// invoked binary name; every other screen shows the configured application
// name.
func screenName(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	if cmdChain[len(cmdChain)-1].IsCompletionCmd {
		return invokedName(cmdChain)
	}

	return displayName(cmdChain)
}

// invokedName returns the root command's name, overridden by the invoked binary
// name. Completion screens use it because their instructions and generated
// script must reference the binary the user actually runs.
func invokedName(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	rootName := cmdChain[0].Name
	if invoked, ok := appname.Binary(); ok {
		rootName = invoked
	}

	return rootName
}

func resolveCmdPath(cmdChain []*spec.Command, rootName string) []string {
	if len(cmdChain) == 0 {
		return nil
	}

	var cmdPath []string

	for i, c := range cmdChain {
		name := c.Name
		if i == 0 {
			name = rootName
		}

		if name != "" {
			cmdPath = append(cmdPath, name)
		}
	}

	return cmdPath
}
