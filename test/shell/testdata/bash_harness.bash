#!/usr/bin/env bash
# Bash Completion Test Harness
# Runs inside the isolated test workspace and captures exact completion candidates.

BIN_NAME="$1"
CMD_LINE="$2"
COMP_TYPE_VAL="${3:-9}"
SETUP_SCRIPT="$4"

export LC_ALL=C
export TERM=dumb

# Enable programmable completion and alias expansion in subshell if available
shopt -s progcomp 2>/dev/null || true
shopt -s expand_aliases 2>/dev/null || true

# Mock compopt, which needs an active completion, by recording its arguments.
# The log is printed as COMPOPT: lines after the completer has run.
compopt_log=$(mktemp)
compopt() {
    printf '%s\n' "$*" >>"$compopt_log"
}

# Source the completion script for the binary (subcommand or --completion flag)
# shellcheck disable=SC1090
source <("$BIN_NAME" completion bash 2>/dev/null || "$BIN_NAME" --completion bash) || true

# Execute setup hook if specified (e.g. aliases or options)
if [[ -n "$SETUP_SCRIPT" ]]; then
    eval "$SETUP_SCRIPT"
elif [[ -n "$CONGA_BASH_POST_SETUP" ]]; then
    eval "$CONGA_BASH_POST_SETUP"
fi

# Setup completion environment variables
export COMP_LINE="$CMD_LINE"
export COMP_POINT="${#CMD_LINE}"
export COMP_TYPE="$COMP_TYPE_VAL"
export COMP_WORDBREAKS=$' \t\n"\'><=;|&(:'

# Parse words into COMP_WORDS array
IFS=' ' read -r -a COMP_WORDS <<< "$CMD_LINE"
if [[ "$CMD_LINE" == *" " ]]; then
    COMP_WORDS+=("")
fi
export COMP_WORDS
export COMP_CWORD=$(( ${#COMP_WORDS[@]} - 1 ))

# Invoke registered bash completion function and capture any stray stdout.
# In interactive Bash, any stdout emitted during completion execution moves the cursor and breaks Readline.
first_word="${COMP_WORDS[0]}"
completer_func=""
if complete_spec=$(complete -p "$first_word" 2>/dev/null); then
    if [[ "$complete_spec" =~ -F[[:space:]]+([^[:space:]]+) ]]; then
        completer_func="${BASH_REMATCH[1]}"
    fi
fi
if [[ -z "$completer_func" ]]; then
    comp_identifier="${BIN_NAME//[^a-zA-Z0-9_]/_}"
    if type "_${comp_identifier}_completion" >/dev/null 2>&1; then
        completer_func="_${comp_identifier}_completion"
    elif type "_${comp_identifier}_completer" >/dev/null 2>&1; then
        completer_func="_${comp_identifier}_completer"
    fi
fi

comp_stdout_tmp=$(mktemp)
if [[ -n "$completer_func" ]] && type "$completer_func" >/dev/null 2>&1; then
    "$completer_func" >"$comp_stdout_tmp" 2>&1
fi

comp_leak=$(cat "$comp_stdout_tmp")
rm -f "$comp_stdout_tmp"

if [[ -n "$comp_leak" ]]; then
    echo "STDOUT_LEAK:${comp_leak}"
fi

compopt_default=0
while IFS= read -r compopt_call; do
    echo "COMPOPT:${compopt_call}"
    [[ " ${compopt_call} " == *" -o default "* ]] && compopt_default=1
done <"$compopt_log"
rm -f "$compopt_log"

# Simulate Readline buffer mutation. Readline replaces only the end of the
# word under the cursor: the text after its last COMP_WORDBREAKS character
# outside quotes and not escaped, or after a quote still open at the end of
# the word, whichever comes later. COMP_WORDS above is split on spaces only,
# so the replaced text is found by scanning the word.
word="${COMP_WORDS[COMP_CWORD]}"
replace_start=0
open_quote=""
escaped=0
for (( i = 0; i < ${#word}; i++ )); do
    char="${word:i:1}"
    if (( escaped )); then
        escaped=0
    elif [[ -n "$open_quote" ]]; then
        if [[ "$char" == "$open_quote" ]]; then
            open_quote=""
        elif [[ "$open_quote" == '"' && "$char" == '\' ]]; then
            escaped=1
        fi
    elif [[ "$char" == '\' ]]; then
        escaped=1
    elif [[ "$char" == "'" || "$char" == '"' ]]; then
        open_quote="$char"
        quote_start=$(( i + 1 ))
    elif [[ "$COMP_WORDBREAKS" == *"$char"* ]]; then
        replace_start=$(( i + 1 ))
    fi
done
if [[ -n "$open_quote" ]] && (( quote_start > replace_start )); then
    replace_start=$quote_start
fi
replaced="${word:replace_start}"
kept="${CMD_LINE:0:${#CMD_LINE}-${#replaced}}"

# Simulate readline's default completion, which compopt -o default selects
# when the completer leaves COMPREPLY empty: the filenames that start with the
# replaced text, a directory marked with a trailing slash, each name quoted
# as readline quotes it without noquote.
if (( compopt_default )) && (( ${#COMPREPLY[@]} == 0 )); then
    while IFS= read -r name; do
        [[ -d "$name" ]] && name+="/"
        COMPREPLY+=("$(printf '%q' "$name")")
    done < <(compgen -f -- "$replaced")
fi

buffer_tab1="$CMD_LINE"
buffer_tab2="$CMD_LINE"

if (( ${#COMPREPLY[@]} == 1 )); then
    buffer_tab1="${kept}${COMPREPLY[0]}"
    buffer_tab2="$buffer_tab1"
elif (( ${#COMPREPLY[@]} > 1 )); then
    lcp="${COMPREPLY[0]}"
    for item in "${COMPREPLY[@]:1}"; do
        while [[ -n "$lcp" && "$item" != "$lcp"* ]]; do
            lcp="${lcp:0:${#lcp}-1}"
        done
    done
    if [[ -n "$lcp" && ${#lcp} -gt ${#replaced} ]]; then
        buffer_tab1="${kept}${lcp}"
    fi
    buffer_tab2="${kept}${COMPREPLY[0]}"
fi

echo "BUFFER_TAB1:${buffer_tab1}"
echo "CURSOR_TAB1:${#buffer_tab1}"
echo "BUFFER_TAB2:${buffer_tab2}"
echo "CURSOR_TAB2:${#buffer_tab2}"

# Print captured COMPREPLY array items
for item in "${COMPREPLY[@]}"; do
    if [[ -n "$item" ]]; then
        echo "ITEM:${item}"
    fi
done

