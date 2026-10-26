package slicex_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/slicex"
)

func TestMap(t *testing.T) {
	t.Parallel()

	input := []int{1, 2, 3, 4}
	assert.Equal(t, []int{2, 4, 6, 8}, slicex.Map(input, func(n int) int { return n * 2 }))

	strInput := []string{"a", "bb", "ccc"}
	assert.Equal(t, []int{1, 2, 3}, slicex.Map(strInput, func(s string) int { return len(s) }))
}

func TestFilter(t *testing.T) {
	t.Parallel()

	input := []int{1, 2, 3, 4, 5, 6}
	assert.Equal(t, []int{2, 4, 6}, slicex.Filter(input, func(n int) bool { return n%2 == 0 }))
}

func TestSplitAndTrim(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string(nil), slicex.SplitAndTrim("", ","))
	assert.Equal(t, []string(nil), slicex.SplitAndTrim("   ", ","))
	assert.Equal(t, []string(nil), slicex.SplitAndTrim(",,,", ","))
	assert.Equal(t, []string{"a", "b", "c"}, slicex.SplitAndTrim("a,b,c", ","))
	assert.Equal(t, []string{"a", "b", "c"}, slicex.SplitAndTrim(" a ,  b , c  ", ","))
	assert.Equal(t, []string{"foo", "bar"}, slicex.SplitAndTrim("foo,,bar", ","))
	assert.Equal(t, []string{"host1", "host2"}, slicex.SplitAndTrim("host1:host2", ":"))
}

func TestUnique(t *testing.T) {
	t.Parallel()

	input := []string{"a", "b", "a", "c", "b"}

	assert.Equal(t, []string{"a", "b", "c"}, slicex.Unique(input))
}

func TestUniqueBy(t *testing.T) {
	t.Parallel()

	type item struct {
		id   int
		name string
	}

	input := []item{
		{id: 1, name: "first"},
		{id: 2, name: "second"},
		{id: 1, name: "duplicate"},
	}

	result := slicex.UniqueBy(input, func(i item) int { return i.id })

	assert.Equal(t, []item{{id: 1, name: "first"}, {id: 2, name: "second"}}, result)
}
