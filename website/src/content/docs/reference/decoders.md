---
title: package decoder
description: Opt-in wire-format decoders — json, yaml, toml, hcl.
---

Each subpackage exposes a single function matching `config.Decoder`:

```go
func RawDecoder(data []byte) (map[string]any, error)
```

| Package | Import | Parse sentinel |
|---------|--------|----------------|
| JSON | `github.com/conga-sh/conga/decoder/json` | `json.ErrJSONParse` |
| YAML | `github.com/conga-sh/conga/decoder/yaml` | `yaml.ErrYAMLParse` |
| TOML | `github.com/conga-sh/conga/decoder/toml` | `toml.ErrTOMLParse` |
| HCL | `github.com/conga-sh/conga/decoder/hcl` | `hcl.ErrHCLParse` |

## Registration

```go
import "github.com/conga-sh/conga/config"

app.RegisterDecoder("yaml", yaml.RawDecoder)
```

`RegisterDecoder` normalizes extension strings by trimming whitespace, converting to lowercase, and stripping leading `*` and `.` characters, so `"json"`, `".json"`, and `"*.JSON"` are identical. `.yaml` and `.yml` are distinct extensions; register both to accept both filename conventions.

## Missing decoders

A named configuration target (an explicit `--config-file`, an environment variable, or `config.Explicit(...)`) must have a registered decoder for its format: resolving one without a decoder is a fatal `ErrConfig` error naming the missing decoder or the unsupported format, never a silent no-op. The config-file flag's help text appends `(no configuration decoder registered)`, and environment targets still appear in `session.CandidateConfigFiles()`.

## Isolation guarantee

Decoder packages import their wire-format library (e.g. `gopkg.in/yaml.v3`, `github.com/pelletier/go-toml/v2`), keeping those dependencies out of core CONGA packages. Only the decoders an application registers are linked into its binary. See [About binary size and linking](/explanation/binary-size/) for the linker-friendly dependency policy.
