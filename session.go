package conga

import (
	"io"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

// Session provides execution context, command hierarchy inspection, configuration
// provenance, and configured I/O streams to commands and lifecycle hooks.
//
// Every lifecycle stage ([Runner.Run], [Validator.Validate], [PreRunner.PreRun],
// [PostRunner.PostRun], and [Finalizer.Finally]) receives a pointer to the active
// Session. Commands should read and write through [Session.Stdin], [Session.Stdout],
// and [Session.Stderr] rather than accessing global process streams directly.
type Session struct {
	root           any
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
	cmdChain       []*spec.Command
	candidateFiles []string
	loadedFiles    []string
	appName        string
	appVersion     string
	helpOptions    help.Options
}

// Stdout returns the configured standard output writer for this session.
//
// Commands should write output to this stream rather than [os.Stdout] to ensure
// output is captured during unit tests and embedded CLI execution. If the session
// or writer is nil, Stdout returns [io.Discard].
func (s *Session) Stdout() io.Writer {
	if s == nil || s.stdout == nil {
		return io.Discard
	}

	return s.stdout
}

// Stderr returns the configured standard error writer for this session.
//
// Commands should write diagnostic logs and progress messages to this stream rather
// than [os.Stderr]. If the session or writer is nil, Stderr returns [io.Discard].
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
// the same engine the built-in `--help` flag uses.
//
// If the session is nil or uninitialized, Help returns an empty string.
func (s *Session) Help() string {
	if s == nil || len(s.cmdChain) == 0 {
		return ""
	}

	return help.FormatWith(s.cmdChain, s.helpOptions)
}

// Usage returns the syntax usage summary for the active command.
//
// If the session is nil or uninitialized, Usage returns an empty string.
func (s *Session) Usage() string {
	if s == nil || len(s.cmdChain) == 0 {
		return ""
	}

	return help.Usage(s.cmdChain)
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

// Root returns a pointer to the root application struct of type T: either the
// instance supplied to [New], or the engine-allocated instance created during schema
// compilation.
//
// In a subcommand's [Runner.Run] or lifecycle hook, use Root to inspect global flags
// and shared settings declared on the root command:
//
//	func (m *MigrateCLI) Run(ctx context.Context, session *conga.Session) error {
//		root := session.Root[AppCLI]()
//		if root != nil && root.Verbose.Get() {
//			fmt.Fprintln(session.Stdout(), "Verbose logging active")
//		}
//		return nil
//	}
//
// Returns nil if the session is nil or if T does not match the actual root struct type.
func (s *Session) Root[T any]() *T {
	if s == nil {
		return nil
	}

	if typed, ok := s.root.(*T); ok {
		return typed
	}

	return nil
}

// eofReader is the nil-session input analogue of io.Discard: every Read
// returns an immediate EOF.
type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }
