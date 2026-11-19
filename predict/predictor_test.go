package predict_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/predict"
)

func TestValues(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 3, len(result.Items))
	assert.Equal(t, "apple", result.Items[0].Value)
	assert.Equal(t, "banana", result.Items[1].Value)
	assert.Equal(t, "cherry", result.Items[2].Value)
	assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, pred.Raw())
}

func TestItems(t *testing.T) {
	t.Parallel()

	items := []predict.Item{
		predict.NewItem("json", "JavaScript Object Notation").WithStyle("green"),
		predict.NewItem("yaml", "YAML Ain't Markup Language"),
	}
	result := predict.Items(items...).Invoke(predict.Context{})

	assert.Equal(t, 2, len(result.Items))
	assert.Equal(t, "json", result.Items[0].Value)
	assert.Equal(t, "JavaScript Object Notation", result.Items[0].Description)
	assert.Equal(t, "green", result.Items[0].Style)
	assert.Equal(t, "yaml", result.Items[1].Value)
	assert.Equal(t, "YAML Ain't Markup Language", result.Items[1].Description)
	assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
}

func TestMap(t *testing.T) {
	t.Parallel()

	m := map[string]string{
		"debug": "Enable debug logging",
		"info":  "Informational messages",
		"error": "Error messages only",
	}
	result := predict.Map(m).Invoke(predict.Context{})

	assert.Equal(t, 3, len(result.Items))
	assert.Equal(t, "debug", result.Items[0].Value)
	assert.Equal(t, "Enable debug logging", result.Items[0].Description)
	assert.Equal(t, "error", result.Items[1].Value)
	assert.Equal(t, "Error messages only", result.Items[1].Description)
	assert.Equal(t, "info", result.Items[2].Value)
	assert.Equal(t, "Informational messages", result.Items[2].Description)
	assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
}

func TestKeyValue(t *testing.T) {
	t.Parallel()

	valuesByKey := map[string][]string{
		"team":  {"core", "frontend", "infra"},
		"tier":  {"frontend", "backend", "database"},
		"owner": {"alice", "bob", "carol"},
	}
	pred := predict.KeyValue(valuesByKey)

	resultKeys := pred.Invoke(predict.Context{})

	assert.Equal(t, 3, len(resultKeys.Items))
	assert.Equal(t, "owner=", resultKeys.Items[0].Value)
	assert.Equal(t, "team=", resultKeys.Items[1].Value)
	assert.Equal(t, "tier=", resultKeys.Items[2].Value)
	assert.True(t, resultKeys.Directive.Has(predict.DirectiveNoSpace))

	resultValues := pred.Invoke(predict.Context{
		ToComplete: "team=",
	})

	assert.Equal(t, 3, len(resultValues.Items))
	assert.Equal(t, "team=core", resultValues.Items[0].Value)
	assert.Equal(t, "team=frontend", resultValues.Items[1].Value)
	assert.Equal(t, "team=infra", resultValues.Items[2].Value)

	resultUnknown := pred.Invoke(predict.Context{
		ToComplete: "unknown=",
	})
	assert.Zero(t, resultUnknown.Items)

	resultDeduplicatedKeys := pred.Invoke(predict.Context{
		ParentParts: []string{"owner=alice"},
	})
	assert.Equal(t, 2, len(resultDeduplicatedKeys.Items))
	assert.Equal(t, "team=", resultDeduplicatedKeys.Items[0].Value)
	assert.Equal(t, "tier=", resultDeduplicatedKeys.Items[1].Value)

	// 8. Act - Value deduplication when key is repeated
	resDeduplicatedValues := pred.Invoke(predict.Context{
		ParentParts: []string{"owner=alice"},
		ToComplete:  "owner=",
	})
	assert.Equal(t, 2, len(resDeduplicatedValues.Items))
	assert.Equal(t, "owner=bob", resDeduplicatedValues.Items[0].Value)
	assert.Equal(t, "owner=carol", resDeduplicatedValues.Items[1].Value)

	// 9. Act - Multi-flag deduplication from FlagMultiValues
	resMultiFlagKeys := pred.Invoke(predict.Context{
		FlagMultiValues: map[string][]string{
			"labels": {"owner=alice", "team=core"},
		},
	})
	assert.Equal(t, 1, len(resMultiFlagKeys.Items))
	assert.Equal(t, "tier=", resMultiFlagKeys.Items[0].Value)
}

func TestExclude(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry").Exclude("banana")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 2, len(result.Items))
	assert.Equal(t, "apple", result.Items[0].Value)
	assert.Equal(t, "cherry", result.Items[1].Value)
	assert.Equal(t, []string{"apple", "cherry"}, pred.Raw())
}

func TestExclude_Batch(t *testing.T) {
	t.Parallel()

	pred := predict.Batch(predict.Values("apple", "banana"), predict.Values("cherry")).Exclude("banana", "cherry")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "apple", Display: "apple"}}, result.Items)
}

func TestExclude_Callback(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(predict.Context) predict.Predictor {
		return predict.Values("apple", "banana")
	}).Exclude("apple")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
}

func TestExclude_IgnoresDisplay(t *testing.T) {
	t.Parallel()

	pred := predict.Items(
		predict.Item{Value: "a", Display: "apple"},
		predict.Item{Value: "b", Display: "banana"},
	).Exclude("apple", "b")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "a", Display: "apple"}}, result.Items)
	assert.Equal(t, []string{"a"}, pred.Raw())
}

func TestExclude_Accumulates(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry").Exclude("apple").Exclude("cherry")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"banana"}, pred.Raw())
}

func TestExclude_AfterPrefix(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana").Prefix("x-").Exclude("x-apple", "banana")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "x-banana", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"banana"}, pred.Raw())
}

func TestExclude_FileInAnotherDirectoryStays(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(dir, "envs"), 0o750))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "dev.yaml"), nil, 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "envs", "dev.yaml"), nil, 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "envs", "prod.yaml"), nil, 0o600))

	pred := predict.Files().Exclude("dev.yaml", "envs/prod.yaml")

	result := pred.Invoke(predict.Context{Dir: dir, ToComplete: "envs/"})

	values := slicex.Map(result.Items, func(item predict.Item) string { return item.Value })
	assert.Equal(t, []string{"envs/dev.yaml"}, values)
}

func TestExclude_TimeoutAlternative(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	slow := predict.Callback(func(predict.Context) predict.Predictor {
		<-release

		return predict.None()
	})
	pred := slow.Timeout(time.Millisecond, predict.Values("apple", "banana")).Exclude("apple")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
}

func TestUsage_CallbackHintWins(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(predict.Context) predict.Predictor {
		return predict.None().Usage("Type the service first")
	}).Usage("Image name")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, "Type the service first", result.Usage)
}

func TestUsage_FallbackWhenCallbackHasNone(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(predict.Context) predict.Predictor {
		return predict.Values("nginx")
	}).Usage("Image name")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, "Image name", result.Usage)
}

func TestUsage_BatchMemberHintWins(t *testing.T) {
	t.Parallel()

	pred := predict.Batch(predict.Values("a"), predict.None().Usage("member hint")).Usage("outer hint")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, "member hint", result.Usage)
}

func TestNone(t *testing.T) {
	t.Parallel()

	result := predict.None().Invoke(predict.Context{})

	assert.Zero(t, result.Items)
	assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
}

func TestCallback(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(c predict.Context) predict.Predictor {
		if c.FlagValue("env", "") == "production" {
			return predict.Values("prod-cluster-1", "prod-cluster-2")
		}

		return predict.Values("dev-cluster", "local-cluster")
	})

	prodRes := pred.Invoke(predict.Context{
		Flags: map[string]string{"env": "production"},
	})
	assert.Equal(t, 2, len(prodRes.Items))
	assert.Equal(t, "prod-cluster-1", prodRes.Items[0].Value)

	devRes := pred.Invoke(predict.Context{
		Flags: map[string]string{"env": "staging"},
	})
	assert.Equal(t, 2, len(devRes.Items))
	assert.Equal(t, "dev-cluster", devRes.Items[0].Value)
}

func TestMultiParts(t *testing.T) {
	t.Parallel()

	t.Run("first segment completion", func(t *testing.T) {
		t.Parallel()

		pred := predict.MultiParts(",", func(c predict.Context) predict.Predictor {
			return predict.Values("apple", "apricot", "banana")
		})

		result := pred.Invoke(predict.Context{
			ToComplete: "ap",
		})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "apple", result.Items[0].Value)
		assert.Equal(t, "apricot", result.Items[1].Value)
	})

	t.Run("second segment completion with prefix retention", func(t *testing.T) {
		t.Parallel()

		pred := predict.MultiParts(",", func(c predict.Context) predict.Predictor {
			assert.Equal(t, []string{"apple"}, c.Parts)

			return predict.Values("banana", "berry", "cherry")
		})

		result := pred.Invoke(predict.Context{
			ToComplete: "apple,b",
		})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "apple,banana", result.Items[0].Value)
		assert.Equal(t, "apple,berry", result.Items[1].Value)
		assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
	})

	t.Run("key value pair with equals delimiter", func(t *testing.T) {
		t.Parallel()

		pred := predict.MultiParts("=", func(c predict.Context) predict.Predictor {
			if len(c.Parts) == 0 {
				return predict.Values("port", "host", "timeout")
			}

			if c.Parts[0] == "port" {
				return predict.Values("8080", "9090", "3000")
			}

			return predict.Values("default")
		})

		result := pred.Invoke(predict.Context{
			ToComplete: "port=",
		})

		assert.Equal(t, 3, len(result.Items))
		assert.Equal(t, "port=8080", result.Items[0].Value)
		assert.Equal(t, "port=9090", result.Items[1].Value)
		assert.Equal(t, "port=3000", result.Items[2].Value)
		assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
	})
}

func TestMultiParts_PartialCandidatesGetNoSpace(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("api:", "web:")
	})

	result := pred.Invoke(predict.Context{})

	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_MiddlePartialCandidatesGetNoSpace(t *testing.T) {
	t.Parallel()

	pred := threePartPredictor()

	result := pred.Invoke(predict.Context{ToComplete: "eu:"})

	assert.Equal(t, []string{"eu:c1:", "eu:c2:"}, itemValues(result.Items))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_FinishedValueGetsSpace(t *testing.T) {
	t.Parallel()

	pred := imagePredictor()

	result := pred.Invoke(predict.Context{ToComplete: "api-gateway:"})

	assert.Equal(t, []string{"api-gateway:v1", "api-gateway:v10", "api-gateway:latest"}, itemValues(result.Items))
	assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_MixedCandidatesFollowSegmentDirective(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("api:", "local")
	})

	result := pred.Invoke(predict.Context{})

	assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_MixedCandidatesNarrowedByIncludeGetNoSpace(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("api:", "local")
	}).Include("api:latest")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []string{"api:"}, itemValues(result.Items))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_MixedCandidatesNarrowedByTypingGetNoSpace(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("api:", "local")
	})

	result := pred.Invoke(predict.Context{ToComplete: "a"})

	assert.Equal(t, []string{"api:"}, itemValues(result.Items))
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestMultiParts_NoCandidatesAddNoSpace(t *testing.T) {
	t.Parallel()

	pred := imagePredictor()

	result := pred.Invoke(predict.Context{ToComplete: "api-gateway:x"})

	assert.Equal(t, 0, len(result.Items))
	assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestBatch(t *testing.T) {
	t.Parallel()

	t.Run("items and directives aggregation", func(t *testing.T) {
		t.Parallel()

		pred1 := predict.Values("apple", "banana")
		pred2 := predict.Values("banana", "cherry").NoSpace()

		result := predict.Batch(pred1, pred2).Invoke(predict.Context{})

		assert.Equal(t, 3, len(result.Items))
		assert.Equal(t, "apple", result.Items[0].Value)
		assert.Equal(t, "banana", result.Items[1].Value)
		assert.Equal(t, "cherry", result.Items[2].Value)
		assert.True(t, result.Directive.Has(predict.DirectiveNoFileCompletion))
		assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
	})
}

func TestCombinators(t *testing.T) {
	t.Parallel()

	t.Run("prefix modifier", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("v1.0", "v2.0").Prefix("release-")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "release-v1.0", result.Items[0].Value)
		assert.Equal(t, "release-v2.0", result.Items[1].Value)
	})

	t.Run("suffix modifier", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("usr", "etc", "var").Suffix("/")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 3, len(result.Items))
		assert.Equal(t, "usr/", result.Items[0].Value)
		assert.Equal(t, "etc/", result.Items[1].Value)
		assert.Equal(t, "var/", result.Items[2].Value)
	})

	t.Run("usage message", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("main", "staging").Usage("Select deployment branch")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, "Select deployment branch", result.Usage)
	})

	t.Run("usagef message with formatting", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("main", "staging").Usagef("target cluster in %s (%d total)", "production", 2)

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, "target cluster in production (2 total)", result.Usage)
	})

	t.Run("usagef message", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("main", "staging").Usagef("target pod in %s/%s", "staging", "backend")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, "target pod in staging/backend", result.Usage)
	})

	t.Run("style modifier", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("feat", "fix").Style("yellow")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, "yellow", result.Items[0].Style)
		assert.Equal(t, "yellow", result.Items[1].Style)
	})

	t.Run("directive modifiers NoSpace and KeepOrder", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("a", "b").NoSpace().KeepOrder()

		result := pred.Invoke(predict.Context{})

		assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
		assert.True(t, result.Directive.Has(predict.DirectiveKeepOrder))
	})

	t.Run("filter matching smart case", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("PostgreSQL", "postgres-dev", "MySQL", "mongo")

		resLower := pred.FilterText("post").Invoke(predict.Context{})

		assert.Equal(t, 2, len(resLower.Items))
		assert.Equal(t, "PostgreSQL", resLower.Items[0].Value)
		assert.Equal(t, "postgres-dev", resLower.Items[1].Value)

		// 4. Act - uppercase matches case-sensitively
		resUpper := pred.FilterText("Post").Invoke(predict.Context{})

		// 5. Assert
		assert.Equal(t, 1, len(resUpper.Items))
		assert.Equal(t, "PostgreSQL", resUpper.Items[0].Value)
	})

	t.Run("filter modifier explicit query", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("cat", "caterpillar", "dog").FilterText("cat")

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "cat", result.Items[0].Value)
		assert.Equal(t, "caterpillar", result.Items[1].Value)
	})

	t.Run("static items bypass timeout machinery", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("a", "b").Timeout(time.Millisecond)

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "a", result.Items[0].Value)
		assert.Equal(t, "b", result.Items[1].Value)
	})
}

func TestTimeout(t *testing.T) {
	t.Parallel()

	t.Run("default timeout applies to slow callback", func(t *testing.T) {
		t.Parallel()

		pred := predict.Callback(func(c predict.Context) predict.Predictor {
			time.Sleep(350 * time.Millisecond)

			return predict.Values("delayed-item")
		})

		start := time.Now()
		result := pred.Invoke(predict.Context{})
		elapsed := time.Since(start)

		assert.True(t, elapsed >= 200*time.Millisecond, "expected to wait at least default timeout")
		assert.True(t, elapsed < 400*time.Millisecond, "expected not to wait for full sleep duration")
		assert.Zero(t, result.Items)
		assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
	})

	t.Run("custom timeout with fallback predictor", func(t *testing.T) {
		t.Parallel()

		pred := predict.Callback(func(c predict.Context) predict.Predictor {
			time.Sleep(200 * time.Millisecond)

			return predict.Values("slow-remote-item")
		}).Timeout(30*time.Millisecond, predict.Values("fallback-item"))

		start := time.Now()
		result := pred.Invoke(predict.Context{})
		elapsed := time.Since(start)

		assert.True(t, elapsed >= 25*time.Millisecond, "expected to wait for custom timeout")
		assert.True(t, elapsed < 150*time.Millisecond, "expected to return before callback completes")
		assert.Equal(t, 1, len(result.Items))
		assert.Equal(t, "fallback-item", result.Items[0].Value)
	})

	t.Run("custom timeout without fallback returns empty items", func(t *testing.T) {
		t.Parallel()

		pred := predict.Callback(func(c predict.Context) predict.Predictor {
			time.Sleep(200 * time.Millisecond)

			return predict.Values("slow-remote-item")
		}).Timeout(30 * time.Millisecond)

		start := time.Now()
		result := pred.Invoke(predict.Context{})
		elapsed := time.Since(start)

		assert.True(t, elapsed >= 25*time.Millisecond, "expected to wait for custom timeout")
		assert.True(t, elapsed < 150*time.Millisecond, "expected to return before callback completes")
		assert.Zero(t, result.Items)
		assert.Equal(t, predict.DirectiveNoFileCompletion, result.Directive)
	})

	t.Run("disabled timeout allows slow execution", func(t *testing.T) {
		t.Parallel()

		pred := predict.Callback(func(c predict.Context) predict.Predictor {
			time.Sleep(280 * time.Millisecond)

			return predict.Values("unbounded-item")
		}).Timeout(-1)

		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 1, len(result.Items))
		assert.Equal(t, "unbounded-item", result.Items[0].Value)
	})

	t.Run("fast callback finishes before timeout", func(t *testing.T) {
		t.Parallel()

		pred := predict.Callback(func(c predict.Context) predict.Predictor {
			return predict.Values("instant-item")
		})

		start := time.Now()
		result := pred.Invoke(predict.Context{})
		elapsed := time.Since(start)

		assert.True(t, elapsed < 50*time.Millisecond, "expected instant execution")
		assert.Equal(t, 1, len(result.Items))
		assert.Equal(t, "instant-item", result.Items[0].Value)
	})

	t.Run("panics on multiple alternative predictors", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("base")

		assert.Panics(t, func() {
			pred.Timeout(50*time.Millisecond, predict.Values("alt1"), predict.Values("alt2"))
		})
	})
}

func TestTimeout_AlternativeKeepsItsModifiers(t *testing.T) {
	t.Parallel()

	alternative := predict.Values("a1", "b1").Tag("alt").Usage("alt usage").Prefix("x-").Exclude("x-b1")
	pred := blockingCallback(t).Timeout(time.Millisecond, alternative)

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "x-a1", Display: "a1", Tag: "alt"}}, result.Items)
	assert.Equal(t, "alt usage", result.Usage)
}

func TestTimeout_OuterModifiersApplyOnTopOfAlternative(t *testing.T) {
	t.Parallel()

	pred := blockingCallback(t).Timeout(time.Millisecond, predict.Values("a").Prefix("in-")).Prefix("out-")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "out-in-a", Display: "a"}}, result.Items)
}

func TestTimeout_MultiPartsAlternativeIsNotFilteredAgain(t *testing.T) {
	t.Parallel()

	alternative := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("aa", "zz").FilterText("z")
	})
	pred := blockingCallback(t).Timeout(time.Millisecond, alternative)

	result := pred.Invoke(predict.Context{ToComplete: "k:a"})

	assert.Equal(t, []predict.Item{{Value: "k:zz", Display: "zz"}}, result.Items)
}

func TestInclude(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry").Include("cherry", "apple")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "apple", Display: "apple"}, {Value: "cherry", Display: "cherry"}}, result.Items)
	assert.Equal(t, []string{"apple", "cherry"}, pred.Raw())
}

func TestInclude_IgnoresDisplay(t *testing.T) {
	t.Parallel()

	pred := predict.Items(
		predict.Item{Value: "a", Display: "apple", Description: "fruit"},
		predict.Item{Value: "b", Display: "banana"},
	).Include("apple", "b")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "b", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"b"}, pred.Raw())
}

func TestInclude_Callback(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(predict.Context) predict.Predictor {
		return predict.Values("apple", "banana")
	}).Include("banana")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
}

func TestInclude_Batch(t *testing.T) {
	t.Parallel()

	pred := predict.Batch(predict.Values("apple", "banana"), predict.Values("cherry")).Include("banana", "cherry")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}, {Value: "cherry", Display: "cherry"}}, result.Items)
}

func TestInclude_MultiParts(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(",", func(predict.Context) predict.Predictor {
		return predict.Values("apple", "banana")
	}).Include("banana")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
}

func TestInclude_TimeoutAlternative(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	slow := predict.Callback(func(predict.Context) predict.Predictor {
		<-release

		return predict.None()
	})
	pred := slow.Timeout(time.Millisecond, predict.Values("apple", "banana")).Include("apple")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "apple", Display: "apple"}}, result.Items)
}

func TestInclude_CallsNarrow(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry").Include("apple", "banana").Include("banana", "cherry")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"banana"}, pred.Raw())
}

func TestInclude_NoValuesKeepsNothing(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana").Include()

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
	assert.Equal(t, []string{}, pred.Raw())
}

func TestInclude_NilSliceKeepsNothing(t *testing.T) {
	t.Parallel()

	var allowed []string

	pred := predict.Values("apple", "banana").Include(allowed...)

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
}

func TestInclude_WithExclude(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana", "cherry").Include("apple", "banana").Exclude("apple")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"banana"}, pred.Raw())
}

func TestInclude_AfterPrefix(t *testing.T) {
	t.Parallel()

	pred := predict.Values("1", "2").Prefix("v").Include("v1", "2")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "v1", Display: "1"}}, result.Items)
	assert.Equal(t, []string{"1"}, pred.Raw())
}

func TestInclude_MultiPartsMatchesWholeValue(t *testing.T) {
	t.Parallel()

	pred := predict.MultiParts(":", func(predict.Context) predict.Predictor {
		return predict.Values("latest", "v1")
	}).Include("api:latest", "v1")

	result := pred.Invoke(predict.Context{ToComplete: "api:"})

	assert.Equal(t, []predict.Item{{Value: "api:latest", Display: "latest"}}, result.Items)
}

func TestInclude_ValueNotProducedIsNotOffered(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana").Include("banana", "durian")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "banana", Display: "banana"}}, result.Items)
	assert.Equal(t, []string{"banana"}, pred.Raw())
}

func TestInclude_NoneProducedOffersNothing(t *testing.T) {
	t.Parallel()

	pred := predict.Values("apple", "banana").Include("durian")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
	assert.Equal(t, []string{}, pred.Raw())
}

func TestInclude_MultiPartsKeepsPartialLeadingToValue(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v10")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "api-gateway:", Display: "api-gateway:"}}, result.Items)
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestInclude_MultiPartsKeepsPartialWhileTyping(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v10")

	result := pred.Invoke(predict.Context{ToComplete: "a"})

	assert.Equal(t, []predict.Item{{Value: "api-gateway:", Display: "api-gateway:"}}, result.Items)
}

func TestInclude_MultiPartsLastSegmentDropsValuePrefix(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v10")

	result := pred.Invoke(predict.Context{ToComplete: "api-gateway:v1"})

	assert.Equal(t, []predict.Item{{Value: "api-gateway:v10", Display: "v10"}}, result.Items)
	assert.False(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestInclude_MultiPartsMiddleSegmentKeepsPartial(t *testing.T) {
	t.Parallel()

	pred := threePartPredictor().Include("eu:c2:n1")

	result := pred.Invoke(predict.Context{ToComplete: "eu:"})

	assert.Equal(t, []predict.Item{{Value: "eu:c2:", Display: "c2:"}}, result.Items)
	assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
}

func TestInclude_MultiPartsListsWithoutCommonValueKeepNoPartial(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v1").Include("api-gateway:latest")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
}

func TestInclude_MultiPartsListsWithCommonValueKeepPartial(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v1", "auth:latest").Include("auth:latest")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, []predict.Item{{Value: "auth:", Display: "auth:"}}, result.Items)
}

func TestInclude_MultiPartsValueNotProducedDeadEnds(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v9")

	result := pred.Invoke(predict.Context{ToComplete: "api-gateway:"})

	assert.Equal(t, 0, len(result.Items))
}

func TestInclude_MultiPartsExcludedPartialIsRemoved(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v1").Exclude("api-gateway:")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
}

func TestInclude_WrappedMultiPartsMatchesWholeValues(t *testing.T) {
	t.Parallel()

	pred := predict.Callback(func(predict.Context) predict.Predictor {
		return imagePredictor()
	}).Include("api-gateway:v1")

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 0, len(result.Items))
}

func TestInclude_MultiPartsRawIsEmpty(t *testing.T) {
	t.Parallel()

	pred := imagePredictor().Include("api-gateway:v1")

	values := pred.Raw()

	assert.Equal(t, 0, len(values))
}

func TestPredictor_RawAndTag(t *testing.T) {
	t.Parallel()

	t.Run("Predictor Tag sets default group tag", func(t *testing.T) {
		t.Parallel()

		pred := predict.Values("val1", "val2").Tag("my-tag")
		result := pred.Invoke(predict.Context{})

		assert.Equal(t, 2, len(result.Items))
		assert.Equal(t, "my-tag", result.Items[0].Tag)
		assert.Equal(t, "my-tag", result.Items[1].Tag)
	})

	t.Run("Predictor Raw returns items values when raw slice empty", func(t *testing.T) {
		t.Parallel()

		pred := predict.Items(predict.NewItem("item1", "d1"), predict.NewItem("item2", "d2"))

		assert.Equal(t, []string{"item1", "item2"}, pred.Raw())
	})
}

func TestKeyValueDescribed(t *testing.T) {
	t.Parallel()

	valuesByKey := map[string][]string{
		"http3": {"true", "false"},
		"admin": {"true", "false"},
	}
	descriptions := map[string]string{
		"http3": "Experimental HTTP/3 listener",
		"admin": "Experimental admin API",
	}
	pred := predict.KeyValueDescribed(valuesByKey, descriptions)

	resKeys := pred.Invoke(predict.Context{})

	assert.Equal(t, 2, len(resKeys.Items))
	assert.Equal(t, "admin=", resKeys.Items[0].Value)
	assert.Equal(t, "Experimental admin API", resKeys.Items[0].Description)
	assert.Equal(t, "http3=", resKeys.Items[1].Value)
	assert.Equal(t, "Experimental HTTP/3 listener", resKeys.Items[1].Description)
	assert.True(t, resKeys.Directive.Has(predict.DirectiveNoSpace))

	resValues := pred.Invoke(predict.Context{
		ToComplete: "http3=",
	})

	assert.Equal(t, 2, len(resValues.Items))
	assert.Equal(t, "http3=true", resValues.Items[0].Value)
	assert.Equal(t, "http3=false", resValues.Items[1].Value)
}

func TestKeyValueDescribed_EmptyDescriptionsMatchesKeyValue(t *testing.T) {
	t.Parallel()

	valuesByKey := map[string][]string{
		"team": {"core", "infra"},
	}

	resKeys := predict.KeyValueDescribed(valuesByKey, nil).Invoke(predict.Context{})

	assert.Equal(t, 1, len(resKeys.Items))
	assert.Equal(t, "team=", resKeys.Items[0].Value)
	assert.Equal(t, "", resKeys.Items[0].Description)
	assert.True(t, resKeys.Directive.Has(predict.DirectiveNoSpace))
}

// imagePredictor completes repository:tag values. Its first segment offers
// repositories without NoSpace, so the trailing-space rule is MultiParts' own.
func imagePredictor() predict.Predictor {
	return predict.MultiParts(":", func(predictContext predict.Context) predict.Predictor {
		if len(predictContext.Parts) == 0 {
			return predict.Values("api:", "api-gateway:", "auth:")
		}

		if predictContext.Parts[0] == "api-gateway" {
			return predict.Values("v1", "v10", "latest")
		}

		return predict.Values("latest")
	})
}

// threePartPredictor completes region:cluster:node values.
func threePartPredictor() predict.Predictor {
	return predict.MultiParts(":", func(predictContext predict.Context) predict.Predictor {
		switch len(predictContext.Parts) {
		case 0:
			return predict.Values("eu:", "us:")
		case 1:
			return predict.Values("c1:", "c2:")
		default:
			return predict.Values("n1", "n2")
		}
	})
}

// blockingCallback returns a callback that blocks until the test ends, so a
// Timeout on it always falls back to its alternative.
func blockingCallback(t *testing.T) predict.Predictor {
	t.Helper()

	release := make(chan struct{})

	t.Cleanup(func() { close(release) })

	return predict.Callback(func(predict.Context) predict.Predictor {
		<-release

		return predict.None()
	})
}
