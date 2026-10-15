// Package predict generates shell autocompletion candidates for command-line tools.
//
// A [Predictor] generates completion candidates. Build predictors using
// constructors like [Values], [Files], [Dirs], or [Map], then compose them
// with combinators like [Batch] and [Callback]:
//
//	// Static candidates
//	formats := predict.Values("json", "yaml", "toml")
//
//	// Candidates with descriptions
//	methods := predict.Map(map[string]string{
//		"GET":  "Retrieve resource",
//		"POST": "Create resource",
//	})
//
//	// Dynamic prediction based on previous command-line flags
//	target := predict.Callback(func(c predict.Context) predict.Predictor {
//		env := c.FlagValue("env", "staging")
//		return predict.Values(env+"-app-1", env+"-app-2")
//	})
//
// Predictors evaluate against a [Context] and return a [Result] containing
// completion items, shell directives, and usage text.
//
// Common constructors include static values ([Values], [Map], [Items],
// [KeyValue], [KeyValueDescribed]), the filesystem ([Files], [Dirs],
// [Executables], [Symlinks]), multi-part paths ([MultiParts]), dynamic
// callbacks ([Callback]), and the empty predictor ([None]).
//
// Modifiers refine prediction behavior:
//
//   - [Predictor.FilterText]: matches against a typed prefix.
//   - [Predictor.Exclude]: filters out already selected candidates.
//   - [Predictor.KeepOrder]: disables alphabetical sorting.
//   - [Predictor.Timeout]: limits how long completion waits for dynamic callbacks.
//
// Candidates keep declaration order; [Map] sorts its keys alphabetically.
// Shells may re-sort candidates unless [Predictor.KeepOrder] is set.
//
// Completion waits up to [DefaultTimeout] (250ms) for dynamic callbacks. A timed-out
// callback is not canceled and may continue running. [Predictor.Timeout] adjusts
// the wait limit; a negative duration disables it.
//
// This package is a standalone SDK with no dependency on the root conga
// package, so it can be reused independently of the framework.
//
// Predictors reach the command line through the root package's App.Predictor
// builder method and the conga.Predictable interface; see
// https://docs.conga.sh/how-to/wire-custom-completion/ for the wiring guide.
//
// Domain-specific types and their predictors live in the topical network,
// system, and timezone packages; see
// https://docs.conga.sh/reference/domain-types/.
package predict
