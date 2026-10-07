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
	Memory    conga.OptionalFlag[conga.ByteSize] `name:"memory" help:"Worker memory limit" range:"1MiB..1GiB" units:"binary" display:"input" precision:"1"`
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
	Extra    conga.DashArgs                   `name:"extra"`
	Leftover conga.RawArgs                    `name:"leftover"`
	Help     conga.HelpCommand                `summary:"Show help"`
	_        struct{}                         `envprefix:"VALID_"`
}

var _ = conga.New[App]().Name("valid")

// newRootUsedOutsideApp exercises the throwaway-root suppression path: the
// caller-owned instance is read after construction, so conga-lint must stay
// silent. The inferred New(root) dedup coverage lives in the invalid fixture,
// where a dedup failure surfaces as a duplicated diagnostic.
func newRootUsedOutsideApp() {
	cli := &App{}
	_ = conga.New(cli)
	_ = cli
}
