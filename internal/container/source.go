package container

const (
	// SourceNone indicates the value was not provided from any source.
	SourceNone Source = iota

	// SourceDefault indicates the value came from the struct tag's `default` definition.
	SourceDefault

	// SourceConfig indicates the value was resolved from a configuration file.
	SourceConfig

	// SourceEnv indicates the value was resolved from an environment variable.
	SourceEnv

	// SourceCLI indicates the value was explicitly passed on the command line.
	SourceCLI
)

// Source identifies the origin of a resolved flag or argument value.
type Source int

// String returns a human-readable name for the value source.
func (s Source) String() string {
	switch s {
	case SourceDefault:
		return "default"
	case SourceConfig:
		return "config"
	case SourceEnv:
		return "env"
	case SourceCLI:
		return "cli"
	default:
		return "none"
	}
}
