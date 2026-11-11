---
title: package decoder
description: Opt-in wire-format decoders — json, yaml, toml, hcl.
---

Each subpackage exposes a single function matching `conga.ConfigDecoder`:

```go
func Decode(data []byte) (map[string]any, error)
```

| Package | Import | Parse sentinel |
|---------|--------|----------------|
| JSON | `github.com/conga-sh/conga/decoder/json` | `json.ErrParse` |
| YAML | `github.com/conga-sh/conga/decoder/yaml` | `yaml.ErrParse` |
| TOML | `github.com/conga-sh/conga/decoder/toml` | `toml.ErrParse` |
| HCL | `github.com/conga-sh/conga/decoder/hcl` | `hcl.ErrParse` |

## Registration

```go
import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/yaml"
)

app.RegisterDecoder("yaml", yaml.Decode)
```

`RegisterDecoder` normalizes extension strings by trimming whitespace, converting to lowercase, and stripping leading `*` and `.` characters, so `"json"`, `".json"`, and `"*.JSON"` are identical. `.yaml` and `.yml` are distinct extensions, so register both to accept both filename conventions.

## Value types

The configuration engine turns each decoded value into text and parses that text with the flag's type, as it parses a command-line value. A decoded value therefore binds to a flag when the same text would bind on the command line.

| Value in the file | JSON | YAML | TOML | HCL |
|---|---|---|---|---|
| Integer | `int64`, or `uint64` above the `int64` range | `int`, or `uint64` above the `int64` range | `int64` | `int64`, or `uint64` above the `int64` range |
| Number with a fraction or exponent | `float64` | `float64` | `float64` | `float64` when it has a fraction |
| Datetime with an offset | string | `time.Time` (unquoted timestamp) | `time.Time` | string |
| Local datetime, local date, local time | string | string | the text as written | string |

A `time.Time` value reaches the flag in RFC 3339 form, so `when = 1979-05-27T07:32:00Z` binds to a `time.Time` flag and gives a string flag `1979-05-27T07:32:00Z`. A TOML local datetime or local date carries no offset. A string flag receives it as written, and a `time.Time` flag rejects it, as it rejects `--when 1979-05-27T07:32:00` on the command line:

```text no-copy
invalid time "1979-05-27T07:32:00" (expected RFC 3339, e.g. 2006-01-02T15:04:05Z) for flag --when (from config key "when")
```

An integer that fits neither `int64` nor `uint64` fails to load the file with the decoder's `ErrParse`, wrapped in `ErrConfig` (exit 1). TOML rejects every integer outside the `int64` range. JSON and HCL also reject a number that overflows `float64`. HCL stores a number's value without its spelling, so it rejects `1e30` as it rejects a 30-digit integer. YAML checks unquoted values only, so a quoted `'999999999999999999999999999999'` stays a string:

```text no-copy
cannot parse config file "config.yaml": yaml parse error: line 1: integer 999999999999999999999999999999 out of range
```

## HCL blocks

An unlabeled block decodes as a nested section keyed by its block type, so `server { port = 8080 }` matches the config key `server.port`. Three block forms fail to load the file with `hcl.ErrParse`:

| HCL input | Error |
|---|---|
| A labeled block, `server "web" { port = 1 }` | `block "server" has labels ["web"]; labeled blocks are not supported, use an unlabeled block` |
| A second `server { }` block | `duplicate block "server", first declared at config.hcl:1,1-7` |
| An attribute `server = 1` and a block `server { }` | `block "server" conflicts with the attribute of the same name at config.hcl:1,1-11` |

Each message starts with the position of the offending block, such as `config.hcl:5,1-7`.

## Missing decoders

A named configuration target (an explicit `--config-file`, an environment variable, or `conga.ConfigExplicit(...)`) requires a registered decoder for its file extension. Resolving a target without a matching decoder returns a fatal `ErrConfig` error identifying the missing decoder or unsupported format. The config-file flag's help text appends `[decoders: .json, .yaml]` listing the registered extensions in sorted order, or `(no configuration decoder registered)` when no decoder is registered, and environment targets still appear in `session.CandidateConfigFiles()`.

## Isolation guarantee

Decoder packages import their wire-format library (e.g. `gopkg.in/yaml.v3`, `github.com/pelletier/go-toml/v2`), keeping those dependencies out of core CONGA packages. Only the decoders an application registers are linked into its binary.
