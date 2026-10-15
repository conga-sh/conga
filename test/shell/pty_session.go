//go:build shelltest

// Package shell PTY driver: launches interactive shells inside tmux-managed
// pseudo-terminals so completion can be exercised with real keystrokes and
// asserted against what the user actually sees on screen.
package shell

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	ptyPrompt          = "CONGA> " // Sentinel prompt injected via PS1/PROMPT overrides.
	ptyPromptMarker    = "CONGA>"  // Trimmed sentinel as it appears in captured lines.
	ptyPaneWidth       = 100       // Pane columns; wide enough that test buffers never wrap.
	ptyPaneHeight      = 60        // Pane rows; tall enough that small candidate lists never page.
	ptyStableInterval  = 120 * time.Millisecond
	ptyStableRounds    = 3
	ptyCaptureDeadline = 3 * time.Second
	// Deadlines only bound broken sessions: healthy sessions proceed as soon
	// as rendering settles or the prompt appears, so they must tolerate slow
	// cold starts (pwsh/nu) running concurrently with the full shell suite.
	ptyStartupDeadline = 45 * time.Second
	ptyPromptDeadline  = 45 * time.Second
)

//nolint:gochecknoglobals // Process-wide unique tmux server labels for concurrent PTY sessions.
var ptySessionCounter atomic.Int64

// PtyResult captures the rendered terminal state after real keystrokes.
type PtyResult struct {
	Screen        string
	ScreenEscaped string
	Lines         []string
	CursorX       int
	CursorY       int
}

// PromptLine returns the index of the last line carrying the sentinel prompt, or -1.
func (r *PtyResult) PromptLine() int {
	for i := range slices.Backward(r.Lines) {
		if strings.HasPrefix(r.Lines[i], ptyPromptMarker) {
			return i
		}
	}

	return -1
}

// BufferAfterPrompt returns the command-line text rendered after the sentinel
// prompt. Capture processing trims trailing whitespace, so a buffer that ends
// in a space (e.g. after a unique completion) is not observable here; assert
// CursorX instead.
func (r *PtyResult) BufferAfterPrompt() string {
	line := r.PromptLine()
	if line == -1 {
		return ""
	}

	return strings.TrimPrefix(strings.TrimPrefix(r.Lines[line], ptyPromptMarker), " ")
}

// PtySession drives one interactive shell running inside a tmux-managed PTY.
type PtySession struct {
	t           *testing.T
	tmuxLabel   string
	sessionName string
	env         []string
}

// Type sends literal text to the shell, as if typed by a user.
func (s *PtySession) Type(text string) {
	s.tmux("send-keys", "-t", s.sessionName, "-l", text)
}

// Tab sends a Tab keystroke to the shell.
func (s *PtySession) Tab() {
	s.tmux("send-keys", "-t", s.sessionName, "Tab")
}

// PressEnter sends an Enter keystroke to the shell.
func (s *PtySession) PressEnter() {
	s.tmux("send-keys", "-t", s.sessionName, "Enter")
}

// Capture waits for the terminal to settle, then returns the visible screen,
// its escape-sequence form, and the cursor position.
func (s *PtySession) Capture() *PtyResult {
	s.awaitStable(ptyCaptureDeadline)

	screen := s.tmux("capture-pane", "-p", "-t", s.sessionName)
	escaped := s.tmux("capture-pane", "-p", "-e", "-t", s.sessionName)
	cursor := s.tmux("display-message", "-p", "-t", s.sessionName, "#{cursor_x} #{cursor_y}")

	return s.newPtyResult(screen, escaped, cursor)
}

func (s *PtySession) tmux(arguments ...string) string {
	// Every client call must address this session's server via -L; without
	// it tmux silently falls back to the default socket of the shared
	// TMUX_TMPDIR and teardown would target a server that never existed.
	fullArguments := append([]string{"-L", s.tmuxLabel}, arguments...)
	command := exec.CommandContext(s.t.Context(), "tmux", fullArguments...) //nolint:gosec // Test harness launches tmux with controlled arguments.
	command.Env = s.env

	output, err := command.Output()
	if err != nil {
		s.t.Fatalf("tmux %s failed: %v\nOutput: %s", strings.Join(fullArguments, " "), err, string(output))
	}

	return string(output)
}

func (s *PtySession) sendBootstrap(bootstrap string) {
	s.Type(bootstrap)
	s.PressEnter()
	s.waitForPrompt(ptyPromptDeadline)
}

func (s *PtySession) capturePlain() string {
	return s.tmux("capture-pane", "-p", "-t", s.sessionName)
}

// awaitStable polls the pane until its visible content stays identical for
// ptyStableRounds consecutive observations, proving asynchronous rendering
// (redraws, candidate lists, menus) has finished.
func (s *PtySession) awaitStable(deadline time.Duration) {
	var previous string

	consecutive := 0
	deadlineTime := time.Now().Add(deadline)

	for {
		current := s.capturePlain()
		if current == previous {
			consecutive++
			if consecutive >= ptyStableRounds {
				return
			}
		} else {
			consecutive = 0
		}

		previous = current

		if time.Now().After(deadlineTime) {
			s.t.Fatalf("terminal output did not stabilize within %s; last screen:\n%s", deadline, current)
		}

		time.Sleep(ptyStableInterval)
	}
}

func (s *PtySession) waitForPrompt(deadline time.Duration) {
	deadlineTime := time.Now().Add(deadline)

	for {
		screen := s.capturePlain()
		if s.screenAtPrompt(screen) {
			return
		}

		if time.Now().After(deadlineTime) {
			s.t.Fatalf("prompt %q did not appear within %s; last screen:\n%s", ptyPrompt, deadline, screen)
		}

		time.Sleep(ptyStableInterval)
	}
}

func (s *PtySession) screenAtPrompt(screen string) bool {
	lines := ptySignificantLines(screen)

	return len(lines) > 0 && lines[len(lines)-1] == ptyPromptMarker
}

func (s *PtySession) newPtyResult(screen, escaped, cursor string) *PtyResult {
	result := &PtyResult{
		Screen:        screen,
		ScreenEscaped: escaped,
		Lines:         ptySignificantLines(screen),
	}

	parts := strings.Fields(cursor)
	if len(parts) != 2 {
		s.t.Fatalf("malformed cursor position %q from tmux display-message", cursor)
	}

	cursorX, errX := strconv.Atoi(parts[0])
	cursorY, errY := strconv.Atoi(parts[1])

	if errX != nil || errY != nil {
		s.t.Fatalf("malformed cursor position %q from tmux display-message", cursor)
	}

	result.CursorX = cursorX
	result.CursorY = cursorY

	return result
}

func (s *PtySession) killServer() {
	// The cleanup context is already cancelled by the time this runs, so the
	// teardown must use a background context to reach the tmux server.
	command := exec.CommandContext(context.Background(), "tmux", "-L", s.tmuxLabel, "kill-server") //nolint:gosec // Test harness tears down its own tmux server.
	command.Env = s.env

	_ = command.Run() //nolint:errcheck // Best-effort teardown; the socket dies with the workspace.
}

func (ws *Workspace) StartPtySession(t *testing.T, binaryName, shellName string) *PtySession {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux binary not found on PATH; skipping PTY completion test")
	}

	shellSpec, ok := ptyShellSpecs[shellName]
	if !ok {
		t.Fatalf("unknown PTY shell %q; supported: %v", shellName, ptySupportedShells())
	}

	shellPath := ws.lookupPtyShell(t, shellSpec)
	scriptPath := ws.writePtyCompletionScript(t, binaryName, shellName)

	if shellSpec.writeRC != nil {
		shellSpec.writeRC(t, ws, scriptPath)
	}

	session := &PtySession{
		t:           t,
		tmuxLabel:   fmt.Sprintf("conga_pty_%d", ptySessionCounter.Add(1)),
		sessionName: "conga",
		env:         append(ws.ptyEnvironment(), shellSpec.extraEnv...),
	}

	t.Cleanup(session.killServer)

	if err := os.MkdirAll(ws.tmuxDir(), 0o700); err != nil {
		t.Fatalf("failed to create tmux socket directory: %v", err)
	}

	commandLine := strings.Join(append([]string{shellPath}, shellSpec.launch...), " ")

	session.tmux("new-session", "-d", "-x", strconv.Itoa(ptyPaneWidth), "-y", strconv.Itoa(ptyPaneHeight), "-s", session.sessionName, commandLine)
	session.awaitStable(ptyStartupDeadline)

	if shellSpec.bootstrap != nil {
		session.sendBootstrap(shellSpec.bootstrap(t, shellPath, scriptPath))
	} else {
		session.waitForPrompt(ptyPromptDeadline)
	}

	return session
}

func (ws *Workspace) lookupPtyShell(t *testing.T, shellSpec ptyShellSpec) string {
	t.Helper()

	if override := os.Getenv(shellSpec.envOverride); override != "" {
		return override
	}

	for _, candidate := range shellSpec.lookups {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	t.Skipf("%s binary not found on PATH; skipping PTY completion test", shellSpec.lookups[0])

	return ""
}

// ptyEnvironment builds the tmux client/server environment: the tmux server
// inherits it and passes it to the spawned interactive shell, so HOME, PATH,
// and locale isolation mirror the non-PTY harness.
func (ws *Workspace) ptyEnvironment() []string {
	return []string{
		"HOME=" + ws.HomeDir,
		"PATH=" + ws.BinDir + ":" + os.Getenv("PATH"),
		"LC_ALL=C",
		"TERM=xterm-256color",
		"TMUX_TMPDIR=" + ws.tmuxDir(),
	}
}

func (ws *Workspace) tmuxDir() string {
	return filepath.Join(ws.Root, "tmux")
}

func (ws *Workspace) writePtyCompletionScript(t *testing.T, binaryName, shellName string) string {
	t.Helper()

	content := ws.generateCompletionScript(t, binaryName, shellName)
	extension, ok := ptyScriptExtensions[shellName]

	if !ok {
		t.Fatalf("no completion script extension for PTY shell %q", shellName)
	}

	scriptFile, err := os.CreateTemp(ws.HomeDir, binaryName+"_pty_completion_*"+extension)
	if err != nil {
		t.Fatalf("failed to create PTY completion script temp file: %v", err)
	}

	scriptPath := scriptFile.Name()

	if _, err := scriptFile.WriteString(content); err != nil {
		_ = scriptFile.Close() //nolint:errcheck // Error already reported.

		t.Fatalf("failed to write PTY completion script: %v", err)
	}

	if err := scriptFile.Close(); err != nil {
		t.Fatalf("failed to close PTY completion script file: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Remove(scriptPath) //nolint:errcheck // Best-effort cleanup.
	})

	return scriptPath
}

// ptySignificantLines splits a captured pane into trimmed lines with trailing
// blank rows removed, keeping interior spacing (list columns) intact.
func ptySignificantLines(screen string) []string {
	trimmed := strings.TrimRight(screen, "\n")
	lines := strings.Split(trimmed, "\n")

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	return lines
}

func ptySupportedShells() []string {
	return slices.Collect(maps.Keys(ptyShellSpecs))
}
