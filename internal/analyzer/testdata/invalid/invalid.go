// Package invalid contains CONGA command definitions that violate conga-lint
// contracts; each expected diagnostic is asserted inline below.
package invalid

import (
	"context"

	"github.com/conga-sh/conga"
)

// MissingRunner is a leaf command without a Run method.
type MissingRunner struct { // want `leaf command "missing-runner" \(struct MissingRunner\) does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

var _ = conga.New[MissingRunner]()

// WrongSignature declares Run with a mismatched signature.
type WrongSignature struct { // want `does not implement conga\.Runner`
	Count conga.OptionalFlag[int] `name:"count" help:"Count"`
}

// Run has the wrong parameter list; conga.Runner requires a *conga.Session.
func (w *WrongSignature) Run(ctx context.Context) error {
	return nil
}

var _ = conga.New[WrongSignature]()

// RequiredTagApp uses the unsupported required tag.
type RequiredTagApp struct { // want `does not implement conga\.Runner`
	APIKey conga.RequiredFlag[string] `name:"api-key" required:"true" help:"API key"` // want `tag "required" is not supported`
}

var _ = conga.New[RequiredTagApp]()

// CounterDefaultApp declares the unsupported default tag on a counter flag.
type CounterDefaultApp struct { // want `does not implement conga\.Runner`
	Verbose conga.Counter `name:"verbose" default:"2" help:"Verbose output"` // want `tag "default" is not supported on counter flags`
}

var _ = conga.New[CounterDefaultApp]()

// HelpChild is a leaf subcommand.
type HelpChild struct {
	Leaf conga.OptionalFlag[bool] `name:"leaf" help:"Leaf flag"`
}

// Run executes the child.
func (h *HelpChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// HelpParent declares the unsupported help tag on a subcommand field.
type HelpParent struct { // want `does not implement conga\.Runner`
	Child conga.Command[HelpChild] `help:"Child command"` // want `tag 'help' is not supported on Command\[T\]`
}

var _ = conga.New[HelpParent]()

// StrictApp runs with strict tag validation and misspells a tag key.
type StrictApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" verbos:"true" help:"Verbose output"` // want `unknown struct tag "verbos"`
}

var _ = conga.New[StrictApp]().StrictTagValidation(true)

// AllowedTagApp permits its foreign tag via app.AllowTags.
type AllowedTagApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" externaltag:"x" help:"Verbose output"`
}

var _ = conga.New[AllowedTagApp]().StrictTagValidation(true).AllowTags("externaltag")

// NonStrictApp uses an unknown foreign tag without strict validation.
type NonStrictApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" externaltag:"x" help:"Verbose output"`
}

var _ = conga.New[NonStrictApp]()

// ShadowParent declares flags and shorts that its child shadows.
type ShadowParent struct {
	Verbose conga.OptionalFlag[bool]   `name:"verbose" short:"v" help:"Verbose output"`
	Child   conga.Command[ShadowChild] `name:"child"`
}

// ShadowChild shadows the parent flag name and short rune.
type ShadowChild struct {
	Again conga.OptionalFlag[bool] `name:"verbose" help:"Shadowed name"`         // want `already declared by ancestor`
	Twin  conga.OptionalFlag[bool] `name:"twin" short:"v" help:"Shadowed short"` // want `short flag -v`
}

// Run executes the child.
func (s *ShadowChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ShadowParent]()

// PredictorTypoApp misspells a built-in predictor name.
type PredictorTypoApp struct { // want `does not implement conga\.Runner`
	Script conga.OptionalFlag[string] `name:"script" predictor:"files" help:"Script"` // want `unknown predictor "files"`
}

var _ = conga.New[PredictorTypoApp]()

// RangeApp applies a numeric range to a string flag.
type RangeApp struct { // want `does not implement conga\.Runner`
	AppName conga.OptionalFlag[string] `name:"app-name" range:"1..10" help:"Application name"` // want `range tag is not supported on type string`
}

var _ = conga.New[RangeApp]()

// ByteSizeRangeApp declares an unparseable byte-size bound; conga-lint parses
// conga.ByteSize bounds with the runtime grammar.
type ByteSizeRangeApp struct {
	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" range:"banana..1GiB" help:"Memory"` // want `invalid byte size min bound "banana"`
}

// Run executes the byte-size range fixture.
func (b *ByteSizeRangeApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ByteSizeRangeApp]()

// ByteSizeUnitsRangeApp declares decimal range bounds on a field restricted to
// binary units; conga-lint parses bounds with the field's tags, mirroring
// runtime schema build.
type ByteSizeUnitsRangeApp struct {
	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" units:"binary" range:"1MB..10MB" help:"Memory"` // want `invalid byte size min bound "1MB"`
}

// Run executes the byte-size units-range fixture.
func (b *ByteSizeUnitsRangeApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ByteSizeUnitsRangeApp]()

// ByteSizeUnitsTagApp declares an invalid units tag value for a ByteSize field;
// conga-lint validates the value through the real TagSpecs.
type ByteSizeUnitsTagApp struct {
	Memory conga.OptionalFlag[conga.ByteSize] `name:"memory" units:"hexadecimal" help:"Memory"` // want `unknown units value "hexadecimal"`
}

// Run executes the byte-size units tag fixture.
func (b *ByteSizeUnitsTagApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ByteSizeUnitsTagApp]()

// MapRangeApp applies a range to a map element type, which runtime schema
// build rejects and the analyzer mirrors.
type MapRangeApp struct {
	Weights conga.OptionalFlag[map[string]int] `name:"weights" range:"1..10" help:"Weights"` // want `range tag is not supported on type map\[string\]int`
}

// Run executes the map range fixture.
func (m *MapRangeApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[MapRangeApp]()

// Opaque implements encoding.TextUnmarshaler on a struct underlying kind
// without an ordered projection.
type Opaque struct{}

func (o *Opaque) UnmarshalText(text []byte) error { return nil }

// OpaqueRangeApp applies a range to a text unmarshaler without a projection.
type OpaqueRangeApp struct {
	Value conga.OptionalFlag[Opaque] `name:"value" range:"1..2" help:"Opaque"` // want `range tag is not supported on type invalid\.Opaque`
}

// Run executes the opaque range fixture.
func (o *OpaqueRangeApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[OpaqueRangeApp]()

// LenApp applies a negative length bound.
type LenApp struct { // want `does not implement conga\.Runner`
	Tags conga.OptionalFlag[[]string] `name:"tags" len:"-1" help:"Tags"` // want `invalid len min bound`
}

var _ = conga.New[LenApp]()

// EmptySummaryParent declares an empty summary tag.
type EmptySummaryParent struct {
	Child conga.Command[EmptySummaryChild] `summary:""` // want `tag 'summary' cannot be empty`
}

// EmptySummaryChild is a leaf subcommand.
type EmptySummaryChild struct {
	Leaf conga.OptionalFlag[bool] `name:"leaf" help:"Leaf flag"`
}

// Run executes the child.
func (e *EmptySummaryChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[EmptySummaryParent]()

// DirectiveTypoApp misspells a conga directive.
type DirectiveTypoApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" conga:"alow-unknown-tags" help:"Verbose output"` // want `unrecognized conga directive`
}

var _ = conga.New[DirectiveTypoApp]()

// MalformedTagApp declares a syntactically invalid struct tag.
type MalformedTagApp struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:oops` // want `invalid struct tag: "name:oops"`
}

var _ = conga.New[MalformedTagApp]()

// BuiltinHelpTagApp uses help instead of summary on a built-in command field.
type BuiltinHelpTagApp struct { // want `does not implement conga\.Runner`
	Help conga.HelpCommand `help:"Show help"` // want `tag 'help' is not supported on built-in command fields`
}

var _ = conga.New[BuiltinHelpTagApp]()

// TrailingHelpApp declares the unsupported help tag on trailing-argument
// collectors; their names render in the usage synopsis only.
type TrailingHelpApp struct {
	Extra    conga.DashArgs `help:"Arguments after --"` // want `tag "help" is invalid on dash args`
	Leftover conga.RawArgs  `help:"Unparsed tokens"`    // want `tag "help" is invalid on raw args`
}

// Run executes the trailing help fixture.
func (t *TrailingHelpApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[TrailingHelpApp]()

// FlatLeaf is a leaf root command without a Run method.
type FlatLeaf struct { // want `does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

var _ = conga.New[FlatLeaf]()

// FieldTagValueApp declares an invalid boolean value for a domain-type tag.
type FieldTagValueApp struct { // want `does not implement conga\.Runner`
	Input conga.OptionalFlag[conga.File] `name:"input" existing:"yes" help:"Input file"` // want `tag "existing" expects "true" or "false", got "yes"`
}

var _ = conga.New[FieldTagValueApp]()

// FieldTagConflictApp declares mutually exclusive domain-type tags.
type FieldTagConflictApp struct { // want `does not implement conga\.Runner`
	Stream conga.OptionalFlag[conga.File] `name:"stream" stdin:"true" stdout:"true" help:"Stream"` // want `tag "stdin" cannot be combined with tag "stdout"`
}

var _ = conga.New[FieldTagConflictApp]()

// FieldTagGlobExecutableApp declares the glob tag together with the
// executable capability, a combination the File type rejects.
type FieldTagGlobExecutableApp struct { // want `does not implement conga\.Runner`
	Tool conga.OptionalFlag[conga.File] `name:"tool" executable:"true" glob:"*.sh" help:"Tool"` // want `tag "executable" cannot be combined with tag "glob"`
}

var _ = conga.New[FieldTagGlobExecutableApp]()

// FieldTagGlobEmptyApp declares an empty glob tag value.
type FieldTagGlobEmptyApp struct { // want `does not implement conga\.Runner`
	Input conga.OptionalFlag[conga.File] `name:"input" glob:"" help:"Input file"` // want `tag "glob" cannot be empty`
}

var _ = conga.New[FieldTagGlobEmptyApp]()

// FieldTagGlobMalformedApp declares a malformed glob pattern.
type FieldTagGlobMalformedApp struct { // want `does not implement conga\.Runner`
	Input conga.OptionalFlag[conga.File] `name:"input" glob:"[" help:"Input file"` // want `glob pattern "\[" is malformed: syntax error in pattern`
}

var _ = conga.New[FieldTagGlobMalformedApp]()

// FieldTagGlobPathSeparatorApp declares a glob pattern containing a path
// separator; patterns match entry names during completion traversal.
type FieldTagGlobPathSeparatorApp struct { // want `does not implement conga\.Runner`
	Input conga.OptionalFlag[conga.File] `name:"input" glob:"src/*.go" help:"Input file"` // want `glob pattern "src/\*\.go" must not contain path separators`
}

var _ = conga.New[FieldTagGlobPathSeparatorApp]()

// FieldTagForeignApp uses a domain-type tag on a type that does not register it.
type FieldTagForeignApp struct { // want `does not implement conga\.Runner`
	Name conga.OptionalFlag[string] `name:"name" existing:"true" help:"Name"` // want `unknown struct tag "existing"`
}

var _ = conga.New[FieldTagForeignApp]().StrictTagValidation(true)

// HelpVarTypoApp references an unknown help-text variable.
type HelpVarTypoApp struct { // want `does not implement conga\.Runner`
	Port conga.OptionalFlag[int] `name:"port" range:"1..10" help:"Port ({{rnage}})"` // want `tag "help" references unknown variable \{\{rnage\}\}`
}

var _ = conga.New[HelpVarTypoApp]()

// HelpVarUndeclaredApp references a help-text variable the field does not declare.
type HelpVarUndeclaredApp struct { // want `does not implement conga\.Runner`
	Port conga.OptionalFlag[int] `name:"port" help:"Port ({{range}})"` // want `tag "help" references \{\{range\}\}, but the field provides no value for that variable`
}

var _ = conga.New[HelpVarUndeclaredApp]()

// HelpVarEnvArgApp references {{env}} from a positional argument, which never
// binds environment variables.
type HelpVarEnvArgApp struct { // want `does not implement conga\.Runner`
	Target conga.RequiredArg[string] `name:"<target>" help:"Target ({{env}})"` // want `tag "help" references \{\{env\}\}, but the field provides no value for that variable`
}

var _ = conga.New[HelpVarEnvArgApp]()

// HelpVarEnvFlagApp references {{env}} from a flag that binds no environment
// variable: the package declares no app.EnvPrefix and no envprefix tag.
type HelpVarEnvFlagApp struct { // want `does not implement conga\.Runner`
	Port conga.OptionalFlag[int] `name:"port" help:"Port from {{env}}"` // want `tag "help" references \{\{env\}\}, but the field provides no value for that variable`
}

var _ = conga.New[HelpVarEnvFlagApp]()

// EmptyConstraintApp declares a constraint tag with no value.
type EmptyConstraintApp struct { // want `does not implement conga\.Runner`
	Port conga.OptionalFlag[int] `name:"port" range:""` // want `tag "range" cannot be empty`
}

var _ = conga.New[EmptyConstraintApp]()

// InferredMissingRunner is a leaf command discovered through the inferred
// conga.New(root) call form rather than an explicit type argument.
type InferredMissingRunner struct { // want `leaf command "inferred-missing-runner" \(struct InferredMissingRunner\) does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

var _ = conga.New(&InferredMissingRunner{}) // want `conga\.New root instance is not referenced outside the app`

// InferredDedup is discovered through both anchor forms; a dedup failure would
// report the missing Runner twice.
type InferredDedup struct { // want `leaf command "inferred-dedup" \(struct InferredDedup\) does not implement conga\.Runner`
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

var _ = conga.New[InferredDedup]()

var _ = conga.New(&InferredDedup{}) // want `conga\.New root instance is not referenced outside the app`

// ThrowawayRootApp is discovered through throwaway inferred anchors; the
// diagnostics flag both fresh allocations and single-use local roots, while a
// caller-owned instance read after construction stays silent.
type ThrowawayRootApp struct {
	Verbose conga.OptionalFlag[bool] `name:"verbose" help:"Verbose output"`
}

// Run executes the throwaway-root fixture.
func (t *ThrowawayRootApp) Run(ctx context.Context, session *conga.Session) error {
	kept := &ThrowawayRootApp{}
	app := conga.New(kept)
	throwaway := new(ThrowawayRootApp)
	_ = conga.New(throwaway) // want `conga\.New root instance is not referenced outside the app`
	_ = app
	_ = kept

	return nil
}

var _ = conga.New(&ThrowawayRootApp{}) // want `conga\.New root instance is not referenced outside the app`

var _ = conga.New(new(ThrowawayRootApp)) // want `conga\.New root instance is not referenced outside the app`

var _ = conga.New[ThrowawayRootApp]()
