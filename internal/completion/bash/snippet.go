package bash

import (
	"fmt"
	"strings"

	_ "embed"

	"github.com/conga-sh/conga/internal/completion/wire"
)

var (
	//go:embed assets/completer.bash
	completerBody string
)

// Snippet renders the bash registration script for the given command name and
// aliases (docs/completion-wire-protocol.md): one completer
// function per name that exports the bash-native completion environment,
// splits the cursor-truncated COMP_LINE with xargs (the appended-quote retry
// chain closes unclosed quotes mid-word and keeps the input non-empty so
// xargs always runs, preserving the trailing empty word), and feeds the
// resolved words — excluding the program name — to the binary's __complete
// entry point. The invocation target is "${cmd}", the first word of the
// truncated line — the command the user actually typed (local ./path, PATH
// lookup, alias, etc.) — so the callback always re-invokes the exact
// binary rather than a generation-time name. The D/M/C lines parse back
// into COMPREPLY without echoing
// anything: bash has no message widget, so M lines are dropped, and the
// function must produce zero terminal leakage. In successive-tab mode
// (COMP_TYPE=63) the payload carries display/value/description triplets,
// rendered as "display (description)" for readline's listing. The completer
// function name derives from the command name via wire.SanitizeIdentifier;
// registration lines keep the raw name, quoted via wire.QuoteTargetPOSIX.
func Snippet(name string, aliases []string, _ string) string {
	identifier := wire.SanitizeIdentifier(name)

	var builder strings.Builder

	builder.WriteString("#!/bin/bash\n")
	fmt.Fprintf(&builder, "_%s_completion() {\n", identifier)
	builder.WriteString(completerBody)

	fmt.Fprintf(&builder, "complete -o noquote -F _%s_completion", identifier)

	for _, candidate := range append([]string{name}, aliases...) {
		builder.WriteByte(' ')
		builder.WriteString(wire.QuoteTargetPOSIX(candidate))
	}

	builder.WriteByte('\n')

	return builder.String()
}
