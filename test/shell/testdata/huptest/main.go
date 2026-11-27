// Command huptest is a PTY hangup fixture that runs a foreground CONGA command
// inside an interactive shell and records Finally execution and exit status
// when the terminal session closes.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/conga-sh/conga"
)

type CLI struct {
	RunCmd  conga.Command[RunCmd]  `name:"run"  summary:"Wait for hangup and run Finally"`
	WrapCmd conga.Command[WrapCmd] `name:"wrap" summary:"Run child and record its exit status"`
}

type RunCmd struct {
	ReadyFile   conga.RequiredArg[string] `help:"Ready marker path"`
	FinallyFile conga.RequiredArg[string] `help:"Finally marker path"`
}

func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

func (c *RunCmd) Run(ctx context.Context, _ *conga.Session) error {
	if err := writeAtomic(c.ReadyFile.Get(), "ready\n"); err != nil {
		return err
	}

	<-ctx.Done()

	return ctx.Err()
}

func (c *RunCmd) Finally(_ context.Context, _ *conga.Session, _ error) error {
	time.Sleep(150 * time.Millisecond)

	return writeAtomic(c.FinallyFile.Get(), "done\n")
}

type WrapCmd struct {
	ExitCodeFile conga.RequiredArg[string] `help:"Exit code marker path"`
	ChildArgs    conga.DashArgs            `help:"Child command and arguments"`
}

func (c *WrapCmd) Run(_ context.Context, _ *conga.Session) error {
	childArgs := c.ChildArgs.Get()
	if len(childArgs) == 0 {
		return errors.New("child command required after --")
	}

	cmd := exec.Command(childArgs[0], childArgs[1:]...) //nolint:gosec,noctx // test fixture runs controlled binary without context cancel
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	code := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		} else {
			return err
		}
	}

	if err := writeAtomic(c.ExitCodeFile.Get(), strconv.Itoa(code)); err != nil {
		return fmt.Errorf("write exit code: %w", err)
	}

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("huptest").
		// Required by workspace.StartPtySession -> writePtyCompletionScript during PTY shell setup.
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
