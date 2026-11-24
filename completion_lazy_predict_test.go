package conga_test

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

// predictCalls holds one Predict call counter per test case, keyed by the
// case's marker type, so parallel cases never share a count. The mutex
// guards creating a case's counter on first use. Tests assert the
// difference across the action, so -count>1 reruns pass.
//
//nolint:gochecknoglobals // Predict runs on an engine-made zero value, so call counts must live at package scope
var predictCalls = struct {
	sync.Mutex

	counts map[reflect.Type]*atomic.Int32
}{counts: map[reflect.Type]*atomic.Int32{}}

// countingService counts its Predict calls under the marker type Case.
type countingService[Case any] string

func (s *countingService[Case]) UnmarshalText(text []byte) error {
	*s = countingService[Case](text)

	return nil
}

func (countingService[Case]) Predict() predict.Predictor {
	predictCallCounter[Case]().Add(1)

	return predict.Values("api", "web")
}

// taggedService returns its field's "service-group" tag as the only candidate.
type taggedService string

func (s *taggedService) UnmarshalText(text []byte) error {
	*s = taggedService(text)

	return nil
}

func (taggedService) Predict(tags conga.FieldTags) predict.Predictor {
	return predict.Values(tags.String("service-group"))
}

func (taggedService) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{Name: "service-group", Kind: conga.TagKindString}}
}

// panickingService panics in Predict.
type panickingService string

func (s *panickingService) UnmarshalText(text []byte) error {
	*s = panickingService(text)

	return nil
}

func (panickingService) Predict() predict.Predictor {
	panic("predict boom")
}

// lazyFlagCLI declares a flag of a predictable type next to a plain flag.
type lazyFlagCLI[Case any] struct {
	noopRunner

	Service conga.OptionalFlag[countingService[Case]] `help:"Service"`
	Other   conga.OptionalFlag[string]                `help:"Other"`
}

// lazyDeployCmd declares a positional of a predictable type.
type lazyDeployCmd[Case any] struct {
	noopRunner

	Force  conga.OptionalFlag[bool]                 `help:"Force"`
	Target conga.RequiredArg[countingService[Case]] `help:"Target"`
}

// lazyBranchCLI routes to a subcommand whose positional is predictable.
type lazyBranchCLI[Case any] struct {
	Deploy conga.Command[lazyDeployCmd[Case]] `summary:"Deploy"`
	Status conga.Command[noopRunner]          `summary:"Status"`
}

// lazyImplicitCLI makes the predictable-positional subcommand implicit.
type lazyImplicitCLI[Case any] struct {
	Deploy conga.Command[lazyDeployCmd[Case]] `implicit:"true"  summary:"Deploy"`
	Status conga.Command[noopRunner]          `summary:"Status"`
}

type taggedServiceCLI struct {
	noopRunner

	Service conga.OptionalFlag[taggedService] `help:"Service" service-group:"backend"`
}

type panickingServiceCLI struct {
	noopRunner

	Service conga.OptionalFlag[panickingService] `help:"Service"`
}

// TestPredict_NotCalledOutsideCompletion verifies that runs, --help and
// --why never call a type's Predict.
func TestPredict_NotCalledOutsideCompletion(t *testing.T) {
	t.Parallel()

	t.Run("run", func(t *testing.T) {
		t.Parallel()

		type runCase struct{}

		before := predictCallCounter[runCase]().Load()
		app := conga.New[lazyFlagCLI[runCase]]().EnableCompletion().Args([]string{"--service", "api"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, int32(0), predictCallCounter[runCase]().Load()-before)
	})

	t.Run("help", func(t *testing.T) {
		t.Parallel()

		type helpCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[helpCase]().Load()
		app := conga.New[lazyFlagCLI[helpCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"--help"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--service")
		assert.Equal(t, int32(0), predictCallCounter[helpCase]().Load()-before)
	})

	t.Run("why", func(t *testing.T) {
		t.Parallel()

		type whyCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[whyCase]().Load()
		app := conga.New[lazyFlagCLI[whyCase]]().EnableCompletion().EnableWhy().Stdout(&buffer).Args([]string{"--service", "api", "--why"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "--service")
		assert.Equal(t, int32(0), predictCallCounter[whyCase]().Load()-before)
	})
}

// TestPredict_CalledOnlyForCompletedFlag verifies that a completion request
// calls Predict only when it completes the flag of the predictable type.
func TestPredict_CalledOnlyForCompletedFlag(t *testing.T) {
	t.Parallel()

	t.Run("other flag value", func(t *testing.T) {
		t.Parallel()

		type otherFlagCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[otherFlagCase]().Load()
		app := conga.New[lazyFlagCLI[otherFlagCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "--other", ""})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.NotContains(t, buffer.String(), `"value":"api"`)
		assert.Equal(t, int32(0), predictCallCounter[otherFlagCase]().Load()-before)
	})

	t.Run("flag names", func(t *testing.T) {
		t.Parallel()

		type flagNamesCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[flagNamesCase]().Load()
		app := conga.New[lazyFlagCLI[flagNamesCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "--"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `"value":"--service"`)
		assert.Equal(t, int32(0), predictCallCounter[flagNamesCase]().Load()-before)
	})

	t.Run("predictable flag value", func(t *testing.T) {
		t.Parallel()

		type flagValueCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[flagValueCase]().Load()
		app := conga.New[lazyFlagCLI[flagValueCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "--service", ""})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `"value":"api"`)
		assert.Contains(t, buffer.String(), `"value":"web"`)
		assert.Equal(t, int32(1), predictCallCounter[flagValueCase]().Load()-before)
	})
}

// TestPredict_CalledOnlyForCompletedPositional verifies that a positional of
// a predictable type calls Predict only when its slot is completed.
func TestPredict_CalledOnlyForCompletedPositional(t *testing.T) {
	t.Parallel()

	t.Run("positional slot", func(t *testing.T) {
		t.Parallel()

		type slotCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[slotCase]().Load()
		app := conga.New[lazyBranchCLI[slotCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "deploy", ""})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `"value":"api"`)
		assert.Equal(t, int32(1), predictCallCounter[slotCase]().Load()-before)
	})

	t.Run("sibling flag names", func(t *testing.T) {
		t.Parallel()

		type flagNamesCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[flagNamesCase]().Load()
		app := conga.New[lazyBranchCLI[flagNamesCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "deploy", "--"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `"value":"--force"`)
		assert.Equal(t, int32(0), predictCallCounter[flagNamesCase]().Load()-before)
	})

	t.Run("command names", func(t *testing.T) {
		t.Parallel()

		type commandNamesCase struct{}

		var buffer bytes.Buffer

		before := predictCallCounter[commandNamesCase]().Load()
		app := conga.New[lazyBranchCLI[commandNamesCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", ""})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `"value":"deploy"`)
		assert.Equal(t, int32(0), predictCallCounter[commandNamesCase]().Load()-before)
	})
}

// TestPredict_ImplicitBareSpaceCompletesImplicitPositional pins that a bare
// space on a parent with an implicit subcommand offers the implicit command's
// first positional next to the subcommands, so its Predict runs once.
func TestPredict_ImplicitBareSpaceCompletesImplicitPositional(t *testing.T) {
	t.Parallel()

	type implicitCase struct{}

	var buffer bytes.Buffer

	before := predictCallCounter[implicitCase]().Load()
	app := conga.New[lazyImplicitCLI[implicitCase]]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"api"`)
	assert.Contains(t, buffer.String(), `"value":"status"`)
	assert.Equal(t, int32(1), predictCallCounter[implicitCase]().Load()-before)
}

// TestPredict_TagAwareReceivesFieldTags verifies that the deferred Predict
// call still receives the owning field's tags.
func TestPredict_TagAwareReceivesFieldTags(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[taggedServiceCLI]().EnableCompletion().Stdout(&buffer).Args([]string{"__complete", "export", "--service", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), `"value":"backend"`)
}

// TestPredict_PanicFailsOnlyCompletion verifies that a panicking Predict
// leaves runs working and fails only the completion request for its field.
func TestPredict_PanicFailsOnlyCompletion(t *testing.T) {
	t.Parallel()

	t.Run("run succeeds", func(t *testing.T) {
		t.Parallel()

		app := conga.New[panickingServiceCLI]().EnableCompletion().Args([]string{"--service", "api"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("completing the field fails", func(t *testing.T) {
		t.Parallel()

		var (
			stdoutBuffer bytes.Buffer
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New[panickingServiceCLI]().
			EnableCompletion().
			Stdout(&stdoutBuffer).
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args([]string{"__complete", "bash", "--service", ""})

		err := app.RunContext(t.Context())
		app.Exit(err)

		assert.Error(t, err)
		coder, isCoder := errors.AsType[conga.ExitCoder](err)
		assert.True(t, isCoder)
		assert.Equal(t, 2, coder.ExitCode())
		assert.Equal(t, 2, exitCode)
		assert.Equal(t, "", stdoutBuffer.String())
		assert.Contains(t, stderrBuffer.String(), "predict boom")
	})
}

func predictCallCounter[Case any]() *atomic.Int32 {
	predictCalls.Lock()
	defer predictCalls.Unlock()

	key := reflect.TypeFor[Case]()
	if predictCalls.counts[key] == nil {
		predictCalls.counts[key] = new(atomic.Int32)
	}

	return predictCalls.counts[key]
}
