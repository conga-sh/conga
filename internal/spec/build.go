package spec

import (
	"cmp"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/slicex"
)

const (
	builtinHelp builtinCommandKind = iota
	builtinVersion
	builtinCompletion
)

// rootCommandMethodGuards rejects lifecycle methods on root command structs
// that conflict with the flat-API configuration conventions.
//
//nolint:gochecknoglobals // static guard table for root command method validation
var rootCommandMethodGuards = []struct {
	rejects func(target any) bool
	message func(structName string) string
}{
	{
		rejects: implements[summaryMethod],
		message: func(structName string) string {
			return "root command struct " + structName + " must not implement Summary(); configure application description via app.Description()"
		},
	},
	{
		rejects: implements[descriptionMethod],
		message: func(structName string) string {
			return "root command struct " + structName + " must not implement Description(); configure application description via app.Description()"
		},
	},
	{
		rejects: implements[helpFooterMethod],
		message: func(structName string) string {
			return "root command struct " + structName + " must not implement HelpFooter(); configure global help footer via app.HelpFooter()"
		},
	},
	{
		rejects: implements[deprecatedMethod],
		message: func(structName string) string {
			return "root command struct " + structName + " must not implement Deprecated(); use app.Deprecated()"
		},
	},
	{
		rejects: implements[examplesMethod],
		message: func(structName string) string {
			return "root command struct " + structName + " must not implement Examples(); configure root examples via app.Examples()"
		},
	},
}

// Build inspects a struct pointer and recursively constructs the Command AST tree.
func Build(target any, defaultName string, opts Options) (*Command, error) {
	cmd, err := buildInternal(target, defaultName, true, opts)
	if err != nil {
		return nil, err
	}

	if err := validateAncestorFlagUniqueness(cmd); err != nil {
		return nil, err
	}

	cmd.ConfigSettings = opts.ConfigSettings

	if err := validateConfigSettings(cmd); err != nil {
		return nil, err
	}

	isConfigCapable := opts.ConfigCapable || cmd.ConfigFileFlag != nil || len(opts.ConfigFilePatterns) > 0
	if isConfigCapable {
		if err := validateFlagSubcommandCollisions(cmd); err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

// NewCompletionCommand creates a completion subcommand definition for target.
// shells enumerates the supported shell names and is supplied by the caller:
// schema building must not reach the completion subsystem directly.
func NewCompletionCommand(name, summary string, target any, aliases []string, hidden bool, shells []string) *Command {
	return &Command{
		Name:            name,
		Aliases:         aliases,
		Summary:         summary,
		Hidden:          hidden,
		Target:          target,
		IsCompletionCmd: true,
		Args: []*Arg{
			{
				Name:     "<shell>",
				Help:     "Target shell name",
				Required: false,
				Enum:     shells,
				// The enum validates the argument and supplies the "shell"
				// predictor's vocabulary; the completion footer already renders
				// the supported shells, so the automatic [enum: ...] hint is
				// suppressed to avoid listing them twice.
				HelpVars: helpVars{Enum: true},
				Predictor: &PredictorSpec{
					Kind: PredictorKindCustom,
					Name: "shell",
				},
			},
		},
	}
}

type schemaBuilder struct {
	cmd             *Command
	structType      reflect.Type
	structValue     reflect.Value
	flagTracker     *flagCollisionTracker
	seenSubcommands map[string]string
	sliceArgField   string
	sliceArgName    string
	mapArgField     string
	mapArgName      string
	hasOptionalArg  bool
	isRoot          bool
	opts            Options
}

func newSchemaBuilder(target any, defaultName string, structValue reflect.Value, isRoot bool, opts Options) (*schemaBuilder, error) {
	structType := structValue.Type()

	tagEnvPrefix, err := scanSentinelDirectives(structType, &opts)
	if err != nil {
		return nil, err
	}

	envPrefix := cmp.Or(tagEnvPrefix, opts.EnvPrefix)
	if envPrefix != "" && !strings.HasSuffix(envPrefix, "_") {
		envPrefix += "_"
	}

	opts.EnvPrefix = envPrefix

	cmd := &Command{
		Name:              defaultName,
		Target:            target,
		EnvPrefix:         envPrefix,
		DisableHelp:       opts.DisableHelp,
		DisableVersion:    opts.DisableVersion,
		DisableWhy:        opts.DisableWhy,
		DisableCompletion: opts.DisableCompletion,
	}

	if err := applyCommandMethodGuardrails(target, cmd, structType.Name(), isRoot); err != nil {
		return nil, err
	}

	flagTracker := newFlagCollisionTracker()
	if !opts.DisableHelp {
		flagTracker.seenNames["help"] = "built-in help (use app.DisableHelpFlag() to override)"
	}

	if isRoot && !opts.DisableVersion {
		flagTracker.seenNames["version"] = "built-in version (use app.DisableVersionFlag() to override)"
	}

	if !opts.DisableWhy {
		flagTracker.seenNames["why"] = "built-in why (enabled by app.EnableWhy())"
	}

	return &schemaBuilder{
		cmd:             cmd,
		structType:      structType,
		structValue:     structValue,
		flagTracker:     flagTracker,
		seenSubcommands: make(map[string]string),
		isRoot:          isRoot,
		opts:            opts,
	}, nil
}

func (b *schemaBuilder) validateFieldTags(structField reflect.StructField, role string, candidates []string) (Directives, error) {
	return ValidateFieldTags(structField.Tag, role, candidates, ValidationOptions{
		StrictTagValidation: b.opts.StrictTagValidation,
		AllowUnknownTags:    b.opts.AllowUnknownTags,
		AllowedTags:         b.opts.AllowedTags,
		EnvPrefix:           b.opts.EnvPrefix,
		EnvPrefixKnown:      true,
	})
}

func (b *schemaBuilder) checkDuplicateSubcommand(name, fieldName string, isAlias bool) error {
	if prevField, ok := b.seenSubcommands[name]; ok {
		desc := "subcommand"
		if isAlias {
			desc = "subcommand alias"
		}

		return fmt.Errorf("%w: %s %q (already declared by field %s)", ErrDuplicateFlag, desc, name, prevField)
	}

	b.seenSubcommands[name] = fieldName

	return nil
}

func (b *schemaBuilder) processSubcommand(structField reflect.StructField, commandHandle any, target any, dirs Directives) error {
	subName := cmp.Or(structField.Tag.Get("name"), casing.Kebab(structField.Name))

	if err := b.checkDuplicateSubcommand(subName, structField.Name, false); err != nil {
		return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
	}

	var aliases []string

	if aliasesTag := structField.Tag.Get("aliases"); aliasesTag != "" {
		aliases = slicex.SplitAndTrim(aliasesTag, ",")
		for _, alias := range aliases {
			if err := b.checkDuplicateSubcommand(alias, structField.Name, true); err != nil {
				return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
			}
		}
	}

	summary, err := ParseSummaryTag(structField.Tag, "")
	if err != nil {
		return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
	}

	subOpts := b.opts
	if dirs.AllowUnknownTags {
		subOpts.AllowUnknownTags = true
		if dirs.Recurse {
			subOpts.AllowUnknownTagsRecurse = true
		}
	} else if !b.opts.AllowUnknownTagsRecurse {
		subOpts.AllowUnknownTags = false
	}

	if subEnvPrefix := structField.Tag.Get("envprefix"); subEnvPrefix != "" {
		if !strings.HasSuffix(subEnvPrefix, "_") {
			subEnvPrefix += "_"
		}

		subOpts.EnvPrefix = subEnvPrefix
	} else if b.cmd.EnvPrefix != "" {
		subOpts.EnvPrefix = b.cmd.EnvPrefix
	}

	subcommand, err := buildInternal(target, subName, false, subOpts)
	if err != nil {
		return err
	}

	subcommand.Handle = commandHandle
	subcommand.Summary = summary
	subcommand.Deprecated = structField.Tag.Get("deprecated")
	subcommand.Group = structField.Tag.Get("group")
	subcommand.Hidden = parsePresenceOrTrueTag(structField.Tag, "hidden")
	subcommand.Aliases = aliases

	if subcommand.EnvPrefix == "" {
		subcommand.EnvPrefix = subOpts.EnvPrefix
	}

	b.cmd.Subcommands = append(b.cmd.Subcommands, subcommand)

	return nil
}

func (b *schemaBuilder) newBuiltinSubcommand(structField reflect.StructField, kind builtinCommandKind, target any) (*Command, error) {
	var defaultName, defaultSummary string

	switch kind {
	case builtinHelp:
		defaultName, defaultSummary = "help", DefaultHelpCommandHelp
	case builtinVersion:
		defaultName, defaultSummary = "version", DefaultVersionFlagHelp
	case builtinCompletion:
		defaultName, defaultSummary = "completion", DefaultCompletionCommandHelp
	default:
		panic("unreachable")
	}

	name := cmp.Or(structField.Tag.Get("name"), defaultName)
	if err := b.checkDuplicateSubcommand(name, structField.Name, false); err != nil {
		return nil, err
	}

	summary, err := ParseSummaryTag(structField.Tag, defaultSummary)
	if err != nil {
		return nil, err
	}

	aliases := SplitTagList(structField.Tag, "aliases")
	hidden := parsePresenceOrTrueTag(structField.Tag, "hidden")

	switch kind {
	case builtinHelp:
		return &Command{
			Name:      name,
			Summary:   summary,
			Hidden:    hidden,
			Target:    target,
			IsHelpCmd: true,
			Args: []*Arg{
				{
					Name:    "[<command>...]",
					Help:    "Subcommand to show help for",
					IsSlice: true,
				},
			},
		}, nil
	case builtinVersion:
		return &Command{
			Name:         name,
			Summary:      summary,
			Hidden:       hidden,
			Target:       target,
			IsVersionCmd: true,
			Flags: []*Flag{
				{
					Name:        VersionJSONFlagName,
					Help:        "Render machine-readable JSON output",
					IsBool:      true,
					EngineOwned: true,
				},
			},
		}, nil
	case builtinCompletion:
		return NewCompletionCommand(name, summary, target, aliases, hidden, b.opts.CompletionShells), nil
	default:
		panic("unreachable")
	}
}

func (b *schemaBuilder) processConfigFile(structField reflect.StructField, configHandle any, fieldTags FieldTags) error {
	if !b.isRoot {
		return ErrInvalidConfigFilePlacement
	}

	if b.cmd.ConfigFileFlag != nil {
		return fmt.Errorf("%w: ConfigFile (already declared by %s)", ErrDuplicateFlag, b.cmd.ConfigFileFlag.Name)
	}

	if err := ValidateConfigFileConfigTag(structField.Tag); err != nil {
		return err
	}

	flagSpec, err := b.buildConfigFileFlagSpec(structField, configHandle, fieldTags)
	if err != nil {
		return err
	}

	if err := b.flagTracker.CheckAndAdd(flagSpec, structField.Name); err != nil {
		return err
	}

	b.cmd.Flags = append(b.cmd.Flags, flagSpec)
	b.cmd.ConfigFileFlag = flagSpec

	return nil
}

func (b *schemaBuilder) buildConfigFileFlagSpec(structField reflect.StructField, configHandle any, fieldTags FieldTags) (*Flag, error) {
	isMulti := fieldTags.Bool("multi")

	flagName := cmp.Or(structField.Tag.Get("name"), "config-file")

	shortRune := extractFlagShort(structField.Tag)
	envVars := b.extractConfigFileEnvVars(structField.Tag)

	helpText, referenced, err := interpolateHelp(extractConfigFileHelp(structField.Tag, isMulti), helpVarValuesFor(nil, nil, nil, nil, "", envVars))
	if err != nil {
		return nil, err
	}

	if len(b.opts.ConfigFilePatterns) == 0 {
		helpText += " (no configuration decoder registered)"
	}

	placeholder := extractConfigFilePlaceholder(structField.Tag)

	targetType := reflect.TypeFor[string]()
	if isMulti {
		targetType = reflect.TypeFor[[]string]()
	}

	sep, sepFound := structField.Tag.Lookup("sep")
	if !sepFound {
		sep = ","
	}

	var aliases []string
	if aliasTag := structField.Tag.Get("aliases"); aliasTag != "" {
		aliases = slicex.SplitAndTrim(aliasTag, ",")
	}

	return &Flag{
		Name:         flagName,
		Short:        shortRune,
		Aliases:      aliases,
		ConfigKey:    "-",
		EnvVars:      envVars,
		Help:         helpText,
		HelpVars:     referenced,
		Placeholder:  placeholder,
		Group:        structField.Tag.Get("group"),
		Hidden:       parsePresenceOrTrueTag(structField.Tag, "hidden"),
		Deprecated:   structField.Tag.Get("deprecated"),
		FieldTags:    fieldTags,
		TargetType:   targetType,
		Handle:       configHandle,
		IsConfigFile: true,
		IsSlice:      isMulti,
		Sep:          sep,
		Predictor:    &PredictorSpec{Kind: PredictorKindFile, Name: "file", Patterns: slices.Clone(b.opts.ConfigFilePatterns)},
	}, nil
}

func (b *schemaBuilder) extractConfigFileEnvVars(tag reflect.StructTag) []string {
	if envTag, ok := tag.Lookup("env"); ok {
		if strings.TrimSpace(envTag) == "-" {
			return nil
		}

		vars := SplitTagList(tag, "env")
		if len(vars) > 0 {
			return vars
		}
	}

	if b.cmd.EnvPrefix != "" {
		return []string{b.cmd.EnvPrefix + "CONFIG"}
	}

	return nil
}

func (b *schemaBuilder) processFlag(structField reflect.StructField, flagHandle any, classification container.Classification, fieldTags FieldTags) error {
	innerType := extractGenericType(structField.Type)
	if hasTag(structField.Tag, "negatable") && innerType.Kind() != reflect.Bool {
		return fmt.Errorf("%w: tag \"negatable\" is only valid on boolean flags, got %s", ErrInvalidStructTag, innerType.String())
	}

	if err := validateMapConstraints(structField, innerType); err != nil {
		return err
	}

	if classification.Required && hasTag(structField.Tag, "deprecated") {
		return fmt.Errorf("%w: tag \"deprecated\" cannot be used on required flags (make the flag optional before deprecating it so callers can migrate without error)", ErrInvalidStructTag)
	}

	flagSpec, err := parseFlagSpec(structField, flagHandle, classification, fieldTags, b.opts)
	if err != nil {
		return err
	}

	flagSpec.FieldTags = fieldTags

	if err := b.flagTracker.CheckAndAdd(flagSpec, structField.Name); err != nil {
		return err
	}

	b.cmd.Flags = append(b.cmd.Flags, flagSpec)

	return nil
}

func (b *schemaBuilder) processArg(structField reflect.StructField, argHandle any, classification container.Classification, fieldTags FieldTags) error {
	innerType := extractGenericType(structField.Type)

	if err := validateMapConstraints(structField, innerType); err != nil {
		return err
	}

	argSpec, err := parseArgSpec(structField, argHandle, classification, fieldTags, b.opts)
	if err != nil {
		return err
	}

	argSpec.FieldTags = fieldTags

	if b.sliceArgField != "" {
		return fmt.Errorf("%w: slice argument %s must be the last positional argument", ErrInvalidArgOrder, b.sliceArgName)
	}

	if b.mapArgField != "" {
		return fmt.Errorf("%w: map argument %s must be the last positional argument", ErrInvalidArgOrder, b.mapArgName)
	}

	if b.hasOptionalArg && argSpec.Required {
		return fmt.Errorf("%w: optional positional argument cannot precede required positional argument %s", ErrInvalidArgOrder, argSpec.Name)
	}

	if !argSpec.Required {
		b.hasOptionalArg = true
	}

	if argSpec.IsSlice {
		b.sliceArgField = structField.Name
		b.sliceArgName = argSpec.Name
	}

	if argSpec.IsMap {
		b.mapArgField = structField.Name
		b.mapArgName = argSpec.Name
	}

	b.cmd.Args = append(b.cmd.Args, argSpec)

	return nil
}

func (b *schemaBuilder) processBuiltinCommand(structField reflect.StructField, classification container.Classification) error {
	var (
		kind builtinCommandKind
		role string
	)

	switch classification.Kind {
	case container.KindHelpCommand:
		kind, role = builtinHelp, "help command"
	case container.KindVersionCommand:
		kind, role = builtinVersion, "version command"
	case container.KindCompletionCommand:
		kind, role = builtinCompletion, "completion command"
	default:
		return nil
	}

	dirs, err := b.validateFieldTags(structField, role, builtinCommandTagCandidates)
	if err != nil {
		return err
	}

	if dirs.Ignore {
		return nil
	}

	// A declared completion command requires completion to be enabled: the
	// schema must not silently drop the field, because the resulting app would
	// reject `completion <shell>` with unrelated parse/validation errors that
	// never name the missing opt-in.
	if kind == builtinCompletion && b.opts.DisableCompletion {
		return CompletionRequiresEnableError()
	}

	subcommand, err := b.newBuiltinSubcommand(structField, kind, classification.Target)
	if err != nil {
		return err
	}

	b.cmd.Subcommands = append(b.cmd.Subcommands, subcommand)

	return nil
}

// processConfigFileField validates the tags of the merged config-file flag
// container and delegates to processConfigFile, reporting whether the field
// was handled. The config-file tag vocabulary (multi) is engine-owned: flag
// arity is an engine concern, so it parses identically across the config-file
// container adapters. Errors stay unwrapped here: the processField
// dispatch adds the struct/field context.
func (b *schemaBuilder) processConfigFileField(structField reflect.StructField, configHandle any) error {
	tagSpecs := ConfigFileFieldTagSpecs()
	candidates := slices.Concat(configFileTagCandidates, FieldTagKeys(tagSpecs))

	dirs, err := b.validateFieldTags(structField, "config file flag", candidates)
	if err != nil {
		return err
	}

	if dirs.Ignore {
		return nil
	}

	fieldTags, err := ParseFieldTags(structField.Tag, tagSpecs)
	if err != nil {
		return err
	}

	return b.processConfigFile(structField, configHandle, fieldTags)
}

func (b *schemaBuilder) processField(structField reflect.StructField, fieldValue reflect.Value) error {
	if !structField.IsExported() {
		return b.processUnexportedField(structField)
	}

	fieldAddr := fieldValue.Addr().Interface()

	classification := container.Classification{}
	if b.opts.ClassifyContainer != nil {
		classification = b.opts.ClassifyContainer(fieldAddr)
	}

	switch classification.Kind {
	case container.KindCommand:
		return b.processCommandField(structField, fieldAddr, classification)
	case container.KindHelpCommand, container.KindVersionCommand, container.KindCompletionCommand:
		return b.fieldContextError(structField, b.processBuiltinCommand(structField, classification))
	case container.KindConfigFile:
		return b.fieldContextError(structField, b.processConfigFileField(structField, fieldAddr))
	case container.KindDashArgs:
		return b.processDashArgsField(structField, fieldAddr)
	case container.KindRawArgs:
		return b.processRawArgsField(structField, fieldAddr)
	case container.KindFlag, container.KindCounter:
		return b.processFlagField(structField, fieldAddr, classification)
	case container.KindArg:
		return b.processArgField(structField, fieldAddr, classification)
	default:
		return nil
	}
}

// processUnexportedField validates the conga directives on blank sentinel
// fields; every other unexported field is ignored.
func (b *schemaBuilder) processUnexportedField(structField reflect.StructField) error {
	if structField.Name != "_" {
		return nil
	}

	if _, err := b.validateFieldTags(structField, "sentinel field", sentinelTagCandidates); err != nil {
		return b.fieldContextError(structField, err)
	}

	return nil
}

// processCommandField validates and builds a subcommand container field.
func (b *schemaBuilder) processCommandField(structField reflect.StructField, commandHandle any, classification container.Classification) error {
	dirs, err := b.validateFieldTags(structField, "subcommand", userCommandTagCandidates)
	if err != nil {
		return b.fieldContextError(structField, err)
	}

	if dirs.Ignore {
		return nil
	}

	return b.processSubcommand(structField, commandHandle, classification.Target, dirs)
}

// processDashArgsField validates and stores the '--' trailing-arguments
// collector field.
func (b *schemaBuilder) processDashArgsField(structField reflect.StructField, handle any) error {
	dirs, err := b.validateFieldTags(structField, "dash args", dashRawTagCandidates)
	if err != nil {
		return b.fieldContextError(structField, err)
	}

	if dirs.Ignore {
		return nil
	}

	dashName := cmp.Or(structField.Tag.Get("name"), "[-- <args...>]")

	b.cmd.DashArgs = &DashArgs{
		Name:   dashName,
		Handle: handle,
	}

	return nil
}

// processRawArgsField validates and stores the raw trailing-tokens collector
// field.
func (b *schemaBuilder) processRawArgsField(structField reflect.StructField, handle any) error {
	dirs, err := b.validateFieldTags(structField, "raw args", dashRawTagCandidates)
	if err != nil {
		return b.fieldContextError(structField, err)
	}

	if dirs.Ignore {
		return nil
	}

	rawName := cmp.Or(structField.Tag.Get("name"), "[command] [args...]")

	b.cmd.RawArgs = &RawArgs{
		Name:   rawName,
		Handle: handle,
	}

	return nil
}

// fieldContextError wraps a schema-build error with the owning struct and
// field, returning nil unchanged.
func (b *schemaBuilder) fieldContextError(structField reflect.StructField, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
}

func (b *schemaBuilder) processFlagField(structField reflect.StructField, flagHandle any, classification container.Classification) error {
	tagSpecs := fieldTagSpecsForField(extractGenericType(structField.Type))

	return b.processContainerField(structField, "flag", flagTagCandidates, tagSpecs, func(fieldTags FieldTags) error {
		return b.processFlag(structField, flagHandle, classification, fieldTags)
	})
}

func (b *schemaBuilder) processArgField(structField reflect.StructField, argHandle any, classification container.Classification) error {
	tagSpecs := fieldTagSpecsForField(extractGenericType(structField.Type))

	return b.processContainerField(structField, "positional argument", argTagCandidates, tagSpecs, func(fieldTags FieldTags) error {
		return b.processArg(structField, argHandle, classification, fieldTags)
	})
}

// processContainerField is the shared skeleton of flag and positional-argument
// field processing: tag validation (with the field's domain-type tag keys
// merged into the strict candidates), the ignore short-circuit, custom
// field-tag parsing, and delegation to the container-specific processor,
// wrapped in the struct/field error context.
func (b *schemaBuilder) processContainerField(structField reflect.StructField, role string, tagCandidates []string, tagSpecs []TagSpec, process func(FieldTags) error) error {
	candidates := tagCandidates
	if len(tagSpecs) > 0 {
		candidates = slices.Concat(tagCandidates, FieldTagKeys(tagSpecs))
	}

	dirs, err := b.validateFieldTags(structField, role, candidates)
	if err != nil {
		return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
	}

	if dirs.Ignore {
		return nil
	}

	fieldTags, err := ParseFieldTags(structField.Tag, tagSpecs)
	if err != nil {
		return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
	}

	if err := process(fieldTags); err != nil {
		return fmt.Errorf("struct %s field %s: %w", b.structType.Name(), structField.Name, err)
	}

	return nil
}

type descriptionMethod interface{ Description() string }

type examplesMethod interface{ Examples() string }

type summaryMethod interface{ Summary() string }

type helpFooterMethod interface{ HelpFooter() string }

type deprecatedMethod interface{ Deprecated() string }

type builtinCommandKind int

type flagCollisionTracker struct {
	seenNames  map[string]string
	seenShorts map[rune]string
}

func newFlagCollisionTracker() *flagCollisionTracker {
	return &flagCollisionTracker{
		seenNames:  make(map[string]string),
		seenShorts: make(map[rune]string),
	}
}

func (t *flagCollisionTracker) CheckAndAdd(flagSpec *Flag, fieldName string) error {
	if prevField, ok := t.seenNames[flagSpec.Name]; ok {
		return fmt.Errorf("%w: --%s (already declared by field %s)", ErrDuplicateFlag, flagSpec.Name, prevField)
	}

	t.seenNames[flagSpec.Name] = fieldName

	for _, alias := range flagSpec.Aliases {
		if prevField, ok := t.seenNames[alias]; ok {
			return fmt.Errorf("%w: alias --%s (already declared by field %s)", ErrDuplicateFlag, alias, prevField)
		}

		t.seenNames[alias] = fieldName
	}

	if flagSpec.Short != 0 {
		if prevField, ok := t.seenShorts[flagSpec.Short]; ok {
			return fmt.Errorf("%w: short flag -%c (already declared by field %s)", ErrDuplicateFlag, flagSpec.Short, prevField)
		}

		t.seenShorts[flagSpec.Short] = fieldName
	}

	return nil
}

// validateConfigSettings enforces the app-level configuration settings against
// the built schema: cascading merge mode loads multiple configuration files,
// so a declared root config-file flag must accept multiple values
// (multi:"true") to report every merged file. Apps without a root config-file
// field cascade without one.
func validateConfigSettings(root *Command) error {
	if !root.ConfigSettings.Cascading || root.ConfigFileFlag == nil || root.ConfigFileFlag.IsSlice {
		return nil
	}

	return CascadingRequiresMultiError(root.ConfigFileFlag.Name)
}

// validateMapConstraints rejects map-typed flag/arg fields whose key type is
// not a string, or whose tags carry per-entry constraints (enum/range/len)
// that maps cannot honor; map entries are constrained via "items" instead.
func validateMapConstraints(structField reflect.StructField, innerType reflect.Type) error {
	if innerType.Kind() != reflect.Map {
		return nil
	}

	if innerType.Key().Kind() != reflect.String {
		return fmt.Errorf("%w, got %s", errNonStringMapKey, innerType.Key())
	}

	for _, tag := range []string{"enum", "range", "len"} {
		if hasTag(structField.Tag, tag) {
			return fmt.Errorf("%w: tag %q is not supported on map types. Use tag \"items\" to constrain the number of map entries, or custom domain types with SelfValidator to validate map entries", ErrInvalidStructTag, tag)
		}
	}

	return nil
}

// implements reports whether target satisfies the interface type T.
func implements[T any](target any) bool {
	_, ok := target.(T)

	return ok
}

// applyCommandMethodGuardrails rejects lifecycle methods that conflict with the
// struct-tag/flat-API conventions and populates subcommand help details from the
// Description()/Examples() methods where allowed. The root command must configure
// its metadata through app.Description()/app.HelpFooter()/app.Deprecated()/app.Examples(),
// while subcommands read Description()/Examples() from their struct and declare the
// 1-line synopsis and deprecation notice through struct tags.
func applyCommandMethodGuardrails(target any, cmd *Command, structName string, isRoot bool) error {
	if isRoot {
		return guardRootCommandMethods(target, structName)
	}

	return guardSubcommandMethods(target, cmd, structName)
}

func guardRootCommandMethods(target any, structName string) error {
	for _, guard := range rootCommandMethodGuards {
		if guard.rejects(target) {
			return fmt.Errorf("%w: %s", ErrSchema, guard.message(structName))
		}
	}

	return nil
}

func guardSubcommandMethods(target any, cmd *Command, structName string) error {
	if _, ok := target.(summaryMethod); ok {
		return fmt.Errorf("%w: subcommand struct %s must not implement Summary(); declare the 1-line synopsis via the summary:\"...\" struct tag", ErrSchema, structName)
	}

	if _, ok := target.(deprecatedMethod); ok {
		return fmt.Errorf("%w: subcommand struct %s must not implement Deprecated(); declare the deprecation notice via the deprecated:\"...\" struct tag", ErrSchema, structName)
	}

	if _, ok := target.(helpFooterMethod); ok {
		return fmt.Errorf("%w: subcommand struct %s must not implement HelpFooter(); help footers are global and must be configured via app.HelpFooter()", ErrSchema, structName)
	}

	if m, ok := target.(descriptionMethod); ok {
		if desc := strings.TrimSpace(m.Description()); desc != "" {
			cmd.Description = desc
		}
	}

	if m, ok := target.(examplesMethod); ok {
		if examples := strings.TrimSpace(m.Examples()); examples != "" {
			cmd.Examples = examples
		}
	}

	return nil
}

// validateAncestorFlagUniqueness rejects flags whose name, alias, or short rune
// collides with a flag declared on an ancestor command. Because the binder
// resolves each flag to a single spec across the full command chain
// (leaf wins), a shadowed ancestor flag would silently keep its default, so
// the collision is a developer setup mistake rather than a runtime choice.
// Sibling branches are checked independently since they never share a chain.
// Engine-owned flags (the synthesized version --json selector) are exempt:
// the engine binds them by identity rather than through the user binding
// pipeline, so a same-named user flag keeps its own binding.
func validateAncestorFlagUniqueness(root *Command) error {
	for _, subcommand := range root.Subcommands {
		ancestorFlags := make(map[string]string)
		ancestorShorts := make(map[rune]string)

		addCommandFlags(root, ancestorFlags, ancestorShorts)

		if err := checkDescendantFlags(subcommand, ancestorFlags, ancestorShorts); err != nil {
			return err
		}
	}

	return nil
}

func addCommandFlags(cmd *Command, names map[string]string, shorts map[rune]string) {
	for _, flagSpec := range cmd.Flags {
		if flagSpec.EngineOwned {
			continue
		}

		names[flagSpec.Name] = cmd.Name
		for _, alias := range flagSpec.Aliases {
			names[alias] = cmd.Name
		}

		if flagSpec.Short != 0 {
			shorts[flagSpec.Short] = cmd.Name
		}
	}
}

func checkDescendantFlags(cmd *Command, ancestorFlags map[string]string, ancestorShorts map[rune]string) error {
	for _, flagSpec := range cmd.Flags {
		if flagSpec.EngineOwned {
			continue
		}

		if ancestor, ok := ancestorFlags[flagSpec.Name]; ok {
			return fmt.Errorf("command %q: %w: --%s (already declared by ancestor command %q)", cmd.Name, ErrDuplicateFlag, flagSpec.Name, ancestor)
		}

		for _, alias := range flagSpec.Aliases {
			if ancestor, ok := ancestorFlags[alias]; ok {
				return fmt.Errorf("command %q: %w: alias --%s (already declared by ancestor command %q)", cmd.Name, ErrDuplicateFlag, alias, ancestor)
			}
		}

		if flagSpec.Short != 0 {
			if ancestor, ok := ancestorShorts[flagSpec.Short]; ok {
				return fmt.Errorf("command %q: %w: short flag -%c (already declared by ancestor command %q)", cmd.Name, ErrDuplicateFlag, flagSpec.Short, ancestor)
			}
		}
	}

	branchFlags := maps.Clone(ancestorFlags)
	branchShorts := maps.Clone(ancestorShorts)
	addCommandFlags(cmd, branchFlags, branchShorts)

	for _, subcommand := range cmd.Subcommands {
		if err := checkDescendantFlags(subcommand, branchFlags, branchShorts); err != nil {
			return err
		}
	}

	return nil
}

func validateFlagSubcommandCollisions(command *Command) error {
	for _, flagSpec := range command.Flags {
		if flagSpec.ConfigKey == "-" {
			continue
		}

		var keysToCheck []string
		if flagSpec.ConfigKey != "" {
			keysToCheck = []string{flagSpec.ConfigKey}
		} else {
			keysToCheck = append([]string{flagSpec.Name}, flagSpec.Aliases...)
		}

		for _, subcommand := range command.Subcommands {
			for _, key := range keysToCheck {
				if key == subcommand.Name {
					return fmt.Errorf("command %q: %w: flag --%s collides with subcommand %q (ambiguous in config files; use config:\"-\" to exclude flag from config or rename)", command.Name, ErrDuplicateFlag, flagSpec.Name, subcommand.Name)
				}

				for _, alias := range subcommand.Aliases {
					if key == alias {
						return fmt.Errorf("command %q: %w: flag --%s collides with subcommand alias %q (ambiguous in config files; use config:\"-\" to exclude flag from config or rename)", command.Name, ErrDuplicateFlag, flagSpec.Name, alias)
					}
				}
			}
		}
	}

	for _, subcommand := range command.Subcommands {
		if err := validateFlagSubcommandCollisions(subcommand); err != nil {
			return err
		}
	}

	return nil
}

func extractConfigFileHelp(tag reflect.StructTag, isMulti bool) string {
	if helpText := tag.Get("help"); helpText != "" {
		return helpText
	}

	if isMulti {
		return "Path to configuration file(s)"
	}

	return "Path to configuration file"
}

func extractConfigFilePlaceholder(tag reflect.StructTag) string {
	if placeholder := tag.Get("placeholder"); placeholder != "" {
		return placeholder
	}

	return "PATH"
}

func buildInternal(target any, defaultName string, isRoot bool, opts Options) (*Command, error) {
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w, got %T", ErrInvalidTarget, target)
	}

	structValue := value.Elem()

	builder, err := newSchemaBuilder(target, defaultName, structValue, isRoot, opts)
	if err != nil {
		return nil, err
	}

	for i := range structValue.NumField() {
		structField := structValue.Type().Field(i)
		fieldValue := structValue.Field(i)

		if err := builder.processField(structField, fieldValue); err != nil {
			return nil, err
		}
	}

	return builder.cmd, nil
}

func validateDisallowedTags(tag reflect.StructTag, keys []string, role string) error {
	if slices.Contains(keys, "required") {
		return fmt.Errorf("%w: tag \"required\" is not supported. Use conga.RequiredFlag[T] or conga.RequiredArg[T] instead", ErrInvalidStructTag)
	}

	if role == "config file flag" {
		if _, ok := tag.Lookup("default"); ok {
			return fmt.Errorf("%w: tag \"default\" is not supported on config-file flags; declare search locations with app.ConfigOptions(conga.ConfigPaths(...)) or conga.ConfigExplicit(...), or show an example value with placeholder:\"...\"", ErrInvalidStructTag)
		}
	}

	if _, ok := tag.Lookup("help"); ok {
		if role == "subcommand" {
			return fmt.Errorf("%w: tag 'help' is not supported on Command[T]; use 'summary' for the 1-line synopsis", ErrInvalidStructTag)
		}

		if strings.HasSuffix(role, " command") {
			return fmt.Errorf("%w: tag 'help' is not supported on built-in command fields; use 'summary' for the 1-line synopsis", ErrInvalidStructTag)
		}
	}

	return nil
}

func validateTagCandidates(keys []string, role string, candidates []string, allowedTags []string) error {
	for _, key := range keys {
		if slices.Contains(candidates, key) || isAllowedForeignTag(key) || slices.Contains(allowedTags, key) {
			continue
		}

		if isAnyCongaTag(key) {
			return fmt.Errorf("%w: tag %q is invalid on %s (tag %q is only valid on %s)", ErrInvalidStructTag, key, role, key, validContextDescription(key))
		}

		suggestion := findClosestTagSuggestion(key, candidates)
		if suggestion != "" {
			return fmt.Errorf("%w: unknown struct tag %q (did you mean %q?). Use conga:\"allow-unknown-tags\" or app.AllowTags(%q) to permit external tags", ErrInvalidStructTag, key, suggestion, key)
		}

		return fmt.Errorf("%w: unknown struct tag %q. Use conga:\"allow-unknown-tags\" or app.AllowTags(%q) to permit external tags", ErrInvalidStructTag, key, key)
	}

	return nil
}

func scanSentinelDirectives(structType reflect.Type, opts *Options) (string, error) {
	var envPrefix string

	for structField := range structType.Fields() {
		if structField.Name == "_" {
			if prefix, ok := structField.Tag.Lookup("envprefix"); ok {
				envPrefix = prefix
			}
		}

		if congaTag, ok := structField.Tag.Lookup("conga"); ok {
			dirs, err := ParseCongaDirectives(congaTag)
			if err != nil {
				return "", fmt.Errorf("struct %s field %s: %w", structType.Name(), structField.Name, err)
			}

			if dirs.AllowUnknownTags {
				opts.AllowUnknownTags = true
				if dirs.Recurse {
					opts.AllowUnknownTagsRecurse = true
				}
			}
		}
	}

	return envPrefix, nil
}
