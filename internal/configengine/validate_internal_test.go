package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestFormatConfigValue_DeterministicMapSorting(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"z": 10,
		"a": 1,
		"m": 5,
		"b": 2,
		"k": 3,
		"c": 4,
	}

	expected := "a=1,b=2,c=4,k=3,m=5,z=10"
	for range 50 {
		assert.Equal(t, expected, formatConfigValue(input))
	}
}
