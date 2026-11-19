package spec

import (
	"os"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/predict"
)

var (
	inferCountingCalls   atomic.Int32
	resolveCountingCalls atomic.Int32
)

// inferCountingType counts Predict calls made while inferring its predictor.
type inferCountingType string

func (inferCountingType) Predict() predict.Predictor {
	inferCountingCalls.Add(1)

	return predict.Values("alpha")
}

// resolveCountingType counts Predict calls made while resolving its predictor.
type resolveCountingType string

func (resolveCountingType) Predict() predict.Predictor {
	resolveCountingCalls.Add(1)

	return predict.Values("alpha")
}

// inferFilterableType is a PathFilterable domain type.
type inferFilterableType string

func (inferFilterableType) MatchPath(string, os.DirEntry) bool { return true }

// TestInferPredictorFromType_DefersPredict verifies that inference records a
// builder instead of calling Predict.
func TestInferPredictorFromType_DefersPredict(t *testing.T) {
	t.Parallel()

	inferred := InferPredictorFromType(reflect.TypeFor[inferCountingType](), FieldTags{})

	assert.Equal(t, PredictorKindPredictable, inferred.Kind)
	assert.NotZero(t, inferred.Build)
	assert.Zero(t, inferred.Predictor)
	assert.Equal(t, int32(0), inferCountingCalls.Load())
}

// TestInferPredictorFromType_DefersFilterable verifies that the filterable
// inferences use the deferred builder too.
func TestInferPredictorFromType_DefersFilterable(t *testing.T) {
	t.Parallel()

	inferred := InferPredictorFromType(reflect.TypeFor[inferFilterableType](), FieldTags{})

	assert.Equal(t, PredictorKindPredictable, inferred.Kind)
	assert.NotZero(t, inferred.Build)
	assert.Zero(t, inferred.Predictor)
}

// TestResolvePredictorSpec_CallsBuild verifies that resolving an inferred
// spec calls the type's Predict once and returns its predictor.
func TestResolvePredictorSpec_CallsBuild(t *testing.T) {
	t.Parallel()

	inferred := InferPredictorFromType(reflect.TypeFor[resolveCountingType](), FieldTags{})

	predictor, ok := resolvePredictorSpec(inferred, nil)

	assert.True(t, ok)
	assert.Equal(t, int32(1), resolveCountingCalls.Load())
	assert.Equal(t, []string{"alpha"}, candidateValues(predictor.Invoke(predict.Context{})))
}

// TestResolvePredictorSpec_EagerPredictor verifies that a spec without a
// builder returns its eagerly known predictor.
func TestResolvePredictorSpec_EagerPredictor(t *testing.T) {
	t.Parallel()

	eager := &PredictorSpec{Kind: PredictorKindPredictable, Name: "eager", Predictor: predict.Values("beta")}

	predictor, ok := resolvePredictorSpec(eager, nil)

	assert.True(t, ok)
	assert.Equal(t, []string{"beta"}, candidateValues(predictor.Invoke(predict.Context{})))
}

func candidateValues(result predict.Result) []string {
	values := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		values = append(values, item.Value)
	}

	return values
}
