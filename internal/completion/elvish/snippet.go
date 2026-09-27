package elvish

import (
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// Snippet renders the elvish registration script for the given command name
// and aliases: one shared completer lambda plus one arg-completer
// registration per name and alias. The lambda sets CONGA_SHELL for the
// engine's shell-aware framing decisions, slices off the command name
// (elvish passes it in front of the arguments; the word being completed is
// the last argument, possibly an empty string), and invokes the binary's
// __complete entry point via argv only (docs/completion-wire-protocol.md).
// The payload's usage and diagnostic messages are omitted to enforce terminal
// display purity (preventing prompt displacement and scrollback pollution in
// elvish), and each candidate renders as an edit:complex-candidate with a
// styled display, a description suffix styled by the payload's
// descriptionStyle, and the codeSuffix controlling post-insert spacing.
func Snippet(name string, aliases []string) string {
	var builder strings.Builder

	builder.WriteString("var completer~ = {|@args|\n")
	builder.WriteString("    set-env CONGA_SHELL elvish\n")
	builder.WriteString("    var cmd = \"\"\n")
	builder.WriteString("    try {\n")
	builder.WriteString("        set cmd = (search-external $args[0])\n")
	builder.WriteString("    } catch { }\n")
	builder.WriteString("    if (not-eq $cmd \"\") {\n")
	builder.WriteString("        try {\n")
	builder.WriteString("            var completion = ($cmd __complete elvish (all $args[1..]) 2>/dev/null | from-json)\n")
	builder.WriteString("            for c $completion[candidates] {\n")
	builder.WriteString("                var styling = $c[style]\n")
	builder.WriteString("                if (eq $styling \"\") {\n")
	builder.WriteString("                    set styling = \"default\"\n")
	builder.WriteString("                }\n")
	builder.WriteString("                var description = $c[description]\n")
	builder.WriteString("                if (not-eq $description \"\") {\n")
	builder.WriteString("                    set description = (styled \" (\"$description\")\" $completion[descriptionStyle])\n")
	builder.WriteString("                }\n")
	builder.WriteString("                edit:complex-candidate $c[value] &display=(styled $c[display] $styling)$description &code-suffix=$c[codeSuffix]\n")
	builder.WriteString("            }\n")
	builder.WriteString("        } catch { }\n")
	builder.WriteString("    }\n")
	builder.WriteString("}\n")

	for _, commandName := range append([]string{name}, aliases...) {
		builder.WriteString("set edit:completion:arg-completer[" + quoteElvishWord(commandName) + "] = $completer~\n")
	}

	return builder.String()
}

// quoteElvishWord quotes a word for use as an elvish map key or command word
// unless it consists only of characters that are literal in an elvish
// bareword (elvish metacharacters — whitespace, quotes, escapes, expansions,
// brackets, grouping, separators, redirection, globbing, and comment
// markers — force quoting).
func quoteElvishWord(word string) string {
	return wire.QuoteIfUnsafe(word, " \t\n\v\"'\\`$[](){};|&<>*?#", wire.QuoteElvish)
}
