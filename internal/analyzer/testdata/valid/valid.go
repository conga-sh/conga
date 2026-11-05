// Package valid contains CONGA command definitions that satisfy every
// conga-lint contract; the analyzer must report no diagnostics for it.
package valid

import (
	"context"
	"strconv"
	"strings"

	"github.com/conga-sh/conga"
)

// MemoryAlias aliases the built-in byte-size type: range validation must
// resolve the alias to conga.ByteSize before parsing bounds.
type MemoryAlias = conga.ByteSize

// Histogram is a text dialect with a Bytes() int64 projection; the analyzer
// verifies its classification but defers bound parsing to schema build.
type Histogram int64

func (h *Histogram) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}

	*h = Histogram(parsed)

	return nil
}

func (h *Histogram) Bytes() int64 { return int64(*h) }

// Ratio is a float text dialect whose bounds compare as float64.
type Ratio float64

func (r *Ratio) UnmarshalText(text []byte) error {
	parsed, err := strconv.ParseFloat(strings.TrimSuffix(string(text), "%"), 64)
	if err != nil {
		return err
	}

	*r = Ratio(parsed)

	return nil
}

// DeployWorker is a leaf command implementing conga.Runner.
type DeployWorker struct {
	Replicas  conga.OptionalFlag[int]            `name:"replicas" short:"r" help:"Worker replica count ({{env}})" range:"1..16"`
	Memory    conga.OptionalFlag[conga.ByteSize] `name:"memory" help:"Worker memory limit" range:"1MiB..1GiB" units:"binary" display:"input"`
	AliasSize conga.OptionalFlag[MemoryAlias]    `name:"alias-size" help:"Aliased memory limit" range:"1MiB..1GiB"`
	Histogram conga.OptionalFlag[Histogram]      `name:"histogram" help:"Histogram size" range:"1..10"`
	Ratio     conga.OptionalFlag[Ratio]          `name:"ratio" help:"Ratio" range:"10%..90%"`
	Input     conga.OptionalFlag[conga.File]     `name:"input" existing:"true" stdin:"true" help:"Input file"`
	Output    conga.OptionalFlag[conga.File]     `name:"output" stdout:"true" help:"Output file"`
	WorkDir   conga.OptionalFlag[conga.Dir]      `name:"workdir" existing:"true" help:"Working directory"`
	Manifest  conga.OptionalFlag[conga.File]     `name:"manifest" glob:"*.yaml,Makefile" help:"Manifest file"`
	Port      conga.OptionalFlag[int]            `name:"port" help:"Port number ({{range}})" range:"1..1024"`
}

// Run executes the worker deployment.
func (d *DeployWorker) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// Deploy is a grouping command; Runner is optional when subcommands exist.
type Deploy struct {
	Worker conga.Command[DeployWorker] `name:"worker" envprefix:"DEPLOY_"`
}

// ValueReceiverLeaf implements conga.Runner with a value receiver.
type ValueReceiverLeaf struct {
	LeafName conga.OptionalFlag[string] `name:"leaf-name" help:"Leaf name"`
	Leftover conga.RawArgs              `name:"leftover" predictor:"dir"`
}

// Run executes the leaf with a value receiver.
func (v ValueReceiverLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// App is the root application struct.
type App struct {
	Deploy   conga.Command[Deploy]            `name:"deploy"`
	Leaf     conga.Command[ValueReceiverLeaf] `name:"leaf"`
	ConfigIn conga.ConfigFile                 `name:"config" short:"c" help:"Path to configuration file"`
	Count    conga.Counter                    `name:"count" help:"Count invocations"`
	Extra    conga.DashArgs                   `name:"extra" predictor:"file"`
	Help     conga.HelpCommand                `summary:"Show help" aliases:"h"`
}

var _ = conga.New[App]().Name("valid")

// ArgsWithBuiltinHelp declares positional arguments alongside a built-in help
// command field; built-in commands are not branching, so the branch/leaf
// invariant stays satisfied.
type ArgsWithBuiltinHelp struct {
	Target conga.RequiredArg[string] `name:"target"`
	Help   conga.HelpCommand         `summary:"Show help"`
}

// Run executes the leaf.
func (a *ArgsWithBuiltinHelp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[ArgsWithBuiltinHelp]()

// BothCollectorsLeaf declares one DashArgs and one RawArgs collector; the
// one-collector-per-kind guard is per kind, so this shape stays legal.
type BothCollectorsLeaf struct {
	Target conga.RequiredArg[string] `name:"target"`
	Dash   conga.DashArgs            `name:"extra" predictor:"file"`
	Raw    conga.RawArgs             `name:"leftover" predictor:"dir"`
}

// Run executes the both-collectors leaf.
func (b *BothCollectorsLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[BothCollectorsLeaf]()

// OneOfLeaf is a leaf command with an exactly-one group.
type OneOfLeaf struct {
	JSON conga.OptionalFlag[bool] `name:"json" oneof:"format" help:"JSON output"`
	YAML conga.OptionalFlag[bool] `name:"yaml" oneof:"format" help:"YAML output"`
}

// Run executes the leaf.
func (o *OneOfLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[OneOfLeaf]()

// OneOfAndLeaf pairs a oneof member with an and-group partner outside the
// group, which stays satisfiable and must not be flagged.
type OneOfAndLeaf struct {
	JSON conga.OptionalFlag[bool]   `name:"json" oneof:"format" and:"auth" help:"JSON output"`
	Key  conga.OptionalFlag[string] `name:"key" and:"auth" help:"Encryption key"`
	YAML conga.OptionalFlag[bool]   `name:"yaml" oneof:"format" help:"YAML output"`
}

// Run executes the leaf.
func (o *OneOfAndLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[OneOfAndLeaf]()

// CounterOneOfLeaf is the legal single-member oneof case: a Counter can be
// required declaratively by a oneof group with one member.
type CounterOneOfLeaf struct {
	Verbose conga.Counter `name:"verbose" oneof:"mode" help:"Verbose output"`
}

// Run executes the leaf.
func (c *CounterOneOfLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[CounterOneOfLeaf]()

// XorAndPartnerLeaf pairs an xor choice with a conditional and companion: the
// and group is shared with one xor member, not both, so it stays satisfiable.
type XorAndPartnerLeaf struct {
	Cert     conga.OptionalFlag[string] `name:"cert" and:"inline-cert" xor:"cert-source" help:"Inline certificate"`
	CertFile conga.OptionalFlag[string] `name:"cert-file" xor:"cert-source" help:"Certificate file"`
	CertKey  conga.OptionalFlag[string] `name:"cert-key" and:"inline-cert" help:"Inline certificate key"`
}

// Run executes the leaf.
func (x *XorAndPartnerLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[XorAndPartnerLeaf]()

// PlainUntaggedLeaf has a plain exported struct field without CONGA tags; the
// schema ignores it and the analyzer must stay silent.
type PlainUntaggedLeaf struct {
	Extra PlainExtra
}

// PlainExtra is a plain helper struct, not a schema container.
type PlainExtra struct {
	Value string
}

// Run executes the leaf.
func (p *PlainUntaggedLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[PlainUntaggedLeaf]()

// newRootUsedOutsideApp exercises the throwaway-root suppression path: the
// caller-owned instance is read after construction, so conga-lint must stay
// silent. The inferred New(root) dedup coverage lives in the invalid fixture,
// where a dedup failure surfaces as a duplicated diagnostic.
func newRootUsedOutsideApp() {
	cli := &App{}
	_ = conga.New(cli)
	_ = cli
}

// AliasedFlag is an alias of a flag container; the runtime classifies the
// alias as the container it names, so the analyzer must validate it as a flag.
type AliasedFlag = conga.OptionalFlag[string]

// AliasedFlagLeaf declares a flag through a type alias.
type AliasedFlagLeaf struct {
	Value AliasedFlag `name:"value" help:"Value"`
}

// Run executes the leaf.
func (a *AliasedFlagLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[AliasedFlagLeaf]()

// EmbeddedFlag embeds a flag container. Go promotes the container marker, so
// the runtime classifies the outer type as a flag.
type EmbeddedFlag struct {
	conga.OptionalFlag[string]
}

// EmbeddedFlagLeaf declares a flag through an embedded container.
type EmbeddedFlagLeaf struct {
	Value EmbeddedFlag `name:"value" help:"Value"`
}

// Run executes the leaf.
func (e *EmbeddedFlagLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[EmbeddedFlagLeaf]()

// AliasedChild is the leaf subcommand used by the command container fixtures.
type AliasedChild struct{}

// Run executes the leaf.
func (a *AliasedChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// AliasedCommand is an alias of a command container.
type AliasedCommand = conga.Command[AliasedChild]

// AliasedCommandLeaf declares a subcommand through a type alias.
type AliasedCommandLeaf struct {
	Child AliasedCommand `summary:"Child"`
}

var _ = conga.New[AliasedCommandLeaf]()

// EmbeddedCommand embeds a command container. Go promotes the command state
// methods, so the runtime routes the outer type as a subcommand.
type EmbeddedCommand struct {
	conga.Command[AliasedChild]
}

// EmbeddedCommandLeaf declares a subcommand through an embedded container.
type EmbeddedCommandLeaf struct {
	Child EmbeddedCommand `summary:"Child"`
}

var _ = conga.New[EmbeddedCommandLeaf]()

// AliasArgChild is the leaf subcommand routed through an aliased type argument.
type AliasArgChild struct{}

// Run executes the leaf.
func (a *AliasArgChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// AliasArgChildAlias aliases the subcommand struct used as a Command[T] argument.
type AliasArgChildAlias = AliasArgChild

// AliasArgCommandLeaf routes through Command[T] with an aliased type argument;
// the analyzer must resolve the alias instead of misreading the parent as a leaf.
type AliasArgCommandLeaf struct {
	Child conga.Command[AliasArgChildAlias] `summary:"Child"`
}

var _ = conga.New[AliasArgCommandLeaf]()

// EmbeddedGenericFlag embeds a flag container and carries an unrelated outer
// type parameter that must not be mistaken for the flag's value type.
type EmbeddedGenericFlag[M any] struct {
	conga.OptionalFlag[int]
	Meta M
}

// EmbeddedGenericFlagLeaf validates value tags against the embedded int value.
type EmbeddedGenericFlagLeaf struct {
	Value EmbeddedGenericFlag[string] `range:"1..10" help:"Value"`
}

// Run executes the leaf.
func (e *EmbeddedGenericFlagLeaf) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[EmbeddedGenericFlagLeaf]()

// PointerSharedRoot holds a pointer-to-container field, which the runtime
// classifies as plain because the field address has an empty method set.
type PointerSharedRoot struct {
	Shared *conga.OptionalFlag[string]
	Child  conga.Command[PointerSharedChild] `summary:"Child"`
}

// PointerSharedChild redeclares the name as a real flag; it must not collide
// with the ignored pointer field on the ancestor.
type PointerSharedChild struct {
	Shared conga.OptionalFlag[string] `name:"shared" help:"Shared value"`
}

// Run executes the leaf.
func (p *PointerSharedChild) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[PointerSharedRoot]()

// ImplicitRun is an implicit leaf command with a positional operand.
type ImplicitRun struct {
	File conga.RequiredArg[string] `name:"file"`
}

// Run executes the implicit command.
func (i *ImplicitRun) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// ImplicitRoot exposes an implicit command alongside explicit siblings.
type ImplicitRoot struct {
	Exec  conga.Command[ImplicitRun] `name:"run" implicit:"true"`
	Build conga.Command[ImplicitRun] `name:"build"`
}

var _ = conga.New[ImplicitRoot]()
