package spec_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

// stage is a Predictable map value type without help of its own.
type stage string

func (stage) Predict() predict.Predictor { return predict.Values("dev", "prod") }

// stageMap is a named map type with its own predictor, which wins over its
// value type's.
type stageMap map[string]stage

func (stageMap) Predict() predict.Predictor { return predict.Values("own=dev") }

// unpredictable has no predictor of its own.
type unpredictable string

type MapValueCLI struct {
	Stages     mockFlag[map[string]stage] `help:"Stage per service"`
	Bare       mockFlag[map[string]stage]
	Named      mockFlag[stageMap]                 `help:"Named"`
	Registered mockFlag[map[string]unpredictable] `help:"Registered"`
	Tagged     mockFlag[map[string]stage]         `help:"Tagged"            predictor:"entries"`
	Toggles    mockFlag[map[string]bool]          `help:"Toggles"`
	Plain      mockFlag[map[string]string]        `help:"Plain"`
}

func (c *MapValueCLI) Run(ctx context.Context) error { return nil }

type MapValueArgCLI struct {
	Stages mockArg[map[string]stage] `help:"Stage per service" name:"<stages>"`
}

func (c *MapValueArgCLI) Run(ctx context.Context) error { return nil }

func mapValueOptions() spec.Options {
	return spec.Options{
		RegisteredPredictors: map[string]predict.Predictor{
			"entries": predict.Values("tagged=yes"),
		},
		TypePredictors: map[reflect.Type]predict.Predictor{
			reflect.TypeFor[unpredictable](): predict.Values("one", "two"),
		},
	}
}

func completeMapValue(t *testing.T, target any, words []string) predict.Result {
	t.Helper()

	options := mapValueOptions()

	cmdSpec, err := buildSchema(target, "app", options)
	assert.NoError(t, err)

	return spec.Complete(cmdSpec, words, spec.CompleteOptions{RegisteredPredictors: options.RegisteredPredictors})
}

func TestMapValue_InferenceMarksValueTypePredictor(t *testing.T) {
	t.Parallel()

	cmdSpec, err := buildSchema(&MapValueCLI{}, "app", mapValueOptions())

	assert.NoError(t, err)
	assert.True(t, cmdSpec.FindFlag("stages").Predictor.MapValue)
	assert.True(t, cmdSpec.FindFlag("registered").Predictor.MapValue)
	assert.False(t, cmdSpec.FindFlag("named").Predictor.MapValue)
	assert.False(t, cmdSpec.FindFlag("tagged").Predictor.MapValue)
	assert.Zero(t, cmdSpec.FindFlag("toggles").Predictor)
	assert.Zero(t, cmdSpec.FindFlag("plain").Predictor)
}

func TestMapValue_RegisteredValuePredictorWithoutInferenceHook(t *testing.T) {
	t.Parallel()

	options := mapValueOptions()
	options.ClassifyContainer = classifyTestContainer
	options.ProcessFeatures = spec.ProcessFeaturesField
	options.ValidateFeatureReferences = spec.ValidateFeatureReferences
	options.ParseFeatureTag = spec.ParseFeatureTag

	cmdSpec, err := spec.Build(&MapValueCLI{}, "app", options)

	assert.NoError(t, err)
	assert.True(t, cmdSpec.FindFlag("registered").Predictor.MapValue)
	assert.Zero(t, cmdSpec.FindFlag("stages").Predictor)
	assert.Zero(t, cmdSpec.FindFlag("toggles").Predictor)
}

func TestMapValue_ValueTypeCompletesAfterKey(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "api="})

	assert.Equal(t, []string{"api=dev", "api=prod"}, itemValues(invoked.Items))
	assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
}

func TestMapValue_ValueTypeFiltersByTypedValue(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "api=p"})

	assert.Equal(t, []string{"api=prod"}, itemValues(invoked.Items))
}

func TestMapValue_NothingBeforeEquals(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "ap"})

	assert.Equal(t, 0, len(invoked.Items))
	assert.Equal(t, "Stage per service", invoked.Usage)
}

func TestMapValue_EmptyKeyOffersNothing(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "="})

	assert.Equal(t, 0, len(invoked.Items))
}

func TestMapValue_KeyValueHintWithoutHelp(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--bare", ""})

	assert.Equal(t, 0, len(invoked.Items))
	assert.Equal(t, "<key>=<value>", invoked.Usage)
}

func TestMapValue_NamedMapKeepsItsOwnPredictor(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--named", ""})

	assert.Equal(t, []string{"own=dev"}, itemValues(invoked.Items))
}

func TestMapValue_RegisteredValuePredictor(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--registered", "k="})

	assert.Equal(t, []string{"k=one", "k=two"}, itemValues(invoked.Items))
}

func TestMapValue_PredictorTagWins(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--tagged", ""})

	assert.Equal(t, []string{"tagged=yes"}, itemValues(invoked.Items))
}

func TestMapValue_BoolValueOffersTrueAndFalse(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--toggles", "cache="})

	assert.Equal(t, []string{"cache=true", "cache=false"}, itemValues(invoked.Items))
}

func TestMapValue_SecondEntryAfterComma(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "api=dev,db="})

	assert.Equal(t, []string{"api=dev,db=dev", "api=dev,db=prod"}, itemValues(invoked.Items))
}

func TestMapValue_PositionalUsageBeforeEquals(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueArgCLI{}, []string{""})

	assert.Equal(t, 0, len(invoked.Items))
	assert.Equal(t, "<stages>: Stage per service", invoked.Usage)
}

func TestMapValue_PositionalSecondEntryAfterComma(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueArgCLI{}, []string{"a=dev,b="})

	assert.Equal(t, []string{"a=dev,b=dev", "a=dev,b=prod"}, itemValues(invoked.Items))
}

func TestMapValue_PositionalSecondToken(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueArgCLI{}, []string{"a=dev", "b="})

	assert.Equal(t, []string{"b=dev", "b=prod"}, itemValues(invoked.Items))
}

// pathValue completes paths, as conga.File does.
type pathValue string

func (pathValue) Predict() predict.Predictor { return predict.Files() }

type MapPathCLI struct {
	Files mockFlag[map[string]pathValue] `help:"Files"`
}

func (c *MapPathCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_ValueContainingEqualsCutsAtFirst(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "a=b")
	assert.NoError(t, os.MkdirAll(dir, 0o750))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), nil, 0o600))

	invoked := completeMapValue(t, &MapPathCLI{}, []string{"--files", "k=" + dir + "/"})

	assert.Equal(t, []string{"k=" + dir + "/x.txt"}, itemValues(invoked.Items))
}

func TestMapValue_DirectoryCompletesWithSlash(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(root, "only"), 0o750))

	invoked := completeMapValue(t, &MapPathCLI{}, []string{"--files", "k=" + root + "/o"})

	assert.Equal(t, []string{"k=" + root + "/only/"}, itemValues(invoked.Items))
}

// segment completes partial values and asks the shell for no trailing space.
type segment string

func (segment) Predict() predict.Predictor { return predict.Values("a.", "b.").NoSpace() }

type MapSegmentCLI struct {
	Segments mockFlag[map[string]segment] `help:"Segments"`
}

func (c *MapSegmentCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_ValuePredictorNoSpaceCarriesThrough(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapSegmentCLI{}, []string{"--segments", "k="})

	assert.Equal(t, []string{"k=a.", "k=b."}, itemValues(invoked.Items))
	assert.Equal(t, predict.DirectiveNoFileCompletion|predict.DirectiveNoSpace, invoked.Directive)
}

func TestMapValue_FinishedValueGetsSpace(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueCLI{}, []string{"--stages", "api=d"})

	assert.Equal(t, []string{"api=dev"}, itemValues(invoked.Items))
	assert.Equal(t, predict.DirectiveNoFileCompletion, invoked.Directive)
}

// zone completes capitalised names, so smart-case matching applies.
type zone string

func (zone) Predict() predict.Predictor { return predict.Values("Europe/London", "Europe/Lisbon") }

// fuzzyRegion filters by its own text, not the typed one.
type fuzzyRegion string

func (fuzzyRegion) Predict() predict.Predictor {
	return predict.Values("us-east-1", "eu-west-1").FilterText("us-")
}

// slowRegion answers after the default timeout and raises its own.
type slowRegion string

func (slowRegion) Predict() predict.Predictor {
	return predict.Callback(func(predict.Context) predict.Predictor {
		time.Sleep(2 * predict.DefaultTimeout)

		return predict.Values("slow")
	}).Timeout(time.Second)
}

type MapValueFilterCLI struct {
	Zones mockFlag[map[string]zone]        `help:"Zones"`
	Fuzzy mockFlag[map[string]fuzzyRegion] `help:"Fuzzy"`
	Slow  mockFlag[map[string]slowRegion]  `help:"Slow"`
	Slows mockFlag[[]slowRegion]           `help:"Slows"`
}

func (c *MapValueFilterCLI) Run(ctx context.Context) error { return nil }

type MapBoolArgCLI struct {
	Toggles mockArg[map[string]bool] `help:"Toggles" name:"<toggles>"`
}

func (c *MapBoolArgCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_UppercaseKeyKeepsSmartCaseValueMatch(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueFilterCLI{}, []string{"--zones", "EU=europe/lo"})

	assert.Equal(t, []string{"EU=Europe/London"}, itemValues(invoked.Items))
}

func TestMapValue_ValuePredictorFilterTextApplies(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueFilterCLI{}, []string{"--fuzzy", "k=east"})

	assert.Equal(t, []string{"k=us-east-1"}, itemValues(invoked.Items))
}

func TestMapValue_ValuePredictorTimeoutGoverns(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueFilterCLI{}, []string{"--slow", "k="})

	assert.Equal(t, []string{"k=slow"}, itemValues(invoked.Items))
}

func TestMapValue_BoolPositionalOffersTrueAndFalse(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapBoolArgCLI{}, []string{"cache="})

	assert.Equal(t, []string{"cache=true", "cache=false"}, itemValues(invoked.Items))
}

func TestMapValue_SliceElementPredictorTimeoutGoverns(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapValueFilterCLI{}, []string{"--slows", "a,"})

	assert.Equal(t, []string{"a,slow"}, itemValues(invoked.Items))
}

// fileLike has help of its own, so a map of it names that help in its hint.
type fileLike string

func (fileLike) Help() string { return "Path to a file" }

type MapHintCLI struct {
	Files  mockFlag[map[string]fileLike]
	Stages mockFlag[map[string]stage]
	Mounts mockFlag[map[string]fileLike] `help:"Files to mount"`
}

func (c *MapHintCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_HintNamesValueTypeHelp(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapHintCLI{}, []string{"--files", ""})

	assert.Equal(t, "<key>=<Path to a file>", invoked.Usage)
}

func TestMapValue_HintWithoutValueTypeHelp(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapHintCLI{}, []string{"--stages", ""})

	assert.Equal(t, "<key>=<value>", invoked.Usage)
}

func TestMapValue_FieldHelpReplacesHint(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapHintCLI{}, []string{"--mounts", ""})

	assert.Equal(t, "Files to mount", invoked.Usage)
}

type MapPathTagCLI struct {
	Mounts mockFlag[map[string]string] `help:"Mounts" predictor:"file"`
	Dirs   mockFlag[map[string]string] `help:"Dirs"   predictor:"dir"`
}

func (c *MapPathTagCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_PathPredictorTagMarksValue(t *testing.T) {
	t.Parallel()

	cmdSpec, err := buildSchema(&MapPathTagCLI{}, "app", mapValueOptions())

	assert.NoError(t, err)
	assert.True(t, cmdSpec.FindFlag("mounts").Predictor.MapValue)
	assert.True(t, cmdSpec.FindFlag("dirs").Predictor.MapValue)
}

func TestMapValue_PathPredictorTagOffersNothingBeforeEquals(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapPathTagCLI{}, []string{"--mounts", ""})

	assert.Equal(t, 0, len(invoked.Items))
	assert.Equal(t, "Mounts", invoked.Usage)
}

func TestMapValue_PathPredictorTagCompletesValue(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(root, "app.yaml"), nil, 0o600))

	invoked := completeMapValue(t, &MapPathTagCLI{}, []string{"--mounts", "data=" + root + "/"})

	assert.Equal(t, []string{"data=" + root + "/app.yaml"}, itemValues(invoked.Items))
}

func TestMapValue_DirPredictorTagCompletesValue(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(root, "conf"), 0o750))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "app.yaml"), nil, 0o600))

	invoked := completeMapValue(t, &MapPathTagCLI{}, []string{"--dirs", "data=" + root + "/"})

	assert.Equal(t, []string{"data=" + root + "/conf/"}, itemValues(invoked.Items))
}

type MapArgStageHintCLI struct {
	Stages mockArg[map[string]stage] `name:"<stages>"`
}

func (c *MapArgStageHintCLI) Run(ctx context.Context) error { return nil }

type MapArgFileHintCLI struct {
	Files mockArg[map[string]fileLike] `name:"<files>"`
}

func (c *MapArgFileHintCLI) Run(ctx context.Context) error { return nil }

func TestMapValue_PositionalHintNamesValueTypeHelp(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapArgFileHintCLI{}, []string{""})

	assert.Equal(t, "<files>: <key>=<Path to a file>", invoked.Usage)
}

func TestMapValue_PositionalHintWithoutValueTypeHelp(t *testing.T) {
	t.Parallel()

	invoked := completeMapValue(t, &MapArgStageHintCLI{}, []string{""})

	assert.Equal(t, "<stages>: <key>=<value>", invoked.Usage)
}

// teamMap completes whole entries and sets its own usage.
type teamMap map[string]string

func (teamMap) Predict() predict.Predictor {
	return predict.KeyValue(map[string][]string{"team": {"core"}}).Usage("Pick a team")
}

type MapEntryUsageCLI struct {
	Teams  mockFlag[teamMap]
	Labels mockFlag[map[string]string] `predictor:"labels"`
}

func (c *MapEntryUsageCLI) Run(ctx context.Context) error { return nil }

func completeMapEntryUsage(t *testing.T, words []string) predict.Result {
	t.Helper()

	options := spec.Options{
		RegisteredPredictors: map[string]predict.Predictor{
			"labels": predict.KeyValue(map[string][]string{"owner": {"alice"}}).Usage("Pick a label"),
		},
	}

	cmdSpec, err := buildSchema(&MapEntryUsageCLI{}, "app", options)
	assert.NoError(t, err)

	return spec.Complete(cmdSpec, words, spec.CompleteOptions{RegisteredPredictors: options.RegisteredPredictors})
}

func TestMapValue_NamedMapPredictorKeepsItsUsage(t *testing.T) {
	t.Parallel()

	invoked := completeMapEntryUsage(t, []string{"--teams", ""})

	assert.Equal(t, []string{"team="}, itemValues(invoked.Items))
	assert.Equal(t, "Pick a team", invoked.Usage)
}

func TestMapValue_TagPredictorKeepsItsUsage(t *testing.T) {
	t.Parallel()

	invoked := completeMapEntryUsage(t, []string{"--labels", ""})

	assert.Equal(t, []string{"owner="}, itemValues(invoked.Items))
	assert.Equal(t, "Pick a label", invoked.Usage)
}
