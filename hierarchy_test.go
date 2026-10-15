package conga_test

import (
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

// Level 0: Root Application.
type MultiLevelRoot struct {
	Verbose conga.OptionalFlag[bool]         `help:"Global verbose flag" short:"v"`
	Env     conga.OptionalFlag[string]       `default:"dev"              help:"Target environment"`
	Cluster conga.Command[ClusterSubcommand] `summary:"Cluster commands"`

	rootPreRunCalled  bool
	rootPostRunCalled bool
	rootFinallyCalled bool
}

func (r *MultiLevelRoot) PreRun(ctx context.Context, session *conga.Session) error {
	r.rootPreRunCalled = true

	// Verify Root can access itself via session.Root[MultiLevelRoot]()
	if root := session.Root[MultiLevelRoot](); root != nil {
		root.Env.Get() // Verify accessible
	}

	return nil
}

func (r *MultiLevelRoot) PostRun(ctx context.Context, session *conga.Session) error {
	r.rootPostRunCalled = true

	return nil
}

func (r *MultiLevelRoot) Finally(ctx context.Context, session *conga.Session, err error) error {
	r.rootFinallyCalled = true

	return nil
}

// Level 1: Cluster Subcommand.
type ClusterSubcommand struct {
	Region      conga.OptionalFlag[string]    `default:"us-east-1"       help:"Cloud region"`
	ClusterName conga.OptionalFlag[string]    `help:"Cluster name"`
	Node        conga.Command[NodeSubcommand] `summary:"Node operations"`

	clusterPreRunCalled bool
}

func (c *ClusterSubcommand) PreRun(ctx context.Context, session *conga.Session) error {
	c.clusterPreRunCalled = true

	return nil
}

// Level 2: Node Sub-Subcommand.
type NodeSubcommand struct {
	Zone  conga.OptionalFlag[string]  `default:"us-east-1a"       help:"Availability zone"`
	Pool  conga.OptionalFlag[string]  `help:"Node pool name"`
	Drain conga.Command[DrainLeafCmd] `summary:"Drain operations"`

	nodePreRunCalled bool
}

func (n *NodeSubcommand) PreRun(ctx context.Context, session *conga.Session) error {
	n.nodePreRunCalled = true

	return nil
}

// Level 3: Drain Leaf Command.
type DrainLeafCmd struct {
	NodeID conga.RequiredArg[string] `help:"Target node ID"`
	Force  conga.OptionalFlag[bool]  `help:"Force drain"    short:"f"`

	// Captured values for test verification
	runExecuted           bool
	capturedRootVerbose   bool
	capturedRootEnv       string
	capturedClusterRegion string
	capturedClusterName   string
	capturedNodeZone      string
	capturedNodePool      string
	capturedSelfNodeID    string
	capturedSelfForce     bool
}

func (d *DrainLeafCmd) Run(ctx context.Context, session *conga.Session) error {
	d.runExecuted = true

	// 1. Read Root and ancestor values by walking the selected Command fields
	// from the root down to the active leaf.
	if root := session.Root[MultiLevelRoot](); root != nil {
		d.capturedRootVerbose = root.Verbose.Get()
		d.capturedRootEnv = root.Env.Get()

		cluster := root.Cluster.Get()
		d.capturedClusterRegion = cluster.Region.Get()
		d.capturedClusterName = cluster.ClusterName.Get()

		node := cluster.Node.Get()
		d.capturedNodeZone = node.Zone.Get()
		d.capturedNodePool = node.Pool.Get()
	}

	// 2. Read Own values
	d.capturedSelfNodeID = d.NodeID.Get()
	d.capturedSelfForce = d.Force.Get()

	return nil
}

func TestMultiLevelCommandHierarchy(t *testing.T) {
	t.Parallel()

	// CLI args with flags placed across root, subcommand, sub-subcommand, and leaf levels in mixed positions
	args := []string{
		"--env", "production",
		"cluster",
		"--region", "eu-central-1",
		"--cluster-name", "k8s-prod-eu",
		"node",
		"--zone", "eu-central-1b",
		"--pool", "gpu-nodes",
		"drain",
		"node-99",
		"-v",
		"-f",
	}

	cli := new(MultiLevelRoot)
	app := conga.New(cli).
		Args(args)

	err := app.RunContext(t.Context())

	assert.NoError(t, err)

	root := cli

	// 1. Verify Selection State
	assert.True(t, root.Cluster.IsSelected())
	cluster := root.Cluster.Get()
	assert.True(t, cluster.Node.IsSelected())
	node := cluster.Node.Get()
	assert.True(t, node.Drain.IsSelected())
	drain := node.Drain.Get()

	// 2. Verify Lifecycle Hooks
	assert.True(t, root.rootPreRunCalled)
	assert.True(t, cluster.clusterPreRunCalled)
	assert.True(t, node.nodePreRunCalled)
	assert.True(t, drain.runExecuted)
	assert.True(t, root.rootPostRunCalled)
	assert.True(t, root.rootFinallyCalled)

	// 3. Verify Flag Values Resolved on Structs
	assert.True(t, root.Verbose.Get())
	assert.Equal(t, "production", root.Env.Get())
	assert.Equal(t, "eu-central-1", cluster.Region.Get())
	assert.Equal(t, "k8s-prod-eu", cluster.ClusterName.Get())
	assert.Equal(t, "eu-central-1b", node.Zone.Get())
	assert.Equal(t, "gpu-nodes", node.Pool.Get())
	assert.Equal(t, "node-99", drain.NodeID.Get())
	assert.True(t, drain.Force.Get())

	// 4. Verify Context Propagation values captured in Drain.Run(ctx)
	assert.True(t, drain.capturedRootVerbose)
	assert.Equal(t, "production", drain.capturedRootEnv)
	assert.Equal(t, "eu-central-1", drain.capturedClusterRegion)
	assert.Equal(t, "k8s-prod-eu", drain.capturedClusterName)
	assert.Equal(t, "eu-central-1b", drain.capturedNodeZone)
	assert.Equal(t, "gpu-nodes", drain.capturedNodePool)
	assert.Equal(t, "node-99", drain.capturedSelfNodeID)
	assert.True(t, drain.capturedSelfForce)
}

func TestNewSessionForTest_UnitTesting(t *testing.T) {
	t.Parallel()

	// Verify that developers can unit-test their leaf commands directly
	// by constructing a session with conga.NewSessionForTest
	var mockRoot MultiLevelRoot
	conga.OptionalFlagContainer(&mockRoot.Verbose).SetValue(true, conga.SourceCLI, "-v")
	conga.OptionalFlagContainer(&mockRoot.Env).SetValue("staging", conga.SourceCLI, "--env")

	session := conga.NewSessionForTest(&mockRoot)

	var drain DrainLeafCmd
	conga.RequiredArgContainer(&drain.NodeID).SetValue("worker-unit-test", conga.SourceCLI, "arg")
	conga.OptionalFlagContainer(&drain.Force).SetValue(true, conga.SourceCLI, "-f")

	err := drain.Run(t.Context(), session)

	assert.NoError(t, err)
	assert.True(t, drain.capturedRootVerbose)
	assert.Equal(t, "staging", drain.capturedRootEnv)
	assert.Equal(t, "worker-unit-test", drain.capturedSelfNodeID)
	assert.True(t, drain.capturedSelfForce)
}
