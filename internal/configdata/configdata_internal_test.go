package configdata

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestNew_EmptyDataCarriesNoValues(t *testing.T) {
	t.Parallel()

	data := New()

	_, hasScalar := data.Scalar("port")

	assert.False(t, hasScalar)
	assert.Zero(t, data.Map("tags"))
	assert.False(t, data.HasKey("port"))
	assert.False(t, data.HasKey("tags"))
}

func TestSetScalar_ReadsBackStoredValue(t *testing.T) {
	t.Parallel()

	data := New()

	data.SetScalar("port", "8080", "app.yaml")

	value, hasScalar := data.Scalar("port")

	assert.True(t, hasScalar)
	assert.Equal(t, "8080", value)
	assert.True(t, data.HasKey("port"))

	scalarVal, hasVal := data.ScalarValue("port")
	assert.True(t, hasVal)
	assert.Equal(t, Value{Raw: "8080", Origin: "app.yaml"}, scalarVal)
}

func TestSetScalar_LaterStoreReplacesEarlierValue(t *testing.T) {
	t.Parallel()

	data := New()
	data.SetScalar("port", "8000", "low.yaml")
	data.SetScalar("port", "9500", "high.yaml")

	value, hasScalar := data.Scalar("port")

	assert.True(t, hasScalar)
	assert.Equal(t, "9500", value)

	scalarVal, hasVal := data.ScalarValue("port")
	assert.True(t, hasVal)
	assert.Equal(t, Value{Raw: "9500", Origin: "high.yaml"}, scalarVal)
}

func TestEnsureMap_SharedAcrossAliasKeys(t *testing.T) {
	t.Parallel()

	data := New()

	aliases := []string{"remote.pairs", "pairs"}
	entries := data.EnsureMap(aliases)
	entries["a"] = Value{Raw: "1", Origin: "sys.toml"}

	remote := data.Map("remote.pairs")
	bare := data.Map("pairs")

	assert.Equal(t, 1, len(remote))
	assert.Equal(t, 1, len(bare))
	assert.Equal(t, Value{Raw: "1", Origin: "sys.toml"}, remote["a"])
	assert.Equal(t, Value{Raw: "1", Origin: "sys.toml"}, bare["a"])

	// Later write through one alias mutates the shared map seen by all aliases.
	entries2 := data.EnsureMap(aliases)
	entries2["b"] = Value{Raw: "2", Origin: "local.toml"}

	assert.Equal(t, 2, len(data.Map("remote.pairs")))
	assert.Equal(t, 2, len(data.Map("pairs")))
}

func TestEnsureMap_LaterKeysOverwriteEarlierInPlace(t *testing.T) {
	t.Parallel()

	data := New()

	aliases := []string{"tags"}
	entries := data.EnsureMap(aliases)
	entries["shared"] = Value{Raw: "sys", Origin: "sys.toml"}
	entries["onlyLow"] = Value{Raw: "1", Origin: "sys.toml"}

	// Overwrite shared key with higher-precedence file.
	entries["shared"] = Value{Raw: "local", Origin: "local.toml"}
	entries["onlyHigh"] = Value{Raw: "2", Origin: "local.toml"}

	tags := data.Map("tags")

	assert.Equal(t, 3, len(tags))
	assert.Equal(t, Value{Raw: "1", Origin: "sys.toml"}, tags["onlyLow"])
	assert.Equal(t, Value{Raw: "local", Origin: "local.toml"}, tags["shared"])
	assert.Equal(t, Value{Raw: "2", Origin: "local.toml"}, tags["onlyHigh"])
}

func TestEnsureMap_EmptyMapSectionSetsHasKeyTrue(t *testing.T) {
	t.Parallel()

	data := New()

	data.EnsureMap([]string{"tags"})

	assert.True(t, data.HasKey("tags"))
	assert.Equal(t, 0, len(data.Map("tags")))
}
