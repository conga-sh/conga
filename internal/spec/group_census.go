package spec

import "github.com/conga-sh/conga/internal/levenshtein"

// GroupCensus counts the members of every flag group declared on one command.
// Group validation is command-scoped, so each command build owns one census.
// Declaration order is preserved so the first offending group is reported
// deterministically. The runtime schema builder and the static analyzer share
// this type as the single source of truth for the single-member policy.
type GroupCensus[T any] struct {
	members map[groupCensusKey]*groupCensusEntry[T]
	order   []groupCensusKey
}

// NewGroupCensus returns an empty census.
func NewGroupCensus[T any]() *GroupCensus[T] {
	return &GroupCensus[T]{members: make(map[groupCensusKey]*groupCensusEntry[T])}
}

// Add records one declared membership; an empty group name is ignored.
func (c *GroupCensus[T]) Add(kind, group, flagName string, isCounter bool, attribution T) {
	if group == "" {
		return
	}

	key := groupCensusKey{kind: kind, group: group}

	entry, ok := c.members[key]
	if !ok {
		entry = &groupCensusEntry[T]{firstFlag: flagName, attribution: attribution, allCounters: true}
		c.members[key] = entry
		c.order = append(c.order, key)
	}

	entry.count++
	entry.allCounters = entry.allCounters && isCounter
}

// SingleMemberGroups returns every group that fails the arity policy, in
// first-declaration order. A one-member oneof on a Counter stays legal: it is
// the tag-declarative way to require at least one occurrence; a one-member
// oneof on an optional flag duplicates RequiredFlag, and one-member xor/and
// groups are always no-ops.
func (c *GroupCensus[T]) SingleMemberGroups() []SingleMemberGroup[T] {
	var violations []SingleMemberGroup[T]

	for _, key := range c.order {
		entry := c.members[key]
		if entry.count >= 2 {
			continue
		}

		if key.kind == "oneof" && entry.allCounters {
			continue
		}

		violations = append(violations, SingleMemberGroup[T]{
			Kind:        key.kind,
			Group:       key.group,
			FlagName:    entry.firstFlag,
			Suggestion:  c.suggest(key),
			Attribution: entry.attribution,
		})
	}

	return violations
}

// SingleMemberGroup records one declared group that fails the arity policy.
// Attribution carries caller metadata: the runtime stores the struct field
// name, the analyzer stores the declaring field object.
type SingleMemberGroup[T any] struct {
	Kind        string
	Group       string
	FlagName    string
	Suggestion  string
	Attribution T
}

// groupCensusKey identifies one declared group on a command. The three group
// tags keep independent namespaces: a oneof group and an xor group may share a
// spelling without sharing members.
type groupCensusKey struct {
	kind  string
	group string
}

// groupCensusEntry records one declared group's membership. The first member
// supplies deterministic attribution for a single-member diagnostic.
type groupCensusEntry[T any] struct {
	count       int
	firstFlag   string
	attribution T
	allCounters bool
}

// suggest offers a populated same-kind group whose spelling is close to the
// single-member group's name.
func (c *GroupCensus[T]) suggest(key groupCensusKey) string {
	var candidates []string

	for _, candidate := range c.order {
		if candidate.kind != key.kind || candidate.group == key.group {
			continue
		}

		// Only a multi-member group is a trustworthy target: a two-flag split
		// leaves both groups with one member, and suggesting either spelling
		// would be arbitrary.
		if c.members[candidate].count < 2 {
			continue
		}

		candidates = append(candidates, candidate.group)
	}

	if len(candidates) == 0 {
		return ""
	}

	return levenshtein.SuggestFor(key.group, candidates)
}
