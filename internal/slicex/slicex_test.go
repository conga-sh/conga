package slicex_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/slicex"
)

func TestMap(t *testing.T) {
	t.Parallel()

	input := []int{1, 2, 3, 4}
	mapped := slicex.Map(input, func(n int) int { return n * 2 })
	assert.Equal(t, []int{2, 4, 6, 8}, mapped)

	strInput := []string{"a", "bb", "ccc"}
	lengths := slicex.Map(strInput, func(s string) int { return len(s) })
	assert.Equal(t, []int{1, 2, 3}, lengths)
}

func TestFilter(t *testing.T) {
	t.Parallel()

	input := []int{1, 2, 3, 4, 5, 6}
	evens := slicex.Filter(input, func(n int) bool { return n%2 == 0 })
	assert.Equal(t, []int{2, 4, 6}, evens)
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
