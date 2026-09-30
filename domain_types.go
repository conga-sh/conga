package conga

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/filex"
	"github.com/conga-sh/conga/internal/pathx"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

const maxFileContentSize int64 = 16 * 1024 * 1024

const (
	pathKindExpectFile           pathKind = iota // path is a directory, but a file was expected
	pathKindExpectDir                            // path is a file, but a directory was expected
	pathKindExpectExecutableFile                 // path is a directory, but an executable file was expected
	pathKindExpectSymlink                        // path is not a symbolic link
)

const (
	byteSizeErrorEmpty         byteSizeErrorKind = iota // empty input
	byteSizeErrorInvalid                                // no leading numeric prefix
	byteSizeErrorInvalidNumber                          // numeric prefix does not parse
	byteSizeErrorUnknownUnit                            // unrecognized unit suffix
)

// File represents a filesystem path with optional capabilities declared
// through custom field tags: existing:"true" requires the path to exist and
// be a regular file, stdin:"true" permits "-" to read standard input,
// stdout:"true" permits "-" to write standard output, executable:"true"
// requires the path to be an executable regular file with $PATH-relative
// names resolved at decode time, and symlink:"true" requires the path to be
// a symbolic link. The glob:"..." tag is completion-only: it filters the
// suggested file candidates against comma-separated glob patterns matched
// on entry names (e.g. glob:"Makefile,Dockerfile" or glob:"*.yaml") and
// cannot be combined with the executable or symlink capabilities. Values
// expand environment variables and tilde paths at decode time, capturing
// the field's declared tags and the engine-configured standard streams
// (App.Stdin/App.Stdout for values bound through an App run); tag-driven
// validation runs during the validation phase.
//
//nolint:recvcheck // Inspection hooks (String/Help/Predict/TagSpecs/Validate) use pointer receivers for nil-safety; file accessors (IsStdio/Reader/Writer/Stat) use value receivers.
type File struct {
	path        string
	existing    bool
	stdin       bool
	stdout      bool
	executable  bool
	symlink     bool
	stdinSource io.Reader // engine-configured stdin (App.Stdin); nil falls back to os.Stdin
	stdoutSink  io.Writer // engine-configured stdout (App.Stdout); nil falls back to os.Stdout
}

// TagSpecs registers the custom field tags accepted by File.
func (f *File) TagSpecs() []spec.TagSpec {
	return []spec.TagSpec{
		{Name: "existing", Kind: spec.TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdin", Kind: spec.TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdout", Kind: spec.TagKindBool, Excludes: []string{"stdin", "existing", "executable", "symlink"}},
		{Name: "executable", Kind: spec.TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "symlink", "glob"}},
		{Name: "symlink", Kind: spec.TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "executable", "glob"}},
		{Name: "glob", Kind: spec.TagKindString, Excludes: []string{"executable", "symlink"}, Validate: validateGlobTagValue},
	}
}

// Validate enforces the tags captured at decode time against the resolved
// path: "-" requires the stdin or stdout capability, the symlink capability
// requires the path to be a symbolic link, the executable capability requires
// the path to exist and be an executable regular file, and the existing
// capability requires the path to exist and be a regular file.
func (f *File) Validate(ctx context.Context) error {
	if f == nil {
		return nil
	}

	if f.path == "-" {
		if f.stdin || f.stdout {
			return nil
		}

		return errStdioCapabilityRequired
	}

	if f.symlink {
		info, err := os.Lstat(f.path)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			return &pathKindError{path: f.path, kind: pathKindExpectSymlink}
		}

		return nil
	}

	if f.executable {
		return validateExecutablePath(f.path)
	}

	if f.existing {
		info, err := os.Stat(f.path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return &pathKindError{path: f.path, kind: pathKindExpectFile}
		}
	}

	return nil
}

// Predict returns file autocompletion candidates, tailored to the field's
// declared tags: executables from $PATH for executable fields, symlinks for
// symlink fields, glob-filtered file paths for glob fields, and file paths
// otherwise.
func (f *File) Predict(tags FieldTags) predict.Predictor {
	switch {
	case tags.Bool("executable"):
		return predict.Executables()
	case tags.Bool("symlink"):
		return predict.Symlinks()
	case tags.Has("glob"):
		return predict.FilesGlob(fileGlobPatterns(tags)...)
	default:
		return predict.Files()
	}
}

// String returns the file path.
func (f *File) String() string {
	if f == nil {
		return ""
	}

	return f.path
}

// UnmarshalText expands environment variables and tilde paths. Plain text
// unmarshaling carries no field tags or engine streams, so the decoded value
// is unconstrained and "-" wiring falls back to the process stdio streams;
// the engine delivers declared tags and configured streams via
// UnmarshalTextWithContext.
func (f *File) UnmarshalText(text []byte) error {
	return f.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext expands environment variables and tilde paths,
// captures the field's declared capability tags for Validate, and captures
// the engine-configured standard streams so "-" in Reader/Writer follows
// App.Stdin/App.Stdout for values bound through an App run. Fields declaring
// the executable capability additionally trim their value and resolve
// $PATH-relative names to their absolute location at decode time.
func (f *File) UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error {
	f.path = pathx.Expand(string(text))
	f.applyTags(decodeContext.Tags)
	f.stdinSource = decodeContext.Stdin
	f.stdoutSink = decodeContext.Stdout

	if !f.executable {
		return nil
	}

	raw := strings.TrimSpace(string(text))
	if raw == "" {
		f.path = ""

		return nil
	}

	expanded := pathx.Expand(raw)

	if strings.ContainsAny(expanded, "/\\") || filepath.IsAbs(expanded) {
		f.path = expanded

		return nil
	}

	foundPath, err := exec.LookPath(expanded)
	if err != nil {
		return err
	}

	f.path = foundPath

	return nil
}

// Help returns the default help description for file flags, tailored to the
// field's declared tags.
func (f *File) Help(tags FieldTags) string {
	switch {
	case tags.Bool("executable"):
		return "Path to an executable"
	case tags.Bool("symlink"):
		return "Path to a symbolic link"
	}

	helpText := "Path to a file"
	if tags.Bool("existing") {
		helpText = "Path to an existing file"
	}

	if patterns := fileGlobPatterns(tags); len(patterns) > 0 {
		helpText += " matching '" + strings.Join(patterns, ",") + "'"
	}

	switch {
	case tags.Has("stdin"):
		return helpText + " or '-' for stdin"
	case tags.Has("stdout"):
		return helpText + " or '-' for stdout"
	default:
		return helpText
	}
}

// IsStdio reports whether the value is "-" (a standard stream). The direction
// follows from the accessor: Reader() wires "-" to standard input and
// Writer() wires it to standard output.
func (f File) IsStdio() bool {
	return f.path == "-"
}

// Reader opens the file for reading. When the value is "-", it wraps the
// engine-configured standard input (App.Stdin) for values bound through an
// App run, falling back to os.Stdin for directly-constructed values. Either
// wrapper is non-closing so caller Close() calls never close the stream.
func (f File) Reader() (io.ReadCloser, error) {
	if f.IsStdio() {
		if f.stdinSource != nil {
			return io.NopCloser(f.stdinSource), nil
		}

		return io.NopCloser(os.Stdin), nil
	}

	return os.Open(f.path)
}

// Writer creates or truncates the file for writing. When the value is "-", it
// returns a non-closing wrapper around the engine-configured standard output
// (App.Stdout) for values bound through an App run, falling back to
// os.Stdout for directly-constructed values.
func (f File) Writer() (io.WriteCloser, error) {
	if f.IsStdio() {
		if f.stdoutSink != nil {
			return nopWriteCloser{Writer: f.stdoutSink}, nil
		}

		return nopWriteCloser{Writer: os.Stdout}, nil
	}

	return os.Create(f.path)
}

// Stat returns the os.FileInfo for the file.
func (f File) Stat() (os.FileInfo, error) {
	return os.Stat(f.path)
}

// FileContent represents file contents loaded into memory during value decoding.
// Supports "-" to read from standard input: the engine-configured standard input
// (App.Stdin) for values bound through an App run, os.Stdin otherwise.
//
//nolint:recvcheck // Inspection hooks (String/Help/Predict) use value receivers for value semantics; text unmarshaling needs pointer receivers to mutate the value.
type FileContent string

// Bytes returns the raw bytes of the file content.
func (f FileContent) Bytes() []byte {
	return []byte(f)
}

// String returns the file content as a string.
func (f FileContent) String() string {
	return string(f)
}

// Help returns the default help description for file content flags.
func (f FileContent) Help() string {
	return "Path to a file (or '-' for stdin)"
}

// Predict returns file autocompletion candidates.
func (f FileContent) Predict() predict.Predictor {
	return predict.Files()
}

// UnmarshalText reads the file at path (or os.Stdin if "-") into memory,
// enforcing a 16MB DoS limit (maxFileContentSize).
func (f *FileContent) UnmarshalText(text []byte) error {
	return f.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext reads the file at path into memory, enforcing a
// 16MB DoS limit (maxFileContentSize). "-" reads the engine-configured
// standard input (App.Stdin) for values bound through an App run, falling
// back to os.Stdin for directly-constructed values.
func (f *FileContent) UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error {
	raw := string(text)
	if raw == "-" {
		stdin := decodeContext.Stdin
		if stdin == nil {
			stdin = os.Stdin
		}

		data, err := filex.ReadLimitedReader(stdin, maxFileContentSize)
		if err != nil {
			return err
		}

		*f = FileContent(data)

		return nil
	}

	path := pathx.Expand(raw)

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return &pathKindError{path: path, kind: pathKindExpectFile}
	}

	data, err := filex.ReadLimited(path, maxFileContentSize)
	if err != nil {
		return err
	}

	*f = FileContent(data)

	return nil
}

// Dir represents a filesystem directory path with an optional existence
// capability declared through the custom field tag existing:"true" (the path
// must exist and be a directory). Values expand environment variables and
// tilde paths at decode time, capturing the field's declared tags; tag-driven
// validation runs during the validation phase.
type Dir struct {
	path     string
	existing bool
}

// TagSpecs registers the custom field tags accepted by Dir.
func (d *Dir) TagSpecs() []spec.TagSpec {
	return []spec.TagSpec{
		{Name: "existing", Kind: spec.TagKindBool},
	}
}

// Validate enforces the tags captured at decode time against the resolved
// path: the existing capability requires the path to exist and be a
// directory.
func (d *Dir) Validate(ctx context.Context) error {
	if d == nil {
		return nil
	}

	if !d.existing {
		return nil
	}

	info, err := os.Stat(d.path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return &pathKindError{path: d.path, kind: pathKindExpectDir}
	}

	return nil
}

// Predict returns directory autocompletion candidates. Unlike File.Predict it
// takes no field tags: Dir's only custom tag (existing:"true") constrains
// validation, not the candidate set, so the candidates are always directories.
// This keeps the parameterless Predictable form usable for domain types whose
// candidates never depend on declared tags.
func (d *Dir) Predict() predict.Predictor {
	return predict.Dirs()
}

// String returns the directory path.
func (d *Dir) String() string {
	if d == nil {
		return ""
	}

	return d.path
}

// UnmarshalText expands environment variables and tilde paths. Plain text
// unmarshaling carries no field tags, so the decoded value is unconstrained;
// the engine delivers declared tags via UnmarshalTextWithContext.
func (d *Dir) UnmarshalText(text []byte) error {
	return d.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext expands environment variables and tilde paths and
// captures the field's declared capability tags for Validate.
func (d *Dir) UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error {
	d.path = pathx.Expand(string(text))
	d.existing = decodeContext.Tags.Bool("existing")

	return nil
}

// Help returns the default help description for directory flags, tailored to
// the field's declared tags.
func (d *Dir) Help(tags FieldTags) string {
	if tags.Bool("existing") {
		return "Path to an existing directory"
	}

	return "Path to a directory"
}

// ByteSize represents human-readable byte sizes (e.g. "10MiB", "1.5GB", "500KiB").
type ByteSize int64

// Bytes returns the number of bytes as int64.
func (b *ByteSize) Bytes() int64 {
	if b == nil {
		return 0
	}

	return int64(*b)
}

// String returns a formatted byte size representation using deterministic
// IEC binary notation (KiB, MiB, GiB, TiB, PiB).
func (b *ByteSize) String() string {
	if b == nil || *b == 0 {
		return "0 B"
	}

	value := float64(*b)

	switch {
	case value >= 1024*1024*1024*1024*1024:
		return fmt.Sprintf("%.2f PiB", value/(1024*1024*1024*1024*1024))
	case value >= 1024*1024*1024*1024:
		return fmt.Sprintf("%.2f TiB", value/(1024*1024*1024*1024))
	case value >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GiB", value/(1024*1024*1024))
	case value >= 1024*1024:
		return fmt.Sprintf("%.2f MiB", value/(1024*1024))
	case value >= 1024:
		return fmt.Sprintf("%.2f KiB", value/1024)
	default:
		return fmt.Sprintf("%d B", int64(*b))
	}
}

// UnmarshalText parses a strict, case-sensitive byte size. It accepts exact
// byte counts ("1024", "500B"), SI metric units ending in "B" ("kB", "KB",
// "MB", "GB", "TB", "PB"), and IEC binary units ("KiB", "MiB", "GiB", "TiB",
// "PiB"). Naked prefixes ("10G", "512Mi") and invalid casings ("mb", "kib")
// are rejected.
func (b *ByteSize) UnmarshalText(text []byte) error {
	rawText := strings.TrimSpace(string(text))
	if rawText == "" {
		return &byteSizeError{kind: byteSizeErrorEmpty}
	}

	i := 0
	for i < len(rawText) && (rawText[i] == '.' || (rawText[i] >= '0' && rawText[i] <= '9')) {
		i++
	}

	numStr := rawText[:i]
	unitStr := rawText[i:]

	if numStr == "" {
		return &byteSizeError{raw: rawText, kind: byteSizeErrorInvalid}
	}

	if unitStr == "" {
		n, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil || n < 0 {
			return &byteSizeError{raw: numStr, kind: byteSizeErrorInvalidNumber}
		}

		*b = ByteSize(n)

		return nil
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return &byteSizeError{raw: numStr, kind: byteSizeErrorInvalidNumber}
	}

	multiplier, ok := byteSizeUnitMultiplier(unitStr)
	if !ok {
		return &byteSizeError{raw: rawText, unit: unitStr, kind: byteSizeErrorUnknownUnit}
	}

	*b = ByteSize(int64(num * multiplier))

	return nil
}

// Help returns the default help description for byte-size flags.
func (b *ByteSize) Help() string {
	return "Byte size (e.g., 10MiB, 1.5GB, 500KiB)"
}

// pathKind identifies the filesystem expectation violated by a path value.
type pathKind uint8

// pathKindError is the root cause for path-kind mismatches: path and kind are
// the structured payload, Error composes the user-facing prose, and Unwrap
// links the errPathWrongKind → ErrCoerce taxonomy for errors.Is.
type pathKindError struct {
	path string
	kind pathKind
}

func (e *pathKindError) Error() string {
	switch e.kind {
	case pathKindExpectFile:
		return fmt.Sprintf("%q is a directory, but a file was expected", e.path)
	case pathKindExpectDir:
		return fmt.Sprintf("%q is a file, but a directory was expected", e.path)
	case pathKindExpectExecutableFile:
		return fmt.Sprintf("%q is a directory, but an executable file was expected", e.path)
	case pathKindExpectSymlink:
		return fmt.Sprintf("%q is not a symbolic link", e.path)
	default:
		return fmt.Sprintf("invalid path %q", e.path)
	}
}

func (e *pathKindError) Unwrap() error { return errPathWrongKind }

// executableError is the root cause for permission deficits on regular files
// (correct path kind, missing execute bit).
type executableError struct {
	path string
}

func (e *executableError) Error() string {
	return fmt.Sprintf("%q is not executable", e.path)
}

func (e *executableError) Unwrap() error { return ErrValidation }

// byteSizeErrorKind identifies the byte-size parse failure mode.
type byteSizeErrorKind uint8

// byteSizeError is the root cause for byte-size parse failures.
type byteSizeError struct {
	raw  string // full input (invalid/unknownUnit) or numeric prefix (invalidNumber)
	unit string // offending unit (unknownUnit only)
	kind byteSizeErrorKind
}

func (e *byteSizeError) Error() string {
	switch e.kind {
	case byteSizeErrorEmpty:
		return "empty byte size"
	case byteSizeErrorInvalid:
		return fmt.Sprintf("invalid byte size %q", e.raw)
	case byteSizeErrorInvalidNumber:
		return fmt.Sprintf("invalid byte size number %q", e.raw)
	case byteSizeErrorUnknownUnit:
		return fmt.Sprintf("unknown byte size unit %q in %q", e.unit, e.raw)
	default:
		return fmt.Sprintf("invalid byte size %q", e.raw)
	}
}

func (e *byteSizeError) Unwrap() error { return ErrCoerce }

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error {
	return nil
}

// applyTags captures the declared capability tags on the value.
func (f *File) applyTags(tags FieldTags) {
	f.existing = tags.Bool("existing")
	f.stdin = tags.Bool("stdin")
	f.stdout = tags.Bool("stdout")
	f.executable = tags.Bool("executable")
	f.symlink = tags.Bool("symlink")
}

// validateGlobTagValue adapts spec.ParseGlobPatterns to the TagSpec.Validate
// hook; ParseGlobPatterns errors already wrap ErrInvalidStructTag and are
// propagated unchanged by ParseFieldTags.
func validateGlobTagValue(value string) error {
	_, err := spec.ParseGlobPatterns(value)

	return err
}

// fileGlobPatterns parses the glob tag value declared on the field. Nil is
// returned for absent or schema-invalid values (schema build rejects those
// before execution; hand-built tag maps yield no file candidates, since an
// invalid glob can never match).
func fileGlobPatterns(tags FieldTags) []string {
	patterns, err := spec.ParseGlobPatterns(tags.String("glob"))
	if err != nil {
		return nil
	}

	return patterns
}

func validateExecutablePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return &pathKindError{path: path, kind: pathKindExpectExecutableFile}
	}

	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		return &executableError{path: path}
	}

	return nil
}

func byteSizeUnitMultiplier(unit string) (float64, bool) {
	switch unit {
	case "B":
		return 1, true
	case "kB", "KB":
		return 1000, true
	case "MB":
		return 1000 * 1000, true
	case "GB":
		return 1000 * 1000 * 1000, true
	case "TB":
		return 1000 * 1000 * 1000 * 1000, true
	case "PB":
		return 1000 * 1000 * 1000 * 1000 * 1000, true
	case "KiB":
		return 1024, true
	case "MiB":
		return 1024 * 1024, true
	case "GiB":
		return 1024 * 1024 * 1024, true
	case "TiB":
		return 1024 * 1024 * 1024 * 1024, true
	case "PiB":
		return 1024 * 1024 * 1024 * 1024 * 1024, true
	default:
		return 0, false
	}
}
