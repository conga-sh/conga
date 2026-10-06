package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

func TestSession_RootAndStreams(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	app := conga.New[sessionTestRoot]().
		Name("cloudctl").
		Args([]string{"-v", "deploy", "api"}).
		Stdout(buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "verbose mode\ndeploying api\n", buffer.String())
}

func TestSession_Help(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	cmd := &sessionHelpCmd{}
	app := conga.New(cmd).
		Name("myapp").
		Args([]string{}).
		Stdout(buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.NotZero(t, cmd.helpText)
	assertGolden(t, "testdata/help/session.golden", goldenHelpSession, buffer.String())
	assert.Equal(t, cmd.helpText+"\n", buffer.String())
}

func TestSession_Usage(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	cmd := &sessionUsageCmd{}
	app := conga.New(cmd).
		Name("myapp").
		Args([]string{}).
		Stderr(buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.NotZero(t, cmd.usageText)
	assertGolden(t, "testdata/help/usage.golden", goldenHelpUsage, buffer.String())
	assert.Equal(t, cmd.usageText+"\n", buffer.String())
}

func TestSession_HelpAndUsage_NilOrEmpty(t *testing.T) {
	t.Parallel()

	var s *conga.Session
	assert.Equal(t, "", s.Help())
	assert.Equal(t, "", s.Usage())
}

func TestSession_Version(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	app := conga.New[sessionVersionCmd]().
		Name("myapp").
		Args([]string{}).
		Version("2.5.0").
		Stdout(buffer)

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "myapp has version 2.5.0 built with ")
}

func TestSession_Metadata(t *testing.T) {
	t.Parallel()

	cli := new(sessionMetadataCmd)
	app := conga.New(cli).
		Name("myapp").
		Args([]string{"sub"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "myapp sub", cli.Sub.Get().RecordedPath)
	assert.Equal(t, "sub", cli.Sub.Get().RecordedName)
}

func TestSession_LifecycleHooks(t *testing.T) {
	t.Parallel()

	cli := new(sessionLifecycleCmd)
	app := conga.New(cli).
		Name("runner").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)

	expected := []string{
		"Validate:runner",
		"PreRun:runner",
		"Run:runner",
		"PostRun:runner",
		"Finally:runner:false",
	}
	assert.Equal(t, expected, cli.Logs)
}

type sessionTestRoot struct {
	Verbose conga.OptionalFlag[bool]        `short:"v"`
	Deploy  conga.Command[sessionDeployCmd] `summary:"Deploy service"`
}

type sessionDeployCmd struct {
	Target conga.RequiredArg[string]
	Ran    bool
}

func (d *sessionDeployCmd) Run(ctx context.Context, session *conga.Session) error {
	d.Ran = true

	root := session.Root[sessionTestRoot]()
	if root != nil && root.Verbose.Get() {
		fmt.Fprintln(session.Stdout(), "verbose mode")
	}

	fmt.Fprintf(session.Stdout(), "deploying %s\n", d.Target.Get())

	return nil
}

type sessionHelpCmd struct {
	helpText string
}

func (h *sessionHelpCmd) Run(ctx context.Context, session *conga.Session) error {
	h.helpText = session.Help()
	fmt.Fprintln(session.Stdout(), h.helpText)

	return nil
}

type sessionUsageCmd struct {
	usageText string
}

func (u *sessionUsageCmd) Run(ctx context.Context, session *conga.Session) error {
	u.usageText = session.Usage()
	fmt.Fprintln(session.Stderr(), u.usageText)

	return nil
}

type sessionVersionCmd struct{}

func (v *sessionVersionCmd) Run(ctx context.Context, session *conga.Session) error {
	session.Version()

	return nil
}

type sessionMetadataCmd struct {
	Sub conga.Command[sessionLeafCmd] `summary:"Sub command"`
}

type sessionLeafCmd struct {
	RecordedPath string
	RecordedName string
}

func (l *sessionLeafCmd) Run(ctx context.Context, session *conga.Session) error {
	l.RecordedPath = session.Path()
	l.RecordedName = session.Name()

	return nil
}

type sessionLifecycleCmd struct {
	Logs []string
}

func (c *sessionLifecycleCmd) Validate(ctx context.Context, session *conga.Session) error {
	c.Logs = append(c.Logs, "Validate:"+session.Name())

	return nil
}

func (c *sessionLifecycleCmd) PreRun(ctx context.Context, session *conga.Session) error {
	c.Logs = append(c.Logs, "PreRun:"+session.Name())

	return nil
}

func (c *sessionLifecycleCmd) Run(ctx context.Context, session *conga.Session) error {
	c.Logs = append(c.Logs, "Run:"+session.Name())

	return nil
}

func (c *sessionLifecycleCmd) PostRun(ctx context.Context, session *conga.Session) error {
	c.Logs = append(c.Logs, "PostRun:"+session.Name())

	return nil
}

func (c *sessionLifecycleCmd) Finally(ctx context.Context, session *conga.Session, err error) error {
	c.Logs = append(c.Logs, fmt.Sprintf("Finally:%s:%v", session.Name(), err != nil))

	return nil
}
