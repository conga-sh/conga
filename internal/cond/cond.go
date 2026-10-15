// Package cond provides lightweight, generic conditional assignment and fallback utilities
// designed to eliminate temporary variable boilerplate and improve code readability.
//
// # Guidelines & Best Practices ("Simple is Best")
//
// 1. Top-Level Clarity: Use cond helpers directly at assignment boundaries to avoid
// declaring intermediate mutable variables.
//
//	// Good:
//	baseDir := cmp.Or(ctx.Dir, ".")
//
//	// Avoid:
//	var baseDir string = "."
//	if ctx.Dir != "" {
//	    baseDir = ctx.Dir
//	}
//
// 2. Avoid Deep Nesting: Do not nest cond calls inside other cond calls or wrap them
// in wide function chains. Keep invocations flat and readable.
//
// 3. Eager Evaluation Awareness (CRITICAL - DO NOT BLINDLY REFACTOR):
// In Go, function arguments are evaluated eagerly before the function executes.
// Never pass expressions that could panic or expensive function calls inside eager branches.
// Always favor standard Go if statements when branches guard against panics or perform side-effects.
//
//	// ANTI-PATTERN 1 (Runtime Panic via Index Out of Range):
//	// When len(paths) == 0, paths[len(paths)-1] evaluates paths[-1] BEFORE cond.If executes!
//	path := cond.If(len(paths) > 0, paths[len(paths)-1], fallback) // CRASH!
//
//	// ANTI-PATTERN 2 (Runtime Panic via Slice Bounds):
//	// When len(args) <= 1, args[1] causes index out of range [1] during argument evaluation!
//	shell := cond.If(len(args) > 1, args[1], "") // CRASH!
//
//	// ANTI-PATTERN 3 (Allocation Regression):
//	// fmt.Sprintf is executed on EVERY single iteration, even when count == 1!
//	desc := cond.If(count > 1, fmt.Sprintf("%d items", count), "1 item") // WASTEFUL ALLOCATION!
//
//	// CORRECT (Use standard Go if statements):
//	var shell string
//	if len(args) > 1 {
//	    shell = args[1]
//	}
package cond

// SetOrDefaultFunc returns value if it is not the zero value of T, otherwise invokes
// defaultFn() and returns its result. Use this when the default computation is expensive
// or invokes a system lookup that should only run if value is unset.
//
// Example:
//
//	shell := cond.SetOrDefaultFunc(targetShell, ps.DetermineShell)
//	appName := cond.SetOrDefaultFunc(a.name, getAppName)
func SetOrDefaultFunc[T comparable](value T, defaultFn func() T) T {
	var zero T
	if value != zero {
		return value
	}

	return defaultFn()
}

// If returns whenTrue if condition is true, otherwise whenFalse.
//
// Note: Both whenTrue and whenFalse are evaluated eagerly. For conditional
// operations involving potential panics or expensive allocations, use standard
// Go if statements.
//
// Example:
//
//	a.usageMode = cond.If(silence, UsageSilence, UsageSummary)
//	prefix := cond.If(isNegatable, "--[no-]", "--")
func If[T any](condition bool, whenTrue, whenFalse T) T {
	if condition {
		return whenTrue
	}

	return whenFalse
}
