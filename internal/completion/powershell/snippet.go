package powershell

import (
	"fmt"
	"runtime"
	"strings"

	_ "embed"

	"github.com/conga-sh/conga/internal/completion/wire"
)

var (
	//go:embed assets/completer.ps1
	completerTemplate string
)

// usingDeclarations opens every generated script: both namespaces are
// required for the unqualified [CompletionResult] and
// [CompletionResultType] types.
const usingDeclarations = `using namespace System.Management.Automation
using namespace System.Management.Automation.Language
`

// dynamicTarget invokes the binary under the command name the user typed.
const dynamicTarget = `$elems[0]`

// Snippet renders the PowerShell registration script for the given command
// name and aliases (docs/completion-wire-protocol.md): the
// single-command form defines one completer function and registers it for
// the command name (plus its .exe variant, active on Windows and commented
// out elsewhere); the multi-command form assigns one completer scriptblock
// (invoking the binary under the name the user typed, .exe suffix stripped)
// shared by one registration line per name and alias. The completer collects
// the cursor-truncated command elements (quotes stripped, PowerShell's
// tokenization), drops the program name, and feeds the words to the binary's
// __complete entry point; the JSON payload is piped through
// ConvertFrom-Json into CompletionResult objects. An empty result set
// returns "" so PowerShell's default file completion is suppressed.
func Snippet(name string, aliases []string, _ string) string {
	if len(aliases) > 0 {
		return multiSnippet(name, aliases)
	}

	return singleSnippet(name)
}

func singleSnippet(name string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString(usingDeclarations)
	builder.WriteString("Function _" + identifier + "_completer {\n")
	builder.WriteString("    param($wordToComplete, $commandAst, $cursorPosition)\n")
	fmt.Fprintf(&builder, completerTemplate, dynamicTarget)
	builder.WriteString("}\n")
	builder.WriteString("Register-ArgumentCompleter -Native -ScriptBlock (Get-Item \"Function:_" + identifier + "_completer\").ScriptBlock -CommandName " + registration(name) + "\n")

	return builder.String()
}

func multiSnippet(name string, aliases []string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString(usingDeclarations)
	builder.WriteString("\n")
	builder.WriteString("$_" + identifier + "_completer = {\n")
	builder.WriteString("    param($wordToComplete, $commandAst, $cursorPosition)\n")
	fmt.Fprintf(&builder, completerTemplate, dynamicTarget)
	builder.WriteString("}\n")

	for _, target := range append([]string{name}, aliases...) {
		builder.WriteString("Register-ArgumentCompleter -Native -ScriptBlock $_" + identifier + "_completer -CommandName " + registration(target) + "\n")
	}

	return builder.String()
}

// registration renders one -CommandName argument: the command name plus its
// .exe variant — both active on Windows (PowerShell resolves bare names to
// their .exe executables), the variant a comment elsewhere.
func registration(name string) string {
	quoted := wire.QuotePowerShell(name)
	quotedExe := wire.QuotePowerShell(name + ".exe")

	if runtime.GOOS == "windows" {
		return quoted + ", " + quotedExe
	}

	return quoted + " # " + quotedExe
}
