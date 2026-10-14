// Package inspector renders the resolved configuration state of a CONGA
// command chain: every flag and argument of every active command from root
// down to the leaf, with its resolved value and origin.
package inspector

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/stringx"
)

// ANSI color escape codes.
const (
	colorReset     = "\033[0m"
	colorBold      = "\033[1m"
	colorDim       = "\033[2m"
	colorCyan      = "\033[36m"
	colorDarkGray  = "\033[90m"
	colorBoldCyan  = "\033[1;36m"
	colorBoldGreen = "\033[1;32m"
	colorBoldYel   = "\033[1;33m"
)

// Layout metrics shared by all rendering helpers.
const (
	sectionIndent = 2
	rowIndent     = 4
	columnGap     = 3
	unsetMarker   = "—"
)

// Row holds the extracted metadata for a single configuration field.
type Row struct {
	DisplayName  string
	TypeName     string
	ValueStr     string
	Source       conga.ValueSource
	SourceDetail string
	IsSet        bool
	Kind         string // "flag", "arg", "dash", "raw", "subcmd", "builtin"
}

// Inspect extracts rows of configuration metadata from a target struct pointer.
func Inspect(target any) []Row {
	value := structValueOf(target)
	if !value.IsValid() {
		return nil
	}

	structType := value.Type()
	rows := make([]Row, 0, structType.NumField())

	for i := range structType.NumField() {
		structField := structType.Field(i)
		if !structField.IsExported() {
			continue
		}

		fieldAddr := value.Field(i).Addr().Interface()

		row := extractFieldRow(structField, fieldAddr)
		if row != nil {
			rows = append(rows, *row)
		}
	}

	return rows
}

// Print renders the configuration of the provided commands to the session-configured stdout.
func Print(session *conga.Session, commands ...any) {
	var writer io.Writer
	if session != nil {
		writer = session.Stdout()
	}

	Fprint(session, writer, commands...)
}

// Fprint renders the configuration of the provided commands to the given writer.
func Fprint(session *conga.Session, writer io.Writer, commands ...any) {
	sections := collectSections(session, commands)
	if len(sections) == 0 {
		return
	}

	if writer == nil {
		if session != nil {
			writer = session.Stdout()
		} else {
			writer = os.Stdout
		}
	}

	commandPath := ""
	if session != nil {
		commandPath = session.Path()
	}

	if commandPath == "" {
		commandPath = "app"
	}

	useColor := isColorEnabled(writer)
	widths := measureColumns(sections)

	renderTitle(writer, commandPath, widths, useColor)
	renderColumnHeader(writer, widths, useColor)

	for _, sectionItem := range sections {
		if len(sections) > 1 {
			renderSectionHeader(writer, sectionItem, useColor)
		}

		for _, row := range sectionItem.rows {
			renderRow(writer, row, widths, useColor)
		}
	}
}

type section struct {
	name string
	role string
	rows []Row
}

type columnWidths struct {
	fieldWidth  int
	typeWidth   int
	valueWidth  int
	sourceWidth int
}

// commandContainer is the method set of conga.Command[T] needed for hierarchy
// discovery, asserted statically instead of reflect.MethodByName dispatch
// (which defeats the linker's dead-code elimination and bloats the binary).
type commandContainer interface {
	IsSelected() bool
}

// stateContainer is the shared method set of conga's flag and argument
// containers, asserted statically for the same reason.
type stateContainer interface {
	IsSet() bool
	Source() conga.ValueSource
	SourceDetail() string
}

func collectSections(session *conga.Session, commands []any) []section {
	var chainPath string
	if session != nil {
		chainPath = session.Path()
	}

	chainNames := strings.Fields(chainPath)
	if len(chainNames) == 0 {
		chainNames = []string{"app"}
	}

	sections := make([]section, 0, len(commands))
	for index, command := range commands {
		if command == nil {
			continue
		}

		value := structValueOf(command)
		if !value.IsValid() {
			continue
		}

		name := resolveSectionName(session, commands, index, chainNames)

		sections = append(sections, section{
			name: name,
			rows: Inspect(command),
		})
	}

	if len(sections) > 1 {
		sections[0].role = "root"
		sections[len(sections)-1].role = "active"
	}

	return sections
}

func resolveSectionName(session *conga.Session, commands []any, index int, chainNames []string) string {
	if session != nil && len(chainNames) > 0 {
		if len(commands) == 1 {
			return chainNames[len(chainNames)-1]
		}

		if index == 0 {
			return chainNames[0]
		}

		if index == len(commands)-1 {
			return chainNames[len(chainNames)-1]
		}

		if index < len(chainNames) {
			return chainNames[index]
		}
	}

	if index > 0 && index < len(commands) {
		if discovered := findSubcommandFieldName(commands[index-1], commands[index]); discovered != "" {
			return discovered
		}
	}

	if index < len(chainNames) {
		return chainNames[index]
	}

	return "app"
}

func findSubcommandFieldName(parent, child any) string {
	parentVal := structValueOf(parent)

	childVal := structValueOf(child)
	if !parentVal.IsValid() || !childVal.IsValid() {
		return ""
	}

	childType := childVal.Type()
	parentType := parentVal.Type()

	for field := range parentType.Fields() {
		if strings.Contains(field.Type.String(), childType.Name()) {
			return subcommandName(field)
		}
	}

	return ""
}

func structValueOf(target any) reflect.Value {
	value := reflect.ValueOf(target)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	if !value.CanAddr() {
		clone := reflect.New(value.Type())
		clone.Elem().Set(value)
		value = clone.Elem()
	}

	return value
}

func renderTitle(writer io.Writer, commandPath string, widths columnWidths, useColor bool) {
	title := "⚙  " + commandPath
	innerWidth := max(headerWidth(widths), sectionIndent+utf8.RuneCountInString(title))

	fmt.Fprintf(writer, "╭%s╮\n", styled(strings.Repeat("─", innerWidth), colorDim, useColor))

	titleText := strings.Repeat(" ", sectionIndent) + title
	titlePadding := strings.Repeat(" ", innerWidth-utf8.RuneCountInString(titleText))
	fmt.Fprintf(writer, "%s%s%s%s\n",
		styled("│", colorDim, useColor),
		styled(titleText, colorBoldCyan, useColor),
		titlePadding,
		styled("│", colorDim, useColor),
	)

	fmt.Fprintf(writer, "╰%s╯\n", styled(strings.Repeat("─", innerWidth), colorDim, useColor))
}

func renderColumnHeader(writer io.Writer, widths columnWidths, useColor bool) {
	header := pad("FIELD", widths.fieldWidth) + strings.Repeat(" ", columnGap) +
		pad("TYPE", widths.typeWidth) + strings.Repeat(" ", columnGap) +
		pad("VALUE", widths.valueWidth) + strings.Repeat(" ", columnGap) +
		"SOURCE"

	fmt.Fprintf(writer, "\n%s%s\n", strings.Repeat(" ", rowIndent), styled(header, colorDim, useColor))
}

func renderSectionHeader(writer io.Writer, sectionItem section, useColor bool) {
	suffix := ""
	if sectionItem.role != "" {
		suffix = " · " + sectionItem.role
	}

	fmt.Fprintf(writer, "\n%s%s%s\n",
		strings.Repeat(" ", sectionIndent),
		styled(sectionItem.name, colorBold, useColor),
		styled(suffix, colorDim, useColor),
	)
}

func renderRow(writer io.Writer, row Row, widths columnWidths, useColor bool) {
	value := valueDisplay(row)

	valueColor := colorCyan
	if value == unsetMarker {
		valueColor = colorDarkGray
	}

	fmt.Fprintf(writer, "%s%s%s%s%s%s%s%s\n",
		strings.Repeat(" ", rowIndent),
		styled(pad(row.DisplayName, widths.fieldWidth), colorBold, useColor),
		strings.Repeat(" ", columnGap),
		styled(pad(row.TypeName, widths.typeWidth), colorDim, useColor),
		strings.Repeat(" ", columnGap),
		styled(pad(value, widths.valueWidth), valueColor, useColor),
		strings.Repeat(" ", columnGap),
		formatSourceCell(row, useColor),
	)
}

func measureColumns(sections []section) columnWidths {
	widths := columnWidths{
		fieldWidth:  len("FIELD"),
		typeWidth:   len("TYPE"),
		valueWidth:  len("VALUE"),
		sourceWidth: len("SOURCE"),
	}

	for _, sectionItem := range sections {
		for _, row := range sectionItem.rows {
			widths.fieldWidth = max(widths.fieldWidth, utf8.RuneCountInString(row.DisplayName))
			widths.typeWidth = max(widths.typeWidth, utf8.RuneCountInString(row.TypeName))
			widths.valueWidth = max(widths.valueWidth, utf8.RuneCountInString(valueDisplay(row)))
			widths.sourceWidth = max(widths.sourceWidth, utf8.RuneCountInString(formatSourceCell(row, false)))
		}
	}

	return widths
}

func headerWidth(widths columnWidths) int {
	return rowIndent + widths.fieldWidth + columnGap + widths.typeWidth + columnGap +
		widths.valueWidth + columnGap + widths.sourceWidth
}

func extractFieldRow(structField reflect.StructField, fieldAddr any) *Row {
	name := structField.Tag.Get("name")
	shortTag := structField.Tag.Get("short")

	if dashArgs, ok := fieldAddr.(*conga.DashArgs); ok {
		displayName := name
		if displayName == "" {
			displayName = "[-- <args...>]"
		}

		return &Row{
			DisplayName: displayName,
			TypeName:    "DashArgs",
			ValueStr:    dashArgs.String(),
			Source:      conga.SourceCLI,
			IsSet:       dashArgs.IsSet(),
			Kind:        "dash",
		}
	}

	if rawArgs, ok := fieldAddr.(*conga.RawArgs); ok {
		displayName := name
		if displayName == "" {
			displayName = "[command] [args...]"
		}

		return &Row{
			DisplayName: displayName,
			TypeName:    "RawArgs",
			ValueStr:    rawArgs.String(),
			Source:      conga.SourceCLI,
			IsSet:       rawArgs.IsSet(),
			Kind:        "raw",
		}
	}

	if builtinRow := extractBuiltinRow(fieldAddr, name); builtinRow != nil {
		return builtinRow
	}

	if _, ok := fieldAddr.(commandContainer); ok {
		return extractSubcommandRow(structField, fieldAddr)
	}

	return extractContainerRow(structField, fieldAddr, name, shortTag)
}

func extractBuiltinRow(fieldAddr any, name string) *Row {
	if _, ok := fieldAddr.(*conga.HelpCommand); ok {
		displayName := name
		if displayName == "" {
			displayName = "help"
		}

		return &Row{
			DisplayName: displayName,
			TypeName:    "command",
			ValueStr:    "(builtin help)",
			IsSet:       true,
			Kind:        "builtin",
		}
	}

	if _, ok := fieldAddr.(*conga.VersionCommand); ok {
		displayName := name
		if displayName == "" {
			displayName = "version"
		}

		return &Row{
			DisplayName: displayName,
			TypeName:    "command",
			ValueStr:    "(builtin version)",
			IsSet:       true,
			Kind:        "builtin",
		}
	}

	if _, ok := fieldAddr.(*conga.CompletionCommand); ok {
		displayName := name
		if displayName == "" {
			displayName = "completion"
		}

		return &Row{
			DisplayName: displayName,
			TypeName:    "command",
			ValueStr:    "(builtin completion)",
			IsSet:       true,
			Kind:        "builtin",
		}
	}

	return nil
}

func extractSubcommandRow(structField reflect.StructField, fieldAddr any) *Row {
	selected := false
	if command, ok := fieldAddr.(commandContainer); ok {
		selected = command.IsSelected()
	}

	return &Row{
		DisplayName: subcommandDisplayName(structField),
		TypeName:    "command",
		IsSet:       selected,
		Kind:        "subcmd",
	}
}

func extractContainerRow(structField reflect.StructField, fieldAddr any, name, shortTag string) *Row {
	kind, displayName := formatFieldDisplayName(structField, name, shortTag)

	fieldKind := structField.Type.Kind()
	isNativeCollection := fieldKind == reflect.Map || fieldKind == reflect.Slice

	container, ok := fieldAddr.(stateContainer)
	if !ok && !isNativeCollection {
		return nil
	}

	var (
		typeName     string
		valueStr     string
		isSet        bool
		source       conga.ValueSource
		sourceDetail string
	)

	if ok {
		if stringer, isStringer := fieldAddr.(fmt.Stringer); isStringer {
			valueStr = stringer.String()
		}

		typeName = extractContainerTypeName(structField.Type)
		isSet = container.IsSet()
		source = container.Source()
		sourceDetail = container.SourceDetail()
	} else {
		typeName = structField.Type.String()
		valueStr = stringx.Stringify(fieldAddr)
	}

	return &Row{
		DisplayName:  displayName,
		TypeName:     typeName,
		ValueStr:     valueStr,
		Source:       source,
		SourceDetail: sourceDetail,
		IsSet:        isSet,
		Kind:         kind,
	}
}

func extractContainerTypeName(containerType reflect.Type) string {
	if containerType.Kind() == reflect.Pointer {
		containerType = containerType.Elem()
	}

	switch containerType {
	case reflect.TypeFor[conga.Counter](), reflect.TypeFor[conga.ConfigFile]():
		return containerType.String()
	}

	if valueField, ok := containerType.FieldByName("value"); ok {
		return valueField.Type.String()
	}

	return containerType.String()
}

func isArgField(structField reflect.StructField) bool {
	typeName := structField.Type.String()

	return strings.Contains(typeName, ".OptionalArg[") || strings.Contains(typeName, ".RequiredArg[")
}

func formatFieldDisplayName(structField reflect.StructField, name, shortTag string) (string, string) {
	if isArgField(structField) {
		displayName := name
		if displayName == "" {
			displayName = fmt.Sprintf("<%s>", casing.Kebab(structField.Name))
		}

		return "arg", displayName
	}

	flagName := name
	if flagName == "" {
		flagName = casing.Kebab(structField.Name)
	}

	var shortRune rune
	if shortTag != "" {
		shortRune, _ = utf8.DecodeRuneInString(shortTag)
	}

	if shortRune != 0 {
		return "flag", fmt.Sprintf("-%c, --%s", shortRune, flagName)
	}

	return "flag", "--" + flagName
}

func formatSourceCell(row Row, useColor bool) string {
	switch row.Kind {
	case "subcmd":
		if row.IsSet {
			return styled("▸ selected", colorBoldGreen, useColor)
		}

		return styled("—", colorDarkGray, useColor)
	case "builtin":
		return styled("· builtin", colorDim, useColor)
	}

	if !row.IsSet {
		return styled("○ unset", colorDarkGray, useColor)
	}

	switch row.Source {
	case conga.SourceCLI:
		return styled("● cli", colorBoldCyan, useColor) + sourceDetailSuffix(row.SourceDetail, useColor)
	case conga.SourceEnv:
		return styled("● env", colorBoldGreen, useColor) + sourceDetailSuffix(row.SourceDetail, useColor)
	case conga.SourceConfig:
		return styled("● config", colorBoldYel, useColor) + sourceDetailSuffix(row.SourceDetail, useColor)
	case conga.SourceDefault:
		return styled("· default", colorDim, useColor)
	default:
		return styled("○ unset", colorDarkGray, useColor)
	}
}

func sourceDetailSuffix(sourceDetail string, useColor bool) string {
	if sourceDetail == "" {
		return ""
	}

	return styled(" "+sourceDetail, colorDarkGray, useColor)
}

func valueDisplay(row Row) string {
	if row.ValueStr == "" {
		return unsetMarker
	}

	if !row.IsSet && (row.ValueStr == "map[]" || row.ValueStr == "[]") {
		return unsetMarker
	}

	return row.ValueStr
}

func pad(cell string, width int) string {
	padding := width - utf8.RuneCountInString(cell)
	if padding <= 0 {
		return cell
	}

	return cell + strings.Repeat(" ", padding)
}

func styled(text, escape string, useColor bool) string {
	if !useColor {
		return text
	}

	return escape + text + colorReset
}

func isColorEnabled(writer io.Writer) bool {
	if value, ok := os.LookupEnv("CONGA_COLOR"); ok && value != "" {
		return value != "0"
	}

	if value, ok := os.LookupEnv("NO_COLOR"); ok && value != "" {
		return false
	}

	if os.Getenv("CLICOLOR") == "0" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return isTerminalWriter(writer)
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

func subcommandName(structField reflect.StructField) string {
	name := structField.Tag.Get("name")
	if name == "" {
		name = casing.Kebab(structField.Name)
	}

	return name
}

func subcommandDisplayName(structField reflect.StructField) string {
	name := subcommandName(structField)

	aliases := formatAliases(structField.Tag.Get("aliases"))
	if aliases != "" {
		name += " (" + aliases + ")"
	}

	return name
}

func formatAliases(tag string) string {
	if tag == "" {
		return ""
	}

	parts := strings.Split(tag, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return strings.Join(parts, ", ")
}
