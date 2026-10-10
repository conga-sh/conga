package conga_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// TrackedLifecycleRoot is a multi-level hierarchy struct for tracking exact hook execution order.
type TrackedLifecycleRoot struct {
	FailValidate conga.OptionalFlag[bool]
	FailPreRun   conga.OptionalFlag[bool] `name:"fail-root-pre"`
	FailPostRun  conga.OptionalFlag[bool] `name:"fail-root-post"`
	FailFinally  conga.OptionalFlag[bool] `name:"fail-root-finally"`

	Cluster conga.Command[TrackedClusterCmd] `summary:"Cluster commands"`

	Logs []string
}

func (r *TrackedLifecycleRoot) Validate(ctx context.Context, session *conga.Session) error {
	r.Logs = append(r.Logs, "Root.Validate")
	if r.FailValidate.Get() {
		return errors.New("root validation failed")
	}

	return nil
}

func (r *TrackedLifecycleRoot) PreRun(ctx context.Context, session *conga.Session) error {
	r.Logs = append(r.Logs, "Root.PreRun")
	if r.FailPreRun.Get() {
		return errors.New("root prerun failed")
	}

	return nil
}

func (r *TrackedLifecycleRoot) PostRun(ctx context.Context, session *conga.Session) error {
	r.Logs = append(r.Logs, "Root.PostRun")
	if r.FailPostRun.Get() {
		return errors.New("root postrun failed")
	}

	return nil
}

func (r *TrackedLifecycleRoot) Finally(ctx context.Context, session *conga.Session, err error) error {
	r.Logs = append(r.Logs, fmt.Sprintf("Root.Finally(err=%v)", err != nil))
	if r.FailFinally.Get() {
		return errors.New("root finally failed")
	}

	return nil
}

type TrackedClusterCmd struct {
	FailPreRun  conga.OptionalFlag[bool] `name:"fail-cluster-pre"`
	FailPostRun conga.OptionalFlag[bool] `name:"fail-cluster-post"`
	FailFinally conga.OptionalFlag[bool] `name:"fail-cluster-finally"`

	Node conga.Command[TrackedNodeCmd] `summary:"Node commands"`
}

func (c *TrackedClusterCmd) Validate(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Cluster.Validate")
	}

	return nil
}

func (c *TrackedClusterCmd) PreRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Cluster.PreRun")
	}

	if c.FailPreRun.Get() {
		return errors.New("cluster prerun failed")
	}

	return nil
}

func (c *TrackedClusterCmd) PostRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Cluster.PostRun")
	}

	if c.FailPostRun.Get() {
		return errors.New("cluster postrun failed")
	}

	return nil
}

func (c *TrackedClusterCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, fmt.Sprintf("Cluster.Finally(err=%v)", err != nil))
	}

	if c.FailFinally.Get() {
		return errors.New("cluster finally failed")
	}

	return nil
}

type TrackedNodeCmd struct {
	FailPreRun  conga.OptionalFlag[bool] `name:"fail-node-pre"`
	FailRun     conga.OptionalFlag[bool] `name:"fail-node-run"`
	FailPostRun conga.OptionalFlag[bool] `name:"fail-node-post"`
	FailFinally conga.OptionalFlag[bool] `name:"fail-node-finally"`

	Target conga.RequiredArg[string]

	capturedPath   string
	capturedActive string
}

func (n *TrackedNodeCmd) Validate(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Node.Validate")
	}

	return nil
}

func (n *TrackedNodeCmd) PreRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Node.PreRun")
	}

	if n.FailPreRun.Get() {
		return errors.New("node prerun failed")
	}

	return nil
}

func (n *TrackedNodeCmd) Run(ctx context.Context, session *conga.Session) error {
	n.capturedPath = session.Path()
	n.capturedActive = session.Name()

	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Node.Run")
	}

	if n.FailRun.Get() {
		return errors.New("node run failed")
	}

	return nil
}

func (n *TrackedNodeCmd) PostRun(ctx context.Context, session *conga.Session) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, "Node.PostRun")
	}

	if n.FailPostRun.Get() {
		return errors.New("node postrun failed")
	}

	return nil
}

func (n *TrackedNodeCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	root := session.Root[TrackedLifecycleRoot]()
	if root != nil {
		root.Logs = append(root.Logs, fmt.Sprintf("Node.Finally(err=%v)", err != nil))
	}

	if n.FailFinally.Get() {
		return errors.New("node finally failed")
	}

	return nil
}

func TestLifecycle_FullHappyPath(t *testing.T) {
	t.Parallel()

	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"cluster", "node", "worker-1"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)

	expectedLogs := []string{
		// 1. Top-down Validate
		"Root.Validate",
		"Cluster.Validate",
		"Node.Validate",
		// 2. Top-down PreRun
		"Root.PreRun",
		"Cluster.PreRun",
		"Node.PreRun",
		// 3. Leaf Run
		"Node.Run",
		// 4. Bottom-up PostRun (success only)
		"Node.PostRun",
		"Cluster.PostRun",
		"Root.PostRun",
		// 5. Bottom-up Finally
		"Node.Finally(err=false)",
		"Cluster.Finally(err=false)",
		"Root.Finally(err=false)",
	}

	assert.Equal(t, expectedLogs, cli.Logs)

	node := cli.Cluster.Get().Node.Get()
	assert.Equal(t, "clustermgr cluster node", node.capturedPath)
	assert.Equal(t, "node", node.capturedActive)
}

func TestLifecycle_PreRunPartialFailure_Issue04(t *testing.T) {
	t.Parallel()

	// Scenario: Root.PreRun succeeds, Cluster.PreRun succeeds, Node.PreRun fails!
	// Requirement: Root.Finally and Cluster.Finally MUST execute in reverse order.
	// Node.Finally, Node.Run, Node.PostRun, Cluster.PostRun, Root.PostRun MUST NOT execute.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"cluster", "node", "--fail-node-pre", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node prerun failed")

	expectedLogs := []string{
		// 1. Top-down Validate
		"Root.Validate",
		"Cluster.Validate",
		"Node.Validate",
		// 2. Top-down PreRun (Node fails)
		"Root.PreRun",
		"Cluster.PreRun",
		"Node.PreRun",
		// 3. Unwind Finally for initialized ancestors only (Cluster then Root)
		"Cluster.Finally(err=true)",
		"Root.Finally(err=true)",
	}

	assert.Equal(t, expectedLogs, cli.Logs)
}

func TestLifecycle_RunFailure_SkipsPostRun_RunsAllFinally(t *testing.T) {
	t.Parallel()

	// Scenario: Node.Run fails!
	// Requirement: PostRun hooks are skipped. Finally hooks run bottom-up for all initialized commands.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"cluster", "node", "--fail-node-run", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node run failed")

	expectedLogs := []string{
		// 1. Top-down Validate
		"Root.Validate",
		"Cluster.Validate",
		"Node.Validate",
		// 2. Top-down PreRun
		"Root.PreRun",
		"Cluster.PreRun",
		"Node.PreRun",
		// 3. Leaf Run (fails)
		"Node.Run",
		// 4. PostRun skipped! Bottom-up Finally runs for all:
		"Node.Finally(err=true)",
		"Cluster.Finally(err=true)",
		"Root.Finally(err=true)",
	}

	assert.Equal(t, expectedLogs, cli.Logs)
}

func TestLifecycle_MultipleTeardownErrors_Joined(t *testing.T) {
	t.Parallel()

	// Scenario: Node.Run fails AND Node.Finally fails AND Root.Finally fails!
	// Requirement: All teardowns execute (Cluster.Finally runs too), and all errors are joined.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"--fail-root-finally", "cluster", "node", "--fail-node-run", "--fail-node-finally", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)

	// Verify all 3 errors are present in the joined error
	assert.Contains(t, err.Error(), "node run failed")
	assert.Contains(t, err.Error(), "node finally failed")
	assert.Contains(t, err.Error(), "root finally failed")

	// Cluster.Finally still executed even though Node.Finally errored!
	assert.True(t, slices.Contains(cli.Logs, "Cluster.Finally(err=true)"))
}

func TestLifecycle_ValidationFailure_ZeroSideEffects(t *testing.T) {
	t.Parallel()

	// Scenario: Validate fails at Root.
	// Requirement: Zero PreRun, Run, PostRun, Finally hooks execute.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"--fail-validate", "cluster", "node", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, []string{"Root.Validate"}, cli.Logs)
}

func TestLifecycle_IncompleteSubcommand_ZeroSideEffects(t *testing.T) {
	t.Parallel()

	// Scenario: User invokes non-runnable intermediate command "clustermgr cluster" without "node"
	// Requirement: Zero PreRun, Run, PostRun, Finally hooks execute. Returned as ErrSubcommandRequired.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("clustermgr").
		Args([]string{"cluster"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Zero(t, cli.Logs)
}
