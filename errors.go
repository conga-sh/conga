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
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/spec"
)

// Base category errors identifying where an error occurred.
var (
	// ErrSchema indicates the command schema could not be built from the target struct.
	ErrSchema = spec.ErrSchema

	// ErrParse indicates command-line tokens could not be parsed.
	ErrParse = errors.New("parse error")

	// ErrCoerce indicates a raw value could not be converted to its target type.
	ErrCoerce = coerce.ErrCoerce

	// ErrValidation indicates a flag or argument constraint was violated.
	ErrValidation = errors.New("validation error")

	// ErrConfig indicates the configuration file could not be loaded.
	ErrConfig error = sentinel.ErrConfig
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
	errUnexpectedArg     = fmt.Errorf("%w: unexpected argument", ErrValidation)
	errInvalidValue      = fmt.Errorf("%w: invalid value", ErrValidation)
	errOutOfRange        = fmt.Errorf("%w: value out of range", ErrValidation)
	errInvalidLength     = fmt.Errorf("%w: invalid length", ErrValidation)
	errInvalidItemCount  = fmt.Errorf("%w: invalid item count", ErrValidation)
	errMutuallyExclusive = fmt.Errorf("%w: mutually exclusive flags", ErrValidation)
	errAndGroup          = fmt.Errorf("%w: flags must be provided together", ErrValidation)

	errPathWrongKind = fmt.Errorf("%w: wrong path kind", ErrCoerce)

	// errStdioCapabilityRequired reports a "-" value on a File field that
	// declared neither the stdin nor the stdout capability tag.
	errStdioCapabilityRequired = errors.New(`"-" requires the stdin or stdout tag on the field`)

	errNoRunner = errors.New("runner not implemented")
)

// UsageError is an interface implemented by errors that represent invalid command-line usage.
type UsageError interface {
	error
	IsUsageError() bool
	CommandPath() string
}

// IsUsageError reports whether err was caused by invalid command-line input.
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
// deliberately do not implement it — their messages benefit from the usage
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
		return fmt.Sprintf("%s. Did you mean %q?", e.msg, e.suggestion)
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
	if e.cause != nil {
		return e.cause
	}

	return ErrCoerce
}

func (e *coercionError) Is(target error) bool {
	if target == ErrCoerce {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}

func (e *coercionError) IsUsageError() bool {
	return true
}

func (e *coercionError) CommandPath() string {
	return commandChainPath(e.cmdChain)
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
	if e.cause != nil {
		return e.cause
	}

	return ErrValidation
}

func (e *validationError) Is(target error) bool {
	if target == ErrValidation {
		return true
	}

	if e.cause != nil {
		return errors.Is(e.cause, target)
	}

	return false
}

func (e *validationError) IsUsageError() bool {
	return true
}

func (e *validationError) CommandPath() string {
	return commandChainPath(e.cmdChain)
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

func (e *completionError) IsUsageError() bool {
	return true
}

func (e *completionError) CommandPath() string {
	return ""
}

func formatCoercionLocation(errObj *coercionError) string {
	if errObj.source == SourceEnv && errObj.sourceName != "" {
		if errObj.isFlag {
			return fmt.Sprintf("for flag --%s (from environment variable %s)", errObj.targetName, errObj.sourceName)
		}

		return "for environment variable " + errObj.sourceName
	}

	if errObj.source == SourceConfig && errObj.sourceName != "" {
		if errObj.isFlag {
			return fmt.Sprintf("for flag --%s (from config key %q)", errObj.targetName, errObj.sourceName)
		}

		return fmt.Sprintf("for config key %q", errObj.sourceName)
	}

	if errObj.isFlag {
		return "for flag --" + errObj.targetName
	}

	if errObj.targetName != "" {
		return "for argument " + errObj.targetName
	}

	return ""
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
