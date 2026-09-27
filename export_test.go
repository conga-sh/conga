package conga

import (
	"reflect"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

// SpecOptionsWithContainers returns schema-build options carrying the engine's
// container classifier, so black-box tests building schemas from real conga
// containers resolve their engine classification.
func SpecOptionsWithContainers(opts spec.Options) spec.Options {
	opts.ClassifyContainer = classifyContainer

	return opts
}

// OptionalFlagContainer exposes the engine state view of an optional flag for
// black-box unit tests.
func OptionalFlagContainer[T any](f *OptionalFlag[T]) flagContainerView {
	return flagContainerView{state: f}
}

// RequiredFlagContainer exposes the engine state view of a required flag for
// black-box unit tests.
func RequiredFlagContainer[T any](f *RequiredFlag[T]) flagContainerView {
	return flagContainerView{state: f, required: true}
}

// OptionalArgContainer exposes the engine state view of an optional argument
// for black-box unit tests.
func OptionalArgContainer[T any](a *OptionalArg[T]) argContainerView {
	return argContainerView{state: a}
}

// RequiredArgContainer exposes the engine state view of a required argument
// for black-box unit tests.
func RequiredArgContainer[T any](a *RequiredArg[T]) argContainerView {
	return argContainerView{state: a, required: true}
}

// ConfigFileContainer exposes the engine state view of a configuration-file
// container for black-box unit tests.
func ConfigFileContainer(c *ConfigFile) configFileContainerView {
	return configFileContainerView{flagContainerView{state: c}}
}

// CounterContainer exposes the engine state view of a counter container for
// black-box unit tests.
func CounterContainer(c *Counter) counterContainerView {
	return counterContainerView{flagContainerView{state: c}}
}

// CommandContainer exposes the engine state view of a subcommand container for
// black-box unit tests.
func CommandContainer[T any](c *Command[T]) commandContainerView {
	return commandContainerView{state: c}
}

// DashArgsContainer exposes the engine state view of a '--' trailing-arguments
// container for black-box unit tests.
func DashArgsContainer(d *DashArgs) dashArgsContainerView {
	return dashArgsContainerView{target: d}
}

// RawArgsContainer exposes the engine state view of a raw trailing-tokens
// container for black-box unit tests.
func RawArgsContainer(r *RawArgs) rawArgsContainerView {
	return rawArgsContainerView{target: r}
}

// flagContainerView is the test-only facade over a flag-like container's
// engine state.
type flagContainerView struct {
	state    flagState
	required bool
}

// IsFlag marks the view as a flag-like container.
func (v flagContainerView) IsFlag() {}

// SetValue binds a value and records its source.
func (v flagContainerView) SetValue(value any, source ValueSource, detail string) {
	v.state.setValue(value, source, detail)
}

// GetValue returns the resolved value as any.
func (v flagContainerView) GetValue() any { return v.state.getValue() }

// Source returns where the value originated.
func (v flagContainerView) Source() ValueSource { return v.state.Source() }

// IsSet reports whether a value has been supplied.
func (v flagContainerView) IsSet() bool { return v.state.IsSet() }

// Changed reports whether the value was explicitly provided.
func (v flagContainerView) Changed() bool { return v.state.Changed() }

// IsRequired reports whether the container is a required flag.
func (v flagContainerView) IsRequired() bool { return v.required }

// configFileContainerView is the test-only facade over a configuration-file
// container's engine state.
type configFileContainerView struct{ flagContainerView }

// IsConfigFile marks the view as a configuration-file container.
func (configFileContainerView) IsConfigFile() {}

// counterContainerView is the test-only facade over a counter container's
// engine state.
type counterContainerView struct{ flagContainerView }

// IsCounter marks the view as a counter container.
func (counterContainerView) IsCounter() {}

// argContainerView is the test-only facade over a positional argument
// container's engine state.
type argContainerView struct {
	state    argState
	required bool
}

// IsArg marks the view as a positional argument container.
func (argContainerView) IsArg() {}

// SetValue binds a value and records its source.
func (v argContainerView) SetValue(value any, source ValueSource, detail string) {
	v.state.setValue(value, source, detail)
}

// GetValue returns the resolved value as any.
func (v argContainerView) GetValue() any { return v.state.getValue() }

// IsSet reports whether a value has been supplied.
func (v argContainerView) IsSet() bool { return v.state.IsSet() }

// Changed reports whether the value was explicitly provided.
func (v argContainerView) Changed() bool { return v.state.Changed() }

// IsRequired reports whether the container is a required argument.
func (v argContainerView) IsRequired() bool { return v.required }

// commandContainerView is the test-only facade over a subcommand container's
// engine state.
type commandContainerView struct{ state commandState }

// IsCommand marks the view as a subcommand container.
func (commandContainerView) IsCommand() {}

// InnerValue returns a pointer to the inner command struct.
func (v commandContainerView) InnerValue() any { return v.state.innerValue() }

// MarkSelected records that the engine routed execution to this command.
func (v commandContainerView) MarkSelected() { v.state.markSelected() }

// IsSelected reports whether the engine routed execution to this command.
func (v commandContainerView) IsSelected() bool { return v.state.IsSelected() }

// dashArgsContainerView is the test-only facade over the '--' trailing
// arguments collector.
type dashArgsContainerView struct{ target *DashArgs }

// IsDashArgs marks the view as a '--' trailing-arguments collector.
func (dashArgsContainerView) IsDashArgs() {}

// SetArgs stores the captured trailing arguments.
func (v dashArgsContainerView) SetArgs(args []string) { v.target.setArgs(args) }

// Get returns the captured arguments slice.
func (v dashArgsContainerView) Get() []string { return v.target.Get() }

// IsSet reports whether trailing arguments were captured.
func (v dashArgsContainerView) IsSet() bool { return v.target.IsSet() }

// rawArgsContainerView is the test-only facade over the raw trailing-tokens
// collector.
type rawArgsContainerView struct{ target *RawArgs }

// IsRawArgs marks the view as a raw trailing-tokens collector.
func (rawArgsContainerView) IsRawArgs() {}

// SetArgs stores the captured trailing tokens.
func (v rawArgsContainerView) SetArgs(args []string) { v.target.setArgs(args) }

// Get returns the captured arguments slice.
func (v rawArgsContainerView) Get() []string { return v.target.Get() }

// IsSet reports whether trailing tokens were captured.
func (v rawArgsContainerView) IsSet() bool { return v.target.IsSet() }

// NewSessionForTest constructs a test session with an attached root struct.
func NewSessionForTest[T any](root *T) *Session {
	return &Session{
		root: root,
	}
}

// FriendlyTypeName exposes unexported friendlyTypeName for testing.
func FriendlyTypeName(targetType reflect.Type) string {
	return friendlyTypeName(targetType)
}

// FormatCoercionLocation exposes unexported formatCoercionLocation for testing.
func FormatCoercionLocation(e *coercionError) string {
	return formatCoercionLocation(e)
}

// TruncateForDisplay exposes unexported truncateForDisplay for testing.
func TruncateForDisplay(s string) string {
	return truncateForDisplay(s)
}

// CompleteForShell exposes unexported completeForShell for black-box unit tests in package conga_test.
func (a *App[T]) CompleteForShell(targetShell string, args []string) (predict.Result, string) {
	return a.completeForShell(targetShell, args)
}

// CompleteCompLineForShell exposes unexported completeCompLineForShell for black-box unit tests in package conga_test.
func (a *App[T]) CompleteCompLineForShell(targetShell string, compLine string, compPoint int) (predict.Result, string) {
	return a.completeCompLineForShell(targetShell, compLine, compPoint)
}
