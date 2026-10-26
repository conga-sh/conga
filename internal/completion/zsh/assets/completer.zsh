  local -a args
  if (( CURRENT >= 2 )); then
    args=("${(@Q)words[2,CURRENT-1]}" "${words[CURRENT]}")
  else
    args=("")
  fi

  local target="${words[1]/#\~/$HOME}"
  target="${aliases[$target]:-$target}"

  local probe_context=":completion:${curcontext}:"

  # Honor the user's zsh completion styles: zstyle -T is true when the style
  # is unset, so an explicit false suppresses descriptions ("verbose") or
  # aliases ("show-aliases", the go-task convention). The sidebands apply only
  # to the __complete invocation, never to the user's environment. The engine's
  # list-colors patterns must match the separator zsh renders between a match
  # and its description, so the list-separator style is forwarded too.
  local -a conga_env
  local user_list_separator
  zstyle -T "$probe_context" verbose || conga_env+=(CONGA_COMPLETION_DESCRIPTIONS=0)
  zstyle -T "$probe_context" show-aliases || conga_env+=(CONGA_COMPLETION_ALIASES=0)
  if zstyle -s "$probe_context" list-separator user_list_separator && [[ -n "$user_list_separator" ]]; then
    conga_env+=("CONGA_ZSH_LIST_SEPARATOR=${user_list_separator}")
  fi

  local lines
  if (( ${#conga_env} )); then
    lines="$(env "${conga_env[@]}" "$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""
  else
    lines="$("$target" __complete zsh "${args[@]}" 2>/dev/null)" || lines=""
  fi

  local nl=$'\n' us=$'\x1f'
  local line tag display description composed prior
  local -a tags fields
  local -A tag_displays tag_values

  local style_pattern=":completion:${curcontext}:*"
  local -a user_group_name user_list_colors
  local conga_list_colors=1

  # Probe-before-set: remove only this completer's own definitions from
  # earlier invocations (zstyle -d matches the pattern literally, so user
  # styles are never touched), then honor the styles the user configured for
  # this context. An empty group-name means "no headings", so an unset or
  # empty user value keeps the engine default; a non-empty value is left
  # untouched. list-colors is forwarded from the L record only when the user
  # has no non-empty value of their own (Oh-My-Zsh's empty default keeps
  # engine colors).
  zstyle -d "$style_pattern" group-name list-colors

  if ! zstyle -a "$probe_context" group-name user_group_name || [[ -z "${(j.:.)user_group_name}" ]]; then
    zstyle "$style_pattern" group-name ''
  fi

  if zstyle -a "$probe_context" list-colors user_list_colors && [[ -n "${(j.:.)user_list_colors}" ]]; then
    conga_list_colors=0
  fi

  for line in "${(@f)lines}"; do
    case "${line%%"$us"*}" in
      D)
        IFS=$us read -r -A fields <<<"${line#*"$us"}"
        [[ ",${fields[1]}," == *,noprefix,* ]] && (( ${+compstate} )) && compstate[insert]=''
        ;;
      L)
        (( conga_list_colors )) && zstyle "$style_pattern" list-colors "${line#*$us}"
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
