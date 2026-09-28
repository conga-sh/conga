package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

func TestDefaultBuilderCallOrderCoversAllAppMethods(t *testing.T) {
	t.Parallel()

	order := defaultBuilderCallOrder()

	appType := reflect.TypeFor[*conga.App[int]]()

	methodNames := make([]string, 0, appType.NumMethod()+2)

	for method := range appType.Methods() {
		methodNames = append(methodNames, method.Name)
	}

	// Reflection omits method-level generic methods (RegisterType[U],
	// RegisterMap[M, K, V]), so the expected set adds them explicitly.
	methodNames = append(methodNames, "RegisterMap", "RegisterType")

	slices.Sort(methodNames)

	sortedOrder := slices.Sorted(slices.Values(order))
	uniqueCount := len(slices.Compact(slices.Clone(sortedOrder)))

	assert.Equal(t, len(sortedOrder), uniqueCount)
	assert.Equal(t, methodNames, sortedOrder)
}

func TestParseBuilderCallOrder(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		specValue string
		wantOrder []string
		wantErr   bool
	}{
		{name: "empty selects the default order", specValue: "", wantOrder: defaultBuilderCallOrder()},
		{name: "comma separated", specValue: "Version, Name", wantOrder: []string{"Version", "Name"}},
		{name: "newlines and comments", specValue: "# identity\nName\nVersion # order\n", wantOrder: []string{"Name", "Version"}},
		{name: "unknown method", specValue: "Nope", wantErr: true},
		{name: "duplicate method", specValue: "Name,Name", wantErr: true},
		{name: "comments only", specValue: "# nothing here", wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			order, err := parseBuilderCallOrder(testCase.specValue)

			if testCase.wantErr {
				assert.Error(t, err)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.wantOrder, order)
		})
	}
}

func TestParseBuilderCallOrderFile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	filename := filepath.Join(directory, "order.txt")

	assert.NoError(t, os.WriteFile(filename, []byte("# custom order\nVersion\n\nName\n"), 0o600))

	order, err := parseBuilderCallOrder("@" + filename)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Version", "Name"}, order)

	_, err = parseBuilderCallOrder("@" + filepath.Join(directory, "missing.txt"))

	assert.Error(t, err)
}

func TestBuilderOrderSpecSet(t *testing.T) {
	t.Parallel()

	spec := &builderOrderSpec{}

	assert.NoError(t, spec.Set("Version, Name"))
	assert.Equal(t, "Version,Name", spec.String())

	assert.Error(t, spec.Set("Nope"))
	assert.Equal(t, "Version,Name", spec.String())
}

func TestBuilderOrderGroupSwaps(t *testing.T) {
	t.Parallel()

	t.Run("sorts participants by rank", func(t *testing.T) {
		t.Parallel()

		slots := []builderOrderSlot{
			{name: "Version", rank: 3, start: 10},
			{name: "Name", rank: 1, start: 20},
			{name: "Description", rank: 2, start: 30},
		}

		swaps := builderOrderGroupSwaps(slots)

		assert.Equal(t, 3, len(swaps))
		assert.Equal(t, "Version", swaps[0].slot.name)
		assert.Equal(t, "Name", swaps[0].source.name)
		assert.Equal(t, "Name", swaps[1].slot.name)
		assert.Equal(t, "Description", swaps[1].source.name)
		assert.Equal(t, "Description", swaps[2].slot.name)
		assert.Equal(t, "Version", swaps[2].source.name)
	})

	t.Run("barriers pin partitions", func(t *testing.T) {
		t.Parallel()

		slots := []builderOrderSlot{
			{name: "Version", rank: 3, start: 10},
			{name: "Name", rank: 1, start: 20},
			{name: "Use", rank: 20, start: 30, barrier: true},
			{name: "Args", rank: 22, start: 40},
		}

		swaps := builderOrderGroupSwaps(slots)

		assert.Equal(t, 2, len(swaps))
		assert.Equal(t, "Version", swaps[0].slot.name)
		assert.Equal(t, "Name", swaps[0].source.name)
		assert.Equal(t, "Name", swaps[1].slot.name)
		assert.Equal(t, "Version", swaps[1].source.name)
	})

	t.Run("ordered group needs no swaps", func(t *testing.T) {
		t.Parallel()

		slots := []builderOrderSlot{
			{name: "Name", rank: 1, start: 10},
			{name: "Version", rank: 3, start: 20},
		}

		swaps := builderOrderGroupSwaps(slots)

		assert.Equal(t, 0, len(swaps))
	})
}
