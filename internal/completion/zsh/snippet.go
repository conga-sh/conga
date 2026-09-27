package zsh

import (
	"fmt"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
)

// completerBody is the shared completer implementation. The completer
// collects the command line words (zsh's completion context), passes the
// words before the one being completed ${(Q)}-unquoted and the last word
// raw — zsh already tokenized the line, so no CLI-side quote re-lexing
// happens — to the binary's __complete entry point, and parses the record
// stream: "D" drives compstate[insert]="" for the noprefix directive
// (candidates are complete word replacements), "L" configures the
// list-colors zstyle, "M" forwards messages to _message -r, and "C"
// records group into one _describe -t call per tag (untagged candidates
// fall through to compadd). Displays compose "display:description" with
// embedded colons escaped, and the empty-suffix/quote flags hand the
// pre-quoted values to zsh verbatim. The invocation target is ${words[1]}
// — the command word the user actually typed — so the callback always
// re-invokes the exact binary (local ./path, PATH lookup, alias, etc.)
// rather than a generation-time name.
// The case word uses ${line%%"$us"*} so the record kind splits at the first
// separator.
const completerBody = `  local -a args
  if (( CURRENT >= 2 )); then
    args=("${(@Q)words[2,CURRENT-1]}" "${words[CURRENT]}")
  else
    args=("")
  fi

  local target="${words[1]/#\~/$HOME}"
  target="${aliases[$target]:-$target}"
  local lines
  lines="$("$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""

  local nl=$'\n' us=$'\x1f'
  local line tag display description composed prior
  local -a tags fields
  local -A tag_displays tag_values

  zstyle ":completion:${curcontext}:*" group-name ''

  for line in "${(@f)lines}"; do
    case "${line%%"$us"*}" in
      D)
        IFS=$us read -r -A fields <<<"${line#*"$us"}"
        [[ ",${fields[1]}," == *,noprefix,* ]] && (( ${+compstate} )) && compstate[insert]=''
        ;;
      L)
        zstyle ":completion:${curcontext}:*" list-colors "${line#*"$us"}"
        ;;
      M)
        _message -r "${line#*"$us"}"
        ;;
      C)
        IFS=$us read -r -A fields <<<"${line#*"$us"}"
        tag="${fields[1]}"
        display="${fields[2]}"
        composed="${display//:/\\:}"
        description="${fields[3]}"
        [[ -n "$description" ]] && composed+=":${description//:/\\:}"
        if [[ -z "${tag_displays[$tag]+x}" ]]; then
          tags+=("$tag")
        fi
        prior="${tag_displays[$tag]}"
        tag_displays[$tag]="${prior}${prior:+$nl}$composed"
        prior="${tag_values[$tag]}"
        tag_values[$tag]="${prior}${prior:+$nl}${fields[4]}"
        ;;
    esac
  done

  local -a displays values
  for tag in "${tags[@]}"; do
    displays=("${(@f)tag_displays[$tag]}")
    values=("${(@f)tag_values[$tag]}")
    if [[ -n "$tag" ]]; then
      _describe -t "$tag" "$tag" displays values -Q -S ''
    else
      compadd -Q -S '' -a values
    fi
  done
`

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
