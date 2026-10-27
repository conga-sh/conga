package nushell

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// Snippet renders the nushell registration script for the given command
// name: a global external completer closure (nushell has one completer for
// all external commands, discriminated by the command name in $spans.0)
// plus a defensive merge into $env.config.completions.external that
// preserves an existing completer and the nushell #15654 enable default.
// The closure validates the span count (guarding against empty spans),
// wraps the completion invocation in try-catch for robustness, drops the
// command name (the first span), and feeds the remaining spans to the
// binary's __complete entry point (docs/completion-wire-protocol.md).
// Aliases need no separate registration: the global completer resolves the name the
// user typed from the spans.
func Snippet(name string, aliases []string, _ string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString("let " + identifier + "_completer = {|spans|\n")
	builder.WriteString("    load-env { CONGA_SHELL: 'nushell' }\n")
	builder.WriteString("    if ($spans | length) > 0 {\n")
	builder.WriteString("        try {\n")
	builder.WriteString("            let words = ($spans | skip 1)\n")
	builder.WriteString("            ^$spans.0 __complete nushell ...$words | from json\n")
	builder.WriteString("        } catch { }\n")
	builder.WriteString("    }\n")
	builder.WriteString("}\n")
	builder.WriteString("\n")
	builder.WriteString("mut current = (($env | default {} config).config | default {} completions)\n")
	builder.WriteString("$current.completions = ($current.completions | default {} external)\n")
	builder.WriteString("$current.completions.external = ($current.completions.external\n")
	builder.WriteString("    | default true enable\n")
	builder.WriteString("    |# backwards compatible workaround for default, see nushell #15654\n")
	builder.WriteString("    | upsert completer { if $in == null { $" + identifier + "_completer } else { $in } })\n")
	builder.WriteString("\n")
	builder.WriteString("$env.config = $current\n")

	return builder.String()
}
