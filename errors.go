package conga

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/intx"
	"github.com/conga-sh/conga/internal/spec"
)

// maxDisplayRunes is the maximum number of runes echoed from a user-supplied
// value before it is truncated in an error message.
const maxDisplayRunes = 50

const (
	// usageShapeMode prints the block the [UsageOnErrorMode] asks for.
	usageShapeMode usageShape = iota

	// usageShapeLineOnly prints the error line alone, in every mode. Value
	// errors use it because their message already names the flag or argument
	// and the value.
	usageShapeLineOnly

	// usageShapeFullHelp prints the full help of the command for
	// [UsageSummary] and [UsageFull], and the hint for [UsageHint].
	usageShapeFullHelp
)

const (
	suggestionKindNone suggestionKind = iota
	suggestionKindFlag
	suggestionKindCommand
)

var (
	// errEmptyPath reports an empty File, Dir, or FileContent path.
	errEmptyPath error = emptyPathError{}

	// errStdioCapabilityRequired reports a "-" value on a File field that
	// declared neither the stdin nor the stdout capability tag.
	errStdioCapabilityRequired error = stdioCapabilityError{}
)

// fromCommandLine is the origin of a value typed on the command line. Its
// failure names no origin and reaches the caller as a [UsageError].
var fromCommandLine = valueOrigin{source: SourceCLI} //nolint:gochecknoglobals // a fixed origin shared by every command-line site

// fromDefault is the origin of a value from a default tag.
var fromDefault = valueOrigin{source: SourceDefault} //nolint:gochecknoglobals // a fixed origin shared by every default-tag site

// ExitCoder is implemented by errors that choose the process exit code.
// [App.Exit] looks for the outermost [ExitCoder] in the error chain and exits
// with its code.
//
// A code from 1 to 255 is used as is. Any other code, 0 included, exits 1. An
// [*os/exec.ExitError] passes the child's exit code through, and a child killed
// by a signal exits with 128 plus the signal number, as a shell reports it.
// On Windows an [*os/exec.ExitError]'s code passes through unchanged. An
// error whose Error method returns the empty string prints no error line.
type ExitCoder interface {
	error

	// ExitCode returns the process exit code for the error.
	ExitCode() int
}

// UsageError is returned for a failure caused by the command line. That covers
// an unknown flag or command, a value that does not convert, a violated
// constraint, a missing required flag or argument, and an error returned
// through [Session.UsageError] or [Session.UsageErrorf]. [App.Exit] prints it
// and exits with code 2.
//
// Failures that come from the environment, configuration files, default tags,
// bootstrap, or the command's own code are not usage errors, and neither is a
// plain error returned by [Validator.Validate]. A conflict between flag group
// members is a usage error when at least one supplied member came from the
// command line.
//
// Unwrap returns the error behind the message. For a value that did not
// convert, that is the error the conversion step returned. It is the error of
// a [encoding.TextUnmarshaler] or type parser, which [errors.Is] and
// [errors.As] reach, or CONGA's own parse error, whose type is not part of the
// API. For a value a [SelfValidator] rejected, it is the error the validator
// returned. For a [Session.UsageErrorf] call it is the %w operand. Unwrap
// returns nil when CONGA found the problem itself, as for an unknown flag, an
// enum, range or length violation, or a flag group conflict.
//
// A UsageError has no exported fields. Obtain one with [Session.UsageError],
// or find one in a returned error with [errors.AsType]:
//
//	if usage, ok := errors.AsType[*conga.UsageError](err); ok {
//		fmt.Println(usage.CommandPath(), usage.Usage())
//	}
//
// The zero value is valid. It has an empty message and no command path.
type UsageError struct {
	msg      string
	cmdChain []*spec.Command
	shape    usageShape

	// cause is the error behind the message. It is the error a conversion or
	// a [SelfValidator] returned, or the %w operand of [Session.UsageErrorf],
	// and nil when CONGA itself found the problem.
	cause error

	// suggestion is the "did you mean" candidate appended to the message, and
	// suggestionKind says whether it names a flag or a subcommand.
	suggestion     string
	suggestionKind suggestionKind
}

// Error returns the message, followed by the "did you mean" suggestion when
// there is one.
func (e *UsageError) Error() string {
	if e.suggestion != "" {
		return fmt.Sprintf("%s (did you mean %q?)", e.msg, e.suggestion)
	}

	return e.msg
}

// Unwrap returns the error behind the message. That is the error a conversion
// step or a [SelfValidator] returned, or the %w operand of
// [Session.UsageErrorf]. It returns nil when CONGA found the problem itself.
func (e *UsageError) Unwrap() error {
	return e.cause
}

// ExitCode returns 2.
func (e *UsageError) ExitCode() int {
	return 2
}

// CommandPath returns the space-separated path of the command where the error
// occurred, or "" when the error has no command.
func (e *UsageError) CommandPath() string {
	return commandChainPath(e.cmdChain)
}

// Usage returns the syntax usage summary of the command where the error
// occurred, or "" when the error has no command.
func (e *UsageError) Usage() string {
	if len(e.cmdChain) == 0 {
		return ""
	}

	return help.Usage(e.cmdChain)
}

// usageShape selects what [App.Exit] prints below a [UsageError]'s message.
type usageShape uint8

// suggestionKind says what a [UsageError]'s suggestion names.
type suggestionKind uint8

// conversionError reports a value that did not convert to its flag's or
// argument's type. From the command line it reaches the caller as a
// [UsageError] (see commandLineError). From anywhere else it is returned as
// is, and [App.Exit] exits 1 without a usage block.
type conversionError struct {
	flag   *spec.Flag  // the flag the value was for, or nil
	arg    *spec.Arg   // the positional argument the value was for, or nil
	value  string      // the value as given
	origin valueOrigin // where the value came from
	cause  error       // what the conversion returned
}

// Error returns the cause's own text, or "invalid <type> value" quoting the
// value when the cause has none, then what the value was for and where it came
// from, then the cause's leading-zero correction when it carries one:
// "invalid integer value "x" for argument N (from default value)".
func (e *conversionError) Error() string {
	var message string
	if e.cause != nil {
		message = e.cause.Error()
	}

	if message == "" {
		typeName := friendlyTypeName(e.valueType())
		display := truncateForDisplay(e.value)
		message = fmt.Sprintf("invalid %s value %q", typeName, display)
	}

	message += " for " + valueSubject(e.flag, e.arg) + e.origin.suffix()

	if hint := intx.HintOf(e.cause); hint != "" {
		message += " (" + hint + ")"
	}

	return message
}

// Unwrap returns what the conversion returned.
func (e *conversionError) Unwrap() error {
	return e.cause
}

// valueType returns the type the value was converted to.
func (e *conversionError) valueType() reflect.Type {
	if e.flag != nil {
		return e.flag.TargetType
	}

	return e.arg.TargetType
}

// inputError reports input CONGA rejected: a converted value that broke a
// constraint (enum, len, range, items), a SelfValidator failure, conflicting
// values (flag groups, stdin read twice), a feature failure, a counter count,
// an empty config path, or a config entry for a gated flag. From the command
// line it reaches the caller as a [UsageError] (see commandLineError). From
// anywhere else it is returned as is, and [App.Exit] exits 1 without a usage
// block.
type inputError struct {
	flag    *spec.Flag  // the flag the value was for, or nil
	arg     *spec.Arg   // the positional argument the value was for, or nil
	feature string      // the feature the value was for, or ""
	entry   *string     // the key of the map entry that failed, or nil
	origin  valueOrigin // where the value came from
	msg     string      // what is wrong with the value
	note    string      // the detail that follows the origin, or ""
	hint    string      // the action that fixes the value, or ""
	cause   error       // what the check returned, or nil
}

// Error returns the message, what the value was for, and where it came from,
// then the note and the hint: "value 11 is out of range [1..10] for flag --size
// (from environment variable SIZE)".
func (e *inputError) Error() string {
	message := e.msg

	if subject := e.subject(); subject != "" {
		message += " for " + subject
	}

	message += e.origin.suffix()

	if e.note != "" {
		message += ": " + e.note
	}

	if e.hint != "" {
		message += " (" + e.hint + ")"
	}

	return message
}

// Unwrap returns what the check returned, or nil when CONGA itself found the
// problem.
func (e *inputError) Unwrap() error {
	return e.cause
}

// subject names what the value was for, such as "flag --port", "argument N" or
// a quoted feature name, followed by the entry key for a map entry. It is ""
// when the error is about conflicting values.
func (e *inputError) subject() string {
	subject := valueSubject(e.flag, e.arg)
	if e.feature != "" {
		subject = fmt.Sprintf("feature %q", e.feature)
	}

	if subject != "" && e.entry != nil {
		subject += fmt.Sprintf(" entry %q", *e.entry)
	}

	return subject
}

// completionError represents an internal failure during completion evaluation (panics or unwritable stdout).
type completionError struct {
	cause string
}

func (e *completionError) Error() string {
	return "internal completion failure: " + e.cause
}

// ExitCode returns 2, so a caller mapping codes through [ExitCoder] agrees
// with [App.Exit].
func (e *completionError) ExitCode() int {
	return 2
}

// decoderRequiredError reports a named configuration target on an application
// with no registered config decoder. It exits 1 even when the target is
// --config-file on the command line.
type decoderRequiredError struct {
	target string // what named the target, such as "flag --config-file"
}

func (e *decoderRequiredError) Error() string {
	return e.target + " requires a registered config decoder: call App.RegisterDecoder with a format decoder (e.g. decoder/json)"
}

// valueOrigin is where a value came from. It holds the source tier and the
// variable, config key, or default text that diagnostics attribute the value
// to.
type valueOrigin struct {
	source ValueSource
	name   string
}

// suffix renders the parenthesized origin shared by value diagnostics: "(from
// environment variable NAME)" for an environment value, "(from config key
// KEY)" with a quoted key for a configuration value, and "(from default
// value)" for a default tag, each after a space. Command-line and unnamed
// environment or configuration origins render nothing.
func (origin valueOrigin) suffix() string {
	if origin.source == SourceDefault {
		return " (from default value)"
	}

	if described := origin.describe(); described != "" {
		return " (from " + described + ")"
	}

	return ""
}

// describe names a named environment or configuration origin: "environment
// variable NAME", or "config key" with a quoted key. Any other origin has no
// name.
func (origin valueOrigin) describe() string {
	if origin.name == "" {
		return ""
	}

	switch origin.source {
	case SourceEnv:
		return "environment variable " + origin.name
	case SourceConfig:
		return fmt.Sprintf("config key %q", origin.name)
	default:
		return ""
	}
}

// withShape sets what [App.Exit] prints below the message. The default is
// [usageShapeMode].
func (e *UsageError) withShape(shape usageShape) *UsageError {
	e.shape = shape

	return e
}

// withSuggestion sets the "did you mean" candidate. An empty suggestion sets
// nothing.
func (e *UsageError) withSuggestion(suggestion string, kind suggestionKind) *UsageError {
	if suggestion == "" {
		return e
	}

	e.suggestion = suggestion
	e.suggestionKind = kind

	return e
}

// withCause sets the error behind the message.
func (e *UsageError) withCause(cause error) *UsageError {
	e.cause = cause

	return e
}

// usageError returns a [UsageError] with message msg for the command chain
// cmdChain. The with methods set the optional shape, suggestion, and cause.
func usageError(cmdChain []*spec.Command, msg string) *UsageError {
	return &UsageError{msg: msg, cmdChain: cmdChain}
}

// usageErrorf returns a [UsageError] with a formatted message for the command
// chain cmdChain.
func usageErrorf(cmdChain []*spec.Command, format string, args ...any) *UsageError {
	return &UsageError{msg: fmt.Sprintf(format, args...), cmdChain: cmdChain}
}

// valueSubject names what a value was for: "flag --port", "argument N", or ""
// when it was for neither.
func valueSubject(flag *spec.Flag, arg *spec.Arg) string {
	switch {
	case flag != nil:
		return "flag --" + flag.Name
	case arg != nil:
		return "argument " + arg.Name
	default:
		return ""
	}
}

// commandLineError reports input the user typed on the command line as a
// [UsageError] for cmdChain. Every conversion failure, and every input error
// that names its flag, argument or feature, prints the error line alone. An
// input error that names no subject (a flag group conflict, say) is followed
// by the usage block [UsageOnErrorMode] asks for. Errors of other types, and
// errors from other sources, are returned unchanged.
func commandLineError(cmdChain []*spec.Command, err error) error {
	if conversion, ok := errors.AsType[*conversionError](err); ok && conversion.origin.source == SourceCLI {
		return &UsageError{msg: conversion.Error(), cmdChain: cmdChain, shape: usageShapeLineOnly, cause: conversion.cause}
	}

	input, ok := errors.AsType[*inputError](err)
	if !ok || input.origin.source != SourceCLI {
		return err
	}

	shape := usageShapeMode
	if input.subject() != "" {
		shape = usageShapeLineOnly
	}

	return &UsageError{msg: input.Error(), cmdChain: cmdChain, shape: shape, cause: input.cause}
}

func friendlyTypeName(targetType reflect.Type) string {
	if targetType == nil {
		return "type"
	}

	if targetType == reflect.TypeFor[time.Duration]() {
		return "duration"
	}

	if targetType == reflect.TypeFor[*url.URL]() || targetType == reflect.TypeFor[url.URL]() {
		return "URL"
	}

	switch targetType.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "unsigned integer"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.String:
		return "string"
	default:
		return targetType.String()
	}
}

// truncateForDisplay truncates a string for display in error messages.
// Returns the first maxDisplayRunes runes followed by "…" if the string exceeds them.
func truncateForDisplay(raw string) string {
	if len(raw) <= maxDisplayRunes {
		return raw
	}

	runes := []rune(raw)
	if len(runes) <= maxDisplayRunes {
		return raw
	}

	return string(runes[:maxDisplayRunes]) + "…"
}

func commandChainPath(chain []*spec.Command) string {
	if len(chain) == 0 {
		return ""
	}

	names := make([]string, 0, len(chain))
	for _, cmd := range chain {
		if cmd != nil && cmd.Name != "" {
			names = append(names, cmd.Name)
		}
	}

	return strings.Join(names, " ")
}
