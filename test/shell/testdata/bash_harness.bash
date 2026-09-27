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

# Mock compopt as a no-op in non-interactive shell test scripts
compopt() {
    :
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

# Simulate Readline buffer mutation
prefix="${COMP_WORDS[COMP_CWORD]}"
buffer_tab1="$CMD_LINE"
buffer_tab2="$CMD_LINE"

if (( ${#COMPREPLY[@]} == 1 )); then
    only_cand="${COMPREPLY[0]}"
    if [[ "$CMD_LINE" == *" " ]]; then
        buffer_tab1="${CMD_LINE}${only_cand}"
    else
        buffer_tab1="${CMD_LINE%${prefix}}${only_cand}"
    fi
    buffer_tab2="$buffer_tab1"
elif (( ${#COMPREPLY[@]} > 1 )); then
    lcp="${COMPREPLY[0]}"
    for item in "${COMPREPLY[@]:1}"; do
        while [[ -n "$lcp" && "$item" != "$lcp"* ]]; do
            lcp="${lcp:0:${#lcp}-1}"
        done
    done
    if [[ -n "$lcp" && ${#lcp} -gt ${#prefix} ]]; then
        if [[ "$CMD_LINE" == *" " ]]; then
            buffer_tab1="${CMD_LINE}${lcp}"
        else
            buffer_tab1="${CMD_LINE%${prefix}}${lcp}"
        fi
    fi
    first_cand="${COMPREPLY[0]}"
    if [[ "$CMD_LINE" == *" " ]]; then
        buffer_tab2="${CMD_LINE}${first_cand}"
    else
        buffer_tab2="${CMD_LINE%${prefix}}${first_cand}"
    fi
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

