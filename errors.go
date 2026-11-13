package conga

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/spec"
)

// Base error categories classifying execution and configuration failures.
var (
	// ErrSchema indicates the command schema could not be constructed from the struct definition.
	//
	// This occurs when a struct tag is unrecognized, a tag combination is invalid,
	// a field type is unsupported, or ancestor flag shadowing is detected.
	// ErrSchema represents a developer defect detected during schema compilation.
	ErrSchema = spec.ErrSchema

	// ErrParse indicates command-line tokens could not be parsed.
	//
	// This category includes unknown flags, missing flag arguments, unexpected positional
	// arguments, unrecognized subcommand names, and values attached to flags that take
	// none. All parse errors classify as usage errors ([IsUsageError] reports true).
	ErrParse = errors.New("parse error")

	// ErrCoerce indicates a command-line value could not be converted to its target type.
	//
	// This category includes numeric parsing failures, unparseable durations, malformed
	// URLs, and errors returned by [encoding.TextUnmarshaler] or [ContextAwareTextUnmarshaler].
	// A conversion failure at command-line binding is a usage error ([IsUsageError] reports
	// true). Values that reach binding from the environment, a configuration file, or a
	// default tag classify under [ErrConfig] instead, and [errors.Is] matches only that
	// category.
	ErrCoerce = coerce.ErrCoerce

	// ErrValidation indicates a flag or argument constraint was violated.
	//
	// This category covers violations of struct tags such as range, len, enum,
	// xor, and and, as well as failures returned by [SelfValidator.Validate].
	// A constraint violation on a command-line value is a usage error ([IsUsageError]
	// reports true). Values that reach validation from the environment, a configuration
	// file, or a default tag classify under [ErrConfig] instead, and [errors.Is] matches
	// only that category.
	ErrValidation = errors.New("validation error")

	// ErrConfig indicates configuration could not be loaded or a value from the
	// environment, a configuration file, or a default tag failed to bind.
	//
	// This category includes missing explicit configuration files, file read permission
	// errors, file size limit violations (exceeding the 16MB DoS threshold), syntax
	// errors returned by configuration decoders, and conversion or constraint failures
	// for environment-, configuration-, and default-sourced values.
	ErrConfig = sentinel.ErrConfig
)

// maxDisplayRunes is the maximum number of runes echoed from a user-supplied
// value before it is truncated in an error message.
const maxDisplayRunes = 50

// Internal sentinel errors for error wrapping and chaining.
var (
	errUnknownFlag        = fmt.Errorf("%w: unknown flag", ErrParse)
	errFlagRequiresValue  = fmt.Errorf("%w: flag requires an argument", ErrParse)
	errUnknownCommand     = fmt.Errorf("%w: unknown command", ErrParse)
	errSubcommandRequired = fmt.Errorf("%w: subcommand required", ErrParse)

	errMissingFlag       = fmt.Errorf("%w: missing required flag", ErrValidation)
	errMissingArg        = fmt.Errorf("%w: missing required positional argument", ErrValidation)
	errUnexpectedArg     = fmt.Errorf("%w: unexpected argument", ErrParse)
	errInvalidValue      = fmt.Errorf("%w: invalid value", ErrValidation)
	errOutOfRange        = fmt.Errorf("%w: value out of range", ErrValidation)
	errInvalidLength     = fmt.Errorf("%w: invalid length", ErrValidation)
	errInvalidItemCount  = fmt.Errorf("%w: invalid item count", ErrValidation)
	errMutuallyExclusive = fmt.Errorf("%w: mutually exclusive flags", ErrValidation)
	errAndGroup          = fmt.Errorf("%w: flags must be given together", ErrValidation)
	errStdinReadTwice    = fmt.Errorf("%w: stdin can only be read once", ErrValidation)

	errPathWrongKind = fmt.Errorf("%w: wrong path kind", ErrValidation)

	// errStdioCapabilityRequired reports a "-" value on a File field that
	// declared neither the stdin nor the stdout capability tag.
	errStdioCapabilityRequired = errors.New(`"-" requires the stdin or stdout tag on the field`)

	// errByteSizeUnitsTag reports an unrecognized units tag value on a ByteSize field.
	errByteSizeUnitsTag = errors.New("unknown units value")

	// errByteSizeDisplayTag reports an unrecognized display tag value on a ByteSize field.
	errByteSizeDisplayTag = errors.New("unknown display value")

	errNoRunner = fmt.Errorf("%w: runner not implemented", ErrSchema)
)

// UsageError is an interface implemented by errors that carry command-path and
// usage-rendering context. Parse, coercion, and validation failures implement
// it. [IsUsageError] decides whether the failure is a command-line usage error:
// command-line value failures report true, while environment-,
// configuration-, and default-sourced value failures report false and classify
// under [ErrConfig].
//
// When [App.Exit] receives an error for which [IsUsageError] reports true, it
// prints the message to standard error, appends usage guidance as configured by
// [UsageOnErrorMode], and terminates the process with exit code 2. The
// interface assertion alone does not imply exit code 2; callers that inspect it
// directly must also check IsUsageError. Self-describing value errors (coercion
// failures and value-constraint violations) print the message alone regardless
// of the configured mode.
type UsageError interface {
	error

	// IsUsageError reports whether the error represents invalid command-line input.
	IsUsageError() bool

	// CommandPath returns the space-separated command path where the error occurred.
	CommandPath() string

	// Usage returns the syntax usage summary for the command where the error occurred.
	Usage() string
}

// CompletionError is an interface implemented by errors that represent an internal
// failure during shell completion evaluation.
type CompletionError interface {
	error

	// IsCompletionError reports whether the error represents an internal completion failure.
	IsCompletionError() bool
}

// IsUsageError reports whether err was caused by invalid command-line input.
//
// Returns true if err implements [UsageError] and reports true, or if err wraps
// [ErrParse], [ErrCoerce], or [ErrValidation]. Returns false for developer schema
// errors ([ErrSchema]), configuration failures ([ErrConfig]), runtime errors
// returned by [Runner.Run], or nil errors. Conversion and constraint failures for
// environment-, configuration-, and default-sourced values classify under
// [ErrConfig] and report false, even though the error value still implements
// [UsageError].
func IsUsageError(err error) bool {
	if err == nil {
		return false
	}

	if ue, ok := errors.AsType[UsageError](err); ok {
		return ue.IsUsageError()
	}

	return errors.Is(err, ErrParse) ||
		errors.Is(err, ErrCoerce) ||
		errors.Is(err, ErrValidation)
}

// IsCompletionError reports whether err was caused by an internal failure during shell
// completion evaluation.
func IsCompletionError(err error) bool {
	if err == nil {
		return false
	}

	if completionErr, ok := errors.AsType[CompletionError](err); ok {
		return completionErr.IsCompletionError()
	}

	return false
}

type commandChainProvider interface {
	error
	commandChain() []*spec.Command
}

// selfDescribingError is implemented by usage errors whose message already
// names the offending flag/argument and value, so rendering usage or help
// text below the error adds noise without context. It drives the rendering
// policy in handleUsageFatal: self-describing value errors (coercion failures
// and value-constraint violations) render only the error line, regardless of
// the configured UsageOnErrorMode. Parse errors and missing-input errors
// deliberately do not implement it: their messages benefit from the usage
// block listing valid commands and flags.
type selfDescribingError interface {
	error
	selfDescribing() bool
}

// parseError represents a CLI syntax, tokenization, or command-routing error.
type parseError struct {
	msg        string
	token      string
	cmdChain   []*spec.Command
	suggestion string
	cause      error
}

func (e *parseError) Error() string {
	if e.suggestion != "" {
		return fmt.Sprintf("%s (did you mean %q?)", e.msg, e.suggestion)
	}

	return e.msg
}

func (e *parseError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}

	return ErrParse
}

func (e *parseError) Is(target error) bool {
	if target == ErrParse {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}

func (e *parseError) IsUsageError() bool {
	return true
}

func (e *parseError) CommandPath() string {
	return commandChainPath(e.cmdChain)
}

func (e *parseError) Usage() string {
	return help.Usage(e.cmdChain)
}

func (e *parseError) commandChain() []*spec.Command {
	return e.cmdChain
}

// coercionError represents a type-conversion failure across CLI flags, positional args, env vars, or configs.
type coercionError struct {
	targetName string
	isFlag     bool
	source     container.Source
	sourceName string
	targetType reflect.Type
	rawValue   string
	msg        string
	cmdChain   []*spec.Command
	cause      error
}

func (e *coercionError) Error() string {
	if e.msg != "" {
		return e.msg
	}

	location := formatCoercionLocation(e)

	if e.cause != nil {
		causeMsg := e.cause.Error()

		if causeMsg != "" && causeMsg != ErrCoerce.Error() {
			if location != "" {
				return fmt.Sprintf("%s %s", causeMsg, location)
			}

			return causeMsg
		}
	}

	typeName := friendlyTypeName(e.targetType)
	if location != "" {
		return fmt.Sprintf("invalid %s value %q %s", typeName, truncateForDisplay(e.rawValue), location)
	}

	return fmt.Sprintf("invalid %s value %q", typeName, truncateForDisplay(e.rawValue))
}

func (e *coercionError) Unwrap() error {
	if !valueErrorIsUsage(e.source) {
		return nil
	}

	if e.cause != nil {
		return e.cause
	}

	return ErrCoerce
}

// As exposes the cause to errors.As for environment-, configuration-, and
// default-sourced failures, whose Unwrap deliberately stops at the category
// boundary: the application's own TextUnmarshaler error or a stdlib cause such
// as *fs.PathError stays reachable even though the failure matches no category
// but [ErrConfig] (see configSourcedIs for errors.Is).
func (e *coercionError) As(target any) bool {
	if e.cause == nil {
		return false
	}

	return errors.As(e.cause, target)
}

func (e *coercionError) Is(target error) bool {
	if !valueErrorIsUsage(e.source) {
		return configSourcedIs(e.cause, target)
	}

	if target == ErrCoerce {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}

func (e *coercionError) IsUsageError() bool {
	return valueErrorIsUsage(e.source)
}

func (e *coercionError) CommandPath() string {
	return commandChainPath(e.cmdChain)
}

func (e *coercionError) Usage() string {
	return help.Usage(e.cmdChain)
}

func (e *coercionError) commandChain() []*spec.Command {
	return e.cmdChain
}

// selfDescribing reports that coercion failures always name the target
// flag/argument and the offending value, since every construction site sets
// targetName from the flag or argument specification.
func (e *coercionError) selfDescribing() bool {
	return true
}

// validationError represents a constraint violation (required flag/arg, enum, XOR, and group).
type validationError struct {
	msg      string
	source   container.Source
	cmdChain []*spec.Command
	cause    error

	// valueShaped marks value-shaped constraint violations (enum, range,
	// length, item count, domain-type validation) whose message already names
	// the target flag/argument and the offending value.
	valueShaped bool
}

func (e *validationError) Error() string {
	return e.msg
}

func (e *validationError) Unwrap() error {
	if !valueErrorIsUsage(e.source) {
		return nil
	}

	if e.cause != nil {
		return e.cause
	}

	return ErrValidation
}

// As exposes the cause to errors.As for environment-, configuration-, and
// default-sourced failures, whose Unwrap deliberately stops at the category
// boundary: an application SelfValidator error or a stdlib cause such as
// *fs.PathError stays reachable even though the failure matches no category
// but [ErrConfig] (see configSourcedIs for errors.Is).
func (e *validationError) As(target any) bool {
	if e.cause == nil {
		return false
	}

	return errors.As(e.cause, target)
}

func (e *validationError) Is(target error) bool {
	if !valueErrorIsUsage(e.source) {
		return configSourcedIs(e.cause, target)
	}

	if target == ErrValidation {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}

func (e *validationError) IsUsageError() bool {
	return valueErrorIsUsage(e.source)
}

func (e *validationError) CommandPath() string {
	return commandChainPath(e.cmdChain)
}

func (e *validationError) Usage() string {
	return help.Usage(e.cmdChain)
}

func (e *validationError) commandChain() []*spec.Command {
	return e.cmdChain
}

func (e *validationError) selfDescribing() bool {
	return e.valueShaped
}

// completionError represents an internal failure during completion evaluation (panics or unwritable stdout).
type completionError struct {
	cause string
}

func (e *completionError) Error() string {
	return "internal completion failure: " + e.cause
}

func (e *completionError) IsCompletionError() bool {
	return true
}

func formatCoercionLocation(errObj *coercionError) string {
	if errObj.isFlag {
		return "for flag --" + errObj.targetName + sourceLocation(errObj.source, errObj.sourceName)
	}

	if errObj.targetName != "" {
		return "for argument " + errObj.targetName + sourceLocation(errObj.source, errObj.sourceName)
	}

	if errObj.source == SourceEnv && errObj.sourceName != "" {
		return "for environment variable " + errObj.sourceName
	}

	if errObj.source == SourceConfig && errObj.sourceName != "" {
		return fmt.Sprintf("for config key %q", errObj.sourceName)
	}

	return ""
}

// valueErrorIsUsage reports whether a value failure from source is a command-line
// usage error. Only command-line tokens produce usage errors; environment,
// configuration, and default values classify under [ErrConfig] instead (#133).
func valueErrorIsUsage(source ValueSource) bool {
	switch source {
	case SourceEnv, SourceConfig, SourceDefault:
		return false
	default:
		return true
	}
}

// sourceLocation renders the parenthesized origin suffix shared by value
// diagnostics: " (from environment variable NAME)" for env-sourced values,
// " (from config key "KEY")" for config-sourced values, and
// " (from default value)" for values that failed while binding a default tag.
// CLI, unnamed env/config, and unset sources render no suffix.
func sourceLocation(source ValueSource, sourceName string) string {
	if source == SourceDefault {
		return " (from default value)"
	}

	if sourceName == "" {
		return ""
	}

	switch source {
	case SourceEnv:
		return fmt.Sprintf(" (from environment variable %s)", sourceName)
	case SourceConfig:
		return fmt.Sprintf(" (from config key %q)", sourceName)
	default:
		return ""
	}
}

// configSourcedIs answers errors.Is for an environment-, configuration-, or
// default-sourced value failure. The failure matches [ErrConfig] and no other
// category sentinel, even when its cause wraps one; any other target, such as
// fs.ErrNotExist under a *fs.PathError, is matched against the cause, so
// errors.Is reaches the same causes errors.As does.
func configSourcedIs(cause, target error) bool {
	switch {
	case errors.Is(target, ErrConfig):
		return true
	case errors.Is(target, ErrSchema), errors.Is(target, ErrParse), errors.Is(target, ErrCoerce), errors.Is(target, ErrValidation):
		return false
	}

	return cause != nil && errors.Is(cause, target)
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
