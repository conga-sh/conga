package conga

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
)

// Internal sentinel errors for session lifecycle misuse.
var (
	errReloaderNotInitialized = errors.New("conga: session not initialized with reloader")
	errReloadTypeMismatch     = errors.New("conga: reload result type mismatch")
)

// Session represents an active CLI execution session.
type Session struct {
	root           any
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
	cmdChain       []*spec.Command
	rawArgs        []string
	candidateFiles []string
	loadedFiles    []string
	reloadFunc     func(ctx context.Context) (any, error)
	appName        string
	appVersion     string
	helpOptions    help.Options
}

// Stdout returns the configured standard output writer for this session.
func (s *Session) Stdout() io.Writer {
	if s == nil || s.stdout == nil {
		return io.Discard
	}

	return s.stdout
}

// Stderr returns the configured standard error writer for this session.
func (s *Session) Stderr() io.Writer {
	if s == nil || s.stderr == nil {
		return io.Discard
	}

	return s.stderr
}

// Stdin returns the configured standard input reader for this session. A nil
// session yields an immediate-EOF reader, the input-side analogue of
// io.Discard. The reader is captured when the App starts executing; changes
// made to the App configuration afterwards do not affect the active session.
func (s *Session) Stdin() io.Reader {
	if s == nil || s.stdin == nil {
		return eofReader{}
	}

	return s.stdin
}

// Help renders the active command's help screen to Stdout.
func (s *Session) Help() {
	if s == nil || s.stdout == nil {
		return
	}

	if len(s.cmdChain) > 0 {
		fmt.Fprintln(s.stdout, help.FormatWith(s.cmdChain, s.helpOptions))
	}
}

// Usage renders the syntax usage summary for the active command to Stderr.
func (s *Session) Usage() {
	if s == nil || s.stderr == nil {
		return
	}

	if len(s.cmdChain) > 0 {
		fmt.Fprintln(s.stderr, help.Usage(s.cmdChain))
	}
}

// Version renders application version information to Stdout. When no version
// string was configured, the output falls back to the module version (or
// "dev") plus the embedded build metadata.
func (s *Session) Version() {
	if s == nil || s.stdout == nil {
		return
	}

	fmt.Fprintln(s.stdout, help.VersionSummary(s.appName, s.appVersion))
}

// Path returns the full invocation path string (e.g. "cloudctl deploy worker").
func (s *Session) Path() string {
	if s == nil {
		return ""
	}

	return commandChainPath(s.cmdChain)
}

// Name returns the leaf command name.
func (s *Session) Name() string {
	if s == nil || len(s.cmdChain) == 0 {
		if s != nil {
			return s.appName
		}

		return ""
	}

	return s.cmdChain[len(s.cmdChain)-1].Name
}

// Args returns trailing or unparsed arguments for this session.
func (s *Session) Args() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.rawArgs...)
}

// CandidateConfigFiles returns the list of candidate configuration file paths evaluated for this session.
func (s *Session) CandidateConfigFiles() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.candidateFiles...)
}

// LoadedConfigFiles returns the list of actually loaded configuration file paths for this session.
func (s *Session) LoadedConfigFiles() []string {
	if s == nil {
		return nil
	}

	return append([]string(nil), s.loadedFiles...)
}

// Root returns the root application struct pointer bound to T: the instance
// supplied to New, or the engine-allocated instance created on the first schema
// build. It returns nil if the session is nil or if T does not match.
func (s *Session) Root[T any]() *T {
	if s == nil {
		return nil
	}

	if typed, ok := s.root.(*T); ok {
		return typed
	}

	return nil
}

// Reload re-reads and re-evaluates the active application configuration, returning a fresh, validated pointer to T.
// The root instance supplied to New is not updated: the reloaded values live on the returned pointer.
func (s *Session) Reload[T any](ctx context.Context) (*T, error) {
	if ctx == nil {
		panic("conga: nil Context")
	}

	if s == nil || s.reloadFunc == nil {
		return nil, errReloaderNotInitialized
	}

	reloaded, err := s.reloadFunc(ctx)
	if err != nil {
		return nil, err
	}

	typed, ok := reloaded.(*T)
	if !ok {
		return nil, fmt.Errorf("%w: type %T does not match requested type *%T", errReloadTypeMismatch, reloaded, *new(T))
	}

	return typed, nil
}

// eofReader is the nil-session input analogue of io.Discard: every Read
// returns an immediate EOF.
type eofReader struct{}

func (eofReader) Read([]byte) (int, error) { return 0, io.EOF }
