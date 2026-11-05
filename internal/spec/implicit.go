package spec

import (
	"slices"

	"github.com/conga-sh/conga/internal/tokenizer"
)

// ImplicitFlagDepth returns the number of implicit descendant levels required
// to resolve the flag tokens in tokens. The chain's leaf is the current command.
// It is the completion walker's entry point; the binder's terminal implicit
// selection computes the same depth through flagDepthInLevels inside
// TerminalImplicitLevels. Both surfaces therefore share one scan and cannot
// drift: flags already declared by the routed chain, built-ins, and flags
// declared nowhere contribute nothing, and tokens after the -- terminator are
// operands that never select a command.
func ImplicitFlagDepth(chain []*Command, tokens []tokenizer.Token) int {
	if len(chain) == 0 || !hasFlagTokenBeforeTerminator(tokens) {
		return 0
	}

	levels := implicitDescendants(chain[len(chain)-1])
	if len(levels) == 0 {
		return 0
	}

	return flagDepthInLevels(chain, implicitLevelChains(chain, levels), tokens)
}

// TerminalImplicitLevels returns the number of implicit descendant levels the
// terminal route enters. The chain's leaf is the current command. A runnable
// command runs itself unless a flag declared only by its implicit descendants
// selects a deeper level; a command that does not own the operands after the --
// terminator forwards them to its implicit child. A built-in rendering request
// counts only on the command where the built-in is visible, so a root-only
// --version never keeps a selected implicit child active. Callers perform the
// entry side effects for the returned levels.
func TerminalImplicitLevels(chain []*Command, tokens []tokenizer.Token, terminatorOperands bool) int {
	if len(chain) == 0 {
		return 0
	}

	current := chain[len(chain)-1]

	levels := implicitDescendants(current)
	if len(levels) == 0 {
		return 0
	}

	levelChains := implicitLevelChains(chain, levels)
	requiredDepth := flagDepthInLevels(chain, levelChains, tokens)
	entered := 0

	for {
		implicit := current.ImplicitCommand()
		if implicit == nil {
			return entered
		}

		activeChain := chain
		if entered > 0 {
			activeChain = levelChains[entered-1]
		}

		ownsTerminatorOperands := current.DashArgs != nil || !terminatorOperands
		// levelChains is relative to the routed chain; slice to the levels
		// below activeChain so resolveShortFlagInChain's depth stays relative to the
		// chain in hand.
		if entered >= requiredDepth &&
			(builtinRequestedAt(activeChain, levelChains[entered:], tokens) ||
				(current.IsRunnable && ownsTerminatorOperands)) {
			return entered
		}

		current = implicit
		entered++
	}
}

// implicitDescendants returns the implicit command chain below cmd, nearest
// first. Schema build accepts at most one implicit command per command.
func implicitDescendants(cmd *Command) []*Command {
	var descendants []*Command

	for implicit := cmd.ImplicitCommand(); implicit != nil; implicit = implicit.ImplicitCommand() {
		descendants = append(descendants, implicit)
	}

	return descendants
}

// implicitLevelChains returns the command chain active after entering each
// implicit level below chain's leaf.
func implicitLevelChains(chain []*Command, levels []*Command) [][]*Command {
	levelChains := make([][]*Command, len(levels))
	cumulativeChain := slices.Clone(chain)

	for depth, level := range levels {
		cumulativeChain = append(cumulativeChain, level)
		levelChains[depth] = slices.Clone(cumulativeChain)
	}

	return levelChains
}

// flagDepthInLevels reports the deepest implicit level required by tokens. It
// follows the token parser's arity handling, so a value consumed by a preceding
// flag is never mistaken for a flag itself.
func flagDepthInLevels(chain []*Command, levelChains [][]*Command, tokens []tokenizer.Token) int {
	requiredDepth := 0

	for position := 0; position < len(tokens); position++ {
		tok := tokens[position]

		if tok.Type == tokenizer.TokenTypeTerminator {
			break
		}

		consumesNext, depth := flagTokenDepth(chain, levelChains, tok)
		if depth > requiredDepth {
			requiredDepth = depth
		}

		if consumesNext && position+1 < len(tokens) && tokens[position+1].CanBindAsFlagValue() {
			position++
		}
	}

	return requiredDepth
}

// builtinRequestedAt reports whether tokens request a built-in rendering flag,
// before the -- terminator, that is visible on chain. Declared user flags shadow
// the built-ins through FindFlagInChain and FindShortFlagInChain. levelChains
// holds the implicit levels strictly below chain; it supplies the arity of
// shorthands declared only there, so a value-taking rune ends the cluster
// exactly as the parser does.
func builtinRequestedAt(chain []*Command, levelChains [][]*Command, tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		switch tok.Type {
		case tokenizer.TokenTypeTerminator:
			return false
		case tokenizer.TokenTypeLongFlag:
			if IsBuiltinFlag(FindFlagInChain(chain, tok.Name)) {
				return true
			}
		case tokenizer.TokenTypeShortFlag:
			if shortClusterRequestsBuiltin(chain, levelChains, tok) {
				return true
			}
		case tokenizer.TokenTypePositional, tokenizer.TokenTypeUnknown:
			continue
		}
	}

	return false
}

// shortClusterRequestsBuiltin reports whether a shorthand cluster binds the
// built-in help or version flag. WalkShorthand stops at a rune that takes a
// value (it consumes the rest of the cluster) and at an unresolved rune, so
// neither can reveal a later built-in rune. levelChains holds the implicit
// levels strictly below chain.
func shortClusterRequestsBuiltin(chain []*Command, levelChains [][]*Command, tok tokenizer.Token) bool {
	walk := WalkShorthand(tok.ShortRunes, tok.HasAttachedValue, func(r rune) (*Flag, int) {
		return resolveShortFlagInChain(chain, levelChains, r)
	})

	return slices.ContainsFunc(walk.Steps, func(step ShorthandStep) bool {
		return IsBuiltinShortFlag(step.Flag)
	})
}

// hasFlagTokenBeforeTerminator reports whether a flag token before the --
// terminator can still select an implicit command.
func hasFlagTokenBeforeTerminator(tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		if tok.Type == tokenizer.TokenTypeTerminator {
			return false
		}

		if tok.Type == tokenizer.TokenTypeLongFlag || tok.Type == tokenizer.TokenTypeShortFlag {
			return true
		}
	}

	return false
}

// flagTokenDepth reports the first implicit level at which tok resolves, 0 when
// it already resolves on the routed chain, and 0 when it resolves nowhere. It
// also reports whether the token consumes the following token as a flag value,
// mirroring the token parser's arity handling so a value is never mistaken for
// a flag.
func flagTokenDepth(chain []*Command, levelChains [][]*Command, tok tokenizer.Token) (bool, int) {
	switch tok.Type {
	case tokenizer.TokenTypeLongFlag:
		if flagSpec := declaredCLILongFlag(chain, tok.Name); flagSpec != nil {
			return consumesNextFlagValue(flagSpec, tok), 0
		}

		if IsBuiltinFlag(FindFlagInChain(chain, tok.Name)) {
			return false, 0
		}

		for depth, levelChain := range levelChains {
			if flagSpec := declaredCLILongFlag(levelChain, tok.Name); flagSpec != nil {
				return consumesNextFlagValue(flagSpec, tok), depth + 1
			}
		}

		return false, 0
	case tokenizer.TokenTypeShortFlag:
		return shortClusterDepth(chain, levelChains, tok)
	default:
		return false, 0
	}
}

// shortClusterDepth walks a shorthand cluster through WalkShorthand, the token
// parser's arity contract, reporting the deepest implicit level any resolved
// rune requires and whether the cluster consumes the following token as a
// value. A negative number is an operand unless its first rune is registered at
// some level. Built-in help and version runes never end the scan (the parser
// resolves them after implicit selection), while an unresolved rune does.
func shortClusterDepth(chain []*Command, levelChains [][]*Command, tok tokenizer.Token) (bool, int) {
	runes := tok.ShortRunes
	if len(runes) == 0 {
		return false, 0
	}

	if tok.IsNegativeNumber() && !shortFlagRegistered(chain, levelChains, runes[0]) {
		return false, 0
	}

	walk := WalkShorthand(runes, tok.HasAttachedValue, func(r rune) (*Flag, int) {
		return resolveShortFlagInChain(chain, levelChains, r)
	})

	requiredDepth := 0
	for _, step := range walk.Steps {
		requiredDepth = max(requiredDepth, step.ImplicitLevel)
	}

	return walk.ConsumesNext, requiredDepth
}

// shortFlagRegistered reports whether r is declared as a shorthand on the
// routed chain, is a built-in shorthand, or is declared anywhere in its
// implicit descendants.
func shortFlagRegistered(chain []*Command, levelChains [][]*Command, r rune) bool {
	flagSpec, _ := resolveShortFlagInChain(chain, levelChains, r)

	return flagSpec != nil
}

// resolveShortFlagInChain returns the flag declared for r and the implicit level that
// first declares it: 0 for chain or a built-in, 1 for the nearest level in
// levelChains. levelChains[i] must be the chain after entering i+1 levels below
// chain; the returned depth is therefore relative to chain.
func resolveShortFlagInChain(chain []*Command, levelChains [][]*Command, r rune) (*Flag, int) {
	if flagSpec := findCLIShortFlagInChain(chain, r); flagSpec != nil {
		return flagSpec, 0
	}

	if flagSpec := FindShortFlagInChain(chain, r); IsBuiltinShortFlag(flagSpec) {
		return flagSpec, 0
	}

	for depth, levelChain := range levelChains {
		if flagSpec := findCLIShortFlagInChain(levelChain, r); flagSpec != nil {
			return flagSpec, depth + 1
		}
	}

	return nil, 0
}

// consumesNextFlagValue reports whether a parsed flag takes its value from the
// next token rather than an attached =value.
func consumesNextFlagValue(flagSpec *Flag, tok tokenizer.Token) bool {
	return flagSpec.TakesValue() && !tok.HasAttachedValue
}

// declaredCLILongFlag resolves a long flag name or alias on chain, honoring the
// no- negation form. NoCLI engine flags are never CLI-routable, so they do not
// contribute depth.
func declaredCLILongFlag(chain []*Command, name string) *Flag {
	if flagSpec := findCLIFlagInChain(chain, name); flagSpec != nil {
		return flagSpec
	}

	return ResolveNegatedFlag(name, func(candidate string) *Flag {
		return findCLIFlagInChain(chain, candidate)
	})
}
