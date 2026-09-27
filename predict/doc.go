// Package predict provides decoupled shell autocompletion predictors for CONGA
// command line tools.
//
// A [Predictor] is a completion candidate generator or combinator. Build
// predictors with constructors such as [Values], [Items], [Map], [MapValues],
// [Files], [Dirs], [Executables], and [Symlinks], then compose them with
// combinators such as [Batch], [Callback], [MultiParts], and [None]. Predictors
// evaluate against a [Context] describing the partial command line and produce
// a [Result] of completion items, shell directives, and usage text. Modifiers
// on [Predictor] refine the result, for example [Predictor.FilterText],
// [Predictor.Exclude], and [Predictor.KeepOrder].
//
// This package is a standalone SDK with no dependency on the root conga
// package, so external consumers can reuse and share completion logic
// independently.
//
// Dynamic callbacks are bounded by [DefaultTimeout]; set a different bound per
// predictor with [Predictor.Timeout].
package predict
