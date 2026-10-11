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

`RegisterDecoder` normalizes extension strings by trimming whitespace, converting to lowercase, and stripping leading `*` and `.` characters, so `"json"`, `".json"`, and `"*.JSON"` are identical. `.yaml` and `.yml` are distinct extensions; register both to accept both filename conventions.

## Missing decoders

A named configuration target (an explicit `--config-file`, an environment variable, or `conga.ConfigExplicit(...)`) requires a registered decoder for its file extension. Resolving a target without a matching decoder returns a fatal `ErrConfig` error identifying the missing decoder or unsupported format. The config-file flag's help text appends `(no configuration decoder registered)`, and environment targets still appear in `session.CandidateConfigFiles()`.

## Isolation guarantee

Decoder packages import their wire-format library (e.g. `gopkg.in/yaml.v3`, `github.com/pelletier/go-toml/v2`), keeping those dependencies out of core CONGA packages. Only the decoders an application registers are linked into its binary.
