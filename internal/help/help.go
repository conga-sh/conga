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

	"github.com/conga-sh/conga/internal/appname"
	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/cond"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
)

// Options carries optional feature renderers for feature-specific help
// screens. The zero value renders every screen without completion-specific
// sections: the completion subsystem is reachable only through renderers
// installed by the completion-enabled engine path.
type Options struct {
	// CompletionInstructions renders the per-shell enable instructions of a
	// completion command pinned to one shell (e.g. `--completion zsh --help`).
	CompletionInstructions func(shell string, useFlag bool, rootName string) string
	// CompletionFooter renders the supported-shells footer of the generic
	// completion screen.
	CompletionFooter func(cmdChain []*spec.Command, useFlag bool) string
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
// injectable for deterministic tests.
func RenderCompletionFooter(cmdChain []*spec.Command, useFlag bool, detect func() (completion.CurrentShell, bool)) string {
	cmdName := rootDisplayName(cmdChain)

	var builder strings.Builder

	entries := slicex.Map(completion.ShellDescriptions(), func(info completion.ShellInfo) tableEntry {
		return tableEntry{label: info.Name, help: info.Description}
	})

	builder.WriteString(renderTable("Supported shells", entries))

	if current, ok := detect(); ok {
		builder.WriteString(completion.NowSection(current, cmdName, useFlag))
	} else {
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

// Usage returns the standard 1-2 line usage syntax for the given command chain.
func Usage(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	cmdPath := resolveCmdPath(cmdChain)
	rootName := cmdPath[0]
	leaf := cmdChain[len(cmdChain)-1]

	if leaf.IsCompletionCmd && leaf.IsCompletionFlag {
		if leaf.Shell != "" {
			return "Usage:\n  " + rootName + " --completion " + leaf.Shell
		}

		return "Usage:\n  " + rootName + " --completion [shell]"
	}

	fullCmd := strings.Join(cmdPath, " ")

	var parts []string

	parts = append(parts, fullCmd)

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
		if arg.Name != "" {
			parts = append(parts, arg.Name)
		}
	}

	if leaf.DashArgs != nil {
		parts = append(parts, leaf.DashArgs.Name)
	}

	if leaf.RawArgs != nil {
		parts = append(parts, leaf.RawArgs.Name)
	}

	if hasOptionalFlags || hasOptionalGlobal {
		parts = append(parts, "[flags]")
	}

	return "Usage:\n  " + strings.Join(parts, " ")
}

// Hint returns the short hint pointing the user to --help.
func Hint(cmdChain []*spec.Command) string {
	if len(cmdChain) > 0 && cmdChain[len(cmdChain)-1].DisableHelp {
		return ""
	}

	cmdPath := resolveCmdPath(cmdChain)
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

	rootName := rootDisplayName(cmdChain)

	// 0. Root deprecation banner
	if cmdChain[0].Deprecated != "" {
		builder.WriteString("WARNING: ")
		builder.WriteString(rootName)
		builder.WriteString(" is deprecated: ")
		builder.WriteString(cmdChain[0].Deprecated)
		builder.WriteString("\n\n")
	}

	// 1. Description
	builder.WriteString(formatHelpDescription(leaf))

	// 2. Per-shell completion body: the enable instructions read as the
	// screen's documentation and precede usage (docker/cobra style).
	if leaf.IsCompletionCmd && leaf.Shell != "" && opts.CompletionInstructions != nil {
		builder.WriteString(opts.CompletionInstructions(leaf.Shell, leaf.IsCompletionFlag, rootName))
		builder.WriteString("\n")
	}

	// 3. Usage
	builder.WriteString(Usage(cmdChain))
	builder.WriteString("\n")

	// 4. Subcommands
	builder.WriteString(formatSubcommands(leaf.Subcommands))

	// 5. Positional Arguments
	// The per-shell completion screen omits Arguments: the shell is already
	// supplied, so restating the <shell> placeholder adds noise.
	if (!leaf.IsCompletionCmd || leaf.Shell == "") && !leaf.IsCompletionFlag {
		builder.WriteString(formatArgs(leaf.Args))
	}

	// 6. Completion-specific footer: supported shells list and enable hints,
	// dynamically generated for the caller's current shell. Only rendered on
	// the generic screen; the per-shell body handled it above.
	if leaf.IsCompletionCmd && leaf.Shell == "" && opts.CompletionFooter != nil {
		builder.WriteString(opts.CompletionFooter(cmdChain, leaf.IsCompletionFlag))
	}

	// 7. Flags
	// The built-in --why flag is offered on every command (like --help); the
	// built-in --version flag is root-only: version information describes the
	// whole application, so subcommand help screens omit it.
	visibleHelp := !leaf.DisableHelp
	visibleVersion := leaf.VersionFlagVisible(len(cmdChain) == 1)
	visibleWhy := leaf.WhyFlagVisible()
	builder.WriteString(formatFlags(leaf.Flags, visibleHelp, visibleVersion, leaf.CompletionFlagVisible(len(cmdChain) == 1), visibleWhy))

	// 8. Global Flags (inherited from ancestor commands)
	builder.WriteString(formatGlobalFlags(cmdChain))

	// 9. Examples
	builder.WriteString(formatExamples(leaf.Examples))

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
func appendUsageFlags(parts []string, flags []*spec.Flag) ([]string, bool) {
	hasOptionalFlags := false

	for _, flag := range flags {
		if flag.Hidden {
			continue
		}

		if flag.Name == "help" || flag.Name == "version" {
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
// flags and the built-in --help/--version flags are skipped, and built-in
// infrastructure leaves (completion, help, version) never surface ancestor
// flags because their operation never uses them.
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

			if flagSpec.Name == "help" || flagSpec.Name == "version" ||
				slices.Contains(flagSpec.Aliases, "help") || slices.Contains(flagSpec.Aliases, "version") {
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

	value := formatFlagValue(flagSpec)

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

func renderTable(title string, entries []tableEntry) string {
	if len(entries) == 0 {
		return ""
	}

	var builder strings.Builder

	builder.WriteByte('\n')
	builder.WriteString(title)
	builder.WriteString(":\n")

	maxLen := 0

	for _, entry := range entries {
		if len(entry.label) > maxLen {
			maxLen = len(entry.label)
		}
	}

	for _, entry := range entries {
		padding := strings.Repeat(" ", maxLen-len(entry.label)+2)
		fmt.Fprintf(&builder, "  %s%s%s\n", entry.label, padding, entry.help)
	}

	return builder.String()
}

func formatSubcommands(subcommands []*spec.Command) string {
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

	return renderSections(builder.sections())
}

// renderSections renders every section as its own titled table. An empty
// section list renders nothing; each title carries renderTable's leading
// blank line so sections stay visually separated.
func renderSections(sections []tableSection) string {
	var builder strings.Builder

	for _, section := range sections {
		builder.WriteString(renderTable(section.title, section.entries))
	}

	return builder.String()
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

	if hint := formatEnumHint(arg.Enum); hint != "" && !arg.HelpVars.Enum {
		helpText = appendNonEmpty(helpText, hint)
	}

	return arg.Name, helpText
}

func formatArgs(args []*spec.Arg) string {
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

	return renderTable("Arguments", entries)
}

// formatFlagLabel renders the flag label column. The value slot follows kong's
// FormatPlaceHolder precedence: an explicit placeholder tag wins, then the
// formatted default value (strings quoted, skipped when the help tag
// references {{default}}), then an informative metavar for well-known domain
// kinds, then the uppercase flag name. Multi-value flags (slices and maps) gain
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

	value := cond.SetOrDefaultFunc(flagSpec.Placeholder, func() string {
		return formatFlagValue(flagSpec)
	})

	if flagSpec.IsSlice || flagSpec.IsMap {
		value += ",..."
	}

	return label + "=" + value
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

	if flagSpec.Default != "" && !flagSpec.HelpVars.Default {
		// A negatable flag with a default (e.g. --[no-]cache default true) has
		// no value slot, so its default only lives in the help column. Plain
		// bools defaulting to false add noise without information.
		if flagSpec.IsNegatable || (!flagSpec.IsBool && !hasDefaultInLabel) {
			helpText = appendNonEmpty(helpText, fmt.Sprintf("[default: %s]", flagSpec.Default))
		}
	}

	if len(flagSpec.EnvVars) > 0 && !flagSpec.HelpVars.Env {
		envHint := fmt.Sprintf("[env: %s]", strings.Join(flagSpec.EnvVars, ", "))
		helpText = appendNonEmpty(helpText, envHint)
	}

	suppression := hintSuppression{
		rangeRef: flagSpec.HelpVars.Range,
		lenRef:   flagSpec.HelpVars.Len,
		itemsRef: flagSpec.HelpVars.Items,
	}

	if hint := formatConstraintHints(flagSpec.Range, flagSpec.Len, flagSpec.Items, suppression); hint != "" {
		helpText = appendNonEmpty(helpText, hint)
	}

	if hint := formatEnumHint(flagSpec.Enum); hint != "" && !flagSpec.HelpVars.Enum {
		helpText = appendNonEmpty(helpText, hint)
	}

	helpText = appendNonEmpty(helpText, deprecationMarker(flagSpec.Deprecated))

	return helpText
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

func formatFlags(flags []*spec.Flag, hasHelpInfo bool, hasVersionInfo bool, hasCompletionInfo bool, hasWhyInfo bool) string {
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

			helpEntries = append(helpEntries, builderEntry(flagSpec))

		case "version":
			hasVersion = true

			versionEntries = append(versionEntries, builderEntry(flagSpec))

		case "why":
			hasWhy = true

			whyEntries = append(whyEntries, builderEntry(flagSpec))

		default:
			builder.add(flagSpec.Group, builderEntry(flagSpec))
		}
	}

	whyEntries = appendBuiltinFallback(whyEntries, hasWhy, hasWhyInfo, builderEntry(spec.BuiltinWhyFlag))
	completionEntries = appendBuiltinFallback(completionEntries, hasCompletion, hasCompletionInfo, tableEntry{
		label: formatFlagLabel(spec.BuiltinCompletionFlag),
		help:  spec.BuiltinCompletionFlag.Help,
	})
	helpEntries = appendBuiltinFallback(helpEntries, hasHelp, hasHelpInfo, tableEntry{
		label: formatFlagLabel(&spec.Flag{Name: "help", IsBool: true}),
		help:  "Show help",
	})
	versionEntries = appendBuiltinFallback(versionEntries, hasVersion, hasVersionInfo, tableEntry{
		label: formatFlagLabel(&spec.Flag{Name: "version", IsBool: true}),
		help:  "Show version",
	})

	sections := builder.sections()

	// No user flags at all: the built-ins still deserve their "Flags" section.
	if len(sections) == 0 {
		sections = append(sections, tableSection{title: "Flags"})
	}

	// Built-in --why/--completion/--help/--version (or user-declared shadows
	// of them) always close the Flags area — regardless of grouping or
	// declaration order — so they stay easy to find at the very end. --why
	// renders before the closing --help/--version lines.
	builtinEntries := slices.Concat(whyEntries, completionEntries, helpEntries, versionEntries)
	last := &sections[len(sections)-1]
	last.entries = append(last.entries, builtinEntries...)

	return renderSections(sections)
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
// (chains longer than one command), excludes the built-in --help/--version
// flags (injected into the binder lookup map rather than cmd.Flags), and is
// suppressed entirely for built-in infrastructure commands (completion, help,
// version), whose operation never uses the root flags.
func formatGlobalFlags(cmdChain []*spec.Command) string {
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

			if flagSpec.Name == "help" || flagSpec.Name == "version" ||
				slices.Contains(flagSpec.Aliases, "help") || slices.Contains(flagSpec.Aliases, "version") {
				continue
			}

			builder.add(flagSpec.Group, builderEntry(flagSpec))
		}
	}

	return renderSections(builder.sections())
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
// lines and indenting every remaining line 2 spaces. It returns "" when there
// are no examples (or only blank lines), so the section is omitted entirely.
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

	var builder strings.Builder

	builder.WriteString("\nExamples:\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			builder.WriteString("\n")

			continue
		}

		builder.WriteString("  ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return builder.String()
}

// deprecationMarker returns the "[deprecated: <reason>]" suffix for a help
// column, or "" when the entity is not deprecated.
func deprecationMarker(reason string) string {
	if reason == "" {
		return ""
	}

	return "[deprecated: " + reason + "]"
}

func formatHelpDescription(leaf *spec.Command) string {
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

	return description + "\n\n"
}

// rootDisplayName returns the root command's declared name, overridden by the
// invoked binary name when the binary was installed or invoked under a
// different name.
func rootDisplayName(cmdChain []*spec.Command) string {
	if len(cmdChain) == 0 {
		return ""
	}

	rootName := cmdChain[0].Name
	if invoked, ok := appname.Binary(); ok {
		rootName = invoked
	}

	return rootName
}

func resolveCmdPath(cmdChain []*spec.Command) []string {
	if len(cmdChain) == 0 {
		return nil
	}

	rootName := rootDisplayName(cmdChain)

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
