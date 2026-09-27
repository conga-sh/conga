package powershell

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// usingDeclarations opens every generated script: both namespaces are
// required for the unqualified [CompletionResult] and
// [CompletionResultType] types.
const usingDeclarations = `using namespace System.Management.Automation
using namespace System.Management.Automation.Language
`

// dynamicTarget invokes the binary under the command name the user typed.
const dynamicTarget = `$elems[0]`

// completerTemplate is the shared completer body: %[1]s is the invocation
// target expression. The elements of the parsed command AST are truncated at
// the cursor, stripped of their surrounding quotes (PowerShell's Extent.Text
// is the raw source), restored from empty tokens, and passed — without the
// program name — to the binary's __complete entry point. The JSON payload is
// piped through ConvertFrom-Json into CompletionResult objects; a failed or
// empty invocation returns "" so PowerShell's default file completion is
// suppressed.
const completerTemplate = `    $esc = [char]27
    $commandElements = $commandAst.CommandElements

    $elems = @()
    foreach ($_ in $commandElements) {
      if ($_.Extent.StartOffset -gt $cursorPosition) {
          break
      }
      $t = $_.Extent.Text
      if ($_.Extent.EndOffset -gt $cursorPosition) {
          $t = $t.Substring(0, $_.Extent.Text.Length - ($_.Extent.EndOffset - $cursorPosition))
      }
      if ($t.Length -gt 0 -and ($t[0] -eq "'" -or $t[0] -eq '"')) {
        $t = $t.Substring(1)
      }
      if ($t.Length -gt 0 -and ($t[$t.Length - 1] -eq "'" -or $t[$t.Length - 1] -eq '"')) {
        $t = $t.Substring(0, $t.Length - 1)
      }
      if ($t.Length -eq 0) {
        $t = '""'
      }
      $elems += $t.replace('` + "`" + `,', ',')
    }

    if ($elems.Count -eq 0) {
      return ""
    }

    $words = @()
    if ($elems.Count -gt 1) {
      $words = $elems[1..($elems.Count - 1)]
    }

    $payload = @(
      if (!$wordToComplete) {
        & %[1]s __complete powershell $words '' 2>$null
      } else {
        & %[1]s __complete powershell $words 2>$null
      }
    )
    if (!$payload) {
      return ""
    }

    $completions = @(
      ($payload | ConvertFrom-Json).candidates | ForEach-Object {
        [CompletionResult]::new($_.value, $_.display.replace('` + "`" + `e[', ($esc + '[')), [CompletionResultType]::ParameterValue, $_.description.replace('` + "`" + `e[', ($esc + '[')))
      }
    )
    if ($completions.Count -eq 0) {
      return ""
    }

    $completions
`

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
