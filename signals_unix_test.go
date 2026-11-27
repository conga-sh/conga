//go:build unix

package conga_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

const (
	envSignalHelperMode         = "CONGA_SIGNAL_HELPER_MODE"
	envSignalHelperFinallyFile  = "CONGA_SIGNAL_HELPER_FINALLY_FILE"
	envSignalHelperFinallyDelay = "CONGA_SIGNAL_HELPER_FINALLY_DELAY"
	signalTestTimeout           = 10 * time.Second
)

type signalHelperCLI struct {
	ctxErr func() error
}

func (c *signalHelperCLI) Validate(ctx context.Context, session *conga.Session) error {
	if os.Getenv(envSignalHelperMode) != "validate-sigint" {
		return nil
	}

	fmt.Fprintln(session.Stdout(), "READY")
	<-ctx.Done()

	return ctx.Err()
}

func (c *signalHelperCLI) Run(ctx context.Context, session *conga.Session) error {
	switch os.Getenv(envSignalHelperMode) {
	case "basic", "slow-finally":
		fmt.Fprintln(session.Stdout(), "READY")
		<-ctx.Done()

		return ctx.Err()
	case "ignore-sigterm-self":
		c.ctxErr = ctx.Err

		fmt.Fprintln(session.Stdout(), "READY")
		_, _ = io.ReadAll(session.Stdin())

		return nil
	case "child-forwarded-sigint":
		child := exec.CommandContext(context.WithoutCancel(ctx), "sh", "-c", "trap 'kill $! 2>/dev/null; exit 130' INT; sleep 30 & echo CHILD_READY; wait $!")

		stdout, err := child.StdoutPipe()
		if err != nil {
			return err
		}

		if err := child.Start(); err != nil {
			return err
		}

		reader := bufio.NewReader(stdout)
		if _, err := reader.ReadString('\n'); err != nil {
			return err
		}

		fmt.Fprintln(session.Stdout(), "READY")

		return child.Wait()
	case "child-sigterm-cli-only":
		child := exec.CommandContext(ctx, "sleep", "30")
		if err := child.Start(); err != nil {
			return err
		}

		fmt.Fprintln(session.Stdout(), "READY")

		return child.Wait()
	default:
		return nil
	}
}

func (c *signalHelperCLI) Finally(_ context.Context, session *conga.Session, _ error) error {
	switch os.Getenv(envSignalHelperMode) {
	case "basic", "child-forwarded-sigint", "child-sigterm-cli-only":
		c.writeMarker("FINALLY_RAN\n")
	case "slow-finally":
		fmt.Fprintln(session.Stdout(), "FINALLY_ENTERED")

		if rawDelay := os.Getenv(envSignalHelperFinallyDelay); rawDelay != "" {
			delay, _ := time.ParseDuration(rawDelay)
			time.Sleep(delay)
		}

		c.writeMarker("FINALLY_DONE\n")
	case "validate-sigint":
		c.writeMarker("FINALLY_SHOULD_NOT_RUN\n")
	}

	return nil
}

func (c *signalHelperCLI) writeMarker(content string) {
	finallyFile := os.Getenv(envSignalHelperFinallyFile)
	if finallyFile == "" {
		return
	}

	_ = os.WriteFile(finallyFile, []byte(content), 0o600)
}

//nolint:paralleltest // Subprocess entrypoint invoked by signal tests via CONGA_SIGNAL_HELPER_MODE.
func TestSignalHelperProcess(t *testing.T) {
	mode := os.Getenv(envSignalHelperMode)
	if mode == "" {
		return
	}

	if rawDelay := os.Getenv(envSignalHelperFinallyDelay); rawDelay != "" {
		_, err := time.ParseDuration(rawDelay)
		assert.NoError(t, err)
	}

	if mode == "ignore-sigterm-self" {
		// Safe in this isolated test subprocess: process-wide ignore does not affect parent tests.
		signal.Ignore(syscall.SIGTERM)
	}

	cli := new(signalHelperCLI)
	app := conga.New(cli).Name("signaltool").Args([]string{})

	err := app.Run()
	if err != nil {
		app.Exit(err)
	}

	if mode == "ignore-sigterm-self" {
		assert.IsError(t, cli.ctxErr(), context.Canceled)
		cli.writeMarker("RUN_FINISHED_CANCELED\n")
	}

	os.Exit(0)
}

type signalChildProcess struct {
	cmd          *exec.Cmd
	reader       *bufio.Reader
	stdin        io.WriteCloser
	stderrBuffer *bytes.Buffer
	stderrDone   chan struct{}
	finallyFile  string
	waitOnce     sync.Once
	waitErr      error
}

func startSignalSubprocess(t *testing.T, mode string, options ...func(*exec.Cmd)) *signalChildProcess {
	t.Helper()

	executable, err := os.Executable()
	assert.NoError(t, err)

	cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestSignalHelperProcess$")

	return launchSignalSubprocess(t, cmd, mode, options...)
}

func startTrappedSignalSubprocess(t *testing.T, trapSpec, mode string, options ...func(*exec.Cmd)) *signalChildProcess {
	t.Helper()

	executable, err := os.Executable()
	assert.NoError(t, err)

	script := fmt.Sprintf("trap '' %s; exec %q -test.run=^TestSignalHelperProcess$", trapSpec, executable)
	cmd := exec.CommandContext(t.Context(), "sh", "-c", script)

	return launchSignalSubprocess(t, cmd, mode, options...)
}

func launchSignalSubprocess(t *testing.T, cmd *exec.Cmd, mode string, options ...func(*exec.Cmd)) *signalChildProcess {
	t.Helper()

	finallyFile := filepath.Join(t.TempDir(), "finally-marker")

	cmd.Env = append(os.Environ(),
		envSignalHelperMode+"="+mode,
		envSignalHelperFinallyFile+"="+finallyFile,
	)

	for _, opt := range options {
		opt(cmd)
	}

	stdout, err := cmd.StdoutPipe()
	assert.NoError(t, err)

	stderrPipe, err := cmd.StderrPipe()
	assert.NoError(t, err)

	stdinPipe, err := cmd.StdinPipe()
	assert.NoError(t, err)

	assert.NoError(t, cmd.Start())

	stderrBuffer := &bytes.Buffer{}
	stderrDone := make(chan struct{})

	go func() {
		defer close(stderrDone)

		_, _ = io.Copy(stderrBuffer, stderrPipe)
	}()

	proc := &signalChildProcess{
		cmd:          cmd,
		reader:       bufio.NewReader(stdout),
		stdin:        stdinPipe,
		stderrBuffer: stderrBuffer,
		stderrDone:   stderrDone,
		finallyFile:  finallyFile,
	}

	t.Cleanup(func() {
		if proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}

		_ = proc.wait()
	})

	line := proc.readLine(t)
	assert.Equal(t, "READY\n", line)

	return proc
}

func (p *signalChildProcess) readLine(t *testing.T) string {
	t.Helper()

	type readResult struct {
		line string
		err  error
	}

	results := make(chan readResult, 1)

	go func() {
		line, err := p.reader.ReadString('\n')
		results <- readResult{line: line, err: err}
	}()

	select {
	case result := <-results:
		assert.NoError(t, result.err)

		return result.line
	case <-time.After(signalTestTimeout):
		t.Fatalf("timed out waiting for line from helper process")

		return ""
	}
}

func (p *signalChildProcess) wait() error {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		<-p.stderrDone
	})

	return p.waitErr
}

func (p *signalChildProcess) waitExit(t *testing.T) (int, syscall.WaitStatus, string) {
	t.Helper()

	done := make(chan error, 1)

	go func() {
		done <- p.wait()
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(signalTestTimeout):
		t.Fatalf("timed out waiting for helper process to exit")
	}

	if err == nil {
		return 0, 0, p.stderrBuffer.String()
	}

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	assert.True(t, ok, "expected *exec.ExitError, got %T: %v", err, err)

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	assert.True(t, ok)

	return exitErr.ExitCode(), status, p.stderrBuffer.String()
}

func (p *signalChildProcess) markerContent(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(p.finallyFile)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}

	assert.NoError(t, err)

	return string(data)
}

func TestSignals_FirstSignalRunsFinallyAndSetsExitCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		sig          syscall.Signal
		expectedCode int
	}{
		{name: "SIGINT exits 130", sig: syscall.SIGINT, expectedCode: 130},
		{name: "SIGTERM exits 143", sig: syscall.SIGTERM, expectedCode: 143},
		{name: "SIGHUP exits 129", sig: syscall.SIGHUP, expectedCode: 129},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			proc := startSignalSubprocess(t, "basic")

			assert.NoError(t, proc.cmd.Process.Signal(testCase.sig))

			code, _, stderr := proc.waitExit(t)

			assert.Equal(t, testCase.expectedCode, code)
			assert.Equal(t, "", stderr)
			assert.Equal(t, "FINALLY_RAN\n", proc.markerContent(t))
		})
	}
}

func TestSignals_SecondSignalBehavior(t *testing.T) {
	t.Parallel()

	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run("second "+sig.String()+" ends slow Finally promptly", func(t *testing.T) {
			t.Parallel()

			proc := startSignalSubprocess(t, "slow-finally", func(cmd *exec.Cmd) {
				cmd.Env = append(cmd.Env, envSignalHelperFinallyDelay+"=10s")
			})

			assert.NoError(t, proc.cmd.Process.Signal(sig))
			assert.Equal(t, "FINALLY_ENTERED\n", proc.readLine(t))

			start := time.Now()

			assert.NoError(t, proc.cmd.Process.Signal(sig))

			_, status, _ := proc.waitExit(t)

			assert.True(t, time.Since(start) < 3*time.Second, "second signal took too long to terminate")
			assert.True(t, status.Signaled())
			assert.Equal(t, sig, status.Signal())
			assert.Equal(t, "", proc.markerContent(t))
		})
	}

	t.Run("second SIGHUP is ignored and cleanup finishes with exit 129", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "slow-finally", func(cmd *exec.Cmd) {
			cmd.Env = append(cmd.Env, envSignalHelperFinallyDelay+"=200ms")
		})

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))
		assert.Equal(t, "FINALLY_ENTERED\n", proc.readLine(t))

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))

		code, status, stderr := proc.waitExit(t)

		assert.False(t, status.Signaled())
		assert.Equal(t, 129, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_DONE\n", proc.markerContent(t))
	})

	t.Run("SIGHUP followed by SIGTERM ends process promptly", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "slow-finally", func(cmd *exec.Cmd) {
			cmd.Env = append(cmd.Env, envSignalHelperFinallyDelay+"=10s")
		})

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))
		assert.Equal(t, "FINALLY_ENTERED\n", proc.readLine(t))

		start := time.Now()

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTERM))

		_, status, _ := proc.waitExit(t)

		assert.True(t, time.Since(start) < 3*time.Second)
		assert.True(t, status.Signaled())
		assert.Equal(t, syscall.SIGTERM, status.Signal())
	})

	t.Run("SIGINT followed by SIGHUP ignores hangup and finishes cleanup with exit 130", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "slow-finally", func(cmd *exec.Cmd) {
			cmd.Env = append(cmd.Env, envSignalHelperFinallyDelay+"=200ms")
		})

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))
		assert.Equal(t, "FINALLY_ENTERED\n", proc.readLine(t))

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))

		code, status, stderr := proc.waitExit(t)

		assert.False(t, status.Signaled())
		assert.Equal(t, 130, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_DONE\n", proc.markerContent(t))
	})
}

func TestSignals_IgnoredAtStartup(t *testing.T) {
	t.Parallel()

	t.Run("started with SIGHUP ignored ignores SIGHUP, stops on SIGTSTP, and ends on second SIGINT", func(t *testing.T) {
		t.Parallel()

		proc := startTrappedSignalSubprocess(t, "HUP", "slow-finally", func(cmd *exec.Cmd) {
			cmd.Env = append(cmd.Env, envSignalHelperFinallyDelay+"=10s")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		})

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, "", proc.markerContent(t))

		// Verify SIGTSTP still stops the process (not swallowed by signal.Notify).
		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTSTP))

		var waitStatus syscall.WaitStatus

		deadline := time.Now().Add(signalTestTimeout)

		for {
			pid, err := syscall.Wait4(proc.cmd.Process.Pid, &waitStatus, syscall.WUNTRACED|syscall.WNOHANG, nil)
			if errors.Is(err, syscall.EINTR) {
				continue
			}

			assert.NoError(t, err)

			if pid == proc.cmd.Process.Pid {
				break
			}

			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for SIGTSTP to stop helper process")
			}

			time.Sleep(10 * time.Millisecond)
		}

		assert.True(t, waitStatus.Stopped())
		assert.Equal(t, syscall.SIGTSTP, waitStatus.StopSignal())

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGCONT))
		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))
		assert.Equal(t, "FINALLY_ENTERED\n", proc.readLine(t))

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))

		_, status, _ := proc.waitExit(t)

		assert.True(t, status.Signaled())
		assert.Equal(t, syscall.SIGINT, status.Signal())
	})

	t.Run("started with SIGINT ignored ignores SIGINT and catches SIGTERM", func(t *testing.T) {
		t.Parallel()

		proc := startTrappedSignalSubprocess(t, "INT", "basic")

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, "", proc.markerContent(t))

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTERM))

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 143, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_RAN\n", proc.markerContent(t))
	})

	t.Run("started with HUP INT TERM all ignored still catches SIGTERM", func(t *testing.T) {
		t.Parallel()

		proc := startTrappedSignalSubprocess(t, "HUP INT TERM", "basic")

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))
		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))
		time.Sleep(50 * time.Millisecond)
		assert.Equal(t, "", proc.markerContent(t))

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTERM))

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 143, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_RAN\n", proc.markerContent(t))
	})

	t.Run("program calling signal.Ignore(SIGTERM) with HUP and INT ignored exits 0", func(t *testing.T) {
		t.Parallel()

		proc := startTrappedSignalSubprocess(t, "HUP INT", "ignore-sigterm-self")

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTERM))
		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGHUP))
		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))
		time.Sleep(50 * time.Millisecond)

		assert.NoError(t, proc.stdin.Close())

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 0, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "RUN_FINISHED_CANCELED\n", proc.markerContent(t))
	})
}

func TestSignals_ChildProcessesAndValidate(t *testing.T) {
	t.Parallel()

	t.Run("forwarded SIGINT to child exits 130 silently", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "child-forwarded-sigint", func(cmd *exec.Cmd) {
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		})

		assert.NoError(t, syscall.Kill(-proc.cmd.Process.Pid, syscall.SIGINT))

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 130, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_RAN\n", proc.markerContent(t))
	})

	t.Run("SIGTERM to CLI only killing CommandContext child with SIGKILL exits 143 silently", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "child-sigterm-cli-only")

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGTERM))

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 143, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "FINALLY_RAN\n", proc.markerContent(t))
	})

	t.Run("SIGINT during Validate exits 130 silently with no usage block", func(t *testing.T) {
		t.Parallel()

		proc := startSignalSubprocess(t, "validate-sigint")

		assert.NoError(t, proc.cmd.Process.Signal(syscall.SIGINT))

		code, _, stderr := proc.waitExit(t)

		assert.Equal(t, 130, code)
		assert.Equal(t, "", stderr)
		assert.Equal(t, "", proc.markerContent(t))
	})
}
