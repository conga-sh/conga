// Package valid contains CONGA command definitions that satisfy every
// conga-lint contract; the analyzer must report no diagnostics for it.
package valid

import (
	"context"

	"github.com/conga-sh/conga"
)

// DeployWorker is a leaf command implementing conga.Runner.
type DeployWorker struct {
	Replicas conga.OptionalFlag[int]        `name:"replicas" short:"r" help:"Worker replica count" range:"1..16"`
	Input    conga.OptionalFlag[conga.File] `name:"input" existing:"true" stdin:"true" help:"Input file"`
	Output   conga.OptionalFlag[conga.File] `name:"output" stdout:"true" help:"Output file"`
	WorkDir  conga.OptionalFlag[conga.Dir]  `name:"workdir" existing:"true" help:"Working directory"`
	Manifest conga.OptionalFlag[conga.File] `name:"manifest" glob:"*.yaml,Makefile" help:"Manifest file"`
}

// Run executes the worker deployment.
func (d *DeployWorker) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// Deploy is a grouping command; Runner is optional when subcommands exist.
type Deploy struct {
	Worker conga.Command[DeployWorker] `name:"worker"`
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
	Extra    conga.DashArgs                   `help:"Arguments after --"`
	Leftover conga.RawArgs                    `help:"Unparsed trailing arguments"`
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
