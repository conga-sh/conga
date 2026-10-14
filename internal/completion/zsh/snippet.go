package zsh

import (
	"fmt"
	"strings"

	_ "embed"

	"github.com/conga-sh/conga/internal/completion/wire"
)

var (
	//go:embed assets/completer.zsh
	completerBody string
)

// Snippet renders the zsh registration script for the given command name and
// aliases (docs/completion-wire-protocol.md): the #compdef
// header (for fpath autoload) and a trailing compdef line (for sourcing)
// register one completer function for the command name and every alias. The
// single-command form names the function _<name>_completion; the
// multi-command form _<name>_completer, where <name> is the command name
// passed through wire.SanitizeIdentifier (registration lines keep the raw
// name, quoted via wire.QuoteTargetPOSIX).
func Snippet(name string, aliases []string) string {
	if len(aliases) > 0 {
		return multiSnippet(name, aliases)
	}

	return singleSnippet(name)
}

// singleSnippet renders the no-aliases form: one completion function and one
// compdef registration for the command name.
func singleSnippet(name string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	fmt.Fprintf(&builder, "#compdef %[1]s\n", name)
	fmt.Fprintf(&builder, "function _%[1]s_completion {\n", identifier)
	builder.WriteString(completerBody)
	fmt.Fprintf(&builder, "}\n")
	fmt.Fprintf(&builder, "compdef _%[1]s_completion ", identifier)
	builder.WriteString(wire.QuoteTargetPOSIX(name))
	builder.WriteString("\n")

	return builder.String()
}

// multiSnippet renders the aliases form: one completer function shared by a
// compdef registration naming the command and every alias.
func multiSnippet(name string, aliases []string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString("#compdef " + name + " " + strings.Join(aliases, " ") + "\n")
	fmt.Fprintf(&builder, "function _%[1]s_completer {\n", identifier)
	builder.WriteString(completerBody)
	builder.WriteString("}\n")

	builder.WriteString("compdef _" + identifier + "_completer")

	for _, registration := range append([]string{name}, aliases...) {
		builder.WriteString(" " + wire.QuoteTargetPOSIX(registration))
	}

	builder.WriteString("\n")

	return builder.String()
}
