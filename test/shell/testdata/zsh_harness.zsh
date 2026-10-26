#!/usr/bin/env zsh
# Zsh Completion Test Harness
# Runs inside the isolated test workspace and captures exact completion metadata.

set -e

BIN_NAME="$1"
CMD_LINE="$2"

export LC_ALL=C
export TERM=dumb
export ZDOTDIR="$HOME"
rehash

# Initialize completion system
autoload -Uz compinit
if [[ -n "$CONGA_ZCOMPDUMP" && -f "$CONGA_ZCOMPDUMP" ]]; then
    compinit -C -d "$CONGA_ZCOMPDUMP"
else
    compinit -D
fi

# Pre-setup hook if specified
if [[ -n "$CONGA_ZSH_PRE_SETUP" ]]; then
    eval "$CONGA_ZSH_PRE_SETUP"
fi

# Source the completion script for the binary (subcommand or --completion flag)
source <("$BIN_NAME" completion zsh 2>/dev/null || "$BIN_NAME" --completion zsh)

# Post-setup hook if specified
if [[ -n "$3" ]]; then
    eval "$3"
elif [[ -n "$CONGA_ZSH_POST_SETUP" ]]; then
    eval "$CONGA_ZSH_POST_SETUP"
fi

# Hook _describe to capture group tags, items, and descriptions
_describe() {
    local tag=""
    local -a pos_args=()
    local -a args
    args=("$@")

    local i=1
    while (( i <= ${#args} )); do
        if [[ "${args[i]}" == "-t" ]]; then
            tag="${args[i+1]}"
            i=$(( i + 2 ))
        elif [[ "${args[i]}" == "-S" || "${args[i]}" == "-p" || "${args[i]}" == "-s" || "${args[i]}" == "-q" || "${args[i]}" == "-d" || "${args[i]}" == "-X" || "${args[i]}" == "-x" ]]; then
            i=$(( i + 2 ))
        elif [[ "${args[i]}" == -* ]]; then
            i=$(( i + 1 ))
        else
            pos_args+=("${args[i]}")
            i=$(( i + 1 ))
        fi
    done

    local group_desc="${pos_args[1]}"
    local displays_name="${pos_args[2]}"
    local values_name="${pos_args[3]}"
    if [[ -z "$values_name" ]]; then
        values_name="$displays_name"
    fi

    local -a _captured_displays
    local -a _captured_values
    _captured_displays=("${(@f)${(@P)displays_name}}")
    _captured_values=("${(@f)${(@P)values_name}}")

    print "TAG:${tag}:${group_desc}"
    for disp in "${_captured_displays[@]}"; do
        if [[ -n "$disp" ]]; then
            print "ITEM:${disp}"
        fi
    done

    for val in "${_captured_values[@]}"; do
        if [[ -n "$val" ]]; then
            _all_captured_candidates+=("$val")
        fi
    done

    return 0
}

# Hook compadd to capture raw matches
compadd() {
    local -a args
    args=("$@")
    local -a matches=()

    local i=1
    while (( i <= ${#args} )); do
        if [[ "${args[i]}" == "-a" ]]; then
            local aname="${args[i+1]}"
            matches+=("${(@f)${(@P)aname}}")
            i=$(( i + 2 ))
        elif [[ "${args[i]}" == "--" ]]; then
            i=$(( i + 1 ))
            matches+=("${args[@:i]}")
            break
        elif [[ "${args[i]}" == "-S" || "${args[i]}" == "-p" || "${args[i]}" == "-s" || "${args[i]}" == "-q" || "${args[i]}" == "-d" || "${args[i]}" == "-X" || "${args[i]}" == "-x" ]]; then
            i=$(( i + 2 ))
        elif [[ "${args[i]}" == -* ]]; then
            i=$(( i + 1 ))
        else
            matches+=("${args[i]}")
            i=$(( i + 1 ))
        fi
    done

    for m in "${matches[@]}"; do
        if [[ -n "$m" ]]; then
            print "RAW_ITEM:${m}"
            _all_captured_candidates+=("$m")
        fi
    done

    return 0
}

# Hook _message to capture ActiveHelp hints
_message() {
    local msg=""
    if [[ "$1" == "-r" ]]; then
        msg="$2"
    else
        msg="$1"
    fi
    print "MESSAGE:${msg}"
    return 0
}

# Hook _files / _path_files to capture file completions
_files() {
    print "FILES_CALLED:$@"
    return 0
}

_path_files() {
    print "DIRS_CALLED:$@"
    return 0
}

typeset -a _all_captured_candidates
_all_captured_candidates=()

# Execute completion for the given command line
run_test_completion() {
    typeset -A compstate
    compstate[insert]="unambiguous"
    local -a words
    words=(${(z)CMD_LINE})
    local CURRENT=${#words}
    if [[ "$CMD_LINE" == *" " ]]; then
        words+=("")
        CURRENT=$(( CURRENT + 1 ))
    fi

    local PREFIX="${words[$CURRENT]}"
    local IPREFIX=""
    local SUFFIX=""
    local ISUFFIX=""

    local first_word="${words[1]}"
    if [[ -z "$first_word" ]]; then
        first_word="$BIN_NAME"
    fi
    local curcontext="${first_word}:::"

    local completer_func="${_comps[$first_word]}"
    if [[ -z "$completer_func" ]]; then
        # The snippet derives its function identifier via
        # wire.SanitizeIdentifier (alnum + underscore, everything else
        # collapsed to underscore).
        local first_identifier="${first_word//[^a-zA-Z0-9_]/_}"
        local bin_identifier="${BIN_NAME//[^a-zA-Z0-9_]/_}"
        if type "_${first_identifier}_completion" >/dev/null 2>&1; then
            completer_func="_${first_identifier}_completion"
        elif type "_${first_identifier}_completer" >/dev/null 2>&1; then
            completer_func="_${first_identifier}_completer"
        elif type "_${bin_identifier}_completion" >/dev/null 2>&1; then
            completer_func="_${bin_identifier}_completion"
        elif type "_${bin_identifier}_completer" >/dev/null 2>&1; then
            completer_func="_${bin_identifier}_completer"
        elif type "__conga_complete_${BIN_NAME}" >/dev/null 2>&1; then
            completer_func="__conga_complete_${BIN_NAME}"
        fi
    fi

    if [[ -n "$completer_func" ]]; then
        "$completer_func"
    fi

    # Check for configured list-colors
    local lc=""
    if zstyle -s ":completion:${curcontext}:*" list-colors lc; then
        print "LIST_COLORS:${lc}"
    fi

    # Check for configured group-name
    local gn=""
    if zstyle -s ":completion:${curcontext}:*" group-name gn; then
        print "GROUP_NAME:${gn}"
    fi

    print "COMPSTATE_INSERT:${compstate[insert]}"

    local buffer_tab1="$CMD_LINE"
    local buffer_tab2="$CMD_LINE"

    local -a matching_candidates=()
    for item in "${_all_captured_candidates[@]}"; do
        if [[ -z "$PREFIX" || "$item" == "$PREFIX"* ]]; then
            matching_candidates+=("$item")
        fi
    done

    if [[ -z "${compstate[insert]}" || "${compstate[insert]}" == "0" || "${compstate[insert]}" == "automenu" ]]; then
        buffer_tab1="$CMD_LINE"
        if (( ${#matching_candidates[@]} > 0 )); then
            local first_cand="${matching_candidates[1]}"
            if [[ "$CMD_LINE" == *" " ]]; then
                buffer_tab2="${CMD_LINE}${first_cand}"
            else
                buffer_tab2="${CMD_LINE%${PREFIX}}${first_cand}"
            fi
        fi
    elif [[ "${compstate[insert]}" == "menu" ]]; then
        if (( ${#matching_candidates[@]} > 0 )); then
            local first_cand="${matching_candidates[1]}"
            if [[ "$CMD_LINE" == *" " ]]; then
                buffer_tab1="${CMD_LINE}${first_cand}"
            else
                buffer_tab1="${CMD_LINE%${PREFIX}}${first_cand}"
            fi
            buffer_tab2="$buffer_tab1"
        fi
    else
        if (( ${#matching_candidates[@]} == 1 )); then
            local only_cand="${matching_candidates[1]}"
            if [[ "$CMD_LINE" == *" " ]]; then
                buffer_tab1="${CMD_LINE}${only_cand}"
            else
                buffer_tab1="${CMD_LINE%${PREFIX}}${only_cand}"
            fi
            buffer_tab2="$buffer_tab1"
        elif (( ${#matching_candidates[@]} > 1 )); then
            local lcp="${matching_candidates[1]}"
            for cand in "${matching_candidates[@]:1}"; do
                while [[ -n "$lcp" && "$cand" != "$lcp"* ]]; do
                    lcp="${lcp[1,-2]}"
                done
            done
            if [[ -n "$lcp" && ${#lcp} -gt ${#PREFIX} ]]; then
                if [[ "$CMD_LINE" == *" " ]]; then
                    buffer_tab1="${CMD_LINE}${lcp}"
                else
                    buffer_tab1="${CMD_LINE%${PREFIX}}${lcp}"
                fi
            fi
            local first_cand="${matching_candidates[1]}"
            if [[ "$CMD_LINE" == *" " ]]; then
                buffer_tab2="${CMD_LINE}${first_cand}"
            else
                buffer_tab2="${CMD_LINE%${PREFIX}}${first_cand}"
            fi
        fi
    fi

    print "BUFFER_TAB1:${buffer_tab1}"
    print "CURSOR_TAB1:${#buffer_tab1}"
    print "BUFFER_TAB2:${buffer_tab2}"
    print "CURSOR_TAB2:${#buffer_tab2}"
}

run_test_completion

