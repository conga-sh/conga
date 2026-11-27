package conga

import (
	"fmt"
	"io"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

// Session provides execution context, command hierarchy inspection, configuration
// provenance, and configured I/O streams to commands and lifecycle hooks.
//
// Every lifecycle stage ([Runner.Run], [Validator.Validate], [PreRunner.PreRun],
// [PostRunner.PostRun], and [Finalizer.Finally]) receives a pointer to the active
// Session.
type Session struct {
	root           any
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
	cmdChain       []*spec.Command
	candidateFiles []string
	// stdinClaim records the first flag or argument value of the run that
	// reads standard input, shared by flag binding and positional binding.
	stdinClaim    stdinClaim
	skippedFiles  []string
	loadedFiles   []string
	envPrefix     string
	appName       string
	appVersion    string
	helpOptions   help.Options
	whyRenderer   func(s *Session, format string) string
	mapProvenance map[*spec.Flag]mapFlagProvenance
	// flagOrigins records where each flag's value came from, so a
	// SelfValidator failure that surfaces after binding, or an error built with
	// [Session.Error], names the file or .env file behind it.
	flagOrigins map[*spec.Flag]valueOrigin
}

// Stdout returns the configured standard output writer for this session.
//
// The writer is captured when execution begins, so unit tests and embedded
// execution observe the application's output. If the session or writer is nil,
// Stdout returns [io.Discard].
func (s *Session) Stdout() io.Writer {
	if s == nil || s.stdout == nil {
		return io.Discard
	}

	return s.stdout
}

// Stderr returns the configured standard error writer for this session.
//
// If the session or writer is nil, Stderr returns [io.Discard].
func (s *Session) Stderr() io.Writer {
	if s == nil || s.stderr == nil {
		return io.Discard
	}

	return s.stderr
}

// Stdin returns the configured standard input reader for this session.
//
// If the session or reader is nil, Stdin returns an immediate-EOF reader (the
// input-side analogue of [io.Discard]), ensuring commands reading from stdin do not
// block or panic when unconfigured.
//
// The stream reader is snapshotted when execution begins; changes made to [App]
// afterwards do not affect the running session.
func (s *Session) Stdin() io.Reader {
	if s == nil || s.stdin == nil {
		return eofReader{}
	}

	return s.stdin
}

// Help returns the formatted help screen for the active command, rendered with
// the same engine the built-in --help flag uses.
//
// If the session is nil or uninitialized, Help returns an empty string.
func (s *Session) Help() string {
	if s == nil || len(s.cmdChain) == 0 {
		return ""
	}

	return help.FormatWith(s.cmdChain, s.helpOptions)
}

// Synopsis returns the active command's "Usage:" syntax lines, such as
// "Usage:\n  tool deploy [flags]". [Session.Help] returns the full help
// screen.
//
// If the session is nil or uninitialized, Synopsis returns an empty string.
func (s *Session) Synopsis() string {
	if s == nil || len(s.cmdChain) == 0 {
		return ""
	}

	return help.Synopsis(s.cmdChain)
}

// Version returns the formatted application version summary line.
//
// When no explicit version string was configured via [App.Version], the summary
// falls back to the Go module version recorded in the binary, or "dev" when that
// is unavailable. An embedded VCS commit hash and date are appended when present.
//
// If the session is nil, Version returns an empty string.
func (s *Session) Version() string {
	if s == nil {
		return ""
	}

	return help.VersionSummary(s.appName, s.appVersion)
}

// Path returns the full space-separated invocation path string for the active command
// (e.g. "cloudctl cluster deploy").
func (s *Session) Path() string {
	if s == nil {
		return ""
	}

	return commandChainPath(s.cmdChain)
}

// Name returns the leaf command name being executed, or the application name if
// executing the root command.
func (s *Session) Name() string {
	if s == nil || len(s.cmdChain) == 0 {
		if s != nil {
			return s.appName
		}

		return ""
	}

	return s.cmdChain[len(s.cmdChain)-1].Name
}

// CandidateConfigFiles returns a copy of all candidate configuration file paths
// evaluated during configuration resolution, including search-tier paths that were
// absent on disk.
func (s *Session) CandidateConfigFiles() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.candidateFiles...)
}

// LoadedConfigFiles returns a copy of all configuration file paths that were
// successfully loaded and merged for this session, ordered from lowest to highest
// precedence.
func (s *Session) LoadedConfigFiles() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.loadedFiles...)
}

// Why returns the formatted value-provenance report for the active command,
// rendered with the same engine the built-in --why flag uses.
//
// If the session is nil, uninitialized, or Why reporting is not enabled on the
// application, Why returns an empty string.
func (s *Session) Why() string {
	if s == nil || s.whyRenderer == nil {
		return ""
	}

	return s.whyRenderer(s, "text")
}

// WhyJSON returns the machine-readable JSON value-provenance report for the
// active command, rendered with the same engine --why=json uses.
//
// If the session is nil, uninitialized, or [WhyOptionJSON] is not enabled on the
// application, WhyJSON returns an empty string.
func (s *Session) WhyJSON() string {
	if s == nil || s.whyRenderer == nil {
		return ""
	}

	return s.whyRenderer(s, "json")
}

// UsageError returns a [*UsageError] for message, reporting a mistake in the
// user's input that only the command can detect, such as a combination of
// values that is invalid together. [App.Exit] prints the message and the usage
// block of the active command that [UsageOnErrorMode] asks for, then exits with
// status 2.
//
// Return it from [Validator.Validate] or any other hook. A plain error from a
// hook exits with status 1 and prints no usage block. A message that is empty
// prints no error line, only the usage block.
//
// On a nil or uninitialized session the error has no command, so [App.Exit]
// prints no usage block.
func (s *Session) UsageError(message string) error {
	return usageError(s.chain(), message)
}

// UsageErrorf returns a [*UsageError] whose message is formatted like
// [fmt.Errorf]. A %w operand becomes the error's cause, reachable through
// [errors.Is], [errors.As], and [errors.Unwrap]. See [Session.UsageError].
func (s *Session) UsageErrorf(format string, args ...any) error {
	formatted := fmt.Errorf(format, args...) //nolint:err113 // the caller supplies the format, as with fmt.Errorf

	return usageError(s.chain(), formatted.Error()).withCause(formattedCause(formatted))
}

// Error returns an [ErrorBuilder] that classifies errors by the sources of
// named [Input] containers and attaches "from:", "note:", and "hint:" lines.
//
// Call [ErrorBuilder.For] with the inputs the check read, optionally call
// [ErrorBuilder.Note], [ErrorBuilder.Hint], or [ErrorBuilder.Usage], and finish
// with [ErrorBuilder.New], [ErrorBuilder.Newf], [ErrorBuilder.Wrap], or
// [ErrorBuilder.Wrapf]. On a nil or uninitialized session, the builder never
// panics and classifies from each container's own Source without "from:" lines
// or a usage block.
func (s *Session) Error() ErrorBuilder {
	return ErrorBuilder{session: s}
}

// Root returns a pointer to the root application struct of type T: either the
// instance supplied to [New], or the engine-allocated instance created during schema
// compilation.
//
// In a subcommand's [Runner.Run] or lifecycle hook, use Root to inspect global flags
// and shared settings declared on the root command.
//
// Returns nil if the session is nil or if no root command instance exists.
//
// Panics:
//   - If T does not match the actual root struct type.
func (s *Session) Root[T any]() *T {
	if s == nil || s.root == nil {
		return nil
	}

	root, ok := s.root.(*T)
	if !ok {
		panic(fmt.Sprintf("conga: Session.Root[%T] called, but root command type is %T", *new(T), s.root))
	}

	return root
}

// eofReader is the nil-session input analogue of io.Discard: every Read
// returns an immediate EOF.
type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }

// chain returns the session's command chain, or nil for a nil session.
func (s *Session) chain() []*spec.Command {
	if s == nil {
		return nil
	}

	return s.cmdChain
}

// verifyInputsBound panics if any input in inputs is not bound to a command in
// s.cmdChain.
func (s *Session) verifyInputsBound(inputs []Input) {
	for _, input := range inputs {
		if !s.isInputBound(input) {
			panic("conga: Session.Error().For: input is not bound to the active command")
		}
	}
}

func (s *Session) isInputBound(input Input) bool {
	if input == nil || isNilPointerInput(input) {
		return false
	}

	for _, cmd := range s.cmdChain {
		if bound, _, _ := s.resolveCommandInput(cmd, input); bound {
			return true
		}
	}

	return false
}

// resolveInputOrigin returns the bound input's resolved source and its "from:"
// line text ("--<flag>: <described>" or "argument <NAME>: <described>"), or ""
// for command-line or unset inputs.
func (s *Session) resolveInputOrigin(input Input) (ValueSource, string) {
	for _, cmd := range s.cmdChain {
		if bound, source, origin := s.resolveCommandInput(cmd, input); bound {
			return source, origin
		}
	}

	return SourceNone, ""
}

func (s *Session) resolveCommandInput(cmd *spec.Command, input Input) (bool, ValueSource, string) {
	for _, flagSpec := range cmd.Flags {
		if flagSpec.Handle != nil && flagSpec.Handle == input {
			return true, flagStateOf(flagSpec.Handle).Source(), s.flagOriginLine(flagSpec)
		}
	}

	for _, argSpec := range cmd.Args {
		if argSpec.Handle != nil && argSpec.Handle == input {
			return true, argStateOf(argSpec.Handle).Source(), argOriginLine(argSpec)
		}
	}

	if cmd.DashArgs != nil && cmd.DashArgs.Handle != nil && cmd.DashArgs.Handle == input {
		return true, trailingInputSource(input), ""
	}

	if cmd.RawArgs != nil && cmd.RawArgs.Handle != nil && cmd.RawArgs.Handle == input {
		return true, trailingInputSource(input), ""
	}

	return false, SourceNone, ""
}

func (s *Session) flagOriginLine(flagSpec *spec.Flag) string {
	source := flagStateOf(flagSpec.Handle).Source()
	if source == SourceCLI || source == SourceNone {
		return ""
	}

	described := originOf(s.flagOrigins, flagSpec, source).describe()
	if described == "" {
		return ""
	}

	return "--" + flagSpec.Name + ": " + described
}

func argOriginLine(argSpec *spec.Arg) string {
	source := argStateOf(argSpec.Handle).Source()
	if source == SourceCLI || source == SourceNone {
		return ""
	}

	described := valueOrigin{source: source}.describe()
	if described == "" {
		return ""
	}

	return valueSubject(nil, argSpec) + ": " + described
}
