package conga

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/conga-sh/conga/internal/casing"
	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/levenshtein"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/stringx"
	"github.com/conga-sh/conga/internal/tokenizer"
)

const (
	mapParserErrorNotMap         mapParserErrorKind = iota // parser returned a non-map value
	mapParserErrorNotConvertible                           // returned map type is not convertible
)

// bindingState holds the binder's working state for one run: runtime options,
// external configuration maps, and the engine-configured standard streams
// projected from the session into per-value decode contexts.
type bindingState struct {
	EnvPrefix   string
	ConfigData  map[string]string
	TypeParsers coerce.Registry
	Stdin       io.Reader
	Stdout      io.Writer
	// EnvLookup overrides os.LookupEnv for environment binding. The completion
	// path supplies the shell's injected environment; nil reads the process
	// environment directly.
	EnvLookup completion.EnvLookup
	// TolerateFeatureErrors degrades feature resolution failures to the
	// canonical defaults plus valid provided keys. Builtin requests set it so
	// rendering never fails on resolution errors.
	TolerateFeatureErrors bool
	// CustomMapMerger merges registered custom map types. Populated when a map
	// type is registered through RegisterMap or RegisterType; nil keeps custom
	// map reflection unreferenced.
	CustomMapMerger func(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState, wantProvenance bool, result *mapMergeResult) error
}

// decodeContext composes the per-value context for one field's coercion: the
// field's declared tags plus the run's engine-configured standard streams.
func (b *bindingState) decodeContext(tags spec.FieldTags) coerce.DecodeContext {
	return coerce.DecodeContext{Tags: tags, Stdin: b.Stdin, Stdout: b.Stdout}
}

type tokenParser struct {
	cmdChain   []*spec.Command
	leaf       *spec.Command
	flagMap    map[string]*spec.Flag
	shortMap   map[rune]*spec.Flag
	scanner    *tokenizer.Scanner
	flagValues map[*spec.Flag][]string
	posTokens  []string
	dashTokens []string
	rawTokens  []string
}

func newTokenParser(cmdChain []*spec.Command, tokens []tokenizer.Token, index *flagIndex) *tokenParser {
	leaf := cmdChain[len(cmdChain)-1]

	if _, hasHelp := index.long["help"]; !hasHelp && !leaf.DisableHelp {
		index.long["help"] = spec.BuiltinHelpFlag
	}

	isRoot := len(cmdChain) == 1

	// The built-in --version flag only exists on the root command. Version
	// information describes the whole application, so accepting --version on
	// subcommands would diverge from every major framework (cobra, clap,
	// click, etc.) and force each help screen to repeat it.
	if _, hasVersion := index.long["version"]; !hasVersion && leaf.VersionFlagVisible(isRoot) {
		index.long["version"] = spec.BuiltinVersionFlag
	}

	// The built-in --why flag is available on every command, like --help: the
	// value-provenance report it renders describes the whole invocation.
	if _, hasWhy := index.long["why"]; !hasWhy && leaf.WhyFlagVisible() {
		index.long["why"] = spec.BuiltinWhyFlag
	}

	// The built-in --completion flag only exists on the root command when no
	// completion subcommand is present on the command hierarchy and the command
	// has no user subcommands (i.e. flat CLI).
	if _, hasCompletion := index.long["completion"]; !hasCompletion && leaf.CompletionFlagVisible(isRoot) {
		index.long["completion"] = spec.BuiltinCompletionFlag
	}

	return &tokenParser{
		cmdChain:   cmdChain,
		leaf:       leaf,
		flagMap:    index.long,
		shortMap:   index.short,
		scanner:    tokenizer.NewScanner(tokens),
		flagValues: make(map[*spec.Flag][]string),
		posTokens:  make([]string, 0),
		dashTokens: nil,
		rawTokens:  make([]string, 0),
	}
}

func (parser *tokenParser) canFallbackToRaw() bool {
	return parser.leaf.RawArgs != nil && len(parser.posTokens) >= len(parser.leaf.Args)
}

func (parser *tokenParser) consumeRemainingToRaw(firstToken string) {
	parser.rawTokens = append(parser.rawTokens, firstToken)
	for parser.scanner.HasNext() {
		t, _ := parser.scanner.Next()
		parser.rawTokens = append(parser.rawTokens, t.Raw)
	}
}

func (parser *tokenParser) isShortRegistered(tok tokenizer.Token) bool {
	if len(tok.ShortRunes) == 0 {
		return false
	}

	_, exists := parser.shortMap[tok.ShortRunes[0]]

	return exists
}

func (parser *tokenParser) findFlagSuggestion(name string) string {
	var candidates []string

	for flagName, flagSpec := range parser.flagMap {
		if flagSpec.Name == flagName {
			// A no- token is only an intentional negation of a negatable bool
			// flag. Skip candidates that a negated token could never legally
			// spell, so suggestions do not point at unrelated flags.
			if _, ok := strings.CutPrefix(name, "no-"); ok && !flagSpec.IsNegatable {
				continue
			}

			candidates = append(candidates, flagName)
		}
	}

	return levenshtein.Suggest(name, candidates, 3)
}

func (parser *tokenParser) lookupLongFlag(name string) (*spec.Flag, bool) {
	if flagSpec, ok := parser.flagMap[name]; ok {
		return flagSpec, false
	}

	if flagSpec := negatedLongFlag(name, func(n string) *spec.Flag { return parser.flagMap[n] }); flagSpec != nil {
		return flagSpec, true
	}

	return nil, false
}

func (parser *tokenParser) parseLongFlag(tok tokenizer.Token) error {
	name := tok.Name

	flagSpec, isNegated := parser.lookupLongFlag(name)
	if flagSpec == nil {
		if parser.canFallbackToRaw() {
			parser.consumeRemainingToRaw(tok.Raw)

			return nil
		}

		suggestion := parser.findFlagSuggestion(name)

		return &parseError{
			msg:        "unknown flag: --" + name,
			token:      tok.Raw,
			cmdChain:   parser.cmdChain,
			suggestion: suggestion,
			cause:      errUnknownFlag,
		}
	}

	if flagSpec.IsCounter {
		if tok.HasAttachedValue {
			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)
		} else {
			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "1")
		}

		return nil
	}

	if flagSpec.IsBool {
		if isNegated && tok.HasAttachedValue {
			return &parseError{
				msg:      "negated flag does not take a value: " + tok.Raw,
				token:    tok.Raw,
				cmdChain: parser.cmdChain,
				cause:    ErrCoerce,
			}
		}

		if isNegated {
			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "false")

			return nil
		}

		if tok.HasAttachedValue {
			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)

			return nil
		}

		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "true")

		return nil
	}

	if tok.HasAttachedValue {
		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)

		return nil
	}

	if flagSpec == spec.BuiltinCompletionFlag {
		return parser.handleCompletionFlag(flagSpec)
	}

	return parser.consumeNextValue(flagSpec, "--"+flagSpec.Name, tok.Raw)
}

func (parser *tokenParser) handleCompletionFlag(flagSpec *spec.Flag) error {
	if !parser.scanner.HasNext() {
		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "")

		return nil
	}

	nextTok, ok := parser.scanner.Peek()
	if !ok {
		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "")

		return nil
	}

	if (nextTok.Type == tokenizer.TokenTypeLongFlag || nextTok.Type == tokenizer.TokenTypeShortFlag || nextTok.Type == tokenizer.TokenTypeTerminator) && !nextTok.IsNegativeNumber() {
		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "")

		return nil
	}

	consumedTok, _ := parser.scanner.Next()
	parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], consumedTok.Raw)

	return nil
}

func (parser *tokenParser) consumeNextValue(flagSpec *spec.Flag, flagName, rawToken string) error {
	flagRequiresValueError := func() *parseError {
		return &parseError{
			msg:      "flag requires an argument: " + flagName,
			token:    rawToken,
			cmdChain: parser.cmdChain,
			cause:    errFlagRequiresValue,
		}
	}

	if parser.scanner.HasNext() {
		nextTok, _ := parser.scanner.Next()

		if (nextTok.Type == tokenizer.TokenTypeLongFlag || nextTok.Type == tokenizer.TokenTypeShortFlag || nextTok.Type == tokenizer.TokenTypeTerminator) && !nextTok.IsNegativeNumber() {
			return flagRequiresValueError()
		}

		parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], nextTok.Raw)

		return nil
	}

	return flagRequiresValueError()
}

func (parser *tokenParser) parseShortFlagCluster(tok tokenizer.Token) error {
	runes := tok.ShortRunes

	for i := range runes {
		r := runes[i]

		flagSpec, ok := parser.shortMap[r]
		if !ok {
			msg := "unknown flag: -" + string(r)
			if len(runes) > 1 {
				msg = "unknown shorthand flag: '" + string(r) + "' in -" + string(runes)
			}

			return &parseError{
				msg:      msg,
				token:    tok.Raw,
				cmdChain: parser.cmdChain,
				cause:    errUnknownFlag,
			}
		}

		if flagSpec.IsCounter {
			if tok.HasAttachedValue && i == len(runes)-1 {
				parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)
			} else {
				parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "1")
			}

			continue
		}

		if flagSpec.IsBool {
			if tok.HasAttachedValue && i == len(runes)-1 {
				parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)
			} else {
				parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], "true")
			}

			continue
		}

		if i < len(runes)-1 {
			value := string(runes[i+1:])
			if tok.HasAttachedValue {
				value = value + "=" + tok.Value
			}

			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], value)

			return nil
		}

		if tok.HasAttachedValue {
			parser.flagValues[flagSpec] = append(parser.flagValues[flagSpec], tok.Value)

			return nil
		}

		return parser.consumeNextValue(flagSpec, fmt.Sprintf("-%c", r), tok.Raw)
	}

	return nil
}

func (parser *tokenParser) parseShortFlag(tok tokenizer.Token) error {
	if tok.IsNegativeNumber() && !parser.isShortRegistered(tok) {
		if parser.canFallbackToRaw() {
			parser.consumeRemainingToRaw(tok.Raw)

			return nil
		}

		parser.posTokens = append(parser.posTokens, tok.Raw)

		return nil
	}

	runes := tok.ShortRunes
	if len(runes) > 0 {
		if _, ok := parser.shortMap[runes[0]]; !ok && parser.canFallbackToRaw() {
			parser.consumeRemainingToRaw(tok.Raw)

			return nil
		}
	}

	return parser.parseShortFlagCluster(tok)
}

func (parser *tokenParser) parsePositional(tok tokenizer.Token) {
	if parser.canFallbackToRaw() {
		parser.consumeRemainingToRaw(tok.Raw)

		return
	}

	parser.posTokens = append(parser.posTokens, tok.Raw)
}

func (parser *tokenParser) parseTerminator() {
	if parser.leaf.DashArgs != nil {
		parser.dashTokens = make([]string, 0)
		for parser.scanner.HasNext() {
			t, _ := parser.scanner.Next()
			parser.dashTokens = append(parser.dashTokens, t.Raw)
		}

		return
	}

	for parser.scanner.HasNext() {
		tok, _ := parser.scanner.Next()
		if parser.canFallbackToRaw() {
			parser.consumeRemainingToRaw(tok.Raw)

			return
		}

		parser.posTokens = append(parser.posTokens, tok.Raw)
	}
}

// flagIndex provides O(1) flag lookups over the commands accumulated during routing.
type flagIndex struct {
	long  map[string]*spec.Flag
	short map[rune]*spec.Flag
}

func newFlagIndex(cmd *spec.Command) *flagIndex {
	index := &flagIndex{
		long:  make(map[string]*spec.Flag),
		short: make(map[rune]*spec.Flag),
	}
	index.addCommand(cmd)

	return index
}

func (index *flagIndex) addCommand(cmd *spec.Command) {
	for _, flagSpec := range cmd.Flags {
		if flagSpec.NoCLI {
			continue
		}

		index.long[flagSpec.Name] = flagSpec
		for _, alias := range flagSpec.Aliases {
			index.long[alias] = flagSpec
		}

		if flagSpec.Short != 0 {
			index.short[flagSpec.Short] = flagSpec
		}
	}
}

func (index *flagIndex) find(tok tokenizer.Token) *spec.Flag {
	switch tok.Type {
	case tokenizer.TokenTypeLongFlag:
		if flagSpec := index.long[tok.Name]; flagSpec != nil {
			return flagSpec
		}

		if flagSpec := negatedLongFlag(tok.Name, func(n string) *spec.Flag { return index.long[n] }); flagSpec != nil {
			return flagSpec
		}

		return nil
	case tokenizer.TokenTypeShortFlag:
		if len(tok.ShortRunes) == 1 {
			return index.short[tok.ShortRunes[0]]
		}

		return nil
	default:
		return nil
	}
}

type constraintSet struct {
	enum       []string
	length     *spec.Range
	valueRange *spec.Range
	items      *spec.Range
}

// builtinRequest captures which builtin flags were requested on the command line.
type builtinRequest struct {
	hasHelp         bool
	hasVersion      bool
	hasWhy          bool
	hasCompletion   bool
	completionValue string
}

// toleratesFeatureErrors reports whether the request is a builtin rendering
// request whose output must survive feature resolution errors.
func (r builtinRequest) toleratesFeatureErrors() bool {
	return r.hasHelp || r.hasVersion || r.hasWhy || r.hasCompletion
}

// versionOnly reports a request that renders solely from app metadata and never
// consults features, flags, or configuration: a lone --version. Any other
// builtin flag (help, why, completion) consumes the feature/config tiers, so it
// is not version-only.
func (r builtinRequest) versionOnly() bool {
	return r.hasVersion && !r.hasHelp && !r.hasWhy && !r.hasCompletion
}

// counterError is the root cause for unparseable counter flag values.
type counterError struct {
	raw   string
	cause error // *strconv.NumError, preserved for errors.As
}

func (e *counterError) Error() string {
	return fmt.Sprintf("invalid integer value %q", e.raw)
}

func (e *counterError) Unwrap() error {
	if e.cause != nil {
		return e.cause
	}

	return ErrCoerce
}

func (e *counterError) Is(target error) bool { return target == ErrCoerce }

// mapRegistrationError is the root cause for unregistered non-string map types.
type mapRegistrationError struct {
	targetType reflect.Type
}

func (e *mapRegistrationError) Error() string {
	return fmt.Sprintf("map type %s requires registration via app.RegisterMap", e.targetType)
}

func (e *mapRegistrationError) Unwrap() error { return ErrCoerce }

// mapParserErrorKind identifies the registered-map-parser shape failure mode.
type mapParserErrorKind uint8

// mapParserError is the root cause for registered map parsers returning a
// value of the wrong shape.
type mapParserError struct {
	targetType   reflect.Type
	returnedType reflect.Type
	kind         mapParserErrorKind
}

func (e *mapParserError) Error() string {
	switch e.kind {
	case mapParserErrorNotMap:
		return fmt.Sprintf("registered parser for %s returned %s, expected a map", e.targetType, e.returnedType)
	case mapParserErrorNotConvertible:
		return fmt.Sprintf("registered parser for %s returned %s", e.targetType, e.returnedType)
	default:
		return fmt.Sprintf("registered parser for %s returned %s", e.targetType, e.returnedType)
	}
}

func (e *mapParserError) Unwrap() error { return ErrCoerce }

// mapMergeResult is the outcome of folding a map flag's tiers: whether any
// tiers contributed, the merged value, the per-key source tiers recorded when
// requested, the failing tier when coercion fails, and the highest contributing tier.
type mapMergeResult struct {
	hasValues   bool
	merged      any
	provenance  map[string]ValueSource
	failedTier  ValueSource
	failedRaw   string
	highestTier ValueSource
	highestName string
}

type mapTierInput struct {
	has    bool
	values []string
	source ValueSource
	name   string
}

func bindSpecialArgs(leaf *spec.Command, dashArgs, rawTokens []string) {
	if leaf.DashArgs != nil && dashArgs != nil {
		argsStateOf(leaf.DashArgs.Handle).setArgs(dashArgs)
	}

	if leaf.RawArgs != nil && len(rawTokens) > 0 {
		argsStateOf(leaf.RawArgs.Handle).setArgs(rawTokens)
	}
}

func validateRunnableContract(leaf *spec.Command, cmdChain []*spec.Command) error {
	leafTarget := getCmdTarget(leaf)

	if _, isRunner := leafTarget.(Runner); !isRunner {
		if leaf.HasUserSubcommands() {
			return &parseError{
				msg:      fmt.Sprintf("subcommand required for %q", leaf.Name),
				cmdChain: cmdChain,
				cause:    errSubcommandRequired,
			}
		}

		return newNoRunnerError(leaf)
	}

	return nil
}

func newNoRunnerError(leaf *spec.Command) error {
	return fmt.Errorf("%w: command %q does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)", errNoRunner, leaf.Name)
}

func runValidators(ctx context.Context, session *Session, cmdChain []*spec.Command) error {
	if len(cmdChain) == 0 {
		return nil
	}

	if leaf := cmdChain[len(cmdChain)-1]; isMetaCommand(leaf) {
		if validator, ok := getCmdTarget(leaf).(Validator); ok {
			if err := validator.Validate(ctx, session); err != nil {
				return &validationError{
					msg:      err.Error(),
					cmdChain: cmdChain,
					cause:    ErrValidation,
				}
			}
		}

		return nil
	}

	for _, cmd := range cmdChain {
		if validator, ok := getCmdTarget(cmd).(Validator); ok {
			if err := validator.Validate(ctx, session); err != nil {
				return &validationError{
					msg:      err.Error(),
					cmdChain: cmdChain,
					cause:    ErrValidation,
				}
			}
		}
	}

	return nil
}

func hasUnknownFlagInTokens(index *flagIndex, tokens []tokenizer.Token) bool {
	for _, tok := range tokens {
		if tok.Type == tokenizer.TokenTypeLongFlag || tok.Type == tokenizer.TokenTypeShortFlag {
			if index.find(tok) == nil {
				return true
			}
		}
	}

	return false
}

func populateHelpCommandPath(helpCmd *spec.Command, tokens []tokenizer.Token, startIndex int) {
	target := getCmdTarget(helpCmd)
	if helpCommand, ok := target.(*HelpCommand); ok {
		var helpPath []string

		for j := startIndex; j < len(tokens); j++ {
			if tokens[j].Type == tokenizer.TokenTypePositional {
				helpPath = append(helpPath, tokens[j].Raw)
			}
		}

		helpCommand.path = helpPath
	}
}

func populateCompletionCommandShell(compCmd *spec.Command, tokens []tokenizer.Token, startIndex int) {
	target := getCmdTarget(compCmd)
	if compCommand, ok := target.(*CompletionCommand); ok {
		for j := startIndex; j < len(tokens); j++ {
			if tokens[j].Type == tokenizer.TokenTypePositional {
				compCommand.shell = tokens[j].Raw
				compCmd.Shell = tokens[j].Raw

				break
			}
		}
	}
}

// bindVersionCommandJSON feeds the parsed --json flag into the opt-in
// VersionCommand target. Version commands are meta commands, so they skip the
// regular binding pipeline and their flags are applied here instead. The
// engine-synthesized flag is matched by identity so a same-named user flag on
// an ancestor command can never feed this value.
func bindVersionCommandJSON(cmdChain []*spec.Command, versionCmd *spec.Command, flagValues map[*spec.Flag][]string) error {
	versionCommand, ok := getCmdTarget(versionCmd).(*VersionCommand)
	if !ok {
		return nil
	}

	for _, flagSpec := range versionCmd.Flags {
		if flagSpec.Name != spec.VersionJSONFlagName {
			continue
		}

		values, ok := flagValues[flagSpec]
		if !ok || len(values) == 0 {
			return nil
		}

		rawValue := values[len(values)-1]

		enabled, err := coerce.Value[bool](rawValue)
		if err != nil {
			return newFlagCoercionError(cmdChain, flagSpec, SourceCLI, "--"+flagSpec.Name, rawValue, err)
		}

		versionCommand.json = enabled

		return nil
	}

	return nil
}

func handlePositionalRouting(
	current *spec.Command,
	chain []*spec.Command,
	index *flagIndex,
	tokens []tokenizer.Token,
	position int,
) (*spec.Command, []*spec.Command, bool, error) {
	tok := tokens[position]
	matched := current.FindSubcommand(tok.Raw)

	if matched != nil {
		if matched.Handle != nil {
			commandStateOf(matched.Handle).markSelected()
		}

		if matched.IsHelpCmd {
			populateHelpCommandPath(matched, tokens, position+1)

			return matched, append(chain, matched), true, nil
		}

		if matched.IsCompletionCmd {
			populateCompletionCommandShell(matched, tokens, position+1)

			return matched, append(chain, matched), false, nil
		}

		return matched, append(chain, matched), false, nil
	}

	if current.IsHelpCmd {
		populateHelpCommandPath(current, tokens, position)

		return current, chain, true, nil
	}

	if current.IsCompletionCmd {
		populateCompletionCommandShell(current, tokens, position)

		return current, chain, false, nil
	}

	if current.HasUserSubcommands() && len(current.Args) == 0 && current.RawArgs == nil {
		if hasUnknownFlagInTokens(index, tokens) {
			return current, chain, false, nil
		}

		suggestion := suggestSubcommand(current, tok.Raw)

		return nil, nil, false, &parseError{
			msg:        "unknown command: " + tok.Raw,
			token:      tok.Raw,
			cmdChain:   chain,
			suggestion: suggestion,
			cause:      errUnknownCommand,
		}
	}

	return current, chain, false, nil
}

func routeSubcommands(cmd *spec.Command, tokens []tokenizer.Token) ([]*spec.Command, []tokenizer.Token, *flagIndex, error) {
	current := cmd
	chain := []*spec.Command{current}
	index := newFlagIndex(current)

	var remaining []tokenizer.Token

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		if skipped, next := skipFlagValue(tokens, i, index); skipped {
			remaining = append(remaining, tok)

			if next != i {
				i = next
				remaining = append(remaining, tokens[i])
			}

			continue
		}

		if tok.Type != tokenizer.TokenTypePositional {
			remaining = append(remaining, tok)

			continue
		}

		newCurrent, newChain, isHelpDone, err := handlePositionalRouting(current, chain, index, tokens, i)
		if err != nil {
			return nil, nil, nil, err
		}

		changed := newCurrent != current
		if changed {
			index.addCommand(newCurrent)
			current = newCurrent
			chain = newChain
		}

		if isHelpDone {
			return newChain, nil, index, nil
		}

		if changed {
			continue
		}

		// A positional routed into the completion command is consumed as
		// the shell argument; keep routing so trailing flags (e.g.
		// completion zsh --help) survive into token parsing.
		if current.IsCompletionCmd {
			continue
		}

		remaining = append(remaining, tok)
	}

	return chain, remaining, index, nil
}

func skipFlagValue(tokens []tokenizer.Token, position int, index *flagIndex) (bool, int) {
	tok := tokens[position]
	if (tok.Type != tokenizer.TokenTypeLongFlag && tok.Type != tokenizer.TokenTypeShortFlag) || tok.HasAttachedValue {
		return false, position
	}

	flagSpec := index.find(tok)
	if flagSpec == nil || flagSpec.IsBool || flagSpec.IsCounter {
		return false, position
	}

	if position+1 < len(tokens) {
		return true, position + 1
	}

	return true, position
}

func parseTokens(cmdChain []*spec.Command, tokens []tokenizer.Token, index *flagIndex) (map[*spec.Flag][]string, []string, []string, []string, error) {
	parser := newTokenParser(cmdChain, tokens, index)

	for parser.scanner.HasNext() {
		tok, _ := parser.scanner.Next()

		var err error

		switch tok.Type {
		case tokenizer.TokenTypeLongFlag:
			err = parser.parseLongFlag(tok)
		case tokenizer.TokenTypeShortFlag:
			err = parser.parseShortFlag(tok)
		case tokenizer.TokenTypePositional:
			parser.parsePositional(tok)
		case tokenizer.TokenTypeTerminator:
			parser.parseTerminator()
		case tokenizer.TokenTypeUnknown:
			// No-op for unknown token type
		}

		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	if leaf := cmdChain[len(cmdChain)-1]; leaf.IsVersionCmd {
		if err := bindVersionCommandJSON(cmdChain, leaf, parser.flagValues); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	return parser.flagValues, parser.posTokens, parser.dashTokens, parser.rawTokens, nil
}

func applyFlagCoercion(cmdChain []*spec.Command, flagSpec *spec.Flag, rawValues []string, source ValueSource, sourceName string, state *bindingState) error {
	if flagSpec.IsCounter {
		totalCount := 0

		for _, raw := range rawValues {
			parsedCount, err := strconv.Atoi(raw)
			if err != nil {
				return &coercionError{
					targetName: flagSpec.Name,
					isFlag:     true,
					source:     source,
					sourceName: sourceName,
					targetType: flagSpec.TargetType,
					rawValue:   raw,
					cmdChain:   cmdChain,
					cause:      &counterError{raw: raw, cause: err},
				}
			}

			totalCount += parsedCount
		}

		flagStateOf(flagSpec.Handle).setValue(totalCount, source, sourceName)

		return nil
	}

	decodeContext := state.decodeContext(flagSpec.FieldTags)

	if flagSpec.IsSlice {
		typedSlice, err := coerce.SliceWith(rawValues, flagSpec.TargetType, flagSpec.Sep, state.TypeParsers, decodeContext)
		if err != nil {
			return newFlagCoercionError(cmdChain, flagSpec, source, sourceName, strings.Join(rawValues, flagSpec.Sep), err)
		}

		flagStateOf(flagSpec.Handle).setValue(typedSlice, source, sourceName)

		return nil
	}

	lastValue := rawValues[len(rawValues)-1]

	typedValue, err := coerce.ScalarWith(lastValue, flagSpec.TargetType, state.TypeParsers, decodeContext)
	if err != nil {
		return newFlagCoercionError(cmdChain, flagSpec, source, sourceName, lastValue, err)
	}

	flagStateOf(flagSpec.Handle).setValue(typedValue, source, sourceName)

	return nil
}

func applyFlagValue(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState) error {
	// Map flags merge every tier key-by-key rather than resolving a single
	// winning tier, so a config file and the command line can each contribute
	// different keys to the same map.
	if flagSpec.IsMap {
		var result mapMergeResult
		if err := foldMapTiers(cmd, cmdChain, flagSpec, cliValues, state, false, &result); err != nil {
			return err
		}

		if result.hasValues {
			flagStateOf(flagSpec.Handle).setValue(result.merged, result.highestTier, result.highestName)
		}

		return nil
	}

	// Tier 1: CLI Flags
	if values, ok := cliValues[flagSpec]; ok && len(values) > 0 {
		return applyFlagCoercion(cmdChain, flagSpec, values, SourceCLI, "--"+flagSpec.Name, state)
	}

	// Tier 2: Environment Variables
	if envValue, envName, ok := lookupEnv(flagSpec, cmd, state); ok {
		return applyFlagCoercion(cmdChain, flagSpec, []string{envValue}, SourceEnv, envName, state)
	}

	// Tier 3: Configuration Files
	if configValue, configKey, ok := lookupConfig(flagSpec, cmd, cmdChain, state); ok {
		return applyFlagCoercion(cmdChain, flagSpec, []string{configValue}, SourceConfig, configKey, state)
	}

	// Tier 4: Default Values
	if flagSpec.Default != "" {
		return applyFlagCoercion(cmdChain, flagSpec, []string{flagSpec.Default}, SourceDefault, flagSpec.Default, state)
	}

	return nil
}

func resolveRequiredFlags(cmdChain []*spec.Command) error {
	leaf := cmdChain[len(cmdChain)-1]
	if isMetaCommand(leaf) {
		return nil
	}

	for _, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Required && !flagStateOf(flagSpec.Handle).IsSet() {
				return &validationError{
					msg:      "missing required flag: --" + flagSpec.Name,
					cmdChain: cmdChain,
					cause:    errMissingFlag,
				}
			}
		}
	}

	return nil
}

func resolveFlags(cmdChain []*spec.Command, cliValues map[*spec.Flag][]string, state *bindingState) error {
	for _, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			// The synthesized feature map flag is already bound and validated
			// by resolveFeatureGates; binding it again would double the tier lookups,
			// parsing, and setValue work.
			if flagSpec == cmd.FeatureFlag {
				continue
			}

			if err := applyFlagValue(cmd, cmdChain, flagSpec, cliValues, state); err != nil {
				return err
			}
		}
	}

	return resolveRequiredFlags(cmdChain)
}

// scanBuiltinRequest inspects the raw CLI flag values for builtin flag requests.
func scanBuiltinRequest(cliValues map[*spec.Flag][]string) builtinRequest {
	var request builtinRequest

	for flagSpec, values := range cliValues {
		if len(values) == 0 {
			continue
		}

		switch flagSpec {
		case spec.BuiltinHelpFlag:
			request.hasHelp = true
		case spec.BuiltinVersionFlag:
			request.hasVersion = true
		case spec.BuiltinWhyFlag:
			request.hasWhy = true
		case spec.BuiltinCompletionFlag:
			request.hasCompletion = true
			request.completionValue = values[0]
		}
	}

	return request
}

func buildCompletionHelpCommand(cmdChain []*spec.Command, completionValue string, shells []string) (*spec.Command, error) {
	shell := completionValue
	if shell != "" {
		parsedShell, err := completion.ParseShell(shell)
		if err != nil {
			return nil, &validationError{
				msg:      err.Error(),
				cmdChain: cmdChain,
				cause:    errInvalidValue,
			}
		}

		shell = string(parsedShell)
	}

	compCmd := spec.NewCompletionCommand("completion", spec.DefaultCompletionCommandHelp, nil, nil, false, shells)
	compCmd.IsCompletionFlag = true
	compCmd.Shell = shell
	compCmd.Flags = []*spec.Flag{spec.BuiltinHelpFlag}

	return compCmd, nil
}

func buildBareCompletionHelpCommand(shells []string) *spec.Command {
	compCmd := spec.NewCompletionCommand("completion", spec.DefaultCompletionCommandHelp, nil, nil, false, shells)
	compCmd.IsCompletionFlag = true
	compCmd.Flags = []*spec.Flag{spec.BuiltinHelpFlag}

	return compCmd
}

func lookupEnv(flagSpec *spec.Flag, cmd *spec.Command, state *bindingState) (string, string, bool) {
	if flagSpec.EnvExcluded {
		return "", "", false
	}

	lookup := envLookupFunc(state)

	isString := flagSpec.TargetType != nil && flagSpec.TargetType.Kind() == reflect.String

	for _, envVar := range flagSpec.EnvVars {
		if value, ok := lookup(envVar); ok && (value != "" || isString) {
			return value, envVar, true
		}
	}

	prefix := cmd.EnvPrefix
	if prefix == "" && state != nil {
		prefix = state.EnvPrefix
	}

	if prefix != "" && len(flagSpec.EnvVars) == 0 {
		derived := prefix + casing.UpperSnake(flagSpec.Name)
		if value, ok := lookup(derived); ok && (value != "" || isString) {
			return value, derived, true
		}
	}

	return "", "", false
}

// envLookupFunc selects the environment reader for a binding run: the
// completion-injected lookup when supplied, otherwise the process environment.
func envLookupFunc(state *bindingState) func(string) (string, bool) {
	if state != nil && state.EnvLookup != nil {
		return state.EnvLookup
	}

	return os.LookupEnv
}

func lookupConfig(flagSpec *spec.Flag, cmd *spec.Command, cmdChain []*spec.Command, state *bindingState) (string, string, bool) {
	if state == nil || state.ConfigData == nil || flagSpec.ConfigKey == "-" {
		return "", "", false
	}

	isString := flagSpec.TargetType != nil && flagSpec.TargetType.Kind() == reflect.String

	// 1. Explicit ConfigKey if set
	if value, key, ok := lookupExplicitConfig(flagSpec, cmdChain, state.ConfigData, isString); ok {
		return value, key, true
	}

	// 2. Hierarchical path: e.g. "cluster.node.zone"
	if value, key, ok := lookupHierarchicalConfig(flagSpec, cmdChain, state.ConfigData, isString); ok {
		return value, key, true
	}

	// 3. Command-local path: e.g. "node.zone"
	if value, key, ok := lookupCommandLocalConfig(flagSpec, cmd, cmdChain, state.ConfigData, isString); ok {
		return value, key, true
	}

	// 4. Direct flag name: e.g. "port"
	if value, exists := state.ConfigData[flagSpec.Name]; exists && (value != "" || isString) {
		return value, flagSpec.Name, true
	}

	return "", "", false
}

func lookupExplicitConfig(flagSpec *spec.Flag, cmdChain []*spec.Command, configData map[string]string, isString bool) (string, string, bool) {
	if flagSpec.ConfigKey == "" {
		return "", "", false
	}

	if len(cmdChain) > 1 {
		cmdPrefix := strings.Join(slicex.Map(cmdChain[1:], func(cmd *spec.Command) string { return cmd.Name }), ".")
		if !strings.HasPrefix(flagSpec.ConfigKey, cmdPrefix+".") {
			namespacedKey := cmdPrefix + "." + flagSpec.ConfigKey
			if value, exists := configData[namespacedKey]; exists && (value != "" || isString) {
				return value, namespacedKey, true
			}
		}
	}

	if value, exists := configData[flagSpec.ConfigKey]; exists && (value != "" || isString) {
		return value, flagSpec.ConfigKey, true
	}

	return "", "", false
}

func lookupHierarchicalConfig(flagSpec *spec.Flag, cmdChain []*spec.Command, configData map[string]string, isString bool) (string, string, bool) {
	if len(cmdChain) <= 1 {
		return "", "", false
	}

	var chainNames []string

	for _, c := range cmdChain[1:] {
		if c.Name != "" {
			chainNames = append(chainNames, c.Name)
		}
	}

	if len(chainNames) > 0 {
		pathKey := strings.Join(chainNames, ".") + "." + flagSpec.Name
		if value, exists := configData[pathKey]; exists && (value != "" || isString) {
			return value, pathKey, true
		}
	}

	return "", "", false
}

func lookupCommandLocalConfig(flagSpec *spec.Flag, cmd *spec.Command, cmdChain []*spec.Command, configData map[string]string, isString bool) (string, string, bool) {
	if cmd != nil && len(cmdChain) > 0 && cmd != cmdChain[0] {
		cmdKey := cmd.Name + "." + flagSpec.Name
		if value, exists := configData[cmdKey]; exists && (value != "" || isString) {
			return value, cmdKey, true
		}
	}

	return "", "", false
}

func validateEnumValue(cmdChain []*spec.Command, descriptor string, enum []string, cmpStr string) error {
	if len(enum) == 0 {
		return nil
	}

	if !slices.Contains(enum, cmpStr) {
		return &validationError{
			msg: fmt.Sprintf("invalid value %q (must be one of: %s) for %s",
				truncateForDisplay(cmpStr), strings.Join(enum, ", "), descriptor),
			cmdChain:    cmdChain,
			cause:       errInvalidValue,
			valueShaped: true,
		}
	}

	return nil
}

func validateScalarValue(cmdChain []*spec.Command, descriptor string, constraints constraintSet, value any, enumStr string) error {
	if err := validateEnumValue(cmdChain, descriptor, constraints.enum, enumStr); err != nil {
		return err
	}

	if constraints.length != nil {
		if reason, ok := checkStringLen(value, constraints.length); !ok {
			return &validationError{
				msg:         fmt.Sprintf("value %q %s for %s", truncateForDisplay(stringx.Stringify(value)), reason, descriptor),
				cmdChain:    cmdChain,
				cause:       errInvalidLength,
				valueShaped: true,
			}
		}
	}

	if constraints.valueRange != nil {
		if reason, ok := checkValueRange(value, constraints.valueRange); !ok {
			return &validationError{
				msg:         fmt.Sprintf("value %v %s for %s", value, reason, descriptor),
				cmdChain:    cmdChain,
				cause:       errOutOfRange,
				valueShaped: true,
			}
		}
	}

	return nil
}

func validateSliceValue(cmdChain []*spec.Command, descriptor string, constraints constraintSet, typedSlice any) error {
	sliceValue := reflect.ValueOf(typedSlice)

	if constraints.items != nil {
		if reason, ok := checkItemCount(sliceValue, constraints.items); !ok {
			return &validationError{
				msg:         fmt.Sprintf("%s for %s", reason, descriptor),
				cmdChain:    cmdChain,
				cause:       errInvalidItemCount,
				valueShaped: true,
			}
		}
	}

	for i := range sliceValue.Len() {
		elem := sliceValue.Index(i).Interface()
		elemStr := stringx.Stringify(elem)

		if err := validateScalarValue(cmdChain, descriptor, constraints, elem, elemStr); err != nil {
			return err
		}
	}

	return nil
}

func validateMapValue(cmdChain []*spec.Command, descriptor string, items *spec.Range, value any) error {
	if value == nil {
		return nil
	}

	mapValue := reflect.ValueOf(value)
	if !mapValue.IsValid() {
		return nil
	}

	if items != nil {
		if reason, ok := checkItemCount(mapValue, items); !ok {
			return &validationError{
				msg:         fmt.Sprintf("%s for %s", reason, descriptor),
				cmdChain:    cmdChain,
				cause:       errInvalidItemCount,
				valueShaped: true,
			}
		}
	}

	return nil
}

func validateXorGroups(cmdChain []*spec.Command, xorGroups map[string][]*spec.Flag) error {
	for _, flags := range xorGroups {
		var setFlags []string

		for _, flag := range flags {
			if flagStateOf(flag.Handle).Changed() {
				setFlags = append(setFlags, "--"+flag.Name)
			}
		}

		if len(setFlags) > 1 {
			return &validationError{
				msg: fmt.Sprintf("flags %s are mutually exclusive",
					strings.Join(setFlags, " and ")),
				cmdChain: cmdChain,
				cause:    errMutuallyExclusive,
			}
		}
	}

	return nil
}

func validateAndGroups(cmdChain []*spec.Command, andGroups map[string][]*spec.Flag) error {
	for _, flags := range andGroups {
		var (
			setFlags   []string
			unsetFlags []string
		)

		for _, flag := range flags {
			if flagStateOf(flag.Handle).Changed() {
				setFlags = append(setFlags, "--"+flag.Name)
			} else {
				unsetFlags = append(unsetFlags, "--"+flag.Name)
			}
		}

		if len(setFlags) > 0 && len(unsetFlags) > 0 {
			return &validationError{
				msg: fmt.Sprintf("flags %s must be provided together with %s",
					strings.Join(setFlags, " and "), strings.Join(unsetFlags, ", ")),
				cmdChain: cmdChain,
				cause:    errAndGroup,
			}
		}
	}

	return nil
}

func validateFlagGroupConstraints(cmdChain []*spec.Command, flagSpec *spec.Flag) error {
	if !flagStateOf(flagSpec.Handle).IsSet() {
		return nil
	}

	value := flagStateOf(flagSpec.Handle).getValue()
	constraints := constraintSet{
		enum:       flagSpec.Enum,
		length:     flagSpec.Len,
		valueRange: flagSpec.Range,
		items:      flagSpec.Items,
	}
	descriptor := "flag --" + flagSpec.Name

	if flagSpec.IsSlice {
		return validateSliceValue(cmdChain, descriptor, constraints, value)
	}

	if flagSpec.IsMap {
		return validateMapValue(cmdChain, descriptor, constraints.items, value)
	}

	return validateScalarValue(cmdChain, descriptor, constraints, value, stringx.Stringify(value))
}

func validateFlagConstraints(cmdChain []*spec.Command) error {
	for _, cmd := range cmdChain {
		xorGroups := make(map[string][]*spec.Flag)
		andGroups := make(map[string][]*spec.Flag)

		for _, flagSpec := range cmd.Flags {
			if err := validateFlagGroupConstraints(cmdChain, flagSpec); err != nil {
				return err
			}

			if flagSpec.XorGroup != "" {
				xorGroups[flagSpec.XorGroup] = append(xorGroups[flagSpec.XorGroup], flagSpec)
			}

			if flagSpec.AndGroup != "" {
				andGroups[flagSpec.AndGroup] = append(andGroups[flagSpec.AndGroup], flagSpec)
			}
		}

		if err := validateXorGroups(cmdChain, xorGroups); err != nil {
			return err
		}

		if err := validateAndGroups(cmdChain, andGroups); err != nil {
			return err
		}
	}

	return nil
}

func resolveSliceArg(cmdChain []*spec.Command, arg *spec.Arg, remaining []string, parsers coerce.Registry, decodeContext coerce.DecodeContext) error {
	decodeContext.Tags = arg.FieldTags

	if len(remaining) == 0 {
		return resolveEmptySliceArg(cmdChain, arg, parsers, decodeContext)
	}

	typedSlice, err := coerce.SliceWith(remaining, arg.TargetType, arg.Sep, parsers, decodeContext)
	if err != nil {
		return newArgCoercionError(cmdChain, arg, SourceCLI, "", strings.Join(remaining, arg.Sep), err)
	}

	if err := validateArgSlice(cmdChain, arg, typedSlice); err != nil {
		return err
	}

	argStateOf(arg.Handle).setValue(typedSlice, SourceCLI, "")

	return nil
}

func resolveEmptySliceArg(cmdChain []*spec.Command, arg *spec.Arg, parsers coerce.Registry, decodeContext coerce.DecodeContext) error {
	if arg.Required {
		return &validationError{
			msg:      "missing required positional argument " + arg.Name,
			cmdChain: cmdChain,
			cause:    errMissingArg,
		}
	}

	if arg.Default != "" {
		typedSlice, err := coerce.SliceWith([]string{arg.Default}, arg.TargetType, arg.Sep, parsers, decodeContext)
		if err != nil {
			return newArgCoercionError(cmdChain, arg, SourceDefault, arg.Default, arg.Default, err)
		}

		if err := validateArgSlice(cmdChain, arg, typedSlice); err != nil {
			return err
		}

		argStateOf(arg.Handle).setValue(typedSlice, SourceDefault, arg.Default)
	}

	return nil
}

func resolveMapArg(cmdChain []*spec.Command, arg *spec.Arg, tokens []string, parsers coerce.Registry) error {
	if len(tokens) > 0 {
		parsedMap, err := parseMapTokens(tokens, arg.TargetType, arg.Sep, parsers)
		if err != nil {
			return newArgCoercionError(cmdChain, arg, SourceCLI, "", strings.Join(tokens, " "), err)
		}

		if err := validateArgMap(cmdChain, arg, parsedMap); err != nil {
			return err
		}

		argStateOf(arg.Handle).setValue(parsedMap, SourceCLI, "")

		return nil
	}

	if arg.Default != "" {
		parsedMap, err := parseMapTokens([]string{arg.Default}, arg.TargetType, arg.Sep, parsers)
		if err != nil {
			return newArgCoercionError(cmdChain, arg, SourceDefault, arg.Default, arg.Default, err)
		}

		if err := validateArgMap(cmdChain, arg, parsedMap); err != nil {
			return err
		}

		argStateOf(arg.Handle).setValue(parsedMap, SourceDefault, arg.Default)

		return nil
	}

	if arg.Required {
		return &validationError{
			msg:      "missing required positional argument " + arg.Name,
			cmdChain: cmdChain,
			cause:    errMissingArg,
		}
	}

	return nil
}

func resolveScalarArg(cmdChain []*spec.Command, arg *spec.Arg, posTokens []string, tokenIdx int, parsers coerce.Registry, decodeContext coerce.DecodeContext) (bool, error) {
	decodeContext.Tags = arg.FieldTags

	if tokenIdx < len(posTokens) {
		raw := posTokens[tokenIdx]

		typedValue, err := coerce.ScalarWith(raw, arg.TargetType, parsers, decodeContext)
		if err != nil {
			return false, newArgCoercionError(cmdChain, arg, SourceCLI, "", raw, err)
		}

		if err := validateArgScalar(cmdChain, arg, typedValue); err != nil {
			return false, err
		}

		argStateOf(arg.Handle).setValue(typedValue, SourceCLI, "")

		return true, nil
	}

	if arg.Default != "" {
		typedValue, err := coerce.ScalarWith(arg.Default, arg.TargetType, parsers, decodeContext)
		if err != nil {
			return false, newArgCoercionError(cmdChain, arg, SourceDefault, arg.Default, arg.Default, err)
		}

		if err := validateArgScalar(cmdChain, arg, typedValue); err != nil {
			return false, err
		}

		argStateOf(arg.Handle).setValue(typedValue, SourceDefault, arg.Default)

		return false, nil
	}

	if arg.Required {
		return false, &validationError{
			msg:      "missing required positional argument " + arg.Name,
			cmdChain: cmdChain,
			cause:    errMissingArg,
		}
	}

	return false, nil
}

func handleUnexpectedPositional(cmd *spec.Command, cmdChain []*spec.Command, unexpected string) error {
	if suggestion := suggestSubcommand(cmd, unexpected); suggestion != "" {
		return &parseError{
			msg:        "unknown command: " + unexpected,
			token:      unexpected,
			cmdChain:   cmdChain,
			suggestion: suggestion,
			cause:      errUnknownCommand,
		}
	}

	return &validationError{
		msg:      fmt.Sprintf("unexpected argument %q", unexpected),
		cmdChain: cmdChain,
		cause:    errUnexpectedArg,
	}
}

func suggestSubcommand(cmd *spec.Command, input string) string {
	if cmd == nil || len(cmd.Subcommands) == 0 {
		return ""
	}

	subNames := make([]string, 0, len(cmd.Subcommands))

	for _, subcommand := range cmd.Subcommands {
		if !subcommand.Hidden {
			subNames = append(subNames, subcommand.Name)
		}
	}

	return levenshtein.Suggest(input, subNames, 3)
}

// resolveArgs binds positional argument tokens. decodeContext carries the
// run's engine-configured standard streams (no tags); each arg's declared
// tags are composed into the context at its coercion site.
func resolveArgs(cmd *spec.Command, cmdChain []*spec.Command, posTokens []string, parsers coerce.Registry, decodeContext coerce.DecodeContext) error {
	tokenIdx := 0

	for _, arg := range cmd.Args {
		if arg.IsSlice {
			if err := resolveSliceArg(cmdChain, arg, posTokens[tokenIdx:], parsers, decodeContext); err != nil {
				return err
			}

			// Slice positional argument consumes all remaining positional tokens,
			// satisfying the trailing token check below (tokenIdx == len(posTokens)).
			tokenIdx = len(posTokens)

			break
		}

		if arg.IsMap {
			if err := resolveMapArg(cmdChain, arg, posTokens[tokenIdx:], parsers); err != nil {
				return err
			}

			// Map positional argument consumes all remaining positional tokens,
			// satisfying the trailing token check below (tokenIdx == len(posTokens)).
			tokenIdx = len(posTokens)

			break
		}

		advanced, err := resolveScalarArg(cmdChain, arg, posTokens, tokenIdx, parsers, decodeContext)
		if err != nil {
			return err
		}

		if advanced {
			tokenIdx++
		}
	}

	// Check for unexpected extra positional arguments
	if tokenIdx < len(posTokens) {
		return handleUnexpectedPositional(cmd, cmdChain, posTokens[tokenIdx])
	}

	return nil
}

func validateArgScalar(cmdChain []*spec.Command, arg *spec.Arg, typedValue any) error {
	constraints := constraintSet{
		enum:       arg.Enum,
		length:     arg.Len,
		valueRange: arg.Range,
		items:      arg.Items,
	}

	return validateScalarValue(cmdChain, "argument "+arg.Name, constraints, typedValue, stringx.Stringify(typedValue))
}

func validateArgSlice(cmdChain []*spec.Command, arg *spec.Arg, typedSlice any) error {
	constraints := constraintSet{
		enum:       arg.Enum,
		length:     arg.Len,
		valueRange: arg.Range,
		items:      arg.Items,
	}

	return validateSliceValue(cmdChain, "argument "+arg.Name, constraints, typedSlice)
}

func validateArgMap(cmdChain []*spec.Command, arg *spec.Arg, typedMap any) error {
	return validateMapValue(cmdChain, "argument "+arg.Name, arg.Items, typedMap)
}

func checkOrderedRange[T cmp.Ordered](value, minValue, maxValue T, hasMin, hasMax bool, minStr, maxStr string) (string, bool) {
	if hasMin && hasMax {
		if value < minValue || value > maxValue {
			return fmt.Sprintf("is out of range [%s..%s]", minStr, maxStr), false
		}

		return "", true
	}

	if hasMin && value < minValue {
		return "must be >= " + minStr, false
	}

	if hasMax && value > maxValue {
		return "must be <= " + maxStr, false
	}

	return "", true
}

// negatedLongFlag re-resolves a long flag name prefixed with no- through lookup,
// enforcing the spec.CanNegate guard: a no- token may only re-bind a boolean
// negatable flag.
func negatedLongFlag(name string, lookup func(string) *spec.Flag) *spec.Flag {
	posName, ok := strings.CutPrefix(name, "no-")
	if !ok {
		return nil
	}

	flagSpec := lookup(posName)
	if !spec.CanNegate(flagSpec) {
		return nil
	}

	return flagSpec
}

func checkTypedRange[T cmp.Ordered](value T, rangeSpec *spec.Range, parse func(string) (T, error)) (string, bool) {
	var (
		minValue T
		maxValue T
	)

	if rangeSpec.HasMin {
		parsed, err := parse(rangeSpec.MinStr)
		if err != nil {
			return fmt.Sprintf("invalid range bound: %v", err), false
		}

		minValue = parsed
	}

	if rangeSpec.HasMax {
		parsed, err := parse(rangeSpec.MaxStr)
		if err != nil {
			return fmt.Sprintf("invalid range bound: %v", err), false
		}

		maxValue = parsed
	}

	return checkOrderedRange(value, minValue, maxValue, rangeSpec.HasMin, rangeSpec.HasMax, rangeSpec.MinStr, rangeSpec.MaxStr)
}

func parseInt64Bound(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func parseUint64Bound(raw string) (uint64, error) {
	return strconv.ParseUint(raw, 10, 64)
}

func parseFloat64Bound(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}

// checkTextRange applies range bounds to a value whose type carries its own
// text grammar: bounds parse through the type's UnmarshalText and compare with
// the value in its native domain. handled is false when the value has no
// text-range projection, leaving the numeric and duration branches in charge.
func checkTextRange(value any, rangeSpec *spec.Range) (string, bool, bool) {
	if value == nil {
		return "", false, false
	}

	elemType := reflect.TypeOf(value)
	if elemType.Kind() == reflect.Pointer {
		elemType = elemType.Elem()
	}

	domain, _, isTextRange := spec.TextRangeDomain(elemType)
	if !isTextRange {
		return "", false, false
	}

	switch domain {
	case spec.RangeSigned:
		return checkProjectedRange[int64](elemType, value, rangeSpec)
	case spec.RangeUnsigned:
		return checkProjectedRange[uint64](elemType, value, rangeSpec)
	case spec.RangeFloat:
		return checkProjectedRange[float64](elemType, value, rangeSpec)
	default:
		return "", false, false
	}
}

func checkProjectedRange[T cmp.Ordered](elemType reflect.Type, value any, rangeSpec *spec.Range) (string, bool, bool) {
	projected, projectedOK := spec.ProjectTextValue[T](elemType, reflect.ValueOf(value))
	if !projectedOK {
		// Schema build rejects range tags on pointer element types, so a
		// classified text range always projects; fail closed if that invariant
		// ever breaks rather than silently skipping validation.
		return "has no comparable value", false, true
	}

	parse, parseOK := spec.TextRangeParser[T](elemType)
	if !parseOK {
		return "has no parseable range bounds", false, true
	}

	reason, ok := checkTypedRange(projected, rangeSpec, parse)

	return reason, ok, true
}

func checkValueRange(value any, rangeSpec *spec.Range) (string, bool) {
	if rangeSpec == nil {
		return "", true
	}

	if reason, ok, handled := checkTextRange(value, rangeSpec); handled {
		return reason, ok
	}

	if duration, ok := value.(time.Duration); ok {
		return checkTypedRange(duration, rangeSpec, time.ParseDuration)
	}

	reflectedValue := reflect.ValueOf(value)

	switch reflectedValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return checkTypedRange(reflectedValue.Int(), rangeSpec, parseInt64Bound)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return checkTypedRange(reflectedValue.Uint(), rangeSpec, parseUint64Bound)

	case reflect.Float32, reflect.Float64:
		return checkTypedRange(reflectedValue.Float(), rangeSpec, parseFloat64Bound)

	default:
		return "", true
	}
}

func parseIntBound(raw string) (int, bool) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}

	return value, true
}

func formatBoundsError(count, bound int, format string, countFirst bool) string {
	if countFirst {
		return fmt.Sprintf(format, count, bound)
	}

	return fmt.Sprintf(format, bound, count)
}

func checkIntRange(count int, rangeSpec *spec.Range, exactFmt, rangeFmt, minFmt, maxFmt string, countFirst bool) (string, bool) {
	if rangeSpec == nil {
		return "", true
	}

	minValue, parsedMin := parseIntBound(rangeSpec.MinStr)
	maxValue, parsedMax := parseIntBound(rangeSpec.MaxStr)

	hasMin := rangeSpec.HasMin && parsedMin
	hasMax := rangeSpec.HasMax && parsedMax

	if rangeSpec.Exact && parsedMin {
		if count != minValue {
			return formatBoundsError(count, minValue, exactFmt, countFirst), false
		}

		return "", true
	}

	if hasMin && hasMax {
		if count < minValue || count > maxValue {
			return fmt.Sprintf(rangeFmt, count, minValue, maxValue), false
		}

		return "", true
	}

	if hasMin && count < minValue {
		return formatBoundsError(count, minValue, minFmt, countFirst), false
	}

	if hasMax && count > maxValue {
		return formatBoundsError(count, maxValue, maxFmt, countFirst), false
	}

	return "", true
}

func checkStringLen(value any, rangeSpec *spec.Range) (string, bool) {
	if rangeSpec == nil {
		return "", true
	}

	length := utf8.RuneCountInString(stringx.Stringify(value))

	return checkIntRange(length, rangeSpec,
		"length %d must be exactly %d",
		"length %d is out of range [%d..%d]",
		"length %d must be >= %d",
		"length %d must be <= %d",
		true,
	)
}

func checkItemCount(collectionValue reflect.Value, rangeSpec *spec.Range) (string, bool) {
	if rangeSpec == nil {
		return "", true
	}

	return checkIntRange(collectionValue.Len(), rangeSpec,
		"expected exactly %d items, got %d",
		"item count %d is out of range [%d..%d]",
		"expected at least %d items, got %d",
		"expected at most %d items, got %d",
		false,
	)
}

// callSelfValidator invokes SelfValidator on value, asserting both the direct
// interface and the pointer-receiver form on an addressable copy, mirroring
// how the coercion pipeline resolves TextUnmarshaler domain types.
func callSelfValidator(ctx context.Context, value any) error {
	if value == nil {
		return nil
	}

	if v, ok := value.(SelfValidator); ok {
		return v.Validate(ctx)
	}

	reflectedValue := reflect.ValueOf(value)
	if reflectedValue.CanAddr() {
		if v, ok := reflect.TypeAssert[SelfValidator](reflectedValue.Addr()); ok {
			return v.Validate(ctx)
		}
	} else if reflectedValue.Kind() != reflect.Pointer && reflectedValue.IsValid() {
		valueType := reflectedValue.Type()
		if !reflect.PointerTo(valueType).Implements(reflect.TypeFor[SelfValidator]()) {
			return nil
		}

		ptr := reflect.New(valueType)
		ptr.Elem().Set(reflectedValue)

		if v, ok := reflect.TypeAssert[SelfValidator](ptr); ok {
			return v.Validate(ctx)
		}
	}

	return nil
}

// validateCustomValues runs SelfValidator domain-type validation for every
// resolved flag and positional argument. It executes after the Session is
// constructed so validation observes the final value of the configuration
// cascade, with the context of the active invocation.
func validateCustomValues(ctx context.Context, cmdChain []*spec.Command) error {
	for _, cmd := range cmdChain {
		for _, flagSpec := range cmd.Flags {
			if !flagStateOf(flagSpec.Handle).IsSet() {
				continue
			}

			if msg, cause := validateCustomValueTree(ctx, flagStateOf(flagSpec.Handle).getValue(), "flag --"+flagSpec.Name); cause != nil {
				return &validationError{msg: msg, cmdChain: cmdChain, cause: cause, valueShaped: true}
			}
		}

		for _, argSpec := range cmd.Args {
			if !argStateOf(argSpec.Handle).IsSet() {
				continue
			}

			if msg, cause := validateCustomValueTree(ctx, argStateOf(argSpec.Handle).getValue(), "argument "+argSpec.Name); cause != nil {
				return &validationError{msg: msg, cmdChain: cmdChain, cause: cause, valueShaped: true}
			}
		}
	}

	return nil
}

// validateCustomValueTree walks a resolved value like the constraint
// validators: maps validate as a whole and per entry in deterministic sorted
// key order, slices validate per element, and scalars validate directly. It
// returns the composed developer-facing message and the validation cause.
func validateCustomValueTree(ctx context.Context, value any, descriptor string) (string, error) {
	reflectedValue := reflect.ValueOf(value)
	if !reflectedValue.IsValid() {
		return "", nil
	}

	switch reflectedValue.Kind() {
	case reflect.Map:
		if err := callSelfValidator(ctx, value); err != nil {
			return fmt.Sprintf("%v for %s", err, descriptor), err
		}

		if reflectedValue.Len() == 0 {
			return "", nil
		}

		strKeys := make([]string, 0, reflectedValue.Len())
		keyMap := make(map[string]reflect.Value, reflectedValue.Len())

		iter := reflectedValue.MapRange()
		for iter.Next() {
			key := iter.Key()
			keyStr := key.String()
			strKeys = append(strKeys, keyStr)
			keyMap[keyStr] = key
		}

		slices.Sort(strKeys)

		for _, keyStr := range strKeys {
			entry := reflectedValue.MapIndex(keyMap[keyStr]).Interface()
			if err := callSelfValidator(ctx, entry); err != nil {
				return fmt.Sprintf("%v for %s entry %q", err, descriptor, keyStr), err
			}
		}

	case reflect.Slice:
		for i := range reflectedValue.Len() {
			if err := callSelfValidator(ctx, reflectedValue.Index(i).Interface()); err != nil {
				return fmt.Sprintf("%v for %s", err, descriptor), err
			}
		}

	default:
		if err := callSelfValidator(ctx, value); err != nil {
			return fmt.Sprintf("%v for %s", err, descriptor), err
		}
	}

	return "", nil
}

func executeLifecycle(ctx context.Context, session *Session) error {
	cmdChain := session.cmdChain
	leaf := cmdChain[len(cmdChain)-1]
	leafTarget := getCmdTarget(leaf)

	runner, ok := leafTarget.(Runner)
	if !ok {
		return newNoRunnerError(leaf)
	}

	if isMetaCommand(leaf) {
		return runner.Run(ctx, session)
	}

	var initialized []*spec.Command

	runFinally := func(causeErr error) error {
		var finErrs error

		for _, cmd := range slices.Backward(initialized) {
			if fin, ok := getCmdTarget(cmd).(Finalizer); ok {
				if err := fin.Finally(ctx, session, causeErr); err != nil {
					finErrs = errors.Join(finErrs, err)
				}
			}
		}

		return errors.Join(causeErr, finErrs)
	}

	// 1. Top-down PreRun execution
	for _, cmd := range cmdChain {
		if pre, ok := getCmdTarget(cmd).(PreRunner); ok {
			if err := pre.PreRun(ctx, session); err != nil {
				return runFinally(err)
			}
		}

		initialized = append(initialized, cmd)
	}

	// 2. Execute Leaf Command Run
	if err := runner.Run(ctx, session); err != nil {
		return runFinally(err)
	}

	// 3. Bottom-up PostRun execution (Only executed on success)
	for _, cmd := range slices.Backward(initialized) {
		if post, ok := getCmdTarget(cmd).(PostRunner); ok {
			if err := post.PostRun(ctx, session); err != nil {
				return runFinally(err)
			}
		}
	}

	// 4. Guaranteed Teardown
	return runFinally(nil)
}

func getCmdTarget(cmd *spec.Command) any {
	if cmd.Handle != nil {
		return commandStateOf(cmd.Handle).innerValue()
	}

	return cmd.Target
}

func isMetaCommand(cmd *spec.Command) bool {
	return cmd != nil && (cmd.IsCompletionCmd || cmd.IsHelpCmd || cmd.IsVersionCmd)
}

func newFlagCoercionError(cmdChain []*spec.Command, flagSpec *spec.Flag, source ValueSource, sourceName, rawValue string, cause error) *coercionError {
	return &coercionError{
		targetName: flagSpec.Name,
		isFlag:     true,
		source:     source,
		sourceName: sourceName,
		targetType: flagSpec.TargetType,
		rawValue:   rawValue,
		cmdChain:   cmdChain,
		cause:      cause,
	}
}

func newArgCoercionError(cmdChain []*spec.Command, arg *spec.Arg, source ValueSource, sourceName, rawValue string, cause error) *coercionError {
	return &coercionError{
		targetName: arg.Name,
		isFlag:     false,
		source:     source,
		sourceName: sourceName,
		targetType: arg.TargetType,
		rawValue:   rawValue,
		cmdChain:   cmdChain,
		cause:      cause,
	}
}

func foldMapTier(cmdChain []*spec.Command, flagSpec *spec.Flag, rawValues []string, source ValueSource, name string, merged map[string]string, provenance map[string]ValueSource, result *mapMergeResult) error {
	if len(rawValues) == 0 {
		return nil
	}

	entries, err := coerce.MapEntries(rawValues, flagSpec.Sep)
	if err != nil {
		result.hasValues = true
		result.failedTier = source
		result.failedRaw = strings.Join(rawValues, whyJoinSep(flagSpec))

		return newFlagCoercionError(cmdChain, flagSpec, source, name, strings.Join(rawValues, flagSpec.Sep), err)
	}

	for _, entry := range entries {
		key := entry.Key
		if flagSpec.KeyCanonicalizer != nil {
			key = flagSpec.KeyCanonicalizer(key)
		}

		merged[key] = entry.Value

		if provenance != nil {
			provenance[key] = source
		}
	}

	result.hasValues = true
	if len(entries) > 0 {
		result.highestTier = source
		result.highestName = name
	}

	return nil
}

func hasCustomMapParser(state *bindingState, targetType reflect.Type) bool {
	if state == nil || state.CustomMapMerger == nil || len(state.TypeParsers) == 0 {
		return false
	}

	_, ok := state.TypeParsers[targetType]

	return ok
}

func isStringStringMap(targetType reflect.Type) bool {
	return targetType.Key().Kind() == reflect.String && targetType.Elem().Kind() == reflect.String
}

// foldMapTiers resolves a map flag's contributing tiers (default, config, env,
// CLI) and merges their contributions key-by-key in ascending precedence.
// For feature-map flags, keys are canonicalized during the merge so spelling
// variants override across tiers and later tokens in the same tier win.
func foldMapTiers(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState, wantProvenance bool, result *mapMergeResult) error {
	if flagSpec == nil {
		return nil
	}

	defaultValue := flagSpec.Default
	configValue, configKey, hasConfig := lookupConfig(flagSpec, cmd, cmdChain, state)
	envValue, envName, hasEnv := lookupEnv(flagSpec, cmd, state)
	cliValuesForFlag := cliValues[flagSpec]

	if defaultValue == "" && !hasConfig && !hasEnv && len(cliValuesForFlag) == 0 {
		return nil
	}

	if hasCustomMapParser(state, flagSpec.TargetType) {
		return state.CustomMapMerger(cmd, cmdChain, flagSpec, cliValues, state, wantProvenance, result)
	}

	if !isStringStringMap(flagSpec.TargetType) {
		result.hasValues = true

		return &mapRegistrationError{targetType: flagSpec.TargetType}
	}

	merged := make(map[string]string)

	var provenance map[string]ValueSource
	if wantProvenance {
		provenance = make(map[string]ValueSource)
	}

	tiers := [...]mapTierInput{
		{has: defaultValue != "", values: []string{defaultValue}, source: SourceDefault, name: defaultValue},
		{has: hasConfig, values: []string{configValue}, source: SourceConfig, name: configKey},
		{has: hasEnv, values: []string{envValue}, source: SourceEnv, name: envName},
		{has: len(cliValuesForFlag) > 0, values: cliValuesForFlag, source: SourceCLI, name: "--" + flagSpec.Name},
	}

	var lastSource ValueSource

	var lastName string

	for _, tier := range tiers {
		if tier.has {
			lastSource = tier.source
			lastName = tier.name

			if err := foldMapTier(cmdChain, flagSpec, tier.values, tier.source, tier.name, merged, provenance, result); err != nil {
				return err
			}
		}
	}

	if !result.hasValues {
		return nil
	}

	if result.highestTier == SourceNone {
		result.highestTier = lastSource
		result.highestName = lastName
	}

	var finalMerged any = merged
	if flagSpec.TargetType != reflect.TypeFor[map[string]string]() {
		finalMerged = reflect.ValueOf(merged).Convert(flagSpec.TargetType).Interface()
	}

	result.merged = finalMerged
	result.provenance = provenance

	return nil
}

func foldRegisteredMapTier(rawValues []string, source ValueSource, name string, flagSpec *spec.Flag, cmdChain []*spec.Command, state *bindingState, merged reflect.Value, provenance map[string]ValueSource, result *mapMergeResult) error {
	if len(rawValues) == 0 {
		return nil
	}

	parsed, err := parseMapTokens(rawValues, flagSpec.TargetType, flagSpec.Sep, state.TypeParsers)
	if err != nil {
		raw := strings.Join(rawValues, whyJoinSep(flagSpec))
		result.hasValues = true
		result.failedTier = source
		result.failedRaw = raw

		return newFlagCoercionError(cmdChain, flagSpec, source, name, strings.Join(rawValues, flagSpec.Sep), err)
	}

	result.hasValues = true

	parsedValue := reflect.ValueOf(parsed)
	if parsedValue.Len() > 0 {
		result.highestTier = source
		result.highestName = name
	}

	iter := parsedValue.MapRange()
	for iter.Next() {
		merged.SetMapIndex(iter.Key(), iter.Value())

		if provenance != nil {
			provenance[stringx.Stringify(iter.Key().Interface())] = source
		}
	}

	return nil
}

func mergeRegisteredMapTiers(cmd *spec.Command, cmdChain []*spec.Command, flagSpec *spec.Flag, cliValues map[*spec.Flag][]string, state *bindingState, wantProvenance bool, result *mapMergeResult) error {
	merged := reflect.MakeMap(flagSpec.TargetType)

	var provenance map[string]ValueSource
	if wantProvenance {
		provenance = make(map[string]ValueSource)
	}

	var lastSource ValueSource

	var lastName string

	foldTier := func(rawValues []string, source ValueSource, name string) error {
		lastSource = source
		lastName = name

		return foldRegisteredMapTier(rawValues, source, name, flagSpec, cmdChain, state, merged, provenance, result)
	}

	if flagSpec.Default != "" {
		if err := foldTier([]string{flagSpec.Default}, SourceDefault, flagSpec.Default); err != nil {
			return err
		}
	}

	if configValue, configKey, ok := lookupConfig(flagSpec, cmd, cmdChain, state); ok {
		if err := foldTier([]string{configValue}, SourceConfig, configKey); err != nil {
			return err
		}
	}

	if envValue, envName, ok := lookupEnv(flagSpec, cmd, state); ok {
		if err := foldTier([]string{envValue}, SourceEnv, envName); err != nil {
			return err
		}
	}

	if values, ok := cliValues[flagSpec]; ok && len(values) > 0 {
		if err := foldTier(values, SourceCLI, "--"+flagSpec.Name); err != nil {
			return err
		}
	}

	if !result.hasValues {
		return nil
	}

	if result.highestTier == SourceNone {
		result.highestTier = lastSource
		result.highestName = lastName
	}

	result.merged = merged.Interface()
	result.provenance = provenance

	return nil
}

func parseMapTokens(rawValues []string, targetType reflect.Type, sep string, parsers coerce.Registry) (any, error) {
	if parser, ok := parsers[targetType]; ok {
		return parseRegisteredMap(rawValues, targetType, sep, parser)
	}

	if targetType.Key().Kind() != reflect.String || targetType.Elem().Kind() != reflect.String {
		return nil, &mapRegistrationError{targetType: targetType}
	}

	parsedMap, err := coerce.Map(rawValues, sep)
	if err != nil {
		return nil, err
	}

	if reflect.TypeOf(parsedMap) == targetType {
		return parsedMap, nil
	}

	return reflect.ValueOf(parsedMap).Convert(targetType).Interface(), nil
}

func parseRegisteredMap(rawValues []string, targetType reflect.Type, sep string, parser coerce.TypeParser) (any, error) {
	merged := reflect.MakeMap(targetType)

	for _, item := range coerce.SplitMapItems(rawValues, sep) {
		parsed, err := parser(item)
		if err != nil {
			return nil, err
		}

		parsedValue := reflect.ValueOf(parsed)
		if parsedValue.Kind() != reflect.Map {
			return nil, &mapParserError{targetType: targetType, returnedType: reflect.TypeOf(parsed), kind: mapParserErrorNotMap}
		}

		if parsedValue.Type() != targetType {
			if parsedValue.Type().ConvertibleTo(targetType) {
				parsedValue = parsedValue.Convert(targetType)
			} else {
				return nil, &mapParserError{targetType: targetType, returnedType: parsedValue.Type(), kind: mapParserErrorNotConvertible}
			}
		}

		iter := parsedValue.MapRange()
		for iter.Next() {
			merged.SetMapIndex(iter.Key(), iter.Value())
		}
	}

	return merged.Interface(), nil
}
