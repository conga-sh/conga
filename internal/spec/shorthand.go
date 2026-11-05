package spec

// ShorthandStep records one resolved rune of a shorthand cluster. Its index in
// ShorthandWalk.Steps is the index of the rune it resolves.
type ShorthandStep struct {
	// Flag is the flag the rune resolves to.
	Flag *Flag
	// ImplicitLevel is the implicit level that first declares Flag: 0 for the
	// routed chain or a built-in.
	ImplicitLevel int
	// ValueStart reports that this rune takes the cluster remainder, the
	// attached value, or the next token as its value. It is always the last
	// step.
	ValueStart bool
}

// ShorthandWalk is the arity classification of one shorthand cluster. It is the
// single source of the cluster arity rules shared by the token parser, routing
// lookahead, implicit-depth scanning, and the completion walker.
type ShorthandWalk struct {
	// Steps holds one entry per resolved rune, in rune order, up to and
	// including the first value-taking rune.
	Steps []ShorthandStep
	// ConsumesNext reports whether the cluster takes the following token as a
	// value: its final rune takes a value and no value is attached.
	ConsumesNext bool
	// UnresolvedAt is the index of the first rune that resolved to no flag, or
	// -1 when every visited rune resolved.
	UnresolvedAt int
}

// WalkShorthand resolves runes in order, stopping at the first value-taking or
// unresolved rune. resolve returns the flag and the implicit level that first
// declares it (0 for the routed chain or a built-in), or a nil flag when the
// rune is not registered. hasAttachedValue mirrors the parser's arity rule: a
// final value-taking rune consumes the next token only when no value is
// attached. Runes after a value-taking rune are part of its value and are never
// resolved. The walker classifies only; callers bind values themselves.
func WalkShorthand(runes []rune, hasAttachedValue bool, resolve func(rune) (*Flag, int)) ShorthandWalk {
	walk := ShorthandWalk{
		Steps:        make([]ShorthandStep, 0, len(runes)),
		UnresolvedAt: -1,
	}

	for position, r := range runes {
		flagSpec, level := resolve(r)
		if flagSpec == nil {
			walk.UnresolvedAt = position

			return walk
		}

		valueStart := flagSpec.TakesValue()
		walk.Steps = append(walk.Steps, ShorthandStep{
			Flag:          flagSpec,
			ImplicitLevel: level,
			ValueStart:    valueStart,
		})

		if valueStart {
			walk.ConsumesNext = position == len(runes)-1 && !hasAttachedValue

			return walk
		}
	}

	return walk
}
