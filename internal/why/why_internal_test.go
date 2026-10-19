package why

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestRenderFeatureRows_AlignedColumns(t *testing.T) {
	t.Parallel()

	rows := []FeatureRow{
		{Name: "gentle-force", Value: "true", Source: "cli"},
		{Name: "x", Value: "0", Source: "default"},
	}

	output := renderFeatureRows(rows, newRenderContext(Report{}))

	assert.Equal(t, "  FEATURE        VALUE   SOURCE\n"+
		"  gentle-force   true    cli\n"+
		"  x              0       default\n", output)
}

func TestRenderRows_MapEntriesWidenValueColumn(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{
			Label:      "--secrets",
			Value:      Redacted,
			Source:     "cli",
			Env:        "-",
			ConfigPath: "secrets",
			Entries:    []MapEntry{{Key: "token", Value: Redacted, Source: "cli"}},
		},
	}

	output := renderRows(rows, newRenderContext(Report{}))

	assert.Equal(t, "\n  FLAG        VALUE     SOURCE   ENV   CONFIG PATH\n"+
		"  --secrets   (1 key)   cli      -     secrets\n"+
		"              └─ token=██████   cli\n", output)
}

func TestRenderRows_SourceDetailRendered(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Label: "--port", Value: "9000", Source: "config", Detail: "app.json", Env: "-", ConfigPath: "port"},
	}

	output := renderRows(rows, newRenderContext(Report{}))

	assert.Equal(t, "\n  FLAG     VALUE   SOURCE              ENV   CONFIG PATH\n"+
		"  --port   9000    config (app.json)   -     port\n", output)
}

func TestRenderRows_SubRowsRendered(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{
			Label:      "--config-files",
			Value:      "(2 files)",
			Source:     "cli",
			Env:        "-",
			ConfigPath: "-",
			SubRows: []SubRow{
				{Value: "a.yaml", Source: "cli"},
				{Value: "b.yaml", Source: "cli"},
			},
		},
	}

	output := renderRows(rows, newRenderContext(Report{}))

	assert.Equal(t, "\n  FLAG             VALUE       SOURCE   ENV   CONFIG PATH\n"+
		"  --config-files   (2 files)   cli      -     -\n"+
		"                   ├─ a.yaml   cli\n"+
		"                   └─ b.yaml   cli\n", output)
}

func TestRenderRows_SubRowsUseParentValueCell(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{
			Label:      "--tags",
			Value:      "(2 values)",
			Source:     "cli",
			Env:        "-",
			ConfigPath: "tags",
			SubRows: []SubRow{
				{Value: "go"},
				{Value: "cli"},
			},
		},
	}

	output := renderRows(rows, newRenderContext(Report{}))

	assert.Equal(t, "\n  FLAG     VALUE        SOURCE   ENV   CONFIG PATH\n"+
		"  --tags   (2 values)   cli      -     tags\n"+
		"           ├─ go\n"+
		"           └─ cli\n", output)
}

func TestReport_Format_ShortensUniqueOriginBasename(t *testing.T) {
	t.Parallel()

	report := Report{
		Rows: []Row{
			{Label: "--port", Value: "9000", Source: "config", Detail: "/srv/app/config.yaml", Env: "-", ConfigPath: "port"},
		},
		ConfigPaths: []ConfigPath{
			{Path: "/srv/app/config.yaml", Status: "loaded"},
		},
	}

	output := report.Format()

	assert.Contains(t, output, "config (config.yaml)")
	assert.NotContains(t, output, "config (/srv/app/config.yaml)")
}

func TestReport_Format_DisambiguatesCollidingBasenames(t *testing.T) {
	t.Parallel()

	report := Report{
		Rows: []Row{
			{Label: "--host", Value: "a", Source: "config", Detail: "/srv/prod/env/config.yaml", Env: "-", ConfigPath: "host"},
			{Label: "--port", Value: "1", Source: "config", Detail: "/srv/dev/env/config.yaml", Env: "-", ConfigPath: "port"},
		},
		ConfigPaths: []ConfigPath{
			{Path: "/srv/prod/env/config.yaml", Status: "loaded"},
			{Path: "/srv/dev/env/config.yaml", Status: "loaded"},
		},
	}

	output := report.Format()

	assert.Contains(t, output, "config (/srv/prod/.../config.yaml)")
	assert.Contains(t, output, "config (/srv/dev/.../config.yaml)")
}

func TestReport_Format_PreservesOriginScopeAnnotation(t *testing.T) {
	t.Parallel()

	report := Report{
		Rows: []Row{
			{Label: "--port", Value: "9000", Source: "config", Detail: "/srv/app/config.yaml", Scope: "profile dev", Env: "-", ConfigPath: "port"},
			{Label: "--host", Value: "localhost", Source: "config", Detail: "/srv/app/config.yaml", Scope: "profile prod", Env: "-", ConfigPath: "host"},
		},
		ConfigPaths: []ConfigPath{
			{Path: "/srv/app/config.yaml", Status: "loaded"},
		},
	}

	output := report.Format()

	assert.Contains(t, output, "config (config.yaml › profile dev)")
	assert.Contains(t, output, "config (config.yaml › profile prod)")
}

func TestReport_Format_ConfigPathOmitsDuplicateLoadedPath(t *testing.T) {
	t.Parallel()

	report := Report{
		ConfigPaths: []ConfigPath{
			{Path: "/srv/app/config.yaml", Status: "loaded"},
			{Path: "/srv/app", Status: "loaded", Loaded: "/srv/app/config.yaml"},
		},
	}

	output := report.Format()

	assert.Contains(t, output, "  /srv/app/config.yaml   loaded\n")
	assert.Contains(t, output, "  /srv/app               loaded: /srv/app/config.yaml")
}

func TestRenderRows_ColorColumnsAligned(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Label: "--port", Value: "8080", Source: "cli", Env: "PORT", ConfigPath: "port"},
		{Label: "--host", Value: "pending", Source: "unset", Env: "-", ConfigPath: "host"},
	}

	output := renderRows(rows, newRenderContext(Report{Color: true}))

	assert.Equal(t, "\n  FLAG     VALUE     SOURCE   ENV    CONFIG PATH\n"+
		"  --port   8080      ● cli    PORT   port\n"+
		"  --host   pending   unset    -      host\n", stripANSI(output))
}

func TestIsColorEnabled_EmptyNoColorIgnored(t *testing.T) {
	t.Setenv("CONGA_COLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TERM", "xterm")

	assert.True(t, IsColorEnabled(openCharDevice(t)))
}

func TestIsColorEnabled_NoColorDisables(t *testing.T) {
	t.Setenv("CONGA_COLOR", "")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR", "")
	t.Setenv("TERM", "xterm")

	assert.False(t, IsColorEnabled(openCharDevice(t)))
}

func openCharDevice(t *testing.T) *os.File {
	t.Helper()

	null, err := os.Open(os.DevNull)
	assert.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, null.Close()) })

	info, err := null.Stat()
	assert.NoError(t, err)

	if info.Mode()&os.ModeCharDevice == 0 {
		t.Skip("os.DevNull is not a character device")
	}

	return null
}

func stripANSI(text string) string {
	for _, escape := range []string{ansiReset, ansiBold, ansiDim, ansiGreen, ansiYellow, ansiCyan, ansiDarkGray} {
		text = strings.ReplaceAll(text, escape, "")
	}

	return text
}

func TestReport_FormatJSON(t *testing.T) {
	t.Parallel()

	report := Report{
		CommandPath: "cloudctl deploy worker",
		Rows: []Row{
			{
				Label:      "--port",
				Value:      "8080",
				Source:     "cli",
				Detail:     "",
				Env:        "PORT",
				ConfigPath: "server.port",
				Hidden:     false,
			},
			{
				Label:      "--headers",
				Value:      Redacted,
				Source:     "config",
				Detail:     "app.yaml",
				Env:        "-",
				ConfigPath: "headers",
				Hidden:     false,
				Entries: []MapEntry{
					{
						Key:    "timeout",
						Value:  "30s",
						Source: "config",
						Detail: "app.yaml",
					},
				},
			},
		},
		GlobalRows: []Row{
			{
				Label:      "--verbose",
				Value:      "true",
				Source:     "default",
				Detail:     "",
				Env:        "-",
				ConfigPath: "-",
				Hidden:     false,
			},
		},
		Features: []FeatureRow{
			{
				Name:   "streaming",
				Value:  "true",
				Source: "env",
				Detail: "FEATURE_STREAMING",
			},
		},
		ConfigDecoders: []string{".json", ".yaml"},
		ConfigPaths: []ConfigPath{
			{
				Path:        "/etc/app.yaml",
				Status:      "loaded",
				ProbedFiles: nil,
			},
			{
				Path:        "~/.config/app",
				Status:      "no config file",
				ProbedFiles: []string{"~/.config/app/config.json"},
			},
		},
		ConfigExamples: []string{"~/.config/app/config.json"},
		RedactedRows:   true,
	}

	jsonOutput := report.FormatJSON()

	expected := `{"$schema":"https://conga.sh/schemas/why/v1.json","schemaVersion":1,"command":"cloudctl deploy worker","redacted":true,"flags":[{"name":"port","value":"8080","source":"cli","env":"PORT","configPath":"server.port"},{"name":"headers","source":"config","detail":"app.yaml","configPath":"headers","entries":[{"key":"timeout","value":"30s","source":"config","detail":"app.yaml"}]}],"globalFlags":[{"name":"verbose","value":"true","source":"default"}],"features":[{"name":"streaming","value":"true","source":"env","detail":"FEATURE_STREAMING"}],"config":{"decoders":[".json",".yaml"],"paths":[{"path":"/etc/app.yaml","status":"loaded"},{"path":"~/.config/app","status":"no_config_file","probedFiles":["~/.config/app/config.json"]}],"examples":["~/.config/app/config.json"]}}`

	assert.Equal(t, expected, jsonOutput)
}

func TestReport_FormatJSON_StructuredTypesAndRedaction(t *testing.T) {
	t.Parallel()

	report := Report{
		CommandPath:      "flowctl",
		Version:          "1.0.0",
		Timestamp:        "2026-10-18T00:00:00Z",
		WorkingDirectory: "/workspace/flow",
		RedactedRows:     true,
		Rows: []Row{
			{
				Name:     "file",
				Type:     "string[]",
				RawValue: []string{"/workspace/flow/pipeline.yaml", "/workspace/flow/pipeline.local.yaml"},
				Source:   "cli",
				Env:      "FLOWCTL_FILE",
			},
			{
				Name:       "parallelism",
				Type:       "int",
				RawValue:   2,
				Source:     "config",
				Detail:     "/workspace/flow/pipeline.local.yaml",
				Env:        "FLOWCTL_PARALLELISM",
				ConfigPath: "parallelism",
			},
			{
				Name:       "log-level",
				Type:       "string",
				RawValue:   "debug",
				Source:     "config",
				Detail:     "/workspace/flow/pipeline.local.yaml",
				Env:        "FLOWCTL_LOG_LEVEL",
				ConfigPath: "log-level",
			},
			{
				Name:       "env",
				Type:       "map",
				Source:     "config",
				Env:        "FLOWCTL_ENV",
				ConfigPath: "env",
				Redacted:   true,
				Entries: []MapEntry{
					{
						Key:      "DEPLOY_REGION",
						Redacted: true,
						Source:   "config",
						Detail:   "/workspace/flow/pipeline.local.yaml",
					},
					{
						Key:      "FLOW_ENV",
						RawValue: "ci",
						Value:    "ci",
						Source:   "config",
						Detail:   "/workspace/flow/pipeline.local.yaml",
					},
				},
			},
		},
		ConfigPaths: []ConfigPath{
			{
				Path:   "/workspace/flow/pipeline.yaml",
				Status: "loaded",
			},
			{
				Path:   "/workspace/flow/pipeline.local.yaml",
				Status: "loaded",
			},
		},
	}

	jsonOutput := report.FormatJSON()

	expected := `{"$schema":"https://conga.sh/schemas/why/v1.json","schemaVersion":1,"command":"flowctl","version":"1.0.0","timestamp":"2026-10-18T00:00:00Z","workingDirectory":"/workspace/flow","redacted":true,"flags":[{"name":"file","type":"string[]","value":["/workspace/flow/pipeline.yaml","/workspace/flow/pipeline.local.yaml"],"source":"cli","env":"FLOWCTL_FILE"},{"name":"parallelism","type":"int","value":2,"source":"config","detail":"/workspace/flow/pipeline.local.yaml","env":"FLOWCTL_PARALLELISM","configPath":"parallelism"},{"name":"log-level","type":"string","value":"debug","source":"config","detail":"/workspace/flow/pipeline.local.yaml","env":"FLOWCTL_LOG_LEVEL","configPath":"log-level"},{"name":"env","type":"map","redacted":true,"source":"config","env":"FLOWCTL_ENV","configPath":"env","entries":[{"key":"DEPLOY_REGION","redacted":true,"value":null,"source":"config","detail":"/workspace/flow/pipeline.local.yaml"},{"key":"FLOW_ENV","value":"ci","source":"config","detail":"/workspace/flow/pipeline.local.yaml"}]}],"config":{"decoders":[],"paths":[{"path":"/workspace/flow/pipeline.yaml","status":"loaded"},{"path":"/workspace/flow/pipeline.local.yaml","status":"loaded"}]}}`

	assert.Equal(t, expected, jsonOutput)
}

func TestReport_Format_ScopeDoesNotLeakIntoOriginShortening(t *testing.T) {
	t.Parallel()

	report := Report{
		Rows: []Row{
			{
				Label:      "--port",
				Value:      "9000",
				Source:     "config",
				Detail:     "/srv/prod/pipelines/pipeline.yaml",
				Scope:      "jobs/deploy",
				Env:        "-",
				ConfigPath: "port",
			},
		},
		ConfigPaths: []ConfigPath{
			{Path: "/srv/prod/pipelines/pipeline.yaml", Status: "loaded"},
		},
	}

	output := report.Format()

	assert.Contains(t, output, "config (pipeline.yaml › jobs/deploy)")
	assert.NotContains(t, output, "config (deploy)")
}

func TestReport_FormatJSON_JoinsOriginAndScopeInDetail(t *testing.T) {
	t.Parallel()

	report := Report{
		CommandPath: "app",
		Rows: []Row{
			{
				Name:       "port",
				Label:      "--port",
				Value:      "9000",
				Source:     "config",
				Detail:     "pipeline.yaml",
				Scope:      "jobs/deploy",
				Env:        "-",
				ConfigPath: "port",
			},
			{
				Name:       "headers",
				Label:      "--headers",
				Source:     "config",
				Detail:     "pipeline.yaml",
				Env:        "-",
				ConfigPath: "headers",
				Entries: []MapEntry{
					{Key: "region", Value: "eu", Source: "config", Detail: "pipeline.yaml", Scope: "jobs/deploy"},
				},
			},
		},
		Features: []FeatureRow{
			{Name: "alpha", Value: "true", Source: "config", Detail: "pipeline.yaml", Scope: "jobs/deploy"},
		},
	}

	output := report.FormatJSON()

	assert.Contains(t, output, `"detail":"pipeline.yaml › jobs/deploy"`)
	assert.Equal(t, 3, strings.Count(output, `"detail":"pipeline.yaml › jobs/deploy"`))
}

type pointerReceiverText struct {
	text string
}

func (v *pointerReceiverText) MarshalText() ([]byte, error) {
	return []byte(v.text), nil
}

func TestReport_FormatJSON_PointerReceiverTextMarshaler(t *testing.T) {
	t.Parallel()

	report := Report{
		CommandPath: "app",
		Rows: []Row{
			{
				Name:       "value",
				Label:      "--value",
				Type:       "string",
				RawValue:   pointerReceiverText{text: "hello"},
				Source:     "cli",
				Env:        "-",
				ConfigPath: "value",
			},
		},
	}

	output := report.FormatJSON()

	assert.Contains(t, output, `"value":"hello"`)
	assert.NotContains(t, output, "{hello}")
}
