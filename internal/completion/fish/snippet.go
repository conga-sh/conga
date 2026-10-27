package fish

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// Snippet renders the fish registration script for the given command name and
// aliases: one completer function (named after the primary command so several
// CONGA applications can coexist in one fish session) plus one complete
// registration pair per name and alias. The function name derives from the
// primary command name via wire.SanitizeIdentifier (registration lines keep
// the raw name, quoted via wire.QuoteFish). The function collects the
// tokenized command line with commandline -opc (words up to the cursor,
// including the command name, quotes already resolved) and commandline -ct
// (the word being completed, possibly empty, possibly still carrying quote
// characters), strips the command name, and feeds the words to the binary's
// __complete entry point (docs/completion-wire-protocol.md), invoking the
// binary under the command name the user actually typed — $words[1] (local
// ./path, PATH lookup, alias, etc.) — so the callback re-invokes the exact
// binary rather than a generation-time name. Because fish itself
// tokenizes, no shell-side re-lexing, sed plumbing, or unclosed-quote retry
// is needed. The 2>/dev/null suppression keeps diagnostics off the terminal:
// a non-zero exit yields an empty substitution, which fish treats as "no
// completions".
func Snippet(name string, aliases []string, _ string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString("function _" + identifier + "_completion\n")
	builder.WriteString("  set --local words (commandline -opc)\n")
	builder.WriteString("  if test (count $words) -gt 0\n")
	builder.WriteString("    set --local current (commandline -ct)\n")
	builder.WriteString("    $words[1] __complete fish $words[2..-1] $current 2>/dev/null\n")
	builder.WriteString("  end\n")
	builder.WriteString("end\n")

	for _, target := range append([]string{name}, aliases...) {
		builder.WriteString("\n")
		builder.WriteString("complete -e " + wire.QuoteFish(target) + "\n")
		builder.WriteString("complete -c " + wire.QuoteFish(target) + " -f -a '(_" + identifier + "_completion)' -r\n")
	}

	return builder.String()
}
