package spec

import (
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/tokenizer"
)

// ImplicitFlagDepth returns the number of implicit descendant levels required
// to resolve the flag tokens in tokens. It is the single authority shared by the
// binder's terminal implicit selection and the completion walker, so routing and
// candidate generation cannot drift: flags already declared by the routed chain,
// built-ins, and flags declared nowhere contribute nothing, and tokens after the
// -- terminator are operands that never select a command.
func ImplicitFlagDepth(current *Command, chain []*Command, tokens []tokenizer.Token) int {
	if !hasFlagTokenBeforeTerminator(tokens) {
		return 0
	}

	levels := implicitDescendants(current)
	if len(levels) == 0 {
		return 0
	}

	levelChains := make([][]*Command, len(levels))
	cumulativeChain := slices.Clone(chain)

	for depth, level := range levels {
		cumulativeChain = append(cumulativeChain, level)
		levelChains[depth] = slices.Clone(cumulativeChain)
	}

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

// implicitDescendants returns the implicit command chain below cmd, nearest
// first. Schema build accepts at most one implicit command per command.
func implicitDescendants(cmd *Command) []*Command {
	var descendants []*Command

	for implicit := cmd.ImplicitCommand(); implicit != nil; implicit = implicit.ImplicitCommand() {
		descendants = append(descendants, implicit)
	}

	return descendants
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

		if isBuiltinLongFlag(chain, tok.Name) {
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

// shortClusterDepth walks a shorthand cluster the way the token parser does,
// reporting the deepest implicit level any resolved rune requires and whether
// the cluster consumes the following token as a value. A negative number is an
// operand unless its first rune is registered at some level. Built-in help and
// version runes never end the scan (the parser resolves them after implicit
// selection), while an unresolved rune does.
func shortClusterDepth(chain []*Command, levelChains [][]*Command, tok tokenizer.Token) (bool, int) {
	runes := tok.ShortRunes
	if len(runes) == 0 {
		return false, 0
	}

	if tok.IsNegativeNumber() && !shortFlagRegistered(chain, levelChains, runes[0]) {
		return false, 0
	}

	requiredDepth := 0

	for position, r := range runes {
		flagSpec, depth := resolveShortFlag(chain, levelChains, r)
		if flagSpec == nil {
			if isBuiltinShortFlag(chain, r) {
				continue
			}

			return false, requiredDepth
		}

		if depth > requiredDepth {
			requiredDepth = depth
		}

		if flagSpec.IsBool || flagSpec.IsCounter {
			continue
		}

		return position == len(runes)-1 && !tok.HasAttachedValue, requiredDepth
	}

	return false, requiredDepth
}

// isBuiltinLongFlag reports whether name resolves to a built-in flag on the
// routed chain.
func isBuiltinLongFlag(chain []*Command, name string) bool {
	flagSpec := FindFlagInChain(chain, name)

	return flagSpec == BuiltinHelpFlag || flagSpec == BuiltinVersionFlag ||
		flagSpec == BuiltinWhyFlag || flagSpec == BuiltinCompletionFlag
}

// isBuiltinShortFlag reports whether r is assigned to the built-in help or
// version flag on the routed chain. Both built-ins are boolean flags, so a
// cluster scan continues past them.
func isBuiltinShortFlag(chain []*Command, r rune) bool {
	flagSpec := FindShortFlagInChain(chain, r)

	return flagSpec == BuiltinHelpFlag || flagSpec == BuiltinVersionFlag
}

// shortFlagRegistered reports whether r is declared as a shorthand on the
// routed chain, is a built-in shorthand, or is declared anywhere in its
// implicit descendants.
func shortFlagRegistered(chain []*Command, levelChains [][]*Command, r rune) bool {
	if findDeclaredCLIShortFlagInChain(chain, r) != nil || isBuiltinShortFlag(chain, r) {
		return true
	}

	for _, levelChain := range levelChains {
		if findDeclaredCLIShortFlagInChain(levelChain, r) != nil {
			return true
		}
	}

	return false
}

// resolveShortFlag returns the flag declared for r and the implicit level that
// first declares it: 0 for the routed chain or a built-in, 1 for its nearest
// implicit child.
func resolveShortFlag(chain []*Command, levelChains [][]*Command, r rune) (*Flag, int) {
	if flagSpec := findDeclaredCLIShortFlagInChain(chain, r); flagSpec != nil {
		return flagSpec, 0
	}

	if flagSpec := FindShortFlagInChain(chain, r); flagSpec == BuiltinHelpFlag || flagSpec == BuiltinVersionFlag {
		return flagSpec, 0
	}

	for depth, levelChain := range levelChains {
		if flagSpec := findDeclaredCLIShortFlagInChain(levelChain, r); flagSpec != nil {
			return flagSpec, depth + 1
		}
	}

	return nil, 0
}

// consumesNextFlagValue reports whether a parsed flag takes its value from the
// next token rather than an attached =value.
func consumesNextFlagValue(flagSpec *Flag, tok tokenizer.Token) bool {
	return !flagSpec.IsBool && !flagSpec.IsCounter && !tok.HasAttachedValue
}

// declaredCLILongFlag resolves a long flag name or alias on chain, honoring the
// no- negation form. NoCLI engine flags are never CLI-routable, so they do not
// contribute depth.
func declaredCLILongFlag(chain []*Command, name string) *Flag {
	if flagSpec := findDeclaredCLIFlagInChain(chain, name); flagSpec != nil {
		return flagSpec
	}

	positiveName, cut := strings.CutPrefix(name, "no-")
	if !cut {
		return nil
	}

	flagSpec := findDeclaredCLIFlagInChain(chain, positiveName)
	if !CanNegate(flagSpec) {
		return nil
	}

	return flagSpec
}

// findDeclaredCLIFlagInChain searches backwards through chain for a CLI-routable
// declared flag matching name or alias.
func findDeclaredCLIFlagInChain(chain []*Command, name string) *Flag {
	for _, cmd := range slices.Backward(chain) {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.NoCLI {
				continue
			}

			if flagSpec.Name == name || slices.Contains(flagSpec.Aliases, name) {
				return flagSpec
			}
		}
	}

	return nil
}

// findDeclaredCLIShortFlagInChain searches backwards through chain for a
// CLI-routable declared flag matching shorthand r.
func findDeclaredCLIShortFlagInChain(chain []*Command, r rune) *Flag {
	for _, cmd := range slices.Backward(chain) {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.NoCLI || flagSpec.Short == 0 {
				continue
			}

			if flagSpec.Short == r {
				return flagSpec
			}
		}
	}

	return nil
}
