# Completion Wire Protocol

- **Status:** Accepted.
- **Scope:** The contract between generated shell registration scripts (snippets) and the hidden completion entry point of a CONGA binary.
- **Consumers:** `internal/completion` drivers (bash, zsh, fish, powershell, elvish, nushell, export), the generated snippets, `conga.go` dispatch, and the conformance corpus.

---

## 1. Overview

Completion is a two-party protocol:

1. A **snippet** — the registration script emitted by `app completion <shell>` — intercepts the host shell's completion event, collects the words of the current command line, and invokes the CONGA binary.
2. The **entry point** — the hidden `__complete` command — runs the CONGA completion engine (`spec.Complete`) and prints the result in the framing of the requesting shell.

The engine (`internal/spec/completion.go`, `internal/tokenizer`, `predict/*`) is shell-agnostic and outside the scope of this document. This specification defines the wire contract between host shell snippets and the CONGA binary: invocation syntax, word semantics, environment variable sidebands, directives, styling, per-shell payload framing, and behavioral requirements.

## 2. Entry convention

```
<cmd> __complete <shell> <words...>
```

- `<shell>` — one of the six protocol shells: `bash`, `zsh`, `fish`, `powershell`, `elvish`, `nushell`. The identifier `export` is also accepted by the entry point as a direct request for the machine-readable framing (§7.1); it is a **framing-only target**: the dispatcher renders it via the unknown-shell fallback, and `completion.ParseShell` — which validates protocol shells only — rejects it.
- `<words...>` — zero or more words of the command line being completed, **excluding** the program name. The **last word is the word being completed** and may be empty (empty string argument). When `<words...>` is entirely empty, the word being completed is the empty string.
- The entry point is hidden: it never appears in help, completion candidates, or docs.

**Shell fallback.** If `<shell>` is empty or not one of the supported identifiers above, the payload is rendered in the `export` framing (§7.1). Rationale: `export` is the canonical machine-readable presentation, and an unknown shell request must not produce an unparseable mixed stream. The fallback is silent (no stderr warning) to keep the completion hot path free of stdout/stderr pollution; tooling that requires strict validation uses `completion.ParseShell`, which accepts the six protocol shells only — `export` is a framing target, never a `ParseShell` result.

**Exit codes.** `0` on success, including zero candidates. `2` on internal error (engine panic recovery, unwritable stdout), with a diagnostic on stderr and no payload on stdout. The snippet treats any non-zero exit as "no completions".

## 3. Word semantics per shell

The words passed after `<shell>` must satisfy: **all words except the last are fully resolved tokens** (quotes already removed); the **last word is passed as-is** (it may still carry quote characters because the user is mid-quote).

| Shell | Word source | Quote handling |
|---|---|---|
| bash | Parsed by the snippet from `COMP_LINE` at `COMP_POINT`, with `COMP_WORDBREAKS` prefix handling applied by the driver's `Patch` seam (§7.2) | Quote state re-lexed by the engine from `COMP_LINE`; words passed via argv are unquoted |
| zsh | `words` array from the completion context, passed via argv | Words `1..CURRENT-1` are passed `${(Q)}`-unquoted; the last word is passed raw (unprocessed). `${(Q)}` correctly unquotes fully-quoted words, escaped characters, and mixed-quote words in completed positions. Partially-quoted final words (e.g. `--env="pro`) keep their quote characters; for attached flags, the completion walker partitions on `=` into `valPart = "pro` and `filterItems` trims leading/trailing quotes, matching `production` cleanly |
| fish | `$argv` of the `complete -c` invocation, minus the command | Fish has already tokenized; words arrive unquoted |
| powershell | `$wordToComplete` plus the invocation words collected by the snippet | PowerShell 5.1/7 tokenization; words arrive unquoted |
| elvish | `@args` of the `edit:completion:arg-completer` invocation, minus the command name | Elvish has already tokenized; words arrive unquoted |
| nushell | `$spans` of the external completer, minus the command name (`$spans.0`) | Nushell passes spans raw: completed positions may still carry quote characters, so the driver's `Patch` seam dequotes them (double/single-quoted spans re-lexed to their first token, backticks trimmed); the last word is passed raw |
| export | Caller-defined (test tooling) | Caller-defined |

The engine receives these words verbatim as `spec.Complete` arguments; the last word becomes `Context.ToComplete`.

## 4. Environment variables

### 4.1 Host shell variables (Bash)

Set and exported by the bash snippet itself, consumed by the driver:

| Variable | Meaning |
|---|---|
| `COMP_LINE` | The current command line |
| `COMP_POINT` | Cursor offset (bytes) into `COMP_LINE` |
| `COMP_TYPE` | Readline completion type (`63` = successive-tab listing) |
| `COMP_WORDBREAKS` | Characters that break words for prefix handling |

### 4.2 CONGA sideband

| Variable | Meaning | Default |
|---|---|---|
| `CONGA_SHELL` | Forces the target shell, overriding detection | *(unset)* |
| `CONGA_COMPLETION_ALIASES` | Shows (`1`/`true`) or suppresses (`0`/`false`) alias candidates; unset or unparseable shows them. Aliases are subcommand/flag aliases declared on the schema and predictor-supplied domain aliases marked with `predict.Item.Alias` | shown |
| `CONGA_COMPLETION_DESCRIPTIONS` | Shows (`1`/`true`) or suppresses (`0`/`false`) candidate descriptions in every framing; unset or unparseable shows them | shown |
| `CONGA_MERGE_FLAGS` | Merges `longhand flags`/`shorthand flags` tags into `flags` (zsh grouping); `true`/`false` | `true` for zsh, unset elsewhere |
| `CONGA_DESCRIPTION_LENGTH` | Maximum description length (runes) for fish/powershell clamping | `80` |
| `CONGA_TOOLTIP` | Enables tooltip-style description rendering (powershell) | off |
| `CONGA_COLOR` | `"0"` disables color styling in rendered output | *(unset)* |
| `CONGA_ZSH_LIST_SEPARATOR` | Separator zsh renders between a match and its description, mirrored from the user's `list-separator` style; glob-escaped into zsh list-colors patterns. Empty, colon-containing, control-character-containing, or over-32-byte values fall back to the default | `--` |
| `CONGA_ZSH_STYLE_LIMIT` | Maximum candidates to inline-style in zsh | `300` |

### 4.3 Standard

`NO_COLOR` and `CLICOLOR=0` disable color styling (checked in that order after `CONGA_COLOR`).

### 4.4 Command-line transfer convention

Command-line words are transferred strictly via argv (§3) or standard host shell mechanisms (`COMP_LINE` in bash); CONGA does not define custom environment variables to transport full command-line strings.

## 5. Directives

Directives are shell-agnostic instructions about how to treat the candidate stream. The engine's `predict.Directive` bit flags map onto the wire vocabulary:

| Token | Engine bit | Semantics |
|---|---|---|
| `nospace` | `DirectiveNoSpace` | Do not append a space after inserting a candidate. Additionally implied per candidate when its value ends with `=` or `/` |
| `noprefix` | *(adapter-level)* | Candidate values are complete word replacements: the shell must replace the entire current word instead of appending after its typed prefix. Emitted for attached-value completion (`--flag=<TAB>` → values are `--flag=value`) |
| `keeporder` | `DirectiveKeepOrder` | Preserve engine candidate order; do not let the shell re-sort |

`DirectiveError` and the engine's `Usage` field surface as **messages** (§7): the usage string, or the literal `ERROR` when usage is empty. `DirectiveNoFileCompletion` is carried on the engine's `Result` and has no wire representation.

## 6. Style vocabulary

Candidates carry an optional `style` — a space-separated token string from a closed vocabulary (see `internal/style`):

- Attributes: `bold`, `dim`, `italic`, `underlined`, `blink`, `inverse`
- Foreground: `black red green yellow blue magenta cyan white`, `bright-*` variants
- Background: `bg-*`, `bg-bright-*` variants, `fg-default`/`bg-default`
- Extended: `color0`–`color255`, `#rrggbb`, `rgb(r,g,b)`

Drivers translate tokens to each shell's native styling (ECMA-48 SGR numbers for bash/zsh; backtick-escaped SGR for powershell; none for fish; elvish `styled` token strings per §7.6; nushell `{fg,bg,attr}` records per §7.7). The default palette lives in `style.Theme` (type `Palette`).

## 7. Per-shell framing

### 7.1 `export` — JSON (machine-readable oracle)

Single-line JSON object, no trailing newline:

```json
{
  "candidates": [
    {"value": "deploy", "display": "deploy", "description": "Deploy service", "style": "blue", "tag": "commands"}
  ],
  "directives": {"keeporder": false, "messages": ["<usage>"], "noprefix": false, "nospace": false},
  "usage": ""
}
```

- Candidate fields `value` and `display` are always present; `description`, `style`, `tag` are omitted when empty; `alias` is present only for alias candidates (`"alias":true`). Array order is engine order.
- `directives` is always complete: the three booleans and the `messages` array (possibly empty).
- `usage` is always present (empty string when none).
- Strings are escaped per RFC 8259 by the zero-dependency escaper (`encoding/json` is restricted by the linker-friendly dependency boundaries). Control characters use `\u00XX`; the exact minimal-escape set is defined by the escaper's tests.
- The powershell framing is identical **plus** an `sgr` field per candidate (ECMA-48 SGR parameter numbers, e.g. `"1;31"`, present when `style` is non-empty and color is enabled).

### 7.2 `bash` — line-oriented

```
D\t<directive tokens>
[M\t<message>]
C\t<candidate line>...
```

- **Directive Line (`D`)** — `D\t<tokens>`: directive tokens, space-separated, drawn from the closed vocabulary `nospace`, `noprefix`, `keeporder`. Always present as the first line; empty tokens string when there are no directives (`D\t`).
- **Message Line (`M`, optional)** — `M\t<message>`: present exactly when a message exists. bash has no message widget; to uphold the hard invariant of zero terminal leakage (no unmanaged stdout/stderr pollution), the snippet does not print `M` lines during interactive completion (they are consumed internally by test harnesses or discarded during normal readline tab-completion).
- **Candidate Lines (`C`)** — `C\t<candidate>`: one line per candidate:
  - Normal mode: the value. `wordbreakPrefix` (e.g. `--flag=` from `COMP_WORDBREAKS`) is stripped before quoting. Values containing whitespace or `COMP_WORDBREAKS` characters are POSIX double-quoted (escaping `"`, `$`, `` ` ``, `\`) or single-quoted (`'…'`); values starting with `~/` use backslash escaping for metacharacters rather than single quotes so that Readline/Bash preserves tilde expansion.
  - Successive-tab mode (`COMP_TYPE=63`): `<display>\t<value>\t<description>` (tab-delimited, unquoted fields; description may be empty). The snippet renders `display (description)` for Readline's listing and uses the second field for insertion.
- Newlines and tabs inside values/displays/descriptions are normalized to a single space (line-oriented framing cannot carry raw newlines/tabs).
- The snippet must not write to stdout/stderr beyond `COMPREPLY` assignment (zero terminal leakage is a hard invariant).

### 7.3 `zsh` — record stream for `_describe`

Line-based records with ASCII Unit Separator `\x1f` (US, byte 31) as the field delimiter. `\x1f` cannot occur in shell words or help text; newlines/tabs/carriage returns in fields are normalized to a single space (a stray US is normalized defensively). The snippet reads lines with `${(f)}` and splits fields on `\x1f`.

```
D\x1f<tok[,tok...]>\x1f<usage>
[L\x1f<list-colors-spec>]
[M\x1f<message>]
[C\x1f<tag>\x1f<display>\x1f<description>\x1f<value>\x1f<sgr>]
[C\x1f<tag>\x1f<display>\x1f<description>\x1f<value>\x1f<sgr>]
...
```

- `D` (directives) — always the first line. `<tok[,tok...]>` is a comma-joined subset of the directive vocabulary (empty when none); `<usage>` is the raw usage string (may be empty; metadata only — the user-visible usage reaches the shell through the message stream). The `noprefix` token is implied when the word being completed is empty and candidates exist: bare-space completion must not auto-insert into the buffer, while completing a typed prefix inserts normally.
- `L` (list-colors) — at most one line, directly after `D`, present when color is enabled (`CONGA_COLOR`/`NO_COLOR`/`CLICOLOR=0`): a zsh `list-colors` specification of colon-joined entries — one two-tone entry `=(#b)(<display>)([ ]## <sep> *)=0=<valueSGR>=<descSGR>` and one display-only entry `=(#b)(<display>)=0=<valueSGR>` per candidate (empty `<valueSGR>` for unstyled candidates pins their rows against user-configured list-colors defaults), closed by the description-column catch-all `=(#b)(*)([ ]## <sep> *)=0==<descSGR>`. `<sep>` is the separator zsh renders between a match and its description — the user's `list-separator` style forwarded as `CONGA_ZSH_LIST_SEPARATOR` (default `--`) — glob-escaped and surrounded by single spaces. Display strings are glob-escaped (`\[]*?()|^~#<>:`); `<valueSGR>` is the candidate's SGR (falling back to the theme's value style), `<descSGR>` the theme's description style. Candidate entry pairs are capped by `CONGA_ZSH_STYLE_LIMIT` (a negative limit is unlimited). Forwarded to `zstyle ":completion:${curcontext}:*" list-colors "${L_spec}"`.
- `M` (message) — zero or more lines; the snippet forwards each to `_message -r "<message>"`. The driver folds the usage string into the message stream (deduplicated against the engine's messages, which already carry it on `DirectiveError`); messages are wrapped in the theme's usage style (dim) closed by the default-foreground reset when color is enabled, and plain otherwise.
- `C` (candidate) — one line per candidate, stably grouped by tag in order of first appearance (untagged candidates last), in engine order within a group:
  - `tag` (may be empty)
  - `display` (raw display string)
  - `description` (may be empty; never clamped — zsh manages its own list truncation)
  - `value` (the value to insert into the buffer): framing characters sanitized, zsh metacharacters backslash-escaped — the set `"`, `'`, `\`, `$`, backtick, `|`, `&`, `;`, `<`, `>`, `(`, `)`, `[`, `]`, `{`, `}`, `*`, `?`, `!`, `^`, `#`, `:`, and space — while `=`, `/`, and a leading `~` stay verbatim so attached values, directory candidates, and tilde expansion survive; a trailing space is embedded unless the `nospace` directive is set or the value ends with `=` or `/`. Colon is escaped because `_describe` treats it as the value/description separator.
  - `sgr` (ECMA-48 parameter numbers of the candidate's style; empty when unstyled or color is disabled — informational, list coloring flows through the `L` zstyle)
- Snippet mapping: `noprefix` in the `D` tokens sets `compstate[insert]=''` (values are complete replacements); the `L` spec is forwarded verbatim **unless the user configured a non-empty `list-colors`**; `zstyle ":completion:${curcontext}:*" group-name ''` is set **only when the user has not configured a non-empty `group-name`**; `M` lines go to `_message -r`. Probe-before-set hygiene: the snippet first deletes only its own prior definitions (`zstyle -d` matches the pattern literally, never user styles) and then inspects the user's styles for the current context (`zstyle -a ":completion:${curcontext}:"`). An empty style value (for `list-colors`, Oh-My-Zsh's default) counts as "not configured", so engine defaults still apply. Suppression sidebands: the snippet reads the `verbose` and `show-aliases` zstyles (`zstyle -T`, true when unset; `show-aliases` is the go-task convention) and passes `CONGA_COMPLETION_DESCRIPTIONS=0` / `CONGA_COMPLETION_ALIASES=0` inline to the `__complete` invocation only, never exporting them into the user's shell. The `list-separator` style is read the same way (`zstyle -s`) and forwarded as `CONGA_ZSH_LIST_SEPARATOR` so the patterns match the rendered separator. Candidates are grouped by non-empty `tag` into one `_describe -t <tag> "<tag>" displaysArr valuesArr -Q -S ''` call per tag (any candidate count); elements of `displaysArr` are formatted `<display>:<description>` when the description is non-empty, with embedded colons escaped as `\:`; untagged candidates fall through to `compadd -Q -S '' -a valuesArr`. The words before the one being completed are passed `${(Q)}`-unquoted and the last word raw, so no CLI-side quote re-lexing happens (§3).
- Flag tag merging (§4.2) happens driver-side before framing: with merging active (default-on for zsh), `shorthand flags` and `longhand flags` candidates are re-tagged `flags`.

### 7.4 `fish` — native `complete` feed

```
<value>\t<description>
<value>\t<description>
...
```

- One line per candidate: value, tab, description. When description is empty, the trailing tab is omitted (`<value>\n`). No directives header, no messages (fish has no channel for either; `nospace` is ignored).
- Description clamping: only the first line of a multi-line description is kept, trimmed; truncated to `CONGA_DESCRIPTION_LENGTH` runes with `...` appended when the limit is exceeded (hard cut without ellipsis when the limit is ≤ 3).
- Tabs and newlines inside both values and descriptions are normalized to a single space to protect fish's tab-separated record parser.

### 7.5 `powershell` — JSON

Identical schema to `export` (§7.1) plus the per-candidate `sgr` field. The fields carry the final presentation content for the snippet's `[System.Management.Automation.CompletionResult]::new(...)` mapping: `value` is the completion text, `display` the list-item text, and `description` the tooltip text.

- **Insertion values.** Newlines, tabs, and carriage returns are removed; values containing whitespace or PowerShell metacharacters (`` ` {}()[]*$?\"|<>&(),;# ' ``) are quoted in single quotes with embedded single quotes doubled (`''`); and a trailing space is appended unless the `nospace` directive is set or the value ends with a delimiter (`/`, `=`). Empty-value candidates are dropped entirely (no insertion text).
- **List-item text.** Normal mode renders `display (description)` — the description clamped per `CONGA_DESCRIPTION_LENGTH` —; tooltip mode (`CONGA_TOOLTIP`) renders the bare display and moves the description into the tooltip.
- **Tooltip text.** Tooltip mode carries the clamped description; normal mode carries a single space placeholder.
- **Styling.** When color is enabled, styled segments are wrapped in backtick-escaped SGR sequences (`` `e[<params>m<text> ``): the display segment uses the candidate's style tokens (falling back to the theme's default value style), the description segment the theme's default description style; a rendering that emitted any escape sequence ends with the full reset (`` `e[0m ``). Color-disabled output (`CONGA_COLOR=0`, `NO_COLOR`, `CLICOLOR=0`) is plain text.
- **`sgr` field.** The ECMA-48 SGR parameter numbers of the candidate's raw style tokens (e.g. `"1;31"`), present when the style is non-empty and color is enabled.
- **Empty-string fallback.** List-item and tooltip text fall back to a single space: PowerShell's `CompletionResult` requires non-empty strings, and a completion with an empty `ToolTip` is silently discarded along with its candidates.
- **Snippet contract.** The completer collects the cursor-truncated `CommandElements` (raw `Extent.Text` with surrounding quotes stripped, empty tokens restored as `""`), drops the program name, appends the empty word when `$wordToComplete` is empty, and invokes `& <target> __complete powershell <words>` with stderr suppressed. The single-command form registers the completer function for the command name; the multi-command form assigns one scriptblock — invoking the binary under the name the user typed with a `.exe` suffix stripped — shared by one `Register-ArgumentCompleter -Native` line per name and alias (the `.exe` variant active on Windows, commented elsewhere). The payload is piped through `ConvertFrom-Json`, the `candidates` array mapped to `CompletionResult` objects (literal `` `e[ `` sequences resolved to real escapes), and an empty or failed invocation returns `""` so PowerShell's default file completion is suppressed.

### 7.6 `elvish` — JSON for `edit:complex-candidate`

Single-line JSON object, no trailing newline:

```json
{"usage":"","messages":[],"descriptionStyle":"dim","candidates":[{"value":"serve","display":"serve","description":"start web server","codeSuffix":" ","style":"blue"}]}
```

- All four top-level fields are always present, and every candidate carries all five fields (`value`, `display`, `description`, `codeSuffix`, `style`) — no omission. Array order is engine order.
- `descriptionStyle` is the theme's default description style in elvish vocabulary (`dim` by default); the snippet styles the ` (description)` suffix with it.
- **Candidate style.** The style token string in elvish `styled` vocabulary. Elvish (0.21) accepts named colors, `bright-*` variants, `bg-*`/`bg-bright-*` variants, the attributes `bold dim italic underlined blink inverse`, `default`, `fg-default`/`bg-default`, and the indexed palette `color0`–`color255` — but not `#rrggbb` or `rgb(r,g,b)`. The driver therefore normalizes truecolor tokens (`#rrggbb`, `rgb(r,g,b)`) to the nearest indexed `colorN` and passes every other token through verbatim. An empty candidate style renders as `default`.
- **`codeSuffix`.** `" "` normally; `""` when the nospace directive applies to the value (§5, including the implied trailing-`=` rule).
- **Sanitization.** Newlines, carriage returns, and tabs are removed from value, display, and description (elvish JSON string literals cannot carry them unescaped and the completion UI cannot render them). Descriptions are clamped per `CONGA_DESCRIPTION_LENGTH` (`TrimmedDescription`).
- **Usage rule.** `usage` is cleared to `""` when any candidates exist; it is only sent for the zero-candidate error case, where `messages` carries the usage string (or the literal `ERROR` when usage is empty).
- **Snippet contract.** Registers one shared lambda per command name and alias via `set edit:completion:arg-completer[<name>] = $completer~` (names quoted when they contain metacharacters). The lambda sets `CONGA_SHELL=elvish`, invokes `<target> __complete elvish <words...>` — the arg-completer's `@args` minus the command name, passed via argv only (no stdin piping) — and `from-json`s the payload. To enforce terminal display purity, usage and diagnostic messages are omitted during completion, and each candidate renders as `edit:complex-candidate <value> &display=(styled <display> <style>)<styled " (description)" descriptionStyle> &code-suffix=<codeSuffix>`. An empty candidate style renders as `default`.

### 7.7 `nushell` — JSON records for the external completer

Bare JSON array of records, no trailing newline (nushell's `from json` produces the list the external completer returns):

```json
[{"value":"commit ","display":"commit","description":"record changes","style":{"fg":"#008000"}}]
```

- **No directives channel.** Nushell's external-completer contract has no directive, message, or usage channel: nospace is expressed by the trailing space baked into `value`; `noprefix`/`keeporder` have no effect; messages and usage are silently dropped as the host completer lacks a presentation channel for them.
- **Insertion values.** The record's `value` is inserted into the command line verbatim, so it is pre-quoted by the driver for direct insertion: newlines and carriage returns are removed (tabs are kept); values containing any of `` { } ( ) [ ] < > $ & " ' | ; # \ ` `` or a space are wrapped in double quotes with `\` and `"` backslash-escaped; a leading `~` stays outside the quotes (`~"my dir"`). A trailing space is appended after the closing quote unless the nospace directive applies (§5, including the implied trailing-`=` rule).
- **List item and description.** `display` is the raw list-item text. `description` is omitted when empty; newlines and carriage returns are stripped first (flattening multi-line descriptions — unlike fish's and elvish's first-line cut), and the flattened text is clamped per `CONGA_DESCRIPTION_LENGTH` with `...`.
- **`style` field.** Optional object per nushell's `ansi --escape` record format — `{"fg": ..., "bg": ..., "attr": ...}` — omitted entirely for unstyled candidates. `fg`/`bg` are nushell color names or hex `#rrggbb`; `attr` is a string of single-letter attributes: `b` bold, `d` dimmed, `i` italic, `u` underline, `r` reverse, `l` blink (concatenated, e.g. `ub`).
- **Color conversion.** CONGA tokens map to nushell's vocabulary: named colors pass through; `bright-*` foregrounds map to nushell's `light_*` names (`bright-red` → `light_red`, `bright-black` → `dark_gray`, `bright-white` → `white`); indexed colors `color0`–`color255` convert to hex via the xterm 256-color palette (16 base colors, the 6×6×6 cube at levels `0x00 0x5f 0x87 0xaf 0xd7 0xff`, the 24-step grayscale ramp `8+10n`); truecolor tokens (`#rrggbb`, `rgb(r,g,b)`) normalize to hex. Attribute tokens map to the single letters above.
- **Snippet contract.** Defines `let <name>_completer = {|spans| ... }` with the `let` identifier sanitized to `[A-Za-z0-9_]` (every other rune → `_`, empty → `_`) and the target quoted with `^"..."` external-command quoting when it contains metacharacters. The closure sets `CONGA_SHELL=nushell`, drops the command name (the first span — §3), invokes `<target> __complete nushell <remaining spans>` and pipes through `from json`. Registration is global: nushell's `completions.external.completer` is one closure shared by all external commands, so the snippet merges defensively — `mut current = (($env | default {} config).config | default {} completions)`, `$current.completions = ($current.completions | default {} external)`, then `$current.completions.external = ($current.completions.external | default true enable | upsert completer { if $in == null { $<name>_completer } else { $in } })` (the `upsert` guard chains a user's existing completer and works around nushell issue #15654), and finally `$env.config = $current`. Enabling requires sourcing a real file (nushell cannot pipe-eval): the enable instructions save the snippet to a file and `source` it.

## 8. Behavioral invariants

Each driver must satisfy specific behavioral requirements across target environments (verified by the test corpus and real-shell integration suites):

1. **bash** — redirect rejection (`echo test >[TAB]` yields no file candidates); `COMP_WORDBREAKS`-aware prefix trimming (a `--flag=` prefix is not re-inserted); common-prefix collapsing on multi-candidate insert; successive-tab description listing; `compopt nospace`; zero stdout leakage.
2. **zsh** — no unwanted character insertion into the buffer; flag color zstyle; tag-grouped menus; flag tag merging default-on; ActiveHelp messages; framework compatibility (Oh-My-Zsh, Prezto, fzf-tab).
3. **fish** — tab-separated value/description with description clamping; no trailing-space anomalies.
4. **powershell** — `Register-ArgumentCompleter -Native` registration (including `.exe` handling on Windows); tooltip mode; color-disable mode; suppression of the default file completion.
5. **elvish** — terminal display purity (no unmanaged stdout/notify pollution or prompt displacement); styled complex-candidate rendering with description suffix; `codeSuffix`-based nospace; no patching (elvish tokenizes itself and does its own prefix filtering).
6. **nushell** — insertion-ready pre-quoted values (metacharacter quoting, `~`-outside-quotes rule); trailing-space-in-value nospace; defensive global external-completer registration that preserves a user's existing completer; dequoting `Patch` seam for raw spans.
7. **Engine** — flat-shell bare-space flag suppression; tag vocabulary (`commands`, `longhand flags`, `shorthand flags`, `values`, `arguments`); negatable `--no-*` candidates; map key dedup; deprecated/hidden filtering; single-command `completion` gating; alias and description suppression through the `CONGA_COMPLETION_ALIASES`/`CONGA_COMPLETION_DESCRIPTIONS` sidebands applied once before framing.

## 9. Conformance corpus

`internal/completion/testdata/corpus/` stores JSON fixtures captured through the live stack at the `predict.Result` level. Each fixture records the input words, the shell-independent engine result (items with value/display/description/style/tag, directive bits, usage), and serves as the oracle for driver conformance tests: feeding the recorded result into a driver's formatter must yield output equivalent to the recorded items modulo the framing of §7.

`internal/completion/testdata/snapshots/{elvish,nushell}/` additionally stores formatter-level fixtures for the elvish and nushell framings (§7.6, §7.7). Each fixture records the wire-shaped input and the expected output; the elvish/nushell conformance tests assert byte equivalence against them, modulo the documented framing rules — the §5 rule that a trailing `=` implies nospace (a space after `=`-suffixed values is not inserted), and the §7.6 truecolor normalization (`#rrggbb`/`rgb()` tokens are normalized to the nearest indexed `colorN`, which elvish's styling parser otherwise rejects).
