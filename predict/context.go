package predict

import (
	"os"
	"slices"
)

// Context provides contextual data and metadata about the current command line invocation being completed.
type Context struct {
	// ToComplete is the word fragment under the cursor being completed. The
	// implicit filtering term of a Predictor is derived from it unless
	// FilterText has been called with a non-empty query.
	ToComplete string

	// Args holds the positional arguments parsed before ToComplete for the
	// target command, in order.
	Args []string

	// Parts holds the already-completed segments of a MultiParts value (e.g.
	// the key segment while completing key=val). The engine leaves it empty: it
	// is populated by MultiParts for the nested predictor only.
	Parts []string

	// ParentParts holds the Parts segments of the enclosing MultiParts
	// invocation. Like Parts, it is populated by MultiParts for the nested
	// predictor, not by the engine.
	ParentParts []string

	// Flags maps each supplied flag name (including aliases and short forms)
	// to the last supplied value. The engine populates it together with
	// FlagMultiValues; the accessors treat FlagMultiValues as authoritative
	// when both are set. Use FlagValue to read it with a fallback.
	Flags map[string]string

	// FlagMultiValues maps each supplied flag name (including aliases and
	// short forms) to all supplied values in order. An empty entry counts as
	// absent. Use FlagValues to read it.
	FlagMultiValues map[string][]string

	// Dir is the working directory used by filesystem predictors to resolve
	// relative paths (empty means the process working directory). The App
	// engine currently leaves it empty; SDK callers may set it explicitly.
	Dir string

	// Env holds the environment variables visible to the completion run,
	// consulted by Getenv with a fallback to os.Getenv for absent keys. The
	// App engine currently leaves it nil.
	Env map[string]string

	// EnvLookup queries the environment variables visible to the completion run.
	// When non-nil, Getenv consults this callback before checking Env and falling
	// back to os.Getenv.
	EnvLookup func(key string) (string, bool)
}

// Arg returns the positional argument at the given 0-based index, or an empty string if out of bounds.
func (c Context) Arg(index int) string {
	if index < 0 || index >= len(c.Args) {
		return ""
	}

	return c.Args[index]
}

// FlagValue returns the value of the named flag, or fallback if the flag is
// absent. It shares one precedence with FlagValues and HasFlag: the
// multi-value map is authoritative when both maps carry the flag.
func (c Context) FlagValue(name string, fallback string) string {
	values := c.flagValues(name)
	if len(values) == 0 {
		return fallback
	}

	return values[len(values)-1]
}

// FlagValues returns all values supplied for the named flag, or nil if absent.
// It shares one precedence with FlagValue and HasFlag: the multi-value map is
// authoritative when both maps carry the flag. The returned slice is a clone,
// so mutating it does not affect the context.
func (c Context) FlagValues(name string) []string {
	values := c.flagValues(name)
	if values == nil {
		return nil
	}

	return slices.Clone(values)
}

// WithFlagValues returns a shallow copy of the context with FlagMultiValues populated.
// If Flags is nil and flagValues is non-empty, Flags is also populated from the last entry of each flag.
func (c Context) WithFlagValues(flagValues map[string][]string) Context {
	c.FlagMultiValues = flagValues
	if c.Flags == nil && len(flagValues) > 0 {
		c.Flags = make(map[string]string, len(flagValues))
		for name, values := range flagValues {
			if len(values) > 0 {
				c.Flags[name] = values[len(values)-1]
			}
		}
	}

	return c
}

// Getenv returns the value of the environment variable named by key from the context's environment,
// checking EnvLookup first (if non-nil), then the Env map, falling back to os.Getenv when the key is absent.
func (c Context) Getenv(key string) string {
	if c.EnvLookup != nil {
		if value, ok := c.EnvLookup(key); ok {
			return value
		}
	}

	if c.Env != nil {
		if value, ok := c.Env[key]; ok {
			return value
		}
	}

	return os.Getenv(key)
}

// HasFlag reports whether a flag with the given name was supplied. It shares
// one precedence with FlagValue and FlagValues: the multi-value map is
// authoritative when both maps carry the flag.
func (c Context) HasFlag(name string) bool {
	return len(c.flagValues(name)) > 0
}

// LastArg returns the last positional argument, or an empty string if no arguments are present.
func (c Context) LastArg() string {
	if len(c.Args) == 0 {
		return ""
	}

	return c.Args[len(c.Args)-1]
}

// flagValues returns the values supplied for name under the shared accessor
// precedence: the multi-value map first (an empty entry counts as absent),
// then the scalar map as a single value. Nil is returned when the flag is
// absent from both maps.
func (c Context) flagValues(name string) []string {
	if c.FlagMultiValues != nil {
		if values := c.FlagMultiValues[name]; len(values) > 0 {
			return values
		}
	}

	if c.Flags != nil {
		if value, ok := c.Flags[name]; ok {
			return []string{value}
		}
	}

	return nil
}
