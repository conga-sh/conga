package why

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestRenderFeatureRows_AlignedColumns(t *testing.T) {
	t.Parallel()

	rows := []FeatureRow{
		{Name: "gentle-force", Value: "true", Source: "cli"},
		{Name: "x", Value: "0", Source: "default"},
	}

	output := renderFeatureRows(rows)

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

	output := renderRows(rows)

	assert.Equal(t, "\n  FLAG        VALUE          SOURCE   ENV   CONFIG PATH\n"+
		"  --secrets   -              -        -     secrets\n"+
		"              token=██████   cli\n", output)
}

func TestRenderRows_SourceDetailRendered(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Label: "--port", Value: "9000", Source: "config", Detail: "app.json", Env: "-", ConfigPath: "port"},
	}

	output := renderRows(rows)

	assert.Equal(t, "\n  FLAG     VALUE   SOURCE              ENV   CONFIG PATH\n"+
		"  --port   9000    config (app.json)   -     port\n", output)
}

func TestRenderRows_ErrorRowStopsAfterValue(t *testing.T) {
	t.Parallel()

	rows := []Row{
		{Label: "--port", Value: "9000", Source: "cli", Env: "APP_PORT", ConfigPath: "port"},
		{Label: "--bad", Value: "x", Error: "invalid value", Env: "-", ConfigPath: "bad"},
	}

	output := renderRows(rows)

	assert.Equal(t, "\n  FLAG     VALUE   SOURCE   ENV        CONFIG PATH\n"+
		"  --port   9000    cli      APP_PORT   port\n"+
		"  --bad    x       error: invalid value\n", output)
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
				Status:      "loaded: /etc/app.yaml",
				ProbedFiles: nil,
			},
			{
				Path:        "~/.config/app",
				Status:      "no config file",
				ProbedFiles: []string{"~/.config/app/config.json"},
			},
		},
		ConfigExamples: []string{"~/.config/app/config.json"},
		Redacted:       true,
	}

	jsonOutput := report.FormatJSON()

	expected := `{"command":"cloudctl deploy worker","flags":[{"label":"--port","value":"8080","source":"cli","detail":"","env":"PORT","configPath":"server.port","error":"","hidden":false,"entries":[]},{"label":"--headers","value":"██████","source":"config","detail":"app.yaml","env":"-","configPath":"headers","error":"","hidden":false,"entries":[{"key":"timeout","value":"30s","source":"config","detail":"app.yaml"}]}],"globalFlags":[{"label":"--verbose","value":"true","source":"default","detail":"","env":"-","configPath":"-","error":"","hidden":false,"entries":[]}],"features":[{"name":"streaming","value":"true","source":"env","detail":"FEATURE_STREAMING"}],"config":{"decoders":[".json",".yaml"],"paths":[{"path":"/etc/app.yaml","status":"loaded: /etc/app.yaml","probedFiles":[]},{"path":"~/.config/app","status":"no config file","probedFiles":["~/.config/app/config.json"]}],"examples":["~/.config/app/config.json"]},"redacted":true}`

	assert.Equal(t, expected, jsonOutput)
}
