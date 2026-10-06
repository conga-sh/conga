package conga

import (
	"context"
	"encoding"
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

var (
	_ fmt.Stringer             = File{}
	_ encoding.TextMarshaler   = File{}
	_ encoding.TextUnmarshaler = (*File)(nil)

	_ fmt.Stringer             = Dir{}
	_ encoding.TextMarshaler   = Dir{}
	_ encoding.TextUnmarshaler = (*Dir)(nil)

	_ fmt.Stringer             = ByteSize(0)
	_ encoding.TextMarshaler   = ByteSize(0)
	_ encoding.TextUnmarshaler = (*ByteSize)(nil)

	_ fmt.Stringer             = FileContent("")
	_ encoding.TextMarshaler   = FileContent("")
	_ encoding.TextUnmarshaler = (*FileContent)(nil)
)

// File represents a filesystem path with optional capability constraints declared
// through struct tags on command fields:
//   - `existing:"true"`: Requires the path to exist and not be a directory.
//   - `stdin:"true"`: Permits "-" to bind to standard input ([App.Stdin] or [os.Stdin]).
//   - `stdout:"true"`: Permits "-" to bind to standard output ([App.Stdout] or [os.Stdout]).
//   - `executable:"true"`: Requires the path to exist, not be a directory, have execute
//     permission, and resolves $PATH-relative names to their absolute location at decode time.
//   - `symlink:"true"`: Requires the path to be a symbolic link.
//   - `glob:"pattern"`: Shell autocompletion filter using comma-separated glob patterns
//     (e.g. `glob:"*.yaml,*.yml"` or `glob:"Dockerfile*"`).
//
// These tags are mutually exclusive in part, because they constrain the same path
// in incompatible ways. Declaring a conflicting pair fails schema compilation:
//   - `existing` conflicts with `stdout`, `executable`, and `symlink`.
//   - `stdin` conflicts with `stdout`, `executable`, and `symlink`.
//   - `stdout` conflicts with `stdin`, `existing`, `executable`, and `symlink`.
//   - `executable` conflicts with `existing`, `stdin`, `stdout`, `symlink`, and `glob`.
//   - `symlink` conflicts with `existing`, `stdin`, `stdout`, `executable`, and `glob`.
//   - `glob` conflicts with `executable` and `symlink`.
//
// File paths automatically expand tilde prefixes (`~` to the user's home directory)
// and environment variables (`$VAR` or `${VAR}`) during value decoding.
//
// In command handlers, call [File.Reader] to open the file (or stdin if "-") or
// [File.Writer] to create/truncate the file (or stdout if "-"):
//
//	type TransformCLI struct {
//		Input  conga.RequiredFlag[conga.File] `short:"i" stdin:"true" help:"Input file or - for stdin"`
//		Output conga.RequiredFlag[conga.File] `short:"o" stdout:"true" help:"Output file or - for stdout"`
//	}
//
//	func (t *TransformCLI) Run(ctx context.Context, session *conga.Session) error {
//		reader, err := t.Input.Get().Reader()
//		if err != nil {
//			return err
//		}
//		defer reader.Close()
//		// Process reader...
//		return nil
//	}
//
// See https://docs.conga.sh/reference/domain-types/.
//
//nolint:recvcheck // Inspection hooks (String/MarshalText/Help/Predict/IsStdio/Reader/Writer/Stat) use value receivers for value semantics; mutating and decoding hooks (UnmarshalText/UnmarshalTextWithContext/Validate/TagSpecs) use pointer receivers.
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

// TagSpecs registers the custom struct tags accepted by [File] fields.
func (f *File) TagSpecs() []TagSpec {
	return []TagSpec{
		{Name: "existing", Kind: TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdin", Kind: TagKindBool, Excludes: []string{"stdout", "executable", "symlink"}},
		{Name: "stdout", Kind: TagKindBool, Excludes: []string{"stdin", "existing", "executable", "symlink"}},
		{Name: "executable", Kind: TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "symlink", "glob"}},
		{Name: "symlink", Kind: TagKindBool, Excludes: []string{"existing", "stdin", "stdout", "executable", "glob"}},
		{Name: "glob", Kind: TagKindString, Excludes: []string{"executable", "symlink"}, Validate: validateGlobTagValue},
	}
}

// Validate verifies that the resolved path satisfies the capability tags declared
// on the owning field. The `glob` tag is a completion filter and is not validated.
//
// Enforced rules:
//   - If the value is "-", the field must declare either `stdin:"true"` or `stdout:"true"`.
//   - If `existing:"true"` is declared, the path must exist and cannot be a directory.
//   - If `executable:"true"` is declared, the file must exist, cannot be a directory, and must have execute permissions.
//   - If `symlink:"true"` is declared, the path must be a symbolic link.
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

// Predict returns shell autocompletion candidates tailored to the field's declared tags.
//
// Generates executable candidates from $PATH for `executable:"true"` fields, symlinks for
// `symlink:"true"`, glob-filtered filenames for `glob:"..."`, and standard filesystem
// files otherwise.
func (f File) Predict(tags FieldTags) predict.Predictor {
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

// String returns the resolved filesystem path string.
func (f File) String() string {
	return f.path
}

// MarshalText returns the resolved filesystem path as UTF-8 bytes.
func (f File) MarshalText() ([]byte, error) {
	return []byte(f.path), nil
}

// UnmarshalText unmarshals a raw path string, expanding environment variables and
// tilde prefixes. Standard text unmarshaling carries no field tags or engine streams,
// so the path is unconstrained and "-" streams fall back to [os.Stdin]/[os.Stdout].
func (f *File) UnmarshalText(text []byte) error {
	return f.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext decodes a path string using ambient field tags and configured
// standard streams ([App.Stdin] / [App.Stdout]).
//
// Expands environment variables and tilde prefixes, captures declared capability tags
// for [File.Validate], and resolves $PATH-relative names to absolute paths when
// `executable:"true"` is declared.
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

// Help returns the default value syntax description for file flags, tailored to
// declared capability tags (e.g. "Path to an existing file or '-' for stdin").
func (f File) Help(tags FieldTags) string {
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

// IsStdio reports whether the file path is "-" (representing standard input or standard output).
func (f File) IsStdio() bool {
	return f.path == "-"
}

// Reader opens the file for reading.
//
// If the path is "-", Reader returns a non-closing wrapper around [App.Stdin] (or
// [os.Stdin] if unconfigured). Callers can safely call Close() on the returned
// [io.ReadCloser] without closing the underlying process standard input stream.
func (f File) Reader() (io.ReadCloser, error) {
	if f.IsStdio() {
		if f.stdinSource != nil {
			return io.NopCloser(f.stdinSource), nil
		}

		return io.NopCloser(os.Stdin), nil
	}

	return os.Open(f.path)
}

// Writer creates or truncates the file for writing.
//
// If the path is "-", Writer returns a non-closing wrapper around [App.Stdout] (or
// [os.Stdout] if unconfigured). Callers can safely call Close() on the returned
// [io.WriteCloser] without closing the underlying process standard output stream.
func (f File) Writer() (io.WriteCloser, error) {
	if f.IsStdio() {
		if f.stdoutSink != nil {
			return nopWriteCloser{Writer: f.stdoutSink}, nil
		}

		return nopWriteCloser{Writer: os.Stdout}, nil
	}

	return os.Create(f.path)
}

// Stat returns [os.FileInfo] for the resolved file path.
func (f File) Stat() (os.FileInfo, error) {
	return os.Stat(f.path)
}

// FileContent represents the in-memory contents of a file read during argument decoding.
//
// If the value is "-", the content is read into memory from [App.Stdin] (or [os.Stdin]).
// FileContent enforces a strict 16MB file size limit to prevent out-of-memory DoS
// vulnerabilities:
//
//	type ConfigCLI struct {
//		Cert conga.RequiredFlag[conga.FileContent] `short:"c" help:"TLS certificate file or - for stdin"`
//	}
//
//	func (c *ConfigCLI) Run(ctx context.Context, session *conga.Session) error {
//		certBytes := c.Cert.Get().Bytes()
//		return loadCertificate(certBytes)
//	}
//
// See https://docs.conga.sh/reference/domain-types/.
type FileContent string

// Bytes returns the raw bytes of the loaded file content.
func (f FileContent) Bytes() []byte {
	return []byte(f)
}

// MarshalText returns the loaded file content as UTF-8 bytes.
func (f FileContent) MarshalText() ([]byte, error) {
	return []byte(f), nil
}

// String returns the loaded file content as a string.
func (f FileContent) String() string {
	return string(f)
}

// Help returns the default value syntax description for file content flags.
func (f FileContent) Help() string {
	return "Path to a file (or '-' for stdin)"
}

// Predict returns filesystem file autocompletion candidates.
func (f FileContent) Predict() predict.Predictor {
	return predict.Files()
}

// UnmarshalText loads the file at path (or [os.Stdin] if "-") into memory,
// enforcing a strict 16MB file size limit to prevent memory exhaustion.
func (f *FileContent) UnmarshalText(text []byte) error {
	return f.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext loads the file at path into memory, enforcing a strict
// 16MB file size limit. When path is "-", content is read from the engine-configured
// standard input ([App.Stdin]), falling back to [os.Stdin] for unconfigured values.
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

// Dir represents a filesystem directory path with an optional existence constraint
// declared via the struct tag `existing:"true"`.
//
// Directory paths automatically expand tilde prefixes (`~` to the user's home directory)
// and environment variables (`$VAR` or `${VAR}`) during value decoding.
//
//	type OutputCLI struct {
//		OutputDir conga.RequiredFlag[conga.Dir] `short:"o" existing:"true" help:"Output directory"`
//	}
//
//	func (o *OutputCLI) Run(ctx context.Context, session *conga.Session) error {
//		dir := o.OutputDir.Get()
//		return os.WriteFile(filepath.Join(dir.String(), "result.json"), []byte("{}\n"), 0o644)
//	}
//
// See https://docs.conga.sh/reference/domain-types/.
type Dir struct {
	path     string
	existing bool
}

// TagSpecs registers the custom struct tags accepted by [Dir] fields (`existing`).
func (d *Dir) TagSpecs() []TagSpec {
	return []TagSpec{
		{Name: "existing", Kind: TagKindBool},
	}
}

// Validate enforces the declared directory tags. With `existing:"true"`, the path
// must exist and be a directory; without it, Validate always succeeds.
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

// Predict returns filesystem directory autocompletion candidates.
func (d Dir) Predict() predict.Predictor {
	return predict.Dirs()
}

// String returns the resolved directory path string.
func (d Dir) String() string {
	return d.path
}

// MarshalText returns the resolved directory path as UTF-8 bytes.
func (d Dir) MarshalText() ([]byte, error) {
	return []byte(d.path), nil
}

// UnmarshalText unmarshals a directory path, expanding environment variables and
// tilde prefixes without tag constraints.
func (d *Dir) UnmarshalText(text []byte) error {
	return d.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext decodes a directory path, expanding environment variables
// and tilde prefixes, and captures declared capability tags from [DecodeContext].
func (d *Dir) UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error {
	d.path = pathx.Expand(string(text))
	d.existing = decodeContext.Tags.Bool("existing")

	return nil
}

// Help returns the default value syntax description for directory flags, tailored
// to declared tags ("Path to an existing directory" or "Path to a directory").
func (d Dir) Help(tags FieldTags) string {
	if tags.Bool("existing") {
		return "Path to an existing directory"
	}

	return "Path to a directory"
}

// ByteSize represents human-readable data volumes (such as "10MiB", "1.5GB", "500KiB").
//
// The underlying type is an int64 byte count, enabling direct numeric comparisons
// and calculations in bytes:
//
//	type CacheCLI struct {
//		MaxMemory conga.OptionalFlag[conga.ByteSize] `default:"512MiB" help:"Max memory limit"`
//	}
//
//	func (c *CacheCLI) Run(ctx context.Context, session *conga.Session) error {
//		return configureCache(c.MaxMemory.Get().Bytes()) // 536870912 for 512MiB
//	}
//
// See https://docs.conga.sh/reference/domain-types/.
type ByteSize int64

// Bytes returns the size in bytes as an int64.
func (b ByteSize) Bytes() int64 {
	return int64(b)
}

// String formats the byte size into a deterministic, lossless human-readable string
// using IEC binary units (KiB, MiB, GiB, TiB, PiB). Whole multiples of a unit are
// formatted as integers without spaces or decimal points (e.g. "1MiB", "512MiB");
// values with indivisible remainders are emitted as exact bytes (e.g. "1048577B").
// Returns "0B" for zero.
func (b ByteSize) String() string {
	if b == 0 {
		return "0B"
	}

	var (
		sign string
		abs  uint64
	)

	if b < 0 {
		sign = "-"
		abs = uint64(-(b + 1)) + 1 //nolint:gosec // G115: -(b+1) is non-negative for all negative int64 values including math.MinInt64.
	} else {
		abs = uint64(b)
	}

	units := []struct {
		name       string
		multiplier uint64
	}{
		{name: "PiB", multiplier: 1024 * 1024 * 1024 * 1024 * 1024},
		{name: "TiB", multiplier: 1024 * 1024 * 1024 * 1024},
		{name: "GiB", multiplier: 1024 * 1024 * 1024},
		{name: "MiB", multiplier: 1024 * 1024},
		{name: "KiB", multiplier: 1024},
	}

	for _, unit := range units {
		if abs >= unit.multiplier && abs%unit.multiplier == 0 {
			return fmt.Sprintf("%s%d%s", sign, abs/unit.multiplier, unit.name)
		}
	}

	return fmt.Sprintf("%s%dB", sign, abs)
}

// MarshalText returns the byte size formatted as UTF-8 text using [ByteSize.String].
func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText parses a strict, case-sensitive byte size string.
//
// Accepted formats:
//   - Exact byte counts: "1024", "500B"
//   - SI metric units ending in "B": "kB", "KB", "MB", "GB", "TB", "PB" (powers of 1000)
//   - IEC binary units: "KiB", "MiB", "GiB", "TiB", "PiB" (powers of 1024)
//
// Naked unit suffixes without 'B' ("10G", "512Mi") and lowercase casings ("mb", "kib")
// are rejected to prevent ambiguity.
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

// Help returns the default value syntax description for byte size flags.
func (b ByteSize) Help() string {
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
