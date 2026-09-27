//go:build shelltest

package shell

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	_ "embed"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/tokenizer"
)

var (
	//go:embed testdata/bash_harness.bash
	bashHarnessScript []byte

	//go:embed testdata/zsh_harness.zsh
	zshHarnessScript []byte

	//go:embed testdata/elvish_harness.elv
	elvishHarnessTemplate string

	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

	zshDumpOnce sync.Once //nolint:gochecknoglobals // One-time zsh compinit dump shared across tests.
	zshDumpPath string    //nolint:gochecknoglobals // One-time zsh compinit dump shared across tests.
	errZshDump  error     //nolint:gochecknoglobals // One-time zsh compinit dump error if initialization failed.
)

// Candidate represents a single completion candidate match.
type Candidate struct {
	Value       string
	Description string
	CodeSuffix  string
}

// BashResult contains completion candidates captured from a real Bash session.
type BashResult struct {
	RawOutput  string
	BufferTab1 string
	CursorTab1 int
	BufferTab2 string
	CursorTab2 int
	Items      []string
	Messages   []string
	StdoutLeak string
}

// HasItem checks whether the candidate value is present.
func (r *BashResult) HasItem(value string) bool {
	return slices.Contains(r.Items, value)
}

// ElvishResult contains completion candidates captured from an Elvish session.
type ElvishResult struct {
	RawOutput  string
	BufferTab1 string
	BufferTab2 string
	Candidates []Candidate
	StdoutLeak string
}

// HasItem checks whether the candidate value is present.
func (r *ElvishResult) HasItem(value string) bool {
	for _, item := range r.Candidates {
		if item.Value == value {
			return true
		}
	}

	return false
}

// ItemDescription returns the description of a candidate value, if present.
func (r *ElvishResult) ItemDescription(value string) (string, bool) {
	for _, item := range r.Candidates {
		if item.Value == value {
			return item.Description, true
		}
	}

	return "", false
}

// ItemCodeSuffix returns the codeSuffix for a candidate value, if present.
func (r *ElvishResult) ItemCodeSuffix(value string) (string, bool) {
	for _, item := range r.Candidates {
		if item.Value == value {
			return item.CodeSuffix, true
		}
	}

	return "", false
}

// FishResult contains completion candidates captured from a real Fish session.
type FishResult struct {
	RawOutput  string
	BufferTab1 string
	CursorTab1 int
	BufferTab2 string
	CursorTab2 int
	Items      []Candidate
	Stderr     string
	StdoutLeak string
}

// HasItem checks whether the specified candidate value is present.
func (r *FishResult) HasItem(value string) bool {
	for _, item := range r.Items {
		if item.Value == value {
			return true
		}
	}

	return false
}

// ItemDescription returns the description of a candidate value, if present.
func (r *FishResult) ItemDescription(value string) (string, bool) {
	for _, item := range r.Items {
		if item.Value == value {
			return item.Description, true
		}
	}

	return "", false
}

// PowerShellItem represents a single PowerShell CompletionResult object.
type PowerShellItem struct {
	CompletionText string `json:"completionText"`
	ListItemText   string `json:"listItemText"`
	ToolTip        string `json:"toolTip"`
}

// PowerShellResult contains completion items captured from a real PowerShell session.
type PowerShellResult struct {
	RawOutput string
	// StdoutLeak holds any text PowerShell printed before the harness JSON
	// payload; the completion function must emit nothing there.
	StdoutLeak string
	// Stderr holds everything the PowerShell session wrote to stderr; the
	// completion function must keep it empty (terminal display purity).
	Stderr string
	// CompletionPrefix is the prefix CompleteInput used for filtering (the
	// word being completed).
	CompletionPrefix string
	// ReplacementIndex is the buffer offset where PowerShell would insert the
	// selected candidate; it must always point at the start of the word being
	// completed so no earlier command-line content is replaced.
	ReplacementIndex int
	BufferTab1       string
	CursorTab1       int
	BufferTab2       string
	CursorTab2       int
	Items            []PowerShellItem
}

// HasItem checks whether the candidate value is present in CompletionText or ListItemText.
func (r *PowerShellResult) HasItem(value string) bool {
	for _, item := range r.Items {
		if strings.TrimSpace(item.CompletionText) == value || item.ListItemText == value {
			return true
		}
	}

	return false
}

// ItemToolTip returns the tooltip or description for a candidate value, if present.
func (r *PowerShellResult) ItemToolTip(value string) (string, bool) {
	for _, item := range r.Items {
		if strings.TrimSpace(item.CompletionText) == value || strings.Contains(item.ListItemText, value) {
			if tip := strings.TrimSpace(item.ToolTip); tip != "" {
				return stripANSI(tip), true
			}

			clean := strings.TrimSpace(stripANSI(item.ListItemText))
			if index := strings.Index(clean, "("); index != -1 && strings.HasSuffix(clean, ")") {
				desc := strings.TrimSuffix(clean[index+1:], ")")

				return desc, true
			}
		}
	}

	return "", false
}

// ZshResult contains the structured completion metadata captured from a real Zsh session.
type ZshResult struct {
	RawOutput       string
	ListColors      string
	CompstateInsert string
	BufferTab1      string
	CursorTab1      int
	BufferTab2      string
	CursorTab2      int
	Groups          map[string][]Candidate
	RawItems        []string
	Messages        []string
	FilesCalled     []string
	DirsCalled      []string
}

// GroupItems returns the list of candidates associated with a specific tag (e.g. "flags", "commands").
func (r *ZshResult) GroupItems(tag string) []Candidate {
	return r.Groups[tag]
}

// HasItem checks whether the specified candidate value is present in any group or raw list.
func (r *ZshResult) HasItem(value string) bool {
	for _, items := range r.Groups {
		for _, item := range items {
			if item.Value == value {
				return true
			}
		}
	}

	return slices.Contains(r.RawItems, value)
}

// ItemDescription returns the description of a candidate value, if present.
func (r *ZshResult) ItemDescription(value string) (string, bool) {
	for _, items := range r.Groups {
		for _, item := range items {
			if item.Value == value {
				return item.Description, true
			}
		}
	}

	return "", false
}

// NushellStyle represents the ANSI style record in Nushell's completion payload.
type NushellStyle struct {
	Fg   string `json:"fg"`
	Bg   string `json:"bg"`
	Attr string `json:"attr"`
}

// NushellResult contains completion candidates captured from a Nushell session.
type NushellResult struct {
	RawOutput  string
	BufferTab1 string
	CursorTab1 int
	BufferTab2 string
	CursorTab2 int
	Candidates []Candidate
	Styles     map[string]NushellStyle
	Stderr     string
	StdoutLeak string
}

// HasItem checks whether the candidate value is present. Values carry the
// driver-baked trailing space, so the comparison trims.
func (r *NushellResult) HasItem(value string) bool {
	for _, item := range r.Candidates {
		if strings.TrimSpace(item.Value) == value {
			return true
		}
	}

	return false
}

// ItemDescription returns the description of a candidate value, if present.
// Values carry the driver-baked trailing space, so the comparison trims.
func (r *NushellResult) ItemDescription(value string) (string, bool) {
	for _, item := range r.Candidates {
		if strings.TrimSpace(item.Value) == value {
			return item.Description, true
		}
	}

	return "", false
}

// ItemStyle returns the style record of a candidate value, if present.
func (r *NushellResult) ItemStyle(value string) (NushellStyle, bool) {
	style, ok := r.Styles[value]

	return style, ok
}

func (r *BashResult) parseLine(line, trimmed string) {
	if value, cut := strings.CutPrefix(line, "BUFFER_TAB1:"); cut {
		r.BufferTab1 = value

		return
	}

	if value, cut := strings.CutPrefix(line, "CURSOR_TAB1:"); cut {
		if c, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			r.CursorTab1 = c
		}

		return
	}

	if value, cut := strings.CutPrefix(line, "BUFFER_TAB2:"); cut {
		r.BufferTab2 = value

		return
	}

	if value, cut := strings.CutPrefix(line, "CURSOR_TAB2:"); cut {
		if c, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			r.CursorTab2 = c
		}

		return
	}

	if item, cut := strings.CutPrefix(line, "ITEM:"); cut {
		r.Items = append(r.Items, item)

		return
	}

	if msg, cut := strings.CutPrefix(line, "MESSAGE:"); cut {
		r.Messages = append(r.Messages, stripANSI(msg))

		return
	}

	if leak, cut := strings.CutPrefix(line, "STDOUT_LEAK:"); cut {
		r.StdoutLeak = leak

		return
	}

	clean := stripANSI(trimmed)
	if clean != "" && !strings.HasPrefix(clean, "BUFFER_") && !strings.HasPrefix(clean, "CURSOR_") {
		r.Messages = append(r.Messages, clean)
	}
}

// RunBashCompletion executes real Bash completion for a given binary and command line string.
func (ws *Workspace) RunBashCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *BashResult {
	t.Helper()

	return ws.RunBashCompletionType(t, binaryName, cmdLine, "9", extraEnv...)
}

// RunBashCompletionWithSetup executes real Bash completion with a specific setup script.
func (ws *Workspace) RunBashCompletionWithSetup(t *testing.T, binaryName, cmdLine, setupScript string, extraEnv ...string) *BashResult {
	t.Helper()

	return ws.RunBashCompletionWithTypeAndSetup(t, binaryName, cmdLine, "9", setupScript, extraEnv...)
}

// RunBashCompletionType executes real Bash completion with a specific COMP_TYPE (e.g. "63" for successive tabs).
func (ws *Workspace) RunBashCompletionType(t *testing.T, binaryName, cmdLine, compType string, extraEnv ...string) *BashResult {
	t.Helper()

	return ws.RunBashCompletionWithTypeAndSetup(t, binaryName, cmdLine, compType, "", extraEnv...)
}

// RunBashCompletionWithTypeAndSetup executes real Bash completion with a specific COMP_TYPE and setup script.
func (ws *Workspace) RunBashCompletionWithTypeAndSetup(t *testing.T, binaryName, cmdLine, compType, setupScript string, extraEnv ...string) *BashResult {
	t.Helper()

	bashPath, err := exec.LookPath("bash")
	if envBash := os.Getenv("CONGA_BASH_BIN"); envBash != "" {
		bashPath = envBash
		err = nil
	}

	if err != nil {
		t.Skip("bash binary not found on PATH; skipping real Bash completion test")
	}

	harnessPath := ws.createHarnessFile(t, "bash_harness_*.bash", bashHarnessScript, 0o700)
	defer func() {
		_ = os.Remove(harnessPath) //nolint:errcheck // Clean up temp harness file.
	}()

	args := []string{"--noprofile", "--norc", harnessPath, binaryName, cmdLine, compType}
	if setupScript != "" {
		args = append(args, setupScript)
	}

	env := append([]string{"TERM=dumb"}, extraEnv...)
	out := ws.executeShellCommand(t, bashPath, args, env...)

	return parseBashResult(out)
}

// RunElvishCompletion executes real Elvish completion for a given binary and command line string.
func (ws *Workspace) RunElvishCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *ElvishResult {
	t.Helper()

	elvishPath, err := exec.LookPath("elvish")
	if envElvish := os.Getenv("CONGA_ELVISH_BIN"); envElvish != "" {
		elvishPath = envElvish
		err = nil
	}

	if err != nil {
		t.Skip("elvish binary not found on PATH; skipping real Elvish completion test")
	}

	// Elvish invokes the arg-completer with the command name in front and the
	// word being completed (possibly an empty string) last; the quote-aware
	// COMP_LINE parser reproduces that word list (docs/completion-wire-protocol.md §3).
	words := tokenizer.ParseCompLine(cmdLine, len(cmdLine))

	quotedWords := make([]string, 0, len(words))
	for _, word := range words {
		quotedWords = append(quotedWords, fmt.Sprintf("%q", word))
	}

	compScript := ws.generateCompletionScript(t, binaryName, "elvish")
	elvishScript := fmt.Sprintf(elvishHarnessTemplate, compScript, binaryName, binaryName, strings.Join(quotedWords, " "))

	out := ws.executeShellCommand(t, elvishPath, []string{"-c", elvishScript}, extraEnv...)

	return parseElvishResult(t, out, cmdLine, words)
}

// RunNushellCompletion executes real Nushell completion for a given binary and command line string.
func (ws *Workspace) RunNushellCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *NushellResult {
	t.Helper()

	nuPath, err := exec.LookPath("nu")
	if err != nil {
		nuPath, err = exec.LookPath("nushell")
	}

	if envNu := os.Getenv("CONGA_NU_BIN"); envNu != "" {
		nuPath = envNu
		err = nil
	}

	if err != nil {
		t.Skip("nu/nushell binary not found on PATH; skipping real Nushell completion test")
	}

	// Nushell passes the spans raw, command name first, word being completed
	// last (docs/completion-wire-protocol.md §3).
	words := tokenizer.ParseCompLine(cmdLine, len(cmdLine))

	quotedSpans := make([]string, 0, len(words))
	for _, word := range words {
		quotedSpans = append(quotedSpans, fmt.Sprintf("%q", word))
	}

	spansArray := fmt.Sprintf("[%s]", strings.Join(quotedSpans, ", "))

	nuFile, err := os.CreateTemp(ws.HomeDir, binaryName+"_completion_*.nu")
	if err != nil {
		t.Fatalf("failed to create nushell script temp file: %v", err)
	}

	nuScriptPath := nuFile.Name()

	defer func() {
		_ = os.Remove(nuScriptPath) //nolint:errcheck // Clean up temp script file.
	}()

	compOut := ws.generateCompletionScript(t, binaryName, "nushell")

	if _, err := nuFile.WriteString(compOut); err != nil {
		t.Fatalf("failed to write nushell script: %v", err)
	}

	if err := nuFile.Close(); err != nil {
		t.Fatalf("failed to close temp nushell file: %v", err)
	}

	nuScript := fmt.Sprintf(`
source %s
do $%s_completer %s | to json
`, nuScriptPath, wire.SanitizeIdentifier(binaryName), spansArray)

	stdout, stderr := ws.executeShellCommandSplit(t, nuPath, []string{"-c", nuScript}, extraEnv...)

	result := parseNushellResult(t, stdout, cmdLine, words)
	result.Stderr = stderr

	return result
}

// RunFishCompletion executes real Fish completion for a given binary and command line string.
func (ws *Workspace) RunFishCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *FishResult {
	t.Helper()

	fishPath, err := exec.LookPath("fish")
	if envFish := os.Getenv("CONGA_FISH_BIN"); envFish != "" {
		fishPath = envFish
		err = nil
	}

	if err != nil {
		t.Skip("fish binary not found on PATH; skipping real Fish completion test")
	}

	fishScript := fmt.Sprintf(`source (begin; %s completion fish 2>/dev/null; or %s --completion fish; end | psub); complete -C %q`, binaryName, binaryName, cmdLine)
	stdout, stderr := ws.executeShellCommandSplit(t, fishPath, []string{"-c", fishScript}, extraEnv...)

	result := parseFishResult(stdout, cmdLine)
	result.Stderr = stderr

	return result
}

// RunPowerShellCompletion executes real PowerShell completion for a given binary and command line string.
// Extra environment variables (NAME=value) are forwarded to the PowerShell session.
func (ws *Workspace) RunPowerShellCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *PowerShellResult {
	t.Helper()

	pwshPath, err := exec.LookPath("pwsh")
	if err != nil {
		pwshPath, err = exec.LookPath("powershell")
	}

	if envPwsh := os.Getenv("CONGA_PWSH_BIN"); envPwsh != "" {
		pwshPath = envPwsh
		err = nil
	}

	if err != nil {
		t.Skip("pwsh/powershell binary not found on PATH; skipping real PowerShell completion test")
	}

	psEscapedCmdLine := strings.ReplaceAll(cmdLine, "'", "''")
	// CompleteInput throws when a native completer returns the documented
	// empty-result "" sentinel (the contract that suppresses PowerShell's
	// default file completion); interactive sessions swallow that exception
	// inside PSReadLine, so the harness mirrors PSReadLine and catches it.
	psScript := fmt.Sprintf(`& { $comp = & '%s' completion powershell 2>$null; if (-not $comp) { $comp = & '%s' --completion powershell 2>$null }; $comp | Out-String | Invoke-Expression; $cc = $null; try { $cc = [System.Management.Automation.CommandCompletion]::CompleteInput('%s', %d, $null) } catch { }; $matches = @(); if ($cc) { $matches = @($cc.CompletionMatches) }; [PSCustomObject]@{ completionPrefix = [string]$cc.CompletionPrefix; replacementIndex = $cc.ReplacementIndex; items = @($matches | ForEach-Object { [PSCustomObject]@{ completionText = $_.CompletionText; listItemText = $_.ListItemText; toolTip = $_.ToolTip } }) } | ConvertTo-Json -Depth 6 }`, binaryName, binaryName, psEscapedCmdLine, len(cmdLine))
	stdout, stderr := ws.executeShellCommandSplit(t, pwshPath, []string{"-NoProfile", "-NonInteractive", "-Command", psScript}, extraEnv...)

	payload, leak := splitPowerShellPayload(stdout)

	var parsed struct {
		CompletionPrefix string           `json:"completionPrefix"`
		ReplacementIndex int              `json:"replacementIndex"`
		Items            []PowerShellItem `json:"items"`
	}

	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			t.Fatalf("failed to unmarshal powershell json: %v\nPayload:\n%s", err, payload)
		}
	}

	bufferTab1 := cmdLine

	bufferTab2 := cmdLine
	if len(parsed.Items) == 1 {
		bufferTab1 = cmdLine[:parsed.ReplacementIndex] + parsed.Items[0].CompletionText
		bufferTab2 = bufferTab1
	} else if len(parsed.Items) > 1 {
		psCandidates := make([]Candidate, len(parsed.Items))
		for i, it := range parsed.Items {
			psCandidates[i] = Candidate{Value: it.CompletionText}
		}

		lcp := commonCandidatePrefix(psCandidates)
		if len(lcp) > len(parsed.CompletionPrefix) {
			bufferTab1 = cmdLine[:parsed.ReplacementIndex] + lcp
		}

		bufferTab2 = cmdLine[:parsed.ReplacementIndex] + parsed.Items[0].CompletionText
	}

	return &PowerShellResult{
		RawOutput:        stdout,
		StdoutLeak:       leak,
		Stderr:           stderr,
		CompletionPrefix: parsed.CompletionPrefix,
		ReplacementIndex: parsed.ReplacementIndex,
		BufferTab1:       bufferTab1,
		CursorTab1:       len(bufferTab1),
		BufferTab2:       bufferTab2,
		CursorTab2:       len(bufferTab2),
		Items:            parsed.Items,
	}
}

// RunZshCompletion executes real Zsh completion for a given binary and command line string
// within the isolated workspace.
func (ws *Workspace) RunZshCompletion(t *testing.T, binaryName, cmdLine string, extraEnv ...string) *ZshResult {
	t.Helper()

	return ws.RunZshCompletionWithSetup(t, binaryName, cmdLine, "", extraEnv...)
}

// RunZshCompletionWithSetup executes real Zsh completion for a given binary and command line string
// with an optional custom setup script executed in the Zsh session.
func (ws *Workspace) RunZshCompletionWithSetup(t *testing.T, binaryName, cmdLine, setupScript string, extraEnv ...string) *ZshResult {
	t.Helper()

	zshPath, err := exec.LookPath("zsh")
	if envZsh := os.Getenv("CONGA_ZSH_BIN"); envZsh != "" {
		zshPath = envZsh
		err = nil
	}

	if err != nil {
		t.Skip("zsh binary not found on PATH; skipping real Zsh completion test")
	}

	harnessPath := ws.createHarnessFile(t, "zsh_harness_*.zsh", zshHarnessScript, 0o600)
	defer func() {
		_ = os.Remove(harnessPath) //nolint:errcheck // Clean up temp harness file.
	}()

	args := []string{"-f", "-d", harnessPath, binaryName, cmdLine}
	if setupScript != "" {
		args = append(args, setupScript)
	}

	env := append([]string{"ZDOTDIR=" + ws.HomeDir, "TERM=dumb", "CONGA_ZCOMPDUMP=" + getZshDumpFile(t, zshPath)}, extraEnv...)
	out := ws.executeShellCommand(t, zshPath, args, env...)

	return parseZshResult(out)
}

func (ws *Workspace) createHarnessFile(t *testing.T, pattern string, script []byte, mode os.FileMode) string {
	t.Helper()

	harnessFile, err := os.CreateTemp(ws.HomeDir, pattern)
	if err != nil {
		t.Fatalf("failed to create harness temp file: %v", err)
	}

	harnessPath := harnessFile.Name()
	if _, err := harnessFile.Write(script); err != nil {
		_ = harnessFile.Close() //nolint:errcheck // Error already reported.

		t.Fatalf("failed to write harness script: %v", err)
	}

	if err := harnessFile.Close(); err != nil {
		t.Fatalf("failed to close harness file: %v", err)
	}

	if err := os.Chmod(harnessPath, mode); err != nil {
		t.Fatalf("failed to chmod harness file: %v", err)
	}

	return harnessPath
}

func (ws *Workspace) executeShellCommand(t *testing.T, binPath string, args []string, extraEnv ...string) string {
	t.Helper()

	stdout, stderr := ws.executeShellCommandSplit(t, binPath, args, extraEnv...)

	return stdout + stderr
}

// executeShellCommandSplit runs the shell command and returns its stdout and
// stderr streams separately so shells whose completer output is not
// self-framing (fish, PowerShell, nushell) can assert terminal display purity.
func (ws *Workspace) executeShellCommandSplit(t *testing.T, binPath string, args []string, extraEnv ...string) (string, string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), binPath, args...) //nolint:gosec // Test harness launches shell.
	cmd.Dir = ws.WorkDir
	cmd.Env = append([]string{
		"HOME=" + ws.HomeDir,
		"PATH=" + ws.BinDir + ":" + os.Getenv("PATH"),
		"LC_ALL=C",
	}, extraEnv...)

	var stdoutBuffer, stderrBuffer bytes.Buffer

	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	if err := cmd.Run(); err != nil {
		t.Fatalf("%s execution failed: %v\nStdout:\n%s\nStderr:\n%s", filepath.Base(binPath), err, stdoutBuffer.String(), stderrBuffer.String())
	}

	return stdoutBuffer.String(), stderrBuffer.String()
}

func (ws *Workspace) generateCompletionScript(t *testing.T, binaryName, shellName string) string {
	t.Helper()

	binPath := filepath.Join(ws.BinDir, binaryName)
	cmd := exec.CommandContext(t.Context(), binPath, "completion", shellName) //nolint:gosec // Test harness runs binary.
	cmd.Dir = ws.WorkDir
	cmd.Env = []string{
		"HOME=" + ws.HomeDir,
		"PATH=" + ws.BinDir + ":" + os.Getenv("PATH"),
		"LC_ALL=C",
	}

	outBytes, err := cmd.Output()
	if err == nil {
		return string(outBytes)
	}

	cmdFlag := exec.CommandContext(t.Context(), binPath, "--completion", shellName) //nolint:gosec // Test harness runs binary.
	cmdFlag.Dir = ws.WorkDir
	cmdFlag.Env = []string{
		"HOME=" + ws.HomeDir,
		"PATH=" + ws.BinDir + ":" + os.Getenv("PATH"),
		"LC_ALL=C",
	}

	outBytesFlag, errFlag := cmdFlag.Output()
	if errFlag == nil {
		return string(outBytesFlag)
	}

	t.Fatalf("failed to generate %s completion for %s: %v\nOutput: %s", shellName, binaryName, err, string(outBytes))

	return ""
}

func getZshDumpFile(t *testing.T, zshPath string) string {
	t.Helper()

	zshDumpOnce.Do(func() {
		dumpDir, err := os.MkdirTemp("", "conga_zsh_dump_*") //nolint:usetesting // Dump cache must persist beyond a single test.
		if err != nil {
			errZshDump = fmt.Errorf("failed to create temp zsh dump dir: %w", err)

			return
		}

		zshDumpPath = filepath.Join(dumpDir, "zcompdump")
		initScript := fmt.Sprintf(`autoload -Uz compinit; compinit -d %q`, zshDumpPath)
		cmd := exec.CommandContext(t.Context(), zshPath, "-f", "-d", "-c", initScript) //nolint:gosec // Test harness launches zsh with controlled script.

		cmd.Env = []string{
			"LC_ALL=C",
			"TERM=dumb",
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			errZshDump = fmt.Errorf("failed to generate zcompdump: %w\nOutput: %s", err, string(out))

			return
		}
	})

	if errZshDump != nil {
		t.Fatalf("%v", errZshDump)
	}

	return zshDumpPath
}

func parseElvishResult(t *testing.T, raw, cmdLine string, words []string) *ElvishResult {
	t.Helper()

	var payload struct {
		Candidates []struct {
			Value       string `json:"value"`
			Display     string `json:"display"`
			Description string `json:"description"`
			CodeSuffix  string `json:"code-suffix"` //nolint:tagliatelle // Mirrors elvish's native kebab-case complex-candidate option emitted by the harness template.
		} `json:"candidates"`
		StdoutLeak []string `json:"stdoutLeak"`
	}

	trimmed := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		lines := strings.Split(trimmed, "\n")

		var found bool

		for _, line := range slices.Backward(lines) {
			candidateLine := strings.TrimSpace(line)
			if strings.HasPrefix(candidateLine, "{") {
				if err2 := json.Unmarshal([]byte(candidateLine), &payload); err2 == nil {
					found = true

					break
				}
			}
		}

		if !found {
			t.Fatalf("failed to unmarshal elvish json: %v\nOutput: %s", err, trimmed)
		}
	}

	result := &ElvishResult{
		RawOutput:  raw,
		BufferTab1: cmdLine,
		BufferTab2: cmdLine,
		Candidates: make([]Candidate, len(payload.Candidates)),
		StdoutLeak: strings.Join(payload.StdoutLeak, "\n"),
	}

	for i, candidate := range payload.Candidates {
		result.Candidates[i] = Candidate{
			Value:       candidate.Value,
			Description: elvishDescriptionFromDisplay(stripANSI(candidate.Display)),
			CodeSuffix:  candidate.CodeSuffix,
		}
	}

	var lastWord string
	if !strings.HasSuffix(cmdLine, " ") && len(words) > 0 {
		lastWord = words[len(words)-1]
	}

	if len(result.Candidates) == 1 {
		cand := result.Candidates[0].Value
		result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + cand
		result.BufferTab2 = result.BufferTab1
	} else if len(result.Candidates) > 1 {
		lcp := commonCandidatePrefix(result.Candidates)
		if len(lcp) > len(lastWord) {
			result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + lcp
		}

		result.BufferTab2 = cmdLine[:len(cmdLine)-len(lastWord)] + result.Candidates[0].Value
	}

	return result
}

// elvishDescriptionFromDisplay derives the candidate description from the
// elvish display text: the registration snippet bakes the description into
// the complex-candidate display as a trailing " (description)" suffix (the
// only channel elvish has for descriptions), so the ANSI-stripped display is
// "value (description)". Returns "" when the display carries no suffix.
func elvishDescriptionFromDisplay(plainDisplay string) string {
	if !strings.HasSuffix(plainDisplay, ")") {
		return ""
	}

	_, suffix, found := strings.Cut(plainDisplay, " (")
	if !found {
		return ""
	}

	return suffix[:len(suffix)-1]
}

func parseBashResult(raw string) *BashResult {
	result := &BashResult{
		RawOutput: raw,
		Items:     make([]string, 0),
		Messages:  make([]string, 0),
	}

	for line := range strings.SplitSeq(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		result.parseLine(line, trimmed)
	}

	return result
}

// splitPowerShellPayload separates the harness JSON document from any text
// PowerShell printed before it: the JSON payload starts at the first line
// beginning with "[" (array) or "{" (object); everything before is leakage.
func splitPowerShellPayload(stdout string) (string, string) {
	lines := strings.Split(stdout, "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
			return strings.Join(lines[index:], "\n"), strings.TrimSpace(strings.Join(lines[:index], "\n"))
		}
	}

	return "", strings.TrimSpace(stdout)
}

func parseNushellResult(t *testing.T, raw, cmdLine string, words []string) *NushellResult {
	t.Helper()

	var parsed []struct {
		Value       string        `json:"value"`
		Display     string        `json:"display"`
		Description string        `json:"description"`
		Style       *NushellStyle `json:"style"`
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		t.Fatalf("failed to unmarshal nushell json: %v\nOutput: %s", err, raw)
	}

	result := &NushellResult{
		RawOutput:  raw,
		BufferTab1: cmdLine,
		BufferTab2: cmdLine,
		Candidates: make([]Candidate, len(parsed)),
		Styles:     make(map[string]NushellStyle),
	}

	for i, candidate := range parsed {
		result.Candidates[i] = Candidate{
			Value:       candidate.Value,
			Description: candidate.Description,
		}
		if candidate.Style != nil {
			result.Styles[strings.TrimSpace(candidate.Value)] = *candidate.Style
		}
	}

	var lastWord string
	if !strings.HasSuffix(cmdLine, " ") && len(words) > 0 {
		lastWord = words[len(words)-1]
	}

	if len(result.Candidates) == 1 {
		cand := result.Candidates[0].Value
		result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + cand
		result.BufferTab2 = result.BufferTab1
	} else if len(result.Candidates) > 1 {
		lcp := commonCandidatePrefix(result.Candidates)
		if len(lcp) > len(lastWord) {
			result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + lcp
		}

		result.BufferTab2 = cmdLine[:len(cmdLine)-len(lastWord)] + result.Candidates[0].Value
	}

	result.CursorTab1 = len(result.BufferTab1)
	result.CursorTab2 = len(result.BufferTab2)

	return result
}

func parseFishResult(raw, cmdLine string) *FishResult {
	result := &FishResult{
		RawOutput:  raw,
		BufferTab1: cmdLine,
		BufferTab2: cmdLine,
		Items:      make([]Candidate, 0),
	}

	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 2)
		value := parts[0]
		desc := ""

		if len(parts) > 1 {
			desc = parts[1]
		}

		result.Items = append(result.Items, Candidate{
			Value:       value,
			Description: desc,
		})
	}

	words := tokenizer.ParseCompLine(cmdLine, len(cmdLine))

	var lastWord string
	if !strings.HasSuffix(cmdLine, " ") && len(words) > 0 {
		lastWord = words[len(words)-1]
	}

	if len(result.Items) == 1 {
		cand := result.Items[0].Value
		result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + cand
		result.BufferTab2 = result.BufferTab1
	} else if len(result.Items) > 1 {
		lcp := commonCandidatePrefix(result.Items)
		if len(lcp) > len(lastWord) {
			result.BufferTab1 = cmdLine[:len(cmdLine)-len(lastWord)] + lcp
		}

		result.BufferTab2 = cmdLine[:len(cmdLine)-len(lastWord)] + result.Items[0].Value
	}

	result.CursorTab1 = len(result.BufferTab1)
	result.CursorTab2 = len(result.BufferTab2)

	return result
}

func parseZshResult(raw string) *ZshResult {
	result := &ZshResult{
		RawOutput:   raw,
		Groups:      make(map[string][]Candidate),
		RawItems:    make([]string, 0),
		Messages:    make([]string, 0),
		FilesCalled: make([]string, 0),
		DirsCalled:  make([]string, 0),
	}

	currentTag := ""

	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}

		if parseZshMetaLine(result, line, &currentTag) {
			continue
		}

		parseZshBufferLine(result, line)
	}

	return result
}

func parseZshMetaLine(result *ZshResult, line string, currentTag *string) bool {
	if listColors, cut := strings.CutPrefix(line, "LIST_COLORS:"); cut {
		result.ListColors = listColors

		return true
	}

	if tag, cut := strings.CutPrefix(line, "TAG:"); cut {
		parts := strings.SplitN(tag, ":", 2)
		*currentTag = parts[0]

		return true
	}

	if itemStr, cut := strings.CutPrefix(line, "ITEM:"); cut {
		value, desc := splitZshDescribeItem(itemStr)

		tagToUse := *currentTag
		if tagToUse == "" {
			tagToUse = "default"
		}

		result.Groups[tagToUse] = append(result.Groups[tagToUse], Candidate{
			Value:       value,
			Description: desc,
		})

		return true
	}

	if rawItem, cut := strings.CutPrefix(line, "RAW_ITEM:"); cut {
		result.RawItems = append(result.RawItems, rawItem)

		return true
	}

	if msg, cut := strings.CutPrefix(line, "MESSAGE:"); cut {
		result.Messages = append(result.Messages, stripANSI(msg))

		return true
	}

	return false
}

func parseZshBufferLine(result *ZshResult, line string) {
	switch {
	case strings.HasPrefix(line, "COMPSTATE_INSERT:"):
		result.CompstateInsert = strings.TrimPrefix(line, "COMPSTATE_INSERT:")
	case strings.HasPrefix(line, "BUFFER_TAB1:"):
		result.BufferTab1 = strings.TrimPrefix(line, "BUFFER_TAB1:")
	case strings.HasPrefix(line, "CURSOR_TAB1:"):
		var c int
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, "CURSOR_TAB1:"), "%d", &c); err == nil {
			result.CursorTab1 = c
		}
	case strings.HasPrefix(line, "BUFFER_TAB2:"):
		result.BufferTab2 = strings.TrimPrefix(line, "BUFFER_TAB2:")
	case strings.HasPrefix(line, "CURSOR_TAB2:"):
		var c int
		if _, err := fmt.Sscanf(strings.TrimPrefix(line, "CURSOR_TAB2:"), "%d", &c); err == nil {
			result.CursorTab2 = c
		}
	case strings.HasPrefix(line, "FILES_CALLED:"):
		result.FilesCalled = append(result.FilesCalled, strings.TrimPrefix(line, "FILES_CALLED:"))
	case strings.HasPrefix(line, "DIRS_CALLED:"):
		result.DirsCalled = append(result.DirsCalled, strings.TrimPrefix(line, "DIRS_CALLED:"))
	}
}

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func splitZshDescribeItem(itemString string) (string, string) {
	var (
		valueBuilder strings.Builder
		inEscape     bool
		splitIndex   = -1
	)

	for i := range len(itemString) {
		char := itemString[i]
		if inEscape {
			valueBuilder.WriteByte(char)

			inEscape = false

			continue
		}

		if char == '\\' {
			inEscape = true

			continue
		}

		if char == ':' {
			splitIndex = i

			break
		}

		valueBuilder.WriteByte(char)
	}

	if splitIndex == -1 {
		if inEscape {
			valueBuilder.WriteByte('\\')
		}

		return valueBuilder.String(), ""
	}

	value := valueBuilder.String()
	description := itemString[splitIndex+1:]

	return value, description
}

func commonCandidatePrefix(candidates []Candidate) string {
	if len(candidates) == 0 {
		return ""
	}

	prefix := candidates[0].Value
	for _, c := range candidates[1:] {
		for !strings.HasPrefix(c.Value, prefix) {
			_, size := utf8.DecodeLastRuneInString(prefix)
			if size == 0 {
				return ""
			}

			prefix = prefix[:len(prefix)-size]
		}
	}

	return prefix
}
