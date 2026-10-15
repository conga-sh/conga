// Package container defines the source and classification metadata used by
// CONGA's built-in command-line containers.
package container

const (
	// KindNone marks a field that is not a built-in container.
	KindNone Kind = iota

	// KindFlag marks an optional or required flag container.
	KindFlag

	// KindCounter marks a counter flag container.
	KindCounter

	// KindConfigFile marks the merged configuration-file flag container.
	KindConfigFile

	// KindArg marks an optional or required positional argument container.
	KindArg

	// KindCommand marks a subcommand container.
	KindCommand

	// KindDashArgs marks the '--' trailing-arguments container.
	KindDashArgs

	// KindRawArgs marks the raw trailing-tokens container.
	KindRawArgs

	// KindHelpCommand marks the opt-in help subcommand container.
	KindHelpCommand

	// KindVersionCommand marks the opt-in version subcommand container.
	KindVersionCommand

	// KindCompletionCommand marks the opt-in completion subcommand container.
	KindCompletionCommand

	// KindFeatures marks the root feature-declaration container.
	KindFeatures
)

// Kind identifies the engine container role a command-struct field plays. The
// engine classifies each field once during schema build; the schema builder
// treats the classification as plain data and never inspects container
// behavior itself.
type Kind uint8

// Classification is the engine's classification of one command-struct field.
// Kind and Required are plain metadata. Target carries the inner command
// struct pointer for the command kinds so the schema builder can reflect over
// it; the field's container itself travels separately as an opaque handle.
type Classification struct {
	Kind     Kind
	Required bool
	Target   any
}
