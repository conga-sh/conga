package conga

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/help"
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

// Originer is implemented by errors that name where a value came from. Each
// string is the text [App.Exit] prints after "from: ", such as
// "environment variable TOOL_PORT" or "config key "port" in config file
// "tool.yaml"". An empty result prints no line.
//
// The interface is structural, so any error type implementing Origins() []string
// participates. [errors.As] stops at the first implementer in a chain even when
// it returns an empty result; read pieces through [ResolveError], which applies
// the chain rule (the outermost non-empty result wins).
type Originer interface {
	error

	// Origins returns the origin of the error, one string per origin.
	Origins() []string
}

// Noter is implemented by errors that carry a note. The note is more detail on
// the issue line 1 states. [App.Exit] prints it on a "note:" line. An empty result
// prints no line.
//
// The interface is structural, so any error type implementing Note() string
// participates. [errors.As] stops at the first implementer in a chain even when
// it returns an empty result; read pieces through [ResolveError], which applies
// the chain rule (the outermost non-empty result wins).
type Noter interface {
	error

	// Note returns the detail on the error.
	Note() string
}

// Hinter is implemented by errors that carry a hint. The hint is something the
// reader can act on, an imperative or a "did you mean" suggestion. [App.Exit] prints it
// on a "hint:" line. An empty result prints no line.
//
// The interface is structural, so any error type implementing Hint() string
// (including third-party types such as samber/oops's OopsError) participates.
// [errors.As] stops at the first implementer in a chain even when it returns an
// empty result; read pieces through [ResolveError], which applies the chain
// rule (the outermost non-empty result wins).
type Hinter interface {
	error

	// Hint returns the action that fixes the error.
	Hint() string
}

// Error is an opt-in view of any error: a CONGA error, a hook's error, a
// developer wrapper around [App.Run]'s result, or a join. Construct one with
// [ResolveError]. [App.Run] and [App.RunContext] return the failing error as
// is and do not wrap it in Error.
//
// [Error.Error] returns the underlying error's text, which is line 1 alone for
// a CONGA error. [Error.Origins], [Error.Note] and [Error.Hint] read the pieces
// [App.Exit] prints on "from:", "note:" and "hint:" lines by the chain rule:
// the outermost error in the chain that returns a non-empty value supplies each
// piece, in [errors.As] traversal order, so an author wrapper's Hint replaces
// CONGA's and the note and hint can come from different errors. For an error
// that stands for several errors (an [errors.Join] of them), the piece methods
// describe the first non-empty member, and [Error.Members] returns each
// non-empty member as its own Error.
//
// [Error.Render] returns the plain multi-line form (line 1 followed by the
// indented lines for every non-empty member, with no prefix or color). Error
// implements [fmt.Formatter] (%v and %s print line 1; %+v prints [Error.Render];
// every other verb formats line 1 with that verb), [encoding/json.Marshaler],
// [encoding.TextMarshaler], and [encoding.TextAppender], so a structured logger
// such as [log/slog] records the pieces cleanly under both JSON and text
// handlers. Error has no ExitCode method; [App.ExitCode] decides from the
// outermost [ExitCoder] in the chain.
//
// The zero value is valid: every method is safe on it, and [Error.MarshalJSON]
// returns null. Do not return a zero Error through an error return value.
type Error struct {
	err     error
	members []error
}

// ResolveError resolves err into an [Error] from the outside in, applying the
// chain rule and splitting any joined error into its members. A nil err returns
// the zero [Error].
//
// ResolveError is for inspecting, formatting, and logging errors. Because it
// starts at err itself, a developer wrapper's [Hinter.Hint] wins over an inner
// error's and the result always agrees with [App.Exit]. Do not return a zero
// [Error] through an error return value.
func ResolveError(err error) Error {
	if err == nil {
		return Error{}
	}

	if resolved, ok := err.(Error); ok { //nolint:errorlint // only a direct Error at the top is reused; a wrapped Error starts a fresh outer walk
		return resolved
	}

	if resolved, ok := err.(*Error); ok { //nolint:errorlint // same for a direct *Error at the top
		if resolved == nil {
			return Error{}
		}

		return *resolved
	}

	return Error{err: err, members: errorMembers(err)}
}

// Error returns the underlying error's text, which is line 1 alone for a CONGA
// error, or "" for the zero [Error]. An error that stands for several errors
// keeps one line per member.
func (e Error) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

// Unwrap returns the underlying error, or nil for the zero [Error].
func (e Error) Unwrap() error {
	return e.err
}

// Origins returns the origins of the first non-empty member by the chain rule,
// or nil.
func (e Error) Origins() []string {
	return errlines.Origins(e.firstMember())
}

// Note returns the note of the first non-empty member by the chain rule, or "".
func (e Error) Note() string {
	return errlines.Note(e.firstMember())
}

// Hint returns the hint of the first non-empty member by the chain rule, or "".
func (e Error) Hint() string {
	return errlines.Hint(e.firstMember())
}

// Members returns each non-empty member as its own [Error]. For a single
// non-empty error it returns a one-element slice; for the zero [Error] or an
// error whose members all have empty messages it returns nil.
func (e Error) Members() []Error {
	var result []Error

	for _, member := range e.members {
		if member == nil || member.Error() == "" {
			continue
		}

		result = append(result, Error{err: member, members: []error{member}})
	}

	return result
}

// Render formats every non-empty member as line 1 followed by its indented
// "from:", "note:" and "hint:" lines, plain, with no prefix, color, or trailing
// newline. It returns "" for the zero [Error] or when every member has an empty
// message.
func (e Error) Render() string {
	return errlines.Plain(e.members)
}

// MarshalJSON returns {"error": <line 1>, "from": [...], "note": "...", "hint":
// "..."} for the first non-empty member, with every key but "error" omitted
// when empty, and "more": [...] holding the other non-empty members when the
// error stands for several. The zero [Error] marshals as null; a non-nil error
// whose message is empty marshals as {"error":""}.
func (e Error) MarshalJSON() ([]byte, error) {
	if e.err == nil {
		return []byte("null"), nil
	}

	return errlines.AppendJSON(nil, e.members), nil
}

// MarshalText returns "<line 1> (from: <origin>; note: ...; hint: ...)" with
// absent parts left out and no parentheses when nothing follows line 1. Several
// members are joined with " | ".
func (e Error) MarshalText() ([]byte, error) {
	return e.AppendText(nil)
}

// AppendText appends the one-line text form of [Error.MarshalText] to dst,
// implementing [encoding.TextAppender].
func (e Error) AppendText(dst []byte) ([]byte, error) {
	if e.err == nil {
		return dst, nil
	}

	return errlines.AppendText(dst, e.members), nil
}

// Format implements [fmt.Formatter]. %+v prints [Error.Render] (line 1 followed
// by the "from:", "note:" and "hint:" lines, plain). Every other form formats
// line 1 with the requested verb.
func (e Error) Format(state fmt.State, verb rune) {
	formatError(state, verb, e.Error(), e.members)
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
// Origins, Note and Hint return the pieces [App.Exit] prints on their own
// lines, by the chain rule [Error] describes: the failed value's own, then those
// of the error behind the message. For an unknown flag or command, Hint is the
// "did you mean" suggestion. Error returns line 1 alone.
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

	// inner is the error the message came from (a value error, or the
	// completion shell error), whose origins, note and hint this error
	// reports. It is nil for a failure CONGA found without a value.
	inner error

	// suggestion is the "did you mean" candidate returned as the hint, and
	// suggestionKind says whether it names a flag or a subcommand.
	suggestion     string
	suggestionKind suggestionKind
}

// Error returns the message, which is line 1 alone.
func (e *UsageError) Error() string {
	return e.msg
}

// Origins returns the origins of the failed value, or nil. For a value error it
// is the inner error's, then the cause chain's.
func (e *UsageError) Origins() []string {
	return errlines.Origins(e.pieces())
}

// Note returns the detail on the failure, from the inner error and then the
// cause chain, or "".
func (e *UsageError) Note() string {
	return errlines.Note(e.pieces())
}

// Hint returns the "did you mean" suggestion for an unknown flag or command,
// otherwise the hint of the inner error and then the cause chain, or "". The
// suggestion is a flag token (--port) or a subcommand name, unquoted.
func (e *UsageError) Hint() string {
	if e.msg == "" {
		return ""
	}

	if e.suggestion != "" {
		return "did you mean " + e.suggestion + "?"
	}

	return errlines.Hint(e.pieces())
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
// value when the cause has none, then what the value was for: "invalid integer
// value "x" for argument N". The origin is the line below it, from Origins.
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

	return message + " for " + valueSubject(e.flag, e.arg)
}

// Origins returns where the value came from, or nil for a command-line value.
func (e *conversionError) Origins() []string {
	return e.origin.lines()
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
	origins []string    // the "from:" texts of a group error's members or of a whole file, or nil
	msg     string      // what is wrong with the value
	note    string      // the detail on what is wrong, or ""
	hint    string      // the action that fixes the value, or ""
	cause   error       // what the check returned, or nil
}

// Error returns the message and what the value was for: "value 11 is out of
// range [1..10] for flag --size". The origin, note and hint are the lines below
// it, from Origins, Note and Hint.
func (e *inputError) Error() string {
	message := e.msg

	if subject := e.subject(); subject != "" {
		message += " for " + subject
	}

	return message
}

// Origins returns where the value came from, or nil for a command-line value.
// A group error names each member that did not come from the command line.
func (e *inputError) Origins() []string {
	if e.origins != nil {
		return e.origins
	}

	return e.origin.lines()
}

// Note returns the detail on the failure, or "".
func (e *inputError) Note() string {
	return e.note
}

// Hint returns the action that fixes the value, or "".
func (e *inputError) Hint() string {
	return e.hint
}

// Unwrap returns what the check returned, or nil when CONGA itself found the
// problem.
func (e *inputError) Unwrap() error {
	return e.cause
}

// withNote sets the detail on what is wrong.
func (e *inputError) withNote(note string) *inputError {
	e.note = note

	return e
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
	return e.target + " requires a registered config decoder"
}

// Hint says how to register a decoder.
func (e *decoderRequiredError) Hint() string {
	return "call App.RegisterDecoder with a format decoder (e.g. decoder/json)"
}

// valueOrigin is where a value came from. It holds the source tier and the
// variable, config key, or default text that diagnostics attribute the value
// to, with the file behind a config key or an environment variable.
type valueOrigin struct {
	source ValueSource
	name   string

	// file is the config file path of a config value, or the document label
	// joined with its scope when document is set. It is empty for a document
	// value with no origin.
	file string

	// document reports that file labels a programmatic document, not a path.
	document bool

	// dotenv is the .env file that set the environment variable, or "" for
	// a variable the shell exported.
	dotenv string
}

// lines returns the origin as the text of its "from:" line, or nil for a
// command-line value and for an environment or configuration value with no name.
func (origin valueOrigin) lines() []string {
	if described := origin.describe(); described != "" {
		return []string{described}
	}

	return nil
}

// describe names the origin: "default value", "environment variable NAME" (with
// the .env file that set it), or "config key" with a quoted key (with the file
// or the document label that set it). Any other origin has no name.
func (origin valueOrigin) describe() string {
	switch origin.source {
	case SourceDefault:
		return "default value"
	case SourceEnv:
		if origin.name == "" {
			return ""
		}

		if origin.dotenv != "" {
			return fmt.Sprintf("environment variable %s in .env file %q", origin.name, origin.dotenv)
		}

		return "environment variable " + origin.name
	case SourceConfig:
		if origin.name == "" {
			return ""
		}

		switch {
		case origin.file == "":
			return fmt.Sprintf("config key %q", origin.name)
		case origin.document:
			return fmt.Sprintf("config key %q set by %s", origin.name, origin.file)
		default:
			return fmt.Sprintf("config key %q in config file %q", origin.name, origin.file)
		}
	default:
		return ""
	}
}

// firstMember returns the first member with a non-empty message, or nil.
func (e Error) firstMember() error {
	for _, member := range e.members {
		if member != nil && member.Error() != "" {
			return member
		}
	}

	return nil
}

// pieces returns the error the origins, note and hint are read from: the inner
// error, else the cause, or nil when the message is empty.
func (e *UsageError) pieces() error {
	if e.msg == "" {
		return nil
	}

	if e.inner != nil {
		return e.inner
	}

	return e.cause
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

// configValueOrigin is the origin of a config value stored under key.
func configValueOrigin(key string, value configdata.Value) valueOrigin {
	return valueOrigin{source: SourceConfig, name: key, file: value.SourceDetail(), document: value.Document}
}

// envValueOrigin is the origin of an environment value read from variable name,
// set by dotenv when a .env file did.
func envValueOrigin(name, dotenv string) valueOrigin {
	return valueOrigin{source: SourceEnv, name: name, dotenv: dotenv}
}

// configFileOrigins returns the "from:" text of an error about a whole config
// file or document, with no key: config file "<path>" for a file, set by
// <label> (with the scope) for a programmatic document, and nil for a value
// that recorded no origin.
func configFileOrigins(value configdata.Value) []string {
	if origin := value.OriginText(); origin != "" {
		return []string{origin}
	}

	return nil
}

// usageError returns a [UsageError] with message msg for the command chain
// cmdChain. The with methods set the optional shape, suggestion, and cause.
func usageError(cmdChain []*spec.Command, msg string) *UsageError {
	return &UsageError{msg: msg, cmdChain: cmdChain}
}

// usageErrorFrom returns a [UsageError] for the command chain cmdChain whose
// message is err's text and whose origins, note and hint are err's.
func usageErrorFrom(cmdChain []*spec.Command, err error) *UsageError {
	return &UsageError{msg: err.Error(), cmdChain: cmdChain, inner: err}
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
		return &UsageError{msg: conversion.Error(), cmdChain: cmdChain, shape: usageShapeLineOnly, cause: conversion.cause, inner: conversion}
	}

	input, ok := errors.AsType[*inputError](err)
	if !ok || input.origin.source != SourceCLI {
		return err
	}

	shape := usageShapeMode
	if input.subject() != "" {
		shape = usageShapeLineOnly
	}

	return &UsageError{msg: input.Error(), cmdChain: cmdChain, shape: shape, cause: input.cause, inner: input}
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

// errorMembers returns the members of err that render separately. Both
// [App.Exit] and [ResolveError] call it so they always split errors the same
// way without linking [Error] into binaries that only call [App.Exit].
func errorMembers(err error) []error {
	return errlines.Members(err)
}

// formatError implements [Error.Format]: %+v prints the members' lines, and
// every other form formats line 1 with the requested format string.
func formatError(state fmt.State, verb rune, line string, members []error) {
	if verb == 'v' && state.Flag('+') {
		io.WriteString(state, errlines.Plain(members)) //nolint:errcheck // a failed write to the formatter's state has nowhere to go

		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, verb), line)
}
