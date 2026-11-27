package conga_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"

	_ "embed"
)

var (
	// Golden Exit output for suppressed Finally error scenarios.
	//go:embed testdata/lifecycle/joined_primary_and_suppressed.golden
	goldenLifecycleJoinedPrimaryAndSuppressed string

	//go:embed testdata/lifecycle/multiple_teardown_errors.golden
	goldenLifecycleMultipleTeardownErrors string

	//go:embed testdata/lifecycle/no_dedupe_suppressed.golden
	goldenLifecycleNoDedupeSuppressed string

	//go:embed testdata/lifecycle/suppressed_coder_and_usage.golden
	goldenLifecycleSuppressedCoderAndUsage string
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

func TestLifecycle_MultipleTeardownErrors_Suppressed(t *testing.T) {
	t.Parallel()

	// Scenario: Node.Run fails AND Node.Finally fails AND Root.Finally fails!
	// Requirement: All teardowns execute (Cluster.Finally runs too), and the
	// run's error decides the outcome while Finally errors are suppressed.
	var (
		stderrBuffer bytes.Buffer
		exitCode     = -1
	)

	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Name("app").
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code }).
		Args([]string{"--fail-root-finally", "cluster", "node", "--fail-node-run", "--fail-node-finally", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "node run failed\nnode finally failed\nroot finally failed", err.Error())
	assert.True(t, slices.Contains(cli.Logs, "Cluster.Finally(err=true)"))

	members := conga.DiagnosticOf(err).Members()
	assert.Equal(t, 3, len(members))
	assert.Equal(t, "node run failed", members[0].Message())
	assert.Equal(t, "node finally failed", members[1].Message())
	assert.Equal(t, "root finally failed", members[2].Message())

	app.Exit(err)

	assert.Equal(t, 1, exitCode)
	assert.Equal(t, goldenLifecycleMultipleTeardownErrors, stderrBuffer.String())
}

func TestLifecycle_ValidationFailure_ZeroSideEffects(t *testing.T) {
	t.Parallel()

	// Scenario: Validate fails at Root.
	// Requirement: Zero PreRun, Run, PostRun, Finally hooks execute.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Args([]string{"--fail-validate", "cluster", "node", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertNotUsageError(t, err)
	assert.Equal(t, "root validation failed", err.Error())
	assert.Equal(t, []string{"Root.Validate"}, cli.Logs)
}

func TestLifecycle_IncompleteSubcommand_ZeroSideEffects(t *testing.T) {
	t.Parallel()

	// Scenario: User invokes non-runnable intermediate command "clustermgr cluster" without "node"
	// Requirement: Zero PreRun, Run, PostRun, Finally hooks execute. Returned as ErrSubcommandRequired.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Args([]string{"cluster"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Zero(t, cli.Logs)
}

func TestLifecycle_PostRunFailure_StopsAndUnwinds(t *testing.T) {
	t.Parallel()

	// Scenario: Node.PostRun fails!
	// Requirement: remaining PostRun hooks are skipped, Finally still unwinds bottom-up for all initialized commands.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Args([]string{"cluster", "node", "--fail-node-post", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node postrun failed")

	expectedLogs := []string{
		// 1. Top-down Validate
		"Root.Validate",
		"Cluster.Validate",
		"Node.Validate",
		// 2. Top-down PreRun
		"Root.PreRun",
		"Cluster.PreRun",
		"Node.PreRun",
		// 3. Leaf Run succeeds
		"Node.Run",
		// 4. Bottom-up PostRun (Node fails; Cluster/Root PostRun skipped)
		"Node.PostRun",
		// 5. Guaranteed bottom-up Finally
		"Node.Finally(err=true)",
		"Cluster.Finally(err=true)",
		"Root.Finally(err=true)",
	}
	assert.Equal(t, expectedLogs, cli.Logs)
}

func TestLifecycle_PostRunFailure_ClusterUnwindsBeforeRoot(t *testing.T) {
	t.Parallel()

	// Scenario: Cluster.PostRun fails after Node.PostRun succeeds!
	// Requirement: Root.PostRun is skipped, Finally still unwinds bottom-up.
	cli := new(TrackedLifecycleRoot)
	app := conga.New(cli).
		Args([]string{"cluster", "--fail-cluster-post", "node", "worker-1"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster postrun failed")

	expectedLogs := []string{
		// 1. Top-down Validate
		"Root.Validate",
		"Cluster.Validate",
		"Node.Validate",
		// 2. Top-down PreRun
		"Root.PreRun",
		"Cluster.PreRun",
		"Node.PreRun",
		// 3. Leaf Run succeeds
		"Node.Run",
		// 4. Bottom-up PostRun (Node succeeds, Cluster fails, Root skipped)
		"Node.PostRun",
		"Cluster.PostRun",
		// 5. Guaranteed bottom-up Finally
		"Node.Finally(err=true)",
		"Cluster.Finally(err=true)",
		"Root.Finally(err=true)",
	}
	assert.Equal(t, expectedLogs, cli.Logs)
}

type matrixTaggedError struct {
	message string
	note    string
	origins []string
	hint    string
}

func (e *matrixTaggedError) Error() string     { return e.message }
func (e *matrixTaggedError) Note() string      { return e.note }
func (e *matrixTaggedError) Origins() []string { return e.origins }
func (e *matrixTaggedError) Hint() string      { return e.hint }

type matrixCodedError struct {
	message string
	note    string
	code    int
}

func (e *matrixCodedError) Error() string { return e.message }
func (e *matrixCodedError) Note() string  { return e.note }
func (e *matrixCodedError) ExitCode() int { return e.code }

type stageBehavior struct {
	err        error
	panicValue any
	hasPanic   bool
}

type finallyBehavior struct {
	err         error
	returnInput bool
	wrapInput   bool
	returnUsage string
	panicValue  any
	hasPanic    bool
	seen        []error
	calls       int
}

type matrixRootCmd struct {
	rootPreRun  stageBehavior
	midPreRun   stageBehavior
	leafPreRun  stageBehavior
	leafRun     stageBehavior
	leafPostRun stageBehavior
	midPostRun  stageBehavior
	rootPostRun stageBehavior

	rootFin *finallyBehavior
	midFin  *finallyBehavior
	leafFin *finallyBehavior

	Mid conga.Command[matrixMidCmd] `summary:"Mid"`
}

func (r *matrixRootCmd) PreRun(_ context.Context, _ *conga.Session) error {
	return runMatrixStage(r.rootPreRun)
}

func (r *matrixRootCmd) PostRun(_ context.Context, _ *conga.Session) error {
	return runMatrixStage(r.rootPostRun)
}

func (r *matrixRootCmd) Finally(_ context.Context, session *conga.Session, err error) error {
	return runMatrixFinally(session, r.rootFin, err)
}

type matrixMidCmd struct {
	Leaf conga.Command[matrixLeafCmd] `summary:"Leaf"`
}

func (m *matrixMidCmd) PreRun(_ context.Context, session *conga.Session) error {
	return runMatrixStage(session.Root[matrixRootCmd]().midPreRun)
}

func (m *matrixMidCmd) PostRun(_ context.Context, session *conga.Session) error {
	return runMatrixStage(session.Root[matrixRootCmd]().midPostRun)
}

func (m *matrixMidCmd) Finally(_ context.Context, session *conga.Session, err error) error {
	return runMatrixFinally(session, session.Root[matrixRootCmd]().midFin, err)
}

type matrixLeafCmd struct{}

func (l *matrixLeafCmd) PreRun(_ context.Context, session *conga.Session) error {
	return runMatrixStage(session.Root[matrixRootCmd]().leafPreRun)
}

func (l *matrixLeafCmd) Run(_ context.Context, session *conga.Session) error {
	return runMatrixStage(session.Root[matrixRootCmd]().leafRun)
}

func (l *matrixLeafCmd) PostRun(_ context.Context, session *conga.Session) error {
	return runMatrixStage(session.Root[matrixRootCmd]().leafPostRun)
}

func (l *matrixLeafCmd) Finally(_ context.Context, session *conga.Session, err error) error {
	return runMatrixFinally(session, session.Root[matrixRootCmd]().leafFin, err)
}

func runMatrixStage(behavior stageBehavior) error {
	if behavior.hasPanic {
		panic(behavior.panicValue)
	}

	return behavior.err
}

func runMatrixFinally(session *conga.Session, behavior *finallyBehavior, err error) error {
	if behavior == nil {
		return nil
	}

	behavior.calls++
	behavior.seen = append(behavior.seen, err)

	if behavior.hasPanic {
		panic(behavior.panicValue)
	}

	if behavior.returnUsage != "" {
		return session.UsageError(behavior.returnUsage)
	}

	if behavior.returnInput {
		return err
	}

	if behavior.wrapInput && err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}

	return behavior.err
}

func TestLifecycle_FinallyErrorMatrix(t *testing.T) {
	t.Parallel()

	t.Run("lone Finally failure on successful run is returned unwrapped", func(t *testing.T) {
		t.Parallel()

		finallyErr := &matrixCodedError{message: "close failed", code: 42}
		rootFin := &finallyBehavior{}
		midFin := &finallyBehavior{err: finallyErr}
		leafFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.rootFin = rootFin
		cli.midFin = midFin
		cli.leafFin = leafFin

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"mid", "leaf"})

		err := app.RunContext(t.Context())

		assert.True(t, err == finallyErr) //nolint:errorlint // identity of unwrapped lone Finally failure is the property under test
		assert.Equal(t, "close failed", err.Error())
		assert.Equal(t, []error{nil}, leafFin.seen)
		assert.Equal(t, []error{nil}, midFin.seen)
		assert.Equal(t, []error{nil}, rootFin.seen)

		app.Exit(err)

		assert.Equal(t, 42, exitCode)
		assert.Equal(t, "app: error: close failed\n", stderrBuffer.String())
	})

	t.Run("multiple Finally failures on successful run use first as primary", func(t *testing.T) {
		t.Parallel()

		leafErr := &matrixCodedError{message: "leaf close failed", code: 21}
		rootErr := &matrixCodedError{message: "root close failed", code: 75}
		rootFin := &finallyBehavior{err: rootErr}
		midFin := &finallyBehavior{}
		leafFin := &finallyBehavior{err: leafErr}

		cli := new(matrixRootCmd)
		cli.rootFin = rootFin
		cli.midFin = midFin
		cli.leafFin = leafFin

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"mid", "leaf"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "leaf close failed\nroot close failed", err.Error())
		assert.True(t, errors.Is(err, leafErr))
		assert.False(t, errors.Is(err, rootErr))
		assert.Equal(t, []error{nil}, leafFin.seen)
		assert.Equal(t, []error{nil}, midFin.seen)
		assert.Equal(t, []error{nil}, rootFin.seen)

		members := conga.DiagnosticOf(err).Members()
		assert.Equal(t, 2, len(members))
		assert.Equal(t, "leaf close failed", members[0].Message())
		assert.Equal(t, "root close failed", members[1].Message())

		app.Exit(err)

		assert.Equal(t, 21, exitCode)
		assert.Equal(t, "app: error: leaf close failed\napp: error: root close failed\n", stderrBuffer.String())
	})

	t.Run("failing stage at each level passes stage error only to initialized ancestors", func(t *testing.T) {
		t.Parallel()

		stageCases := []struct {
			name      string
			configure func(cli *matrixRootCmd, stageErr error)
			wantSeen  func(rootFin, midFin, leafFin *finallyBehavior, stageErr error)
			wantError string
		}{
			{
				name: "Root.PreRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.rootPreRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, _ error) {
					assert.Equal(t, 0, leafFin.calls)
					assert.Equal(t, 0, midFin.calls)
					assert.Equal(t, 0, rootFin.calls)
				},
				wantError: "stage boom",
			},
			{
				name: "Mid.PreRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.midPreRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, 0, leafFin.calls)
					assert.Equal(t, 0, midFin.calls)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nroot finally failed",
			},
			{
				name: "Leaf.PreRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.leafPreRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, 0, leafFin.calls)
					assert.Equal(t, []error{stageErr}, midFin.seen)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nmid finally failed\nroot finally failed",
			},
			{
				name: "Leaf.Run",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.leafRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, []error{stageErr}, leafFin.seen)
					assert.Equal(t, []error{stageErr}, midFin.seen)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nleaf finally failed\nmid finally failed\nroot finally failed",
			},
			{
				name: "Leaf.PostRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.leafPostRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, []error{stageErr}, leafFin.seen)
					assert.Equal(t, []error{stageErr}, midFin.seen)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nleaf finally failed\nmid finally failed\nroot finally failed",
			},
			{
				name: "Mid.PostRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.midPostRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, []error{stageErr}, leafFin.seen)
					assert.Equal(t, []error{stageErr}, midFin.seen)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nleaf finally failed\nmid finally failed\nroot finally failed",
			},
			{
				name: "Root.PostRun",
				configure: func(cli *matrixRootCmd, stageErr error) {
					cli.rootPostRun = stageBehavior{err: stageErr}
				},
				wantSeen: func(rootFin, midFin, leafFin *finallyBehavior, stageErr error) {
					assert.Equal(t, []error{stageErr}, leafFin.seen)
					assert.Equal(t, []error{stageErr}, midFin.seen)
					assert.Equal(t, []error{stageErr}, rootFin.seen)
				},
				wantError: "stage boom\nleaf finally failed\nmid finally failed\nroot finally failed",
			},
		}

		for _, testCase := range stageCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				stageErr := errors.New("stage boom")
				leafFinErr := errors.New("leaf finally failed")
				midFinErr := errors.New("mid finally failed")
				rootFinErr := errors.New("root finally failed")

				rootFin := &finallyBehavior{err: rootFinErr}
				midFin := &finallyBehavior{err: midFinErr}
				leafFin := &finallyBehavior{err: leafFinErr}

				cli := new(matrixRootCmd)
				cli.rootFin = rootFin
				cli.midFin = midFin
				cli.leafFin = leafFin
				testCase.configure(cli, stageErr)

				err := conga.New(cli).Args([]string{"mid", "leaf"}).RunContext(t.Context())

				assert.Error(t, err)
				assert.Equal(t, testCase.wantError, err.Error())
				assert.True(t, errors.Is(err, stageErr))
				assert.False(t, errors.Is(err, leafFinErr))
				assert.False(t, errors.Is(err, midFinErr))
				assert.False(t, errors.Is(err, rootFinErr))
				testCase.wantSeen(rootFin, midFin, leafFin, stageErr)
			})
		}
	})

	t.Run("no dedupe prints repeated and wrapped received error", func(t *testing.T) {
		t.Parallel()

		runErr := errors.New("run failed")
		rootFin := &finallyBehavior{wrapInput: true}
		midFin := &finallyBehavior{returnInput: true}
		leafFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: runErr}
		cli.rootFin = rootFin
		cli.midFin = midFin
		cli.leafFin = leafFin

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"mid", "leaf"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "run failed\nrun failed\ncleanup: run failed", err.Error())
		assert.True(t, errors.Is(err, runErr))
		assert.True(t, leafFin.seen[0] == runErr) //nolint:errorlint // identity check
		assert.True(t, midFin.seen[0] == runErr)  //nolint:errorlint // identity check
		assert.True(t, rootFin.seen[0] == runErr) //nolint:errorlint // identity check

		app.Exit(err)

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, goldenLifecycleNoDedupeSuppressed, stderrBuffer.String())
	})

	t.Run("suppressed ExitCoder and UsageError do not override primary exit code or print usage block", func(t *testing.T) {
		t.Parallel()

		primaryErr := &matrixCodedError{message: "primary failure", note: "primary note", code: 19}
		suppressedCoder := &matrixCodedError{message: "suppressed coder", code: 75}
		rootFin := &finallyBehavior{returnUsage: "suppressed usage"}
		midFin := &finallyBehavior{err: suppressedCoder}
		leafFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: primaryErr}
		cli.rootFin = rootFin
		cli.midFin = midFin
		cli.leafFin = leafFin

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"mid", "leaf"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.True(t, errors.Is(err, primaryErr))
		assert.False(t, errors.Is(err, suppressedCoder))
		coded, hasCoded := errors.AsType[*matrixCodedError](err)
		assert.True(t, hasCoded)
		assert.Equal(t, 19, coded.code)
		_, hasUsage := errors.AsType[*conga.UsageError](err)
		assert.False(t, hasUsage)
		assert.Equal(t, "", app.UsageBlock(err))

		app.Exit(err)

		assert.Equal(t, 19, exitCode)
		assert.Equal(t, goldenLifecycleSuppressedCoderAndUsage, stderrBuffer.String())
	})
}

func TestLifecycle_FinallyRendering(t *testing.T) {
	t.Parallel()

	t.Run("joined primary and joined suppressed errors flatten with prefixes", func(t *testing.T) {
		t.Parallel()

		primaryFirst := &matrixTaggedError{message: "first primary", note: "p1 note"}
		primarySecond := &matrixTaggedError{message: "second primary", hint: "p2 hint"}
		suppressedFirst := errors.New("first suppressed")
		suppressedSecond := &matrixTaggedError{message: "second suppressed", origins: []string{"cleanup"}}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: errors.Join(primaryFirst, primarySecond)}
		cli.leafFin = &finallyBehavior{err: errors.Join(suppressedFirst, suppressedSecond)}

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"mid", "leaf"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, 4, len(conga.DiagnosticOf(err).Members()))

		app.Exit(err)

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, goldenLifecycleJoinedPrimaryAndSuppressed, stderrBuffer.String())
	})

	t.Run("errors.Join containing a finallyError flattens suppressed errors with prefixes and lines", func(t *testing.T) {
		t.Parallel()

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: &matrixTaggedError{message: "inner run", note: "inner run note"}}
		cli.leafFin = &finallyBehavior{err: &matrixTaggedError{message: "inner finally", hint: "inner fin hint"}}

		innerErr := conga.New(cli).Args([]string{"mid", "leaf"}).RunContext(t.Context())
		extraErr := &matrixTaggedError{message: "outer extra", hint: "extra hint"}
		joined := errors.Join(innerErr, extraErr)

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New[matrixRootCmd]().
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code })

		members := conga.DiagnosticOf(joined).Members()

		assert.Equal(t, 3, len(members))
		assert.Equal(t, "inner run", members[0].Message())
		assert.Equal(t, "inner run note", members[0].Note())
		assert.Equal(t, "inner finally", members[1].Message())
		assert.Equal(t, "inner fin hint", members[1].Hint())
		assert.Equal(t, "outer extra", members[2].Message())
		assert.Equal(t, "extra hint", members[2].Hint())

		app.Exit(joined)

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, "app: error: inner run\n  note: inner run note\napp: error: inner finally\n  hint: inner fin hint\napp: error: outer extra\n  hint: extra hint\n", stderrBuffer.String())
	})

	t.Run("fmt.Errorf wrapping a finallyError stays on single-error path", func(t *testing.T) {
		t.Parallel()

		primaryErr := errors.New("run failed")
		finallyErr := errors.New("finally failed")

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: primaryErr}
		cli.leafFin = &finallyBehavior{err: finallyErr}

		innerErr := conga.New(cli).Args([]string{"mid", "leaf"}).RunContext(t.Context())
		wrapped := fmt.Errorf("outer: %w", innerErr)

		var stderrBuffer bytes.Buffer

		conga.New[matrixRootCmd]().
			Name("app").
			Stderr(&stderrBuffer).
			ExitFunc(func(int) {}).
			Exit(wrapped)

		assert.True(t, errors.Is(wrapped, primaryErr))
		assert.False(t, errors.Is(wrapped, finallyErr))
		assert.Equal(t, 1, len(conga.DiagnosticOf(wrapped).Members()))
		assert.Equal(t, "app: error: outer: run failed\nfinally failed\n", stderrBuffer.String())
	})
}

func TestLifecycle_Panic(t *testing.T) {
	t.Parallel()

	t.Run("stage panic runs initialized Finally hooks with panicError, writes hook errors, and repanics", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{err: &matrixTaggedError{message: "leaf cleanup failed", hint: "check disk"}}
		midFin := &finallyBehavior{}
		rootFin := &finallyBehavior{err: errors.New("root cleanup failed")}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{hasPanic: true, panicValue: "boom"}
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		var stderrBuffer bytes.Buffer

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			Args([]string{"mid", "leaf"})

		stack := capturePanicStack(t, "boom", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, "panic: boom", leafFin.seen[0].Error())
		assert.Equal(t, "panic: boom", midFin.seen[0].Error())
		assert.Equal(t, "panic: boom", rootFin.seen[0].Error())
		assert.Equal(t, "app: error: leaf cleanup failed\n  hint: check disk\napp: error: root cleanup failed\n", stderrBuffer.String())
		assert.Contains(t, stack, "(*matrixLeafCmd).Run")
	})

	t.Run("stage panic with uncomparable value runs hooks and repanics with same slice", func(t *testing.T) {
		t.Parallel()

		panicValue := []int{1, 2}
		leafFin := &finallyBehavior{err: errors.New("cleanup error")}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{hasPanic: true, panicValue: panicValue}
		cli.leafFin = leafFin

		var stderrBuffer bytes.Buffer

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			Args([]string{"mid", "leaf"})

		assertx.PanicsWithValue(t, panicValue, func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, "panic: [1 2]", leafFin.seen[0].Error())
		assert.Equal(t, "app: error: cleanup error\n", stderrBuffer.String())
	})

	t.Run("Mid.PreRun panic skips Mid.Finally and Leaf.Finally and runs Root.Finally", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{}
		midFin := &finallyBehavior{}
		rootFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.midPreRun = stageBehavior{hasPanic: true, panicValue: "mid prerun panic"}
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		app := conga.New(cli).Name("app").Args([]string{"mid", "leaf"})

		assertx.PanicsWithValue(t, "mid prerun panic", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 0, leafFin.calls)
		assert.Equal(t, 0, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, "panic: mid prerun panic", rootFin.seen[0].Error())
	})

	t.Run("PostRun panic runs all initialized Finally hooks", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{}
		midFin := &finallyBehavior{}
		rootFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.midPostRun = stageBehavior{hasPanic: true, panicValue: "postrun panic"}
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		app := conga.New(cli).Name("app").Args([]string{"mid", "leaf"})

		assertx.PanicsWithValue(t, "postrun panic", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, "panic: postrun panic", leafFin.seen[0].Error())
	})

	t.Run("panic in Finally hook on successful run passes panicError to ancestors", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{hasPanic: true, panicValue: "leaf finally panic"}
		midFin := &finallyBehavior{err: errors.New("mid cleanup err")}
		rootFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		var stderrBuffer bytes.Buffer

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			Args([]string{"mid", "leaf"})

		stack := capturePanicStack(t, "leaf finally panic", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, []error{nil}, leafFin.seen)
		assert.Equal(t, "panic: leaf finally panic", midFin.seen[0].Error())
		assert.Equal(t, "panic: leaf finally panic", rootFin.seen[0].Error())
		assert.Equal(t, "app: error: mid cleanup err\n", stderrBuffer.String())
		assert.Contains(t, stack, "(*matrixLeafCmd).Finally")
	})

	t.Run("panic in Finally hook after normal stage error writes stage error first without usage block", func(t *testing.T) {
		t.Parallel()

		stageErr := errors.New("stage failed")
		leafFin := &finallyBehavior{err: errors.New("leaf cleanup err")}
		midFin := &finallyBehavior{hasPanic: true, panicValue: "mid finally panic"}
		rootFin := &finallyBehavior{returnUsage: "bad usage during panic"}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{err: stageErr}
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		var stderrBuffer bytes.Buffer

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			Args([]string{"mid", "leaf"})

		stack := capturePanicStack(t, "mid finally panic", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, []error{stageErr}, leafFin.seen)
		assert.Equal(t, []error{stageErr}, midFin.seen)
		assert.Equal(t, "panic: mid finally panic", rootFin.seen[0].Error())
		assert.Equal(t, "app: error: stage failed\napp: error: leaf cleanup err\napp: error: bad usage during panic\n", stderrBuffer.String())
		assert.False(t, strings.Contains(stderrBuffer.String(), "Usage:"))
		assert.Contains(t, stack, "(*matrixMidCmd).Finally")
	})

	t.Run("panic in two Finally hooks after normal run passes mid panic to root once", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{hasPanic: true, panicValue: "leaf boom"}
		midFin := &finallyBehavior{hasPanic: true, panicValue: "mid boom"}
		rootFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		app := conga.New(cli).Name("app").Args([]string{"mid", "leaf"})

		stack := capturePanicStack(t, "mid boom", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, "panic: leaf boom", midFin.seen[0].Error())
		assert.Equal(t, "panic: mid boom", rootFin.seen[0].Error())
		assert.Contains(t, stack, "(*matrixLeafCmd).Finally")
		assert.Contains(t, stack, "(*matrixMidCmd).Finally")
	})

	t.Run("panic in two Finally hooks after stage panic passes mid panic to root once", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{hasPanic: true, panicValue: "leaf boom"}
		midFin := &finallyBehavior{hasPanic: true, panicValue: "mid boom"}
		rootFin := &finallyBehavior{}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{hasPanic: true, panicValue: "stage boom"}
		cli.leafFin = leafFin
		cli.midFin = midFin
		cli.rootFin = rootFin

		app := conga.New(cli).Name("app").Args([]string{"mid", "leaf"})

		stack := capturePanicStack(t, "mid boom", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, 1, leafFin.calls)
		assert.Equal(t, 1, midFin.calls)
		assert.Equal(t, 1, rootFin.calls)
		assert.Equal(t, "panic: stage boom", leafFin.seen[0].Error())
		assert.Equal(t, "panic: leaf boom", midFin.seen[0].Error())
		assert.Equal(t, "panic: mid boom", rootFin.seen[0].Error())
		assert.Contains(t, stack, "(*matrixLeafCmd).Run")
		assert.Contains(t, stack, "(*matrixLeafCmd).Finally")
		assert.Contains(t, stack, "(*matrixMidCmd).Finally")
	})

	t.Run("panic path with UsageError prints line only without usage block", func(t *testing.T) {
		t.Parallel()

		leafFin := &finallyBehavior{returnUsage: "bad flag in cleanup"}

		cli := new(matrixRootCmd)
		cli.leafRun = stageBehavior{hasPanic: true, panicValue: "stage crash"}
		cli.leafFin = leafFin

		var stderrBuffer bytes.Buffer

		app := conga.New(cli).
			Name("app").
			Stderr(&stderrBuffer).
			Args([]string{"mid", "leaf"})

		assertx.PanicsWithValue(t, "stage crash", func() {
			_ = app.RunContext(t.Context())
		})

		assert.Equal(t, "app: error: bad flag in cleanup\n", stderrBuffer.String())
		assert.False(t, strings.Contains(stderrBuffer.String(), "Usage:"))
	})
}

func TestLifecycle_Panic_ColorOutput(t *testing.T) {
	t.Setenv("CONGA_COLOR", "1")

	leafFin := &finallyBehavior{err: &matrixTaggedError{message: "cleanup failed", hint: "check disk"}}

	cli := new(matrixRootCmd)
	cli.leafRun = stageBehavior{hasPanic: true, panicValue: "boom"}
	cli.leafFin = leafFin

	var stderrBuffer bytes.Buffer

	app := conga.New(cli).
		Name("app").
		Stderr(&stderrBuffer).
		Args([]string{"mid", "leaf"})

	assertx.PanicsWithValue(t, "boom", func() {
		_ = app.RunContext(t.Context())
	})

	assert.Equal(t, "app: \x1b[1;31merror:\x1b[0m cleanup failed\n  \x1b[1;36mhint:\x1b[0m check disk\n", stderrBuffer.String())
}

type runCancelsContextCLI struct {
	ctxErr      func() error
	ctxErrInRun error
	afterDone   chan struct{}
}

func (c *runCancelsContextCLI) Run(ctx context.Context, _ *conga.Session) error {
	c.ctxErr = ctx.Err
	c.ctxErrInRun = ctx.Err()
	c.afterDone = make(chan struct{})

	context.AfterFunc(ctx, func() {
		close(c.afterDone)
	})

	return nil
}

func TestLifecycle_AppRunCancelsContextOnReturn(t *testing.T) {
	t.Parallel()

	cli := new(runCancelsContextCLI)
	app := conga.New(cli).Name("app").Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.NoError(t, cli.ctxErrInRun)
	assert.IsError(t, cli.ctxErr(), context.Canceled)

	select {
	case <-cli.afterDone:
	case <-time.After(time.Second):
		t.Fatal("context.AfterFunc did not fire after App.Run returned")
	}
}

func capturePanicStack(t *testing.T, expected any, action func()) string {
	t.Helper()

	var stack string

	func() {
		defer func() {
			t.Helper()

			recovered := recover()
			if recovered == nil {
				t.Fatal("Expected function to panic")

				return
			}

			assert.Equal(t, expected, recovered)
			stack = string(debug.Stack())
		}()

		action()
	}()

	return stack
}
