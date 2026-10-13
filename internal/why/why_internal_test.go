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
