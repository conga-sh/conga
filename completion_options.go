package conga

// CompletionOption configures shell completion on [App.EnableCompletion].
//
// CompletionOption is a sealed interface: values are produced exclusively by
// CONGA option constructors such as [CompletionEnvPrefix], and cannot be
// implemented by other packages.
type CompletionOption interface {
	applyCompletion(settings *completionSettings)
}

// CompletionEnvPrefix overrides the environment-variable prefix that names the
// completion sideband toggles.
//
// The completion driver reads three per-application variables:
//
//	<APP>_COMPLETION_DESCRIPTIONS      disables candidate descriptions
//	<APP>_COMPLETION_ALIASES           disables alias candidates
//	<APP>_COMPLETION_BARE_SPACE_FLAGS  restores flags at a bare space
//
// The default prefix derives from the application name set with [App.Name],
// normalized to UPPER_SNAKE_CASE, so an application named task reads
// TASK_COMPLETION_DESCRIPTIONS. Pass CompletionEnvPrefix to rename that surface
// when the application name is not a usable variable prefix.
//
// The value is normalized the same way: letters are upper-cased, separators
// become underscores, and a trailing underscore is appended when absent, so
// both "TaskCtl" and "task-ctl" resolve to TASK_CTL_. An empty prefix keeps the
// application-name derivation. A prefix that normalizes to a leading digit is
// rejected at schema build because POSIX shells cannot assign such a variable
// name.
//
// See https://docs.conga.sh/how-to/enable-shell-completion/.
func CompletionEnvPrefix(prefix string) CompletionOption {
	return completionOptionFunc(func(settings *completionSettings) {
		settings.envPrefix = prefix
	})
}

type completionOptionFunc func(*completionSettings)

func (f completionOptionFunc) applyCompletion(settings *completionSettings) { f(settings) }

type completionSettings struct {
	envPrefix string
}
