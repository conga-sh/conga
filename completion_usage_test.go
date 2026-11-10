package conga_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

type predictorUsageCLI struct {
	noopRunner

	Image conga.OptionalFlag[string] `help:"Image name" predictor:"images"`
}

func TestComplete_PredictorUsageOutranksFlagHelp(t *testing.T) {
	t.Parallel()

	t.Run("zsh shows the predictor hint", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		err := predictorUsageApp(&buffer, "zsh").Run()

		assert.NoError(t, err)
		assert.Equal(t,
			"D\x1f\x1fType the service first\nL\x1f=(#b)(*)([ ]## -- *)=0==2\nM\x1f\x1b[2mType the service first\x1b[39m",
			buffer.String())
	})

	t.Run("powershell shows the predictor hint", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		err := predictorUsageApp(&buffer, "powershell").Run()

		assert.NoError(t, err)
		assert.Equal(t,
			`{"candidates":[],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":false},"usage":"Type the service first"}`,
			buffer.String())
	})

	t.Run("bash writes no hint text", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		err := predictorUsageApp(&buffer, "bash").Run()

		assert.NoError(t, err)
		assert.Equal(t, "D\t", buffer.String())
	})

	t.Run("fish writes no hint text", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		err := predictorUsageApp(&buffer, "fish").Run()

		assert.NoError(t, err)
		assert.Equal(t, "", buffer.String())
	})
}

func predictorUsageApp(buffer *bytes.Buffer, shell string) *conga.App[predictorUsageCLI] {
	images := predict.Callback(func(predict.Context) predict.Predictor {
		return predict.None().Usage("Type the service first")
	})

	return conga.New[predictorUsageCLI]().
		EnableCompletion().
		Name("testapp").
		Predictor("images", images).
		Stdout(buffer).
		Args([]string{"__complete", shell, "--image", ""})
}
