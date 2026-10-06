package predict_test

import (
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

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
	assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
	assert.Equal(t, []string{"apple", "banana", "cherry"}, pred.Raw())
}

func TestItems(t *testing.T) {
	t.Parallel()

	items := []predict.Item{
		predict.NewItem("json", "JavaScript Object Notation").WithStyle("green"),
		predict.NewItem("yaml", "YAML Ain't Markup Language"),
	}
	pred := predict.Items(items...)

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 2, len(result.Items))
	assert.Equal(t, "json", result.Items[0].Value)
	assert.Equal(t, "JavaScript Object Notation", result.Items[0].Description)
	assert.Equal(t, "green", result.Items[0].Style)
	assert.Equal(t, "yaml", result.Items[1].Value)
	assert.Equal(t, "YAML Ain't Markup Language", result.Items[1].Description)
	assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
}

func TestMap(t *testing.T) {
	t.Parallel()

	m := map[string]string{
		"debug": "Enable debug logging",
		"info":  "Informational messages",
		"error": "Error messages only",
	}
	pred := predict.Map(m)

	result := pred.Invoke(predict.Context{})

	assert.Equal(t, 3, len(result.Items))
	assert.Equal(t, "debug", result.Items[0].Value)
	assert.Equal(t, "Enable debug logging", result.Items[0].Description)
	assert.Equal(t, "error", result.Items[1].Value)
	assert.Equal(t, "Error messages only", result.Items[1].Description)
	assert.Equal(t, "info", result.Items[2].Value)
	assert.Equal(t, "Informational messages", result.Items[2].Description)
	assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
}

func TestKeyValue(t *testing.T) {
	t.Parallel()

	valuesByKey := map[string][]string{
		"team":  {"core", "frontend", "infra"},
		"tier":  {"frontend", "backend", "database"},
		"owner": {"alice", "bob", "carol"},
	}
	pred := predict.KeyValue(valuesByKey)

	resKeys := pred.Invoke(predict.Context{})

	assert.Equal(t, 3, len(resKeys.Items))
	assert.Equal(t, "owner=", resKeys.Items[0].Value)
	assert.Equal(t, "team=", resKeys.Items[1].Value)
	assert.Equal(t, "tier=", resKeys.Items[2].Value)
	assert.True(t, resKeys.Directive.Has(predict.DirectiveNoSpace))

	// 4. Act - Value completion phase
	resValues := pred.Invoke(predict.Context{
		ToComplete: "team=",
	})

	// 5. Assert values
	assert.Equal(t, 3, len(resValues.Items))
	assert.Equal(t, "team=core", resValues.Items[0].Value)
	assert.Equal(t, "team=frontend", resValues.Items[1].Value)
	assert.Equal(t, "team=infra", resValues.Items[2].Value)

	// 6. Act - Unknown key
	resUnknown := pred.Invoke(predict.Context{
		ToComplete: "unknown=",
	})
	assert.Zero(t, resUnknown.Items)

	// 7. Act - Key deduplication from ParentParts (e.g. following a comma in slice context)
	resDeduplicatedKeys := pred.Invoke(predict.Context{
		ParentParts: []string{"owner=alice"},
	})
	assert.Equal(t, 2, len(resDeduplicatedKeys.Items))
	assert.Equal(t, "team=", resDeduplicatedKeys.Items[0].Value)
	assert.Equal(t, "tier=", resDeduplicatedKeys.Items[1].Value)

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

func TestNone(t *testing.T) {
	t.Parallel()

	pred := predict.None()

	result := pred.Invoke(predict.Context{})

	assert.Zero(t, result.Items)
	assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
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
		assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
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
		assert.True(t, result.Directive.Has(predict.DirectiveNoSpace))
	})
}

func TestBatch(t *testing.T) {
	t.Parallel()

	t.Run("items and directives aggregation", func(t *testing.T) {
		t.Parallel()

		pred1 := predict.Values("apple", "banana")
		pred2 := predict.Values("banana", "cherry").NoSpace()
		batch := predict.Batch(pred1, pred2)

		result := batch.Invoke(predict.Context{})

		assert.Equal(t, 3, len(result.Items))
		assert.Equal(t, "apple", result.Items[0].Value)
		assert.Equal(t, "banana", result.Items[1].Value)
		assert.Equal(t, "cherry", result.Items[2].Value)
		assert.True(t, result.Directive.Has(predict.DirectiveNoFileComp))
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
		assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
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
		assert.Equal(t, predict.DirectiveNoFileComp, result.Directive)
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
		raw := pred.Raw()

		assert.Equal(t, []string{"item1", "item2"}, raw)
	})
}
