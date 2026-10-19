package why

import (
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestEnrichJSONReport_IgnoresInvalidPWD(t *testing.T) {
	t.Setenv("PWD", "/definitely/not/the/cwd")

	report := Report{}

	enrichJSONReport(&report)

	workingDirectory, err := os.Getwd()
	assert.NoError(t, err)
	assert.Equal(t, workingDirectory, report.WorkingDirectory)
}
