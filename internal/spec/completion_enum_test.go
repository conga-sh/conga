package spec_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

// customRegionType is a Predictable domain type whose candidates carry
// descriptions, so tests can check that an enum keeps them.
type customRegionType string

func (c customRegionType) Predict() predict.Predictor {
	return predict.Items(
		predict.Item{Value: "us-east-1", Description: "N. Virginia"},
		predict.Item{Value: "us-west-2", Description: "Oregon"},
		predict.Item{Value: "eu-west-1", Description: "Ireland"},
	)
}

// customImageType is a Predictable domain type that completes repository:tag
// values in segments.
type customImageType string

func (c customImageType) Predict() predict.Predictor {
	return predict.MultiParts(":", func(predictContext predict.Context) predict.Predictor {
		if len(predictContext.Parts) == 0 {
			return predict.Values("api:", "api-gateway:", "auth:")
		}

		return predict.Values("v1", "latest")
	})
}

type EnumNarrowingCLI struct {
	Region     mockFlag[customRegionType]   `enum:"eu-west-1,us-east-1"   help:"Region"`
	Regions    mockFlag[[]customRegionType] `enum:"eu-west-1,us-east-1"   help:"Regions"`
	Branch     mockFlag[string]             `enum:"main,release"          help:"Branch"     predictor:"branches"`
	Quiet      mockFlag[string]             `enum:"low,high"              help:"Quiet"      predictor:"none"`
	Missing    mockFlag[customRegionType]   `enum:"ap-south-1"            help:"Missing"`
	Plain      mockFlag[string]             `enum:"json,yaml"             help:"Plain"`
	Confirm    mockFlag[bool]               `enum:"true"                  help:"Confirm"`
	Unnarrowed mockFlag[customRegionType]   `help:"Unnarrowed"`
	Image      mockFlag[customImageType]    `enum:"api-gateway:latest"    help:"Image"`
}

func (c *EnumNarrowingCLI) Run(ctx context.Context) error { return nil }

type EnumNarrowingArgCLI struct {
	Target mockArg[customRegionType] `enum:"us-west-2" help:"Target region"`
}

func (c *EnumNarrowingArgCLI) Run(ctx context.Context) error { return nil }

func enumNarrowingOptions() spec.Options {
	return spec.Options{
		RegisteredPredictors: map[string]predict.Predictor{
			"branches": predict.Items(
				predict.Item{Value: "dev", Description: "Development"},
				predict.Item{Value: "release", Description: "Release line"},
				predict.Item{Value: "main", Description: "Default branch"},
			),
		},
	}
}

func completeEnumNarrowing(t *testing.T, words []string) predict.Result {
	t.Helper()

	var cli EnumNarrowingCLI

	options := enumNarrowingOptions()

	cmdSpec, err := buildSchema(&cli, "app", options)
	assert.NoError(t, err)

	return spec.Complete(cmdSpec, words, spec.CompleteOptions{RegisteredPredictors: options.RegisteredPredictors})
}

func itemDescriptions(items []predict.Item) []string {
	return slicex.Map(items, func(item predict.Item) string { return item.Description })
}

func TestCompletion_EnumNarrowsTypePredictor(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--region", ""})

	assert.Equal(t, []string{"us-east-1", "eu-west-1"}, itemValues(invoked.Items))
	assert.Equal(t, []string{"N. Virginia", "Ireland"}, itemDescriptions(invoked.Items))
}

func TestCompletion_TypePredictorWithoutEnumOffersEverything(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--unnarrowed", ""})

	assert.Equal(t, []string{"us-east-1", "us-west-2", "eu-west-1"}, itemValues(invoked.Items))
}

func TestCompletion_EnumNarrowsTagPredictor(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--branch", ""})

	assert.Equal(t, []string{"release", "main"}, itemValues(invoked.Items))
	assert.Equal(t, []string{"Release line", "Default branch"}, itemDescriptions(invoked.Items))
}

func TestCompletion_EnumWithNonePredictorOffersNothing(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--quiet", ""})

	assert.Equal(t, 0, len(invoked.Items))
	assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
}

func TestCompletion_EnumNotProducedByPredictorOffersNothing(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--missing", ""})

	assert.Equal(t, 0, len(invoked.Items))
}

func TestCompletion_EnumWithoutPredictorOffersEnumValues(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--plain", ""})

	assert.Equal(t, []string{"json", "yaml"}, itemValues(invoked.Items))
}

func TestCompletion_EnumOnSegmentedTypeOffersLeadingSegment(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--image", ""})

	assert.Equal(t, []string{"api-gateway:"}, itemValues(invoked.Items))
	assert.True(t, invoked.Directive.Has(predict.DirectiveNoSpace))
}

func TestCompletion_EnumOnSegmentedTypeOffersEnumValueAtLastSegment(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--image", "api-gateway:"})

	assert.Equal(t, []string{"api-gateway:latest"}, itemValues(invoked.Items))
	assert.False(t, invoked.Directive.Has(predict.DirectiveNoSpace))
}

func TestCompletion_EnumNarrowsBoolValues(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--confirm="})

	assert.Equal(t, []string{"--confirm=true"}, itemValues(invoked.Items))
}

func TestCompletion_EnumNarrowsSliceElements(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--regions", ""})

	assert.Equal(t, []string{"us-east-1", "eu-west-1"}, itemValues(invoked.Items))
}

func TestCompletion_EnumNarrowedSliceExcludesChosenElements(t *testing.T) {
	t.Parallel()

	invoked := completeEnumNarrowing(t, []string{"--regions", "eu-west-1,"})

	assert.Equal(t, []string{"eu-west-1,us-east-1"}, itemValues(invoked.Items))
}

func TestCompletion_EnumNarrowsArgPredictor(t *testing.T) {
	t.Parallel()

	var cli EnumNarrowingArgCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	invoked := spec.Complete(cmdSpec, []string{""}, spec.CompleteOptions{})

	assert.Equal(t, []string{"us-west-2"}, itemValues(invoked.Items))
	assert.Equal(t, []string{"Oregon"}, itemDescriptions(invoked.Items))
}

type RepeatedFilesCLI struct {
	Files mockFlag[[]conga.File] `help:"Input files"`
}

func (c *RepeatedFilesCLI) Run(ctx context.Context) error { return nil }

func TestCompletion_RepeatedSliceKeepsSameNameInAnotherDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(dir, "envs"), 0o750))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "dev.yaml"), nil, 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "envs", "dev.yaml"), nil, 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "envs", "prod.yaml"), nil, 0o600))

	var cli RepeatedFilesCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{})
	assert.NoError(t, err)

	invoked := spec.Complete(cmdSpec, []string{"--files", "dev.yaml", "--files", "envs/prod.yaml", "--files", "envs/"}, spec.CompleteOptions{Dir: dir})

	assert.Equal(t, []string{"envs/dev.yaml"}, itemValues(invoked.Items))
}
