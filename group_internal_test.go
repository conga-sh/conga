package conga

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

// TestCollectFlagGroups_PreservesDeclarationOrder pins the structural ordering
// that makes multi-group validation failures deterministic: group names follow
// first declaration, and members keep declaration order within each group.
func TestCollectFlagGroups_PreservesDeclarationOrder(t *testing.T) {
	t.Parallel()

	handle := new(OptionalFlag[bool])

	flags := []*spec.Flag{
		{Name: "verbose", Handle: handle, XorGroup: "output"},
		{Name: "quiet", Handle: handle},
		{Name: "json", Handle: handle, XorGroup: "format"},
		{Name: "yaml", Handle: handle, XorGroup: "output"},
		{Name: "builtin", XorGroup: "ignored"},
	}

	groups := collectFlagGroups(flags, func(flagSpec *spec.Flag) string { return flagSpec.XorGroup })

	assert.Equal(t, []string{"output", "format"}, groups.order)

	memberNames := make([]string, 0, len(groups.members["output"]))
	for _, flagSpec := range groups.members["output"] {
		memberNames = append(memberNames, flagSpec.Name)
	}

	assert.Equal(t, []string{"verbose", "yaml"}, memberNames)
	assert.Equal(t, 1, len(groups.members["format"]))
}
