package spec

const (
	// DefaultHelpFlagHelp is the default description for the --help flag.
	DefaultHelpFlagHelp = "Show help for command"

	// DefaultHelpCommandHelp is the default description for the help subcommand.
	DefaultHelpCommandHelp = "Show help for a command"

	// DefaultVersionFlagHelp is the default description for the --version flag.
	DefaultVersionFlagHelp = "Show version information"

	// DefaultCompletionCommandHelp is the default description for the completion subcommand.
	DefaultCompletionCommandHelp = "Generate shell completion scripts"

	// DefaultCompletionFlagHelp is the default description for the --completion flag.
	DefaultCompletionFlagHelp = "Generate shell completion script"

	// DefaultWhyFlagHelp is the default description for the --why flag.
	DefaultWhyFlagHelp = "Show resolved field values and where they came from (values redacted unless tagged safe); accepts text or json"

	// VersionJSONFlagName is the engine-owned name of the opt-in version
	// command's machine-readable output selector. Schema building synthesizes
	// the flag with this name and binding matches the synthesized spec by
	// identity, so the two sites stay coupled through one source of truth.
	VersionJSONFlagName = "json"
)

//nolint:gochecknoglobals // shared immutable builtin flag definitions
var (
	// BuiltinHelpFlag is the shared definition for the standard --help flag.
	BuiltinHelpFlag = &Flag{Name: "help", Help: DefaultHelpFlagHelp, IsBool: true}

	// BuiltinVersionFlag is the shared definition for the standard --version flag.
	BuiltinVersionFlag = &Flag{Name: "version", Help: DefaultVersionFlagHelp, IsBool: true}

	// BuiltinCompletionFlag is the shared definition for the standard --completion flag.
	// Shell candidates are resolved through the completion-enabled "shell"
	// predictor (the same predictor the `completion <shell>` argument uses), so
	// the supported-shell list is never referenced from schema-building code.
	BuiltinCompletionFlag = &Flag{
		Name:        "completion",
		Help:        DefaultCompletionFlagHelp,
		Placeholder: "SHELL",
		Predictor:   &PredictorSpec{Kind: PredictorKindCustom, Name: "shell"},
	}

	// BuiltinWhyFlag is the shared definition for the standard --why flag.
	BuiltinWhyFlag = &Flag{Name: "why", Help: DefaultWhyFlagHelp, IsBool: true}
)
