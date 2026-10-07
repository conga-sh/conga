package conga

import (
	"context"
	"encoding"
	"fmt"
	"io"
	"math"
	"math/bits"
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
	byteSizeErrorEmpty           byteSizeErrorKind = iota // empty input
	byteSizeErrorInvalid                                  // malformed numeric prefix
	byteSizeErrorInvalidNumber                            // numeric prefix does not parse or overflows
	byteSizeErrorInexactFraction                          // fraction does not resolve to whole bytes
	byteSizeErrorUnknownUnit                              // unrecognized unit suffix
	byteSizeErrorUnitNotAllowed                           // unit excluded by the field's units tag
)

const (
	byteSizeFamilyUnset   byteSizeFamily = iota // constructed values: canonical IEC integer model
	byteSizeFamilyBytes                         // parsed exact bytes (B suffix or bare count)
	byteSizeFamilyBinary                        // KiB..PiB
	byteSizeFamilyDecimal                       // kB/KB..PB
)

var (
	_ fmt.Stringer                = File{}
	_ encoding.TextMarshaler      = File{}
	_ encoding.TextUnmarshaler    = (*File)(nil)
	_ ContextAwareTextUnmarshaler = (*File)(nil)

	_ fmt.Stringer                = Dir{}
	_ encoding.TextMarshaler      = Dir{}
	_ encoding.TextUnmarshaler    = (*Dir)(nil)
	_ ContextAwareTextUnmarshaler = (*Dir)(nil)

	_ fmt.Stringer                = ByteSize{}
	_ encoding.TextMarshaler      = ByteSize{}
	_ encoding.TextUnmarshaler    = (*ByteSize)(nil)
	_ ContextAwareTextUnmarshaler = (*ByteSize)(nil)
	_ TagSpecProvider             = (*ByteSize)(nil)
	_ TagAwareHelpProvider        = ByteSize{}

	_ fmt.Stringer                = FileContent("")
	_ encoding.TextMarshaler      = FileContent("")
	_ encoding.TextUnmarshaler    = (*FileContent)(nil)
	_ ContextAwareTextUnmarshaler = (*FileContent)(nil)
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
// A parsed value remembers the unit family it was written in: one decoded from
// "1.5GB" renders back in decimal units ("1500MB"), while one decoded from "1GiB"
// renders in binary units ("1GiB"). Values constructed directly through
// [NewByteSize] render with the canonical IEC integer model, where only whole
// multiples of a binary unit are abbreviated (e.g. "1MiB", "512MiB") and any
// indivisible remainder is emitted as exact bytes (e.g. "1048577B"). Fractions
// such as "1.5GB" are emitted only when a field declares the `precision` tag and
// the fraction round-trips exactly.
//
// The byte count is available through [ByteSize.Bytes] for comparisons and
// calculations:
//
//	type CacheCLI struct {
//		MaxMemory conga.OptionalFlag[conga.ByteSize] `default:"512MiB" help:"Max memory limit"`
//	}
//
//	func (c *CacheCLI) Run(ctx context.Context, session *conga.Session) error {
//		return configureCache(c.MaxMemory.Get().Bytes()) // 536870912 for 512MiB
//	}
//
// ByteSize values are deliberately non-comparable: the captured render family
// and precision are not part of a size's identity, so [ByteSize.Equal] (or
// [ByteSize.Compare], or [ByteSize.Bytes]) expresses equality and ordering.
// The compiler rejects == between two ByteSize values and rejects ByteSize as a
// map key, preventing silently wrong lookups between values that render
// differently but hold the same byte count.
//
// Fields may declare the `units` (`binary`, `decimal`, or `both`), `display`
// (`binary`, `decimal`, or `input`), and `precision` (`0`–`3`) tags to restrict
// the accepted unit families, force the rendering family, or allow exact
// fractions.
//
// See https://docs.conga.sh/reference/domain-types/.
type ByteSize struct {
	bytes     int64
	family    byteSizeFamily
	precision uint8

	// The blank field makes the struct non-comparable: array types are
	// comparable only when their element type is, and func is not comparable.
	_ [0]func()
}

// NewByteSize constructs a ByteSize from a raw byte count. The value renders with
// the canonical IEC integer model ([ByteSize.String]); decode a value from text to
// preserve an input's unit family.
func NewByteSize(bytes int64) ByteSize {
	return ByteSize{bytes: bytes}
}

// Equal reports whether b and other represent the same number of bytes,
// independent of the unit families and precisions used to render them.
func (b ByteSize) Equal(other ByteSize) bool {
	return b.bytes == other.bytes
}

// Compare compares b and other by byte count and returns -1, 0, or +1.
func (b ByteSize) Compare(other ByteSize) int {
	switch {
	case b.bytes < other.bytes:
		return -1
	case b.bytes > other.bytes:
		return 1
	default:
		return 0
	}
}

// TagSpecs registers the custom struct tags accepted by [ByteSize] fields:
// `units`, `display`, and `precision`.
func (b *ByteSize) TagSpecs() []TagSpec {
	return []TagSpec{
		{Name: "units", Kind: TagKindString, Validate: validateByteSizeUnitsTag},
		{Name: "display", Kind: TagKindString, Validate: validateByteSizeDisplayTag},
		{Name: "precision", Kind: TagKindString, Validate: validateByteSizePrecisionTag},
	}
}

// Bytes returns the size in bytes as an int64.
func (b ByteSize) Bytes() int64 {
	return b.bytes
}

// String formats the byte size into a deterministic, lossless human-readable string.
//
// The rendering family comes from the value's captured unit family (or the
// canonical IEC model for constructed values): whole multiples of a unit are
// formatted as integers without spaces or decimal points (e.g. "1MiB",
// "1500MB"), values with indivisible remainders are emitted as exact bytes (e.g.
// "1048577B"), and fractions appear only when the declared precision renders them
// exactly (e.g. "1.5GB"). Parsed exact-byte input ("B" or a bare count) echoes as
// exact bytes (e.g. "1024B"). Returns "0B" for zero.
func (b ByteSize) String() string {
	if b.bytes == 0 {
		return "0B"
	}

	var (
		sign string
		abs  uint64
	)

	if b.bytes < 0 {
		sign = "-"
		abs = uint64(-(b.bytes + 1)) + 1 //nolint:gosec // G115: -(b+1) is non-negative for all negative int64 values including math.MinInt64.
	} else {
		abs = uint64(b.bytes)
	}

	precision := b.precision
	if b.family == byteSizeFamilyUnset {
		// Constructed values keep the canonical integer-only model regardless
		// of any declared precision tag.
		precision = 0
	}

	for _, unit := range byteSizeUnits(b.family) {
		whole := abs / unit.multiplier
		remainder := abs % unit.multiplier

		if remainder == 0 {
			return fmt.Sprintf("%s%d%s", sign, whole, unit.name)
		}

		if precision == 0 {
			continue
		}

		decimals, ok := byteSizeDecimals(remainder, unit.multiplier, precision)
		if !ok {
			continue
		}

		fraction := remainder * byteSizePow10(decimals) / unit.multiplier

		return fmt.Sprintf("%s%d.%0*d%s", sign, whole, decimals, fraction, unit.name)
	}

	return fmt.Sprintf("%s%dB", sign, abs)
}

// MarshalText returns the byte size formatted as UTF-8 text using [ByteSize.String].
func (b ByteSize) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

// UnmarshalText parses a strict, case-sensitive byte size string without field
// tags. The input's unit suffix fixes the value's render family.
//
// Accepted formats:
//   - Exact byte counts: "1024", "500B"
//   - SI metric units ending in "B": "kB", "KB", "MB", "GB", "TB", "PB" (powers of 1000)
//   - IEC binary units: "KiB", "MiB", "GiB", "TiB", "PiB" (powers of 1024)
//
// The numeric prefix is digits with an optional fraction whose contribution
// resolves exactly to bytes (e.g. "1.5GB", "0.5GiB"); a sub-byte remainder such
// as "0.1GiB" is rejected. Text input is non-negative: negative byte counts are
// representable only programmatically through [NewByteSize] even though
// [ByteSize.MarshalText] renders them with a leading '-'.
//
// Naked unit suffixes without 'B' ("10G", "512Mi") and lowercase casings ("mb", "kib")
// are rejected to prevent ambiguity.
func (b *ByteSize) UnmarshalText(text []byte) error {
	return b.UnmarshalTextWithContext(text, DecodeContext{})
}

// UnmarshalTextWithContext parses a strict, case-sensitive byte size string using the
// field's declared tags: `units` restricts the accepted unit family (`binary`,
// `decimal`, or `both`), `display` forces the render family (`binary`, `decimal`, or
// `input`), and `precision` allows exact fractions of up to that many decimals.
//
// The value captures the parsed unit's family so it renders back in the family the
// input used (Kubernetes-Quantity style). Bare counts and the "B" suffix capture
// the exact-bytes family and echo as exact bytes. Absent tags accept every unit
// and echo the input family.
func (b *ByteSize) UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error {
	rawText := strings.TrimSpace(string(text))
	if rawText == "" {
		return &byteSizeError{kind: byteSizeErrorEmpty}
	}

	numStr, unitStr, err := byteSizeSplitNumber(rawText)
	if err != nil {
		return err
	}

	byteCount, family, err := byteSizeParseParts(numStr, unitStr, rawText, decodeContext.Tags)
	if err != nil {
		return err
	}

	*b = ByteSize{
		bytes:     byteCount,
		family:    byteSizeResolveFamily(family, decodeContext.Tags),
		precision: byteSizePrecision(decodeContext.Tags),
	}

	return nil
}

// Help returns the default value syntax description for byte size flags, tailored
// to the declared `units`, `display`, and `precision` tags.
func (b ByteSize) Help(tags FieldTags) string {
	helpText := "Byte size (e.g., 10MiB, 1.5GB, 500KiB)"

	switch tags.String("units") {
	case "binary":
		helpText = "Byte size in binary units (e.g., 10KiB, 512MiB)"
	case "decimal":
		helpText = "Byte size in decimal units (e.g., 10kB, 512MB)"
	}

	switch tags.String("display") {
	case "binary":
		helpText += ", shown in binary units"
	case "decimal":
		helpText += ", shown in decimal units"
	case "input":
		helpText += ", shown in the entered unit family"
	}

	if precision := tags.String("precision"); precision != "" {
		helpText += ", up to " + precision + " decimal places"
	}

	return helpText
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

// byteSizeFamily identifies the unit family a ByteSize renders in: the family
// captured from the parsed input, or the family forced by the field's display tag.
type byteSizeFamily uint8

// byteSizeUnit is one abbreviation in a render family's unit ladder.
type byteSizeUnit struct {
	name       string
	multiplier uint64
}

// byteSizeErrorKind identifies the byte-size parse failure mode.
type byteSizeErrorKind uint8

// byteSizeError is the root cause for byte-size parse failures.
type byteSizeError struct {
	raw     string // full input (invalid/inexactFraction/unknownUnit/unitNotAllowed) or numeric prefix (invalidNumber)
	unit    string // offending unit (unknownUnit/unitNotAllowed only)
	allowed string // unit family the field restricts to (unitNotAllowed only)
	kind    byteSizeErrorKind
}

func (e *byteSizeError) Error() string {
	switch e.kind {
	case byteSizeErrorEmpty:
		return "empty byte size"
	case byteSizeErrorInvalid:
		return fmt.Sprintf("invalid byte size %q", e.raw)
	case byteSizeErrorInvalidNumber:
		return fmt.Sprintf("invalid byte size number %q", e.raw)
	case byteSizeErrorInexactFraction:
		return fmt.Sprintf("byte size %q is not a whole number of bytes", e.raw)
	case byteSizeErrorUnknownUnit:
		return fmt.Sprintf("unknown byte size unit %q in %q", e.unit, e.raw)
	case byteSizeErrorUnitNotAllowed:
		return fmt.Sprintf("byte size unit %q in %q is not allowed: the field accepts %s units only", e.unit, e.raw, e.allowed)
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

// byteSizeParseParts converts the split numeric prefix and unit suffix into a
// byte count and input family, enforcing the field's declared units tag. A bare
// count and the "B" suffix capture the exact-bytes family.
func byteSizeParseParts(numStr, unitStr, rawText string, tags FieldTags) (int64, byteSizeFamily, error) {
	if unitStr == "" {
		parsed, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil || parsed < 0 {
			return 0, byteSizeFamilyUnset, &byteSizeError{raw: numStr, kind: byteSizeErrorInvalidNumber}
		}

		return parsed, byteSizeFamilyBytes, nil
	}

	multiplier, family, ok := byteSizeUnitMultiplier(unitStr)
	if !ok {
		return 0, byteSizeFamilyUnset, &byteSizeError{raw: rawText, unit: unitStr, kind: byteSizeErrorUnknownUnit}
	}

	if err := byteSizeCheckUnitFamily(rawText, unitStr, family, tags.String("units")); err != nil {
		return 0, byteSizeFamilyUnset, err
	}

	scaled, exact, ok := byteSizeScaleBytes(numStr, multiplier)
	if !ok {
		return 0, byteSizeFamilyUnset, &byteSizeError{raw: numStr, kind: byteSizeErrorInvalidNumber}
	}

	if !exact {
		return 0, byteSizeFamilyUnset, &byteSizeError{raw: rawText, kind: byteSizeErrorInexactFraction}
	}

	return scaled, family, nil
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

func byteSizeUnitMultiplier(unit string) (uint64, byteSizeFamily, bool) {
	switch unit {
	case "B":
		return 1, byteSizeFamilyBytes, true
	case "kB", "KB":
		return 1000, byteSizeFamilyDecimal, true
	case "MB":
		return 1000 * 1000, byteSizeFamilyDecimal, true
	case "GB":
		return 1000 * 1000 * 1000, byteSizeFamilyDecimal, true
	case "TB":
		return 1000 * 1000 * 1000 * 1000, byteSizeFamilyDecimal, true
	case "PB":
		return 1000 * 1000 * 1000 * 1000 * 1000, byteSizeFamilyDecimal, true
	case "KiB":
		return 1024, byteSizeFamilyBinary, true
	case "MiB":
		return 1024 * 1024, byteSizeFamilyBinary, true
	case "GiB":
		return 1024 * 1024 * 1024, byteSizeFamilyBinary, true
	case "TiB":
		return 1024 * 1024 * 1024 * 1024, byteSizeFamilyBinary, true
	case "PiB":
		return 1024 * 1024 * 1024 * 1024 * 1024, byteSizeFamilyBinary, true
	default:
		return 0, byteSizeFamilyUnset, false
	}
}

// byteSizeSplitNumber splits a trimmed byte size into its numeric prefix and
// unit suffix. The prefix grammar is digits with an optional fractional part;
// malformed prefixes are reported as byte-size errors so the parser and the
// range-bound validator phrase failures identically.
func byteSizeSplitNumber(rawText string) (string, string, error) {
	numberEnd := 0
	for numberEnd < len(rawText) && rawText[numberEnd] >= '0' && rawText[numberEnd] <= '9' {
		numberEnd++
	}

	if numberEnd == 0 {
		return "", "", &byteSizeError{raw: rawText, kind: byteSizeErrorInvalid}
	}

	if numberEnd < len(rawText) && rawText[numberEnd] == '.' {
		numberEnd++
		fractionStart := numberEnd

		for numberEnd < len(rawText) && rawText[numberEnd] >= '0' && rawText[numberEnd] <= '9' {
			numberEnd++
		}

		if numberEnd == fractionStart {
			return "", "", &byteSizeError{raw: rawText, kind: byteSizeErrorInvalidNumber}
		}
	}

	numStr := rawText[:numberEnd]
	unitStr := rawText[numberEnd:]

	if strings.Contains(unitStr, ".") {
		return "", "", &byteSizeError{raw: rawText, kind: byteSizeErrorInvalidNumber}
	}

	return numStr, unitStr, nil
}

// byteSizeScaleBytes converts a numeric prefix (digits with an optional
// fractional part) into a byte count for multiplier using exact integer
// arithmetic. ok is false for malformed prefixes or byte counts that overflow
// int64; exact is false when the fractional contribution does not resolve to a
// whole number of bytes, so callers can reject lossy input. Every decimal value
// the renderer can emit is exact, so a rendered size always reparses to the
// identical byte count.
func byteSizeScaleBytes(numStr string, multiplier uint64) (int64, bool, bool) {
	wholeStr, fractionStr, _ := strings.Cut(numStr, ".")

	var whole uint64

	if wholeStr != "" {
		parsed, err := strconv.ParseUint(wholeStr, 10, 64)
		if err != nil || parsed > uint64(math.MaxInt64)/multiplier {
			return 0, false, false
		}

		whole = parsed
	}

	scaled := whole * multiplier

	fractionStr = strings.TrimRight(fractionStr, "0")
	if fractionStr == "" {
		return int64(scaled), true, true //nolint:gosec // G115: scaled is bounded by math.MaxInt64 above.
	}

	// Truncating a fraction longer than 19 significant digits would silently
	// drop information; no supported multiplier can render such a fraction
	// exactly, so it is reported as inexact.
	if len(fractionStr) > 19 {
		return 0, false, true
	}

	fraction, err := strconv.ParseUint(fractionStr, 10, 64)
	if err != nil {
		return 0, false, false
	}

	power := uint64(1)
	for range len(fractionStr) {
		power *= 10
	}

	high, low := bits.Mul64(fraction, multiplier)
	contribution, remainder := bits.Div64(high, low, power)

	if scaled > uint64(math.MaxInt64)-contribution {
		return 0, false, false
	}

	return int64(scaled + contribution), remainder == 0, true //nolint:gosec // G115: the sum is bounded by math.MaxInt64 above.
}

// byteSizeUnits returns the abbreviation units of a render family, ordered from
// largest to smallest. The unset and binary families share the IEC ladder; they
// differ only in whether the declared precision applies (see [ByteSize.String]).
// The bytes family has no ladder, so String emits exact bytes for it.
func byteSizeUnits(family byteSizeFamily) []byteSizeUnit {
	switch family {
	case byteSizeFamilyBytes:
		return nil
	case byteSizeFamilyDecimal:
		return []byteSizeUnit{
			{name: "PB", multiplier: 1000 * 1000 * 1000 * 1000 * 1000},
			{name: "TB", multiplier: 1000 * 1000 * 1000 * 1000},
			{name: "GB", multiplier: 1000 * 1000 * 1000},
			{name: "MB", multiplier: 1000 * 1000},
			{name: "kB", multiplier: 1000},
		}
	default:
		return []byteSizeUnit{
			{name: "PiB", multiplier: 1024 * 1024 * 1024 * 1024 * 1024},
			{name: "TiB", multiplier: 1024 * 1024 * 1024 * 1024},
			{name: "GiB", multiplier: 1024 * 1024 * 1024},
			{name: "MiB", multiplier: 1024 * 1024},
			{name: "KiB", multiplier: 1024},
		}
	}
}

// byteSizeDecimals returns the smallest number of fraction digits (up to
// maxPrecision) that renders remainder/multiplier exactly, or false when no
// precision in range round-trips.
func byteSizeDecimals(remainder, multiplier uint64, maxPrecision uint8) (uint8, bool) {
	scaled := remainder

	for decimals := uint8(1); decimals <= maxPrecision; decimals++ {
		scaled *= 10
		if scaled%multiplier == 0 {
			return decimals, true
		}
	}

	return 0, false
}

// byteSizePow10 returns 10^exponent for the small precision range.
func byteSizePow10(exponent uint8) uint64 {
	result := uint64(1)

	for range exponent {
		result *= 10
	}

	return result
}

// byteSizePrecision reads the precision tag as a fraction-digit count, defaulting
// to 0 for absent or out-of-range values (schema build rejects the latter).
func byteSizePrecision(tags FieldTags) uint8 {
	parsed, err := strconv.ParseUint(tags.String("precision"), 10, 8)
	if err != nil || parsed > 3 {
		return 0
	}

	return uint8(parsed)
}

// byteSizeResolveFamily applies the display tag to the input-derived family:
// "binary" and "decimal" force a family, "input" (or absence) keeps the input's.
func byteSizeResolveFamily(inputFamily byteSizeFamily, tags FieldTags) byteSizeFamily {
	switch tags.String("display") {
	case "binary":
		return byteSizeFamilyBinary
	case "decimal":
		return byteSizeFamilyDecimal
	default:
		return inputFamily
	}
}

// byteSizeCheckUnitFamily rejects a parsed unit the field's units tag excludes.
func byteSizeCheckUnitFamily(rawText, unit string, family byteSizeFamily, unitsTag string) error {
	switch unitsTag {
	case "binary":
		if family == byteSizeFamilyDecimal {
			return &byteSizeError{raw: rawText, unit: unit, allowed: "binary", kind: byteSizeErrorUnitNotAllowed}
		}
	case "decimal":
		if family == byteSizeFamilyBinary {
			return &byteSizeError{raw: rawText, unit: unit, allowed: "decimal", kind: byteSizeErrorUnitNotAllowed}
		}
	}

	return nil
}

// validateByteSizeUnitsTag restricts the units tag to the supported unit families.
func validateByteSizeUnitsTag(value string) error {
	switch value {
	case "binary", "decimal", "both":
		return nil
	default:
		return fmt.Errorf("%w %q (want \"binary\", \"decimal\", or \"both\")", errByteSizeUnitsTag, value)
	}
}

// validateByteSizeDisplayTag restricts the display tag to the supported render families.
func validateByteSizeDisplayTag(value string) error {
	switch value {
	case "binary", "decimal", "input":
		return nil
	default:
		return fmt.Errorf("%w %q (want \"binary\", \"decimal\", or \"input\")", errByteSizeDisplayTag, value)
	}
}

// validateByteSizePrecisionTag restricts the precision tag to 0..3 exact fraction digits.
func validateByteSizePrecisionTag(value string) error {
	parsed, err := strconv.ParseUint(value, 10, 8)
	if err != nil || parsed > 3 {
		return fmt.Errorf("%w %q (want 0, 1, 2, or 3)", errByteSizePrecisionTag, value)
	}

	return nil
}
