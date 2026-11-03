package spec

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/conga-sh/conga/internal/completion/wire"
	"github.com/conga-sh/conga/internal/style"
	"github.com/conga-sh/conga/internal/tokenizer"
	"github.com/conga-sh/conga/predict"
)

// CompleteOptions configures the execution of the completion engine.
//
// Env and EnvLookup scope the environment view handed to predictors through
// predict.Context; EnvLookup is consulted first, then Env. They do not override
// the application-prefixed sideband toggles (aliases, descriptions, bare-space
// flags), which the framing layer reads from the process environment because
// they describe the invoking shell session.
type CompleteOptions struct {
	RegisteredPredictors map[string]predict.Predictor
	Env                  map[string]string
	EnvLookup            func(string) (string, bool)
	Dir                  string
}

// CompletionAnalysis carries the invocation facts parsed from the typed
// portion of a completion command line. It is produced before feature pruning
// so the engine can resolve configuration and bind bootstrap fields from the
// same tokens the candidate pass will later see.
type CompletionAnalysis struct {
	CommandChain   []*Command
	FlagValues     map[*Flag][]string
	PositionalArgs []string
	DashArgs       []string
	RawArgs        []string
}

// AnalyzeCompletion parses every complete word in args (all words except the
// fragment being completed) with the same tolerant walker used for candidate
// generation. It performs no candidate resolution and does not mutate the
// command tree. Command routing runs first so raw fallback is evaluated
// against the routed leaf, matching the runtime parser.
func AnalyzeCompletion(rootCmd *Command, args []string) CompletionAnalysis {
	if rootCmd == nil {
		return CompletionAnalysis{}
	}

	normArgs := args
	if len(normArgs) == 0 {
		normArgs = []string{""}
	}

	typedTokens := normArgs[:len(normArgs)-1]
	toks := tokenizer.Tokenize(typedTokens)

	routingWalker := newASTCompletionWalker(rootCmd, CompleteOptions{})
	routingWalker.walkTokens(toks)

	walker := newASTCompletionWalker(rootCmd, CompleteOptions{})
	walker.analysisLeaf = routingWalker.currentCmd
	walker.walkTokens(toks)

	return walker.analysis()
}

// Complete evaluates command-line arguments up to the cursor against the AST Command tree
// and returns the resolved Result containing completion items and shell directives.
func Complete(rootCmd *Command, args []string, opts CompleteOptions) predict.Result {
	if rootCmd == nil {
		return predict.None().Invoke(predict.Context{})
	}

	normArgs := args
	if len(normArgs) == 0 {
		normArgs = []string{""}
	}

	typedTokens, toComplete := normArgs[:len(normArgs)-1], normArgs[len(normArgs)-1]
	toks := tokenizer.Tokenize(typedTokens)

	walker := newASTCompletionWalker(rootCmd, opts)
	walker.walkTokens(toks)

	return walker.resolveTarget(toComplete)
}

// completionGroupKey identifies a mutually-exclusive group on the command that
// declares its members. Runtime validation scopes xor/and/oneof groups per
// command, so a same-named group on an ancestor must not suppress a
// descendant's candidates.
type completionGroupKey struct {
	owner *Command
	name  string
}

type astCompletionWalker struct {
	rootCmd          *Command
	opts             CompleteOptions
	currentCmd       *Command
	cmdChain         []*Command
	flagsSet         map[*Flag][]string
	xorGroupsSet     map[completionGroupKey]*Flag
	oneOfGroupsSet   map[completionGroupKey]*Flag
	nonRepeatableSet map[*Flag]bool
	posArgs          []string
	inTerminator     bool
	rawActive        bool
	pendingFlagValue *Flag

	analysisLeaf         *Command
	analysisPositionals  []string
	analysisDashArgs     []string
	analysisRawArgs      []string
	analysisInTerminator bool
	analysisRawFallback  bool
}

func newASTCompletionWalker(rootCmd *Command, opts CompleteOptions) *astCompletionWalker {
	return &astCompletionWalker{
		rootCmd:          rootCmd,
		opts:             opts,
		currentCmd:       rootCmd,
		cmdChain:         []*Command{rootCmd},
		flagsSet:         make(map[*Flag][]string),
		xorGroupsSet:     make(map[completionGroupKey]*Flag),
		oneOfGroupsSet:   make(map[completionGroupKey]*Flag),
		nonRepeatableSet: make(map[*Flag]bool),
		posArgs:          make([]string, 0),
	}
}

func (w *astCompletionWalker) walkTokens(toks []tokenizer.Token) {
	for i := range toks {
		w.consumeToken(toks[i])
	}
}

// canFallbackToRaw reports whether the runtime parser would route the next
// unbound token into RawArgs: the routed command declares RawArgs and every
// declared positional argument is already filled.
func (w *astCompletionWalker) canFallbackToRaw() bool {
	return w.currentCmd.RawArgs != nil && len(w.posArgs) >= len(w.currentCmd.Args)
}

func (w *astCompletionWalker) consumeToken(tok tokenizer.Token) {
	if w.pendingFlagValue != nil {
		if tok.CanBindAsFlagValue() {
			w.consumePendingFlagValue(tok.Raw)

			return
		}

		// binder.go consumeNextValue refuses this token as a flag value and
		// reports "flag requires an argument"; the walker stays tolerant: the
		// flag is left unfilled and the token is processed on its own shape.
		// The built-in --completion flag takes an empty value instead
		// (handleCompletionFlag) and keeps parsing.
		if w.pendingFlagValue == BuiltinCompletionFlag {
			w.consumePendingFlagValue("")
		} else {
			w.pendingFlagValue = nil
		}
	}

	matchedSubcommand := w.matchSubcommand(tok)

	w.trackAnalysisToken(tok, matchedSubcommand)

	if w.rawActive {
		return
	}

	if w.inTerminator {
		w.consumeTerminatorToken(tok)

		return
	}

	switch tok.Type {
	case tokenizer.TokenTypeTerminator:
		w.inTerminator = true
	case tokenizer.TokenTypeLongFlag:
		w.consumeLongFlag(tok)
	case tokenizer.TokenTypeShortFlag:
		w.consumeShortFlag(tok)
	case tokenizer.TokenTypePositional:
		w.consumePositional(tok, matchedSubcommand)
	case tokenizer.TokenTypeUnknown:
		// No-op for unclassified token
	}
}

// consumeTerminatorToken attributes a token after the -- terminator. Words
// captured by DashArgs are opaque and never fill declared positionals; without
// DashArgs they bind positionals until the runtime parser overflows into
// RawArgs.
func (w *astCompletionWalker) consumeTerminatorToken(tok tokenizer.Token) {
	if w.currentCmd.DashArgs != nil {
		return
	}

	if w.canFallbackToRaw() {
		w.rawActive = true

		return
	}

	w.posArgs = append(w.posArgs, tok.Raw)
}

// matchSubcommand resolves a positional token against the current command's
// subcommands. Tokens after the -- terminator never route to a subcommand.
func (w *astCompletionWalker) matchSubcommand(tok tokenizer.Token) *Command {
	if tok.Type != tokenizer.TokenTypePositional || w.inTerminator {
		return nil
	}

	return w.currentCmd.FindSubcommand(tok.Raw)
}

// trackAnalysisToken mirrors the runtime token parser's positional, dash, and
// raw attribution for the bootstrap document's completion-time view. The candidate walker
// stays tolerant of tokens the runtime parser rejects, so analysis state is
// tracked alongside it rather than derived from posArgs. Raw fallback uses the
// routed leaf because routing removes subcommand tokens before parsing.
func (w *astCompletionWalker) trackAnalysisToken(tok tokenizer.Token, matchedSubcommand *Command) {
	if matchedSubcommand != nil {
		return
	}

	if w.analysisRawFallback {
		w.analysisRawArgs = append(w.analysisRawArgs, tok.Raw)

		return
	}

	if w.analysisInTerminator {
		w.trackAnalysisTerminatorToken(tok)

		return
	}

	switch tok.Type {
	case tokenizer.TokenTypeTerminator:
		w.analysisInTerminator = true

		if w.currentCmd.DashArgs != nil {
			w.analysisDashArgs = make([]string, 0)
		}
	case tokenizer.TokenTypeLongFlag:
		w.trackAnalysisLongFlag(tok)
	case tokenizer.TokenTypeShortFlag:
		w.trackAnalysisShortFlag(tok)
	case tokenizer.TokenTypePositional:
		w.trackAnalysisPositional(tok.Raw)
	case tokenizer.TokenTypeUnknown:
		// No-op for unclassified token
	}
}

// trackAnalysisTerminatorToken attributes a token after the -- terminator:
// DashArgs captures everything, while RawArgs captures only the overflow.
func (w *astCompletionWalker) trackAnalysisTerminatorToken(tok tokenizer.Token) {
	if w.currentCmd.DashArgs != nil {
		w.analysisDashArgs = append(w.analysisDashArgs, tok.Raw)

		return
	}

	w.trackAnalysisPositional(tok.Raw)
}

// trackAnalysisPositional binds a positional token until the declared
// arguments are full, then falls back to RawArgs like the runtime parser.
func (w *astCompletionWalker) trackAnalysisPositional(raw string) {
	if w.canAnalysisRawFallback() {
		w.startAnalysisRawFallback(raw)

		return
	}

	w.analysisPositionals = append(w.analysisPositionals, raw)
}

// trackAnalysisLongFlag starts the raw fallback for an unknown long flag once
// the declared positional arguments are full. Known flags bind normally.
func (w *astCompletionWalker) trackAnalysisLongFlag(tok tokenizer.Token) {
	if FindFlagInChain(w.cmdChain, tok.Name) != nil || FindNegatableFlagInChain(w.cmdChain, tok.Name) != nil {
		return
	}

	if w.canAnalysisRawFallback() {
		w.startAnalysisRawFallback(tok.Raw)
	}
}

// trackAnalysisShortFlag mirrors the runtime short-flag rules: a negative
// number binds as a positional value until the declared arguments are full,
// and an unknown shorthand starts the raw fallback at the same point.
func (w *astCompletionWalker) trackAnalysisShortFlag(tok tokenizer.Token) {
	runes := tok.ShortRunes

	if tok.IsNegativeNumber() && (len(runes) == 0 || FindShortFlagInChain(w.cmdChain, runes[0]) == nil) {
		if w.canAnalysisRawFallback() {
			w.startAnalysisRawFallback(tok.Raw)

			return
		}

		w.analysisPositionals = append(w.analysisPositionals, tok.Raw)

		return
	}

	if len(runes) > 0 && FindShortFlagInChain(w.cmdChain, runes[0]) == nil && w.canAnalysisRawFallback() {
		w.startAnalysisRawFallback(tok.Raw)
	}
}

// canAnalysisRawFallback reports whether the runtime parser would route the
// next unbound token into RawArgs.
func (w *astCompletionWalker) canAnalysisRawFallback() bool {
	leaf := w.currentCmd
	if w.analysisLeaf != nil {
		leaf = w.analysisLeaf
	}

	return leaf.RawArgs != nil && len(w.analysisPositionals) >= len(leaf.Args)
}

// startAnalysisRawFallback enters raw mode, capturing this token and every
// following token into RawArgs.
func (w *astCompletionWalker) startAnalysisRawFallback(raw string) {
	w.analysisRawFallback = true
	w.analysisRawArgs = append(w.analysisRawArgs, raw)
}

// registerGroups records the flag's xor and oneof groups as used, scoped to the
// command that declares the flag.
func (w *astCompletionWalker) registerGroups(flag *Flag) {
	owner := w.flagOwner(flag)

	if flag.XorGroup != "" {
		w.xorGroupsSet[completionGroupKey{owner: owner, name: flag.XorGroup}] = flag
	}

	if flag.OneOfGroup != "" {
		w.oneOfGroupsSet[completionGroupKey{owner: owner, name: flag.OneOfGroup}] = flag
	}
}

// markNonRepeatable flags the flag as already consumed for repeat-suppression
// unless its kind legitimately repeats (slices, counters, maps).
func (w *astCompletionWalker) markNonRepeatable(flag *Flag) {
	if !flag.IsSlice && !flag.IsCounter && !flag.IsMap {
		w.nonRepeatableSet[flag] = true
	}
}

func (w *astCompletionWalker) consumePendingFlagValue(value string) {
	flag := w.pendingFlagValue
	w.flagsSet[flag] = append(w.flagsSet[flag], value)
	w.registerGroups(flag)
	w.markNonRepeatable(flag)

	w.pendingFlagValue = nil
}

func (w *astCompletionWalker) markFlagConsumed(flag *Flag, hasAttached bool, attachedValue string, negated bool) {
	w.registerGroups(flag)

	if flag.IsBool {
		value := "true"

		switch {
		case negated:
			value = "false"
		case hasAttached:
			value = attachedValue
		}

		w.flagsSet[flag] = append(w.flagsSet[flag], value)
		w.markNonRepeatable(flag)

		return
	}

	if flag.IsCounter {
		value := "1"
		if hasAttached {
			value = attachedValue
		}

		w.flagsSet[flag] = append(w.flagsSet[flag], value)

		return
	}

	if hasAttached {
		w.flagsSet[flag] = append(w.flagsSet[flag], attachedValue)
		w.markNonRepeatable(flag)
	} else {
		w.pendingFlagValue = flag
	}
}

// consumeUnregisteredShorthand handles a short-flag token before cluster
// parsing when it is an unregistered shorthand — including a negative number
// that is not a registered shorthand — and reports whether the token was
// consumed. The runtime parser binds an unregistered shorthand to RawArgs when
// the declared positionals are full and reports an unknown flag otherwise; a
// negative number binds as a positional when raw fallback is not available.
func (w *astCompletionWalker) consumeUnregisteredShorthand(tok tokenizer.Token) bool {
	runes := tok.ShortRunes

	if tok.IsNegativeNumber() && (len(runes) == 0 || FindShortFlagInChain(w.cmdChain, runes[0]) == nil) {
		if w.canFallbackToRaw() {
			w.rawActive = true
		} else {
			w.posArgs = append(w.posArgs, tok.Raw)
		}

		return true
	}

	if len(runes) > 0 && FindShortFlagInChain(w.cmdChain, runes[0]) == nil {
		if w.canFallbackToRaw() {
			w.rawActive = true
		}

		return true
	}

	return false
}

func (w *astCompletionWalker) consumeLongFlag(tok tokenizer.Token) {
	flag := FindFlagInChain(w.cmdChain, tok.Name)

	negated := false

	if flag == nil {
		flag = FindNegatableFlagInChain(w.cmdChain, tok.Name)
		negated = flag != nil
	}

	if flag == nil {
		if w.canFallbackToRaw() {
			w.rawActive = true
		}

		return
	}

	w.markFlagConsumed(flag, tok.HasAttachedValue, tok.Value, negated)
}

func (w *astCompletionWalker) consumeShortFlag(tok tokenizer.Token) {
	runes := tok.ShortRunes

	if w.consumeUnregisteredShorthand(tok) {
		return
	}

	for index := range runes {
		flag := FindShortFlagInChain(w.cmdChain, runes[index])
		if flag == nil {
			return
		}

		w.registerGroups(flag)

		if flag.IsCounter {
			value := "1"
			if tok.HasAttachedValue && index == len(runes)-1 {
				value = tok.Value
			}

			w.flagsSet[flag] = append(w.flagsSet[flag], value)

			continue
		}

		if flag.IsBool {
			value := "true"
			if tok.HasAttachedValue && index == len(runes)-1 {
				value = tok.Value
			}

			w.flagsSet[flag] = append(w.flagsSet[flag], value)
			w.markNonRepeatable(flag)

			continue
		}

		if index < len(runes)-1 {
			value := string(runes[index+1:])
			if tok.HasAttachedValue {
				value += "=" + tok.Value
			}

			w.flagsSet[flag] = append(w.flagsSet[flag], value)
			w.markNonRepeatable(flag)

			return
		}

		if tok.HasAttachedValue {
			w.flagsSet[flag] = append(w.flagsSet[flag], tok.Value)
			w.markNonRepeatable(flag)

			return
		}

		w.pendingFlagValue = flag

		return
	}
}

func (w *astCompletionWalker) consumePositional(tok tokenizer.Token, matchedSubcommand *Command) {
	if matchedSubcommand != nil {
		w.currentCmd = matchedSubcommand
		w.cmdChain = append(w.cmdChain, w.currentCmd)
		w.posArgs = make([]string, 0)

		return
	}

	if w.canFallbackToRaw() {
		w.rawActive = true

		return
	}

	w.posArgs = append(w.posArgs, tok.Raw)
}

func (w *astCompletionWalker) buildCompletionContext(toComplete string) predict.Context {
	flagValueMap := make(map[string]string)
	flagMultiValuesMap := make(map[string][]string)

	for flag, values := range w.flagsSet {
		if len(values) == 0 {
			continue
		}

		lastValue := values[len(values)-1]
		clonedValues := slices.Clone(values)

		flagValueMap[flag.Name] = lastValue
		flagMultiValuesMap[flag.Name] = clonedValues

		for _, alias := range flag.Aliases {
			flagValueMap[alias] = lastValue
			flagMultiValuesMap[alias] = clonedValues
		}

		if flag.Short != 0 {
			shortStr := string(flag.Short)
			flagValueMap[shortStr] = lastValue
			flagMultiValuesMap[shortStr] = clonedValues
		}
	}

	return predict.Context{
		ToComplete:      toComplete,
		Args:            slices.Clone(w.posArgs),
		Flags:           flagValueMap,
		FlagMultiValues: flagMultiValuesMap,
		Dir:             w.opts.Dir,
		Env:             w.opts.Env,
		EnvLookup:       w.opts.EnvLookup,
	}
}

// analysis returns the parsed invocation facts for the engine's early bootstrap
// pass. Positional, dash, and raw attribution mirrors the runtime token parser:
// declared arguments consume positionals first, and RawArgs captures the
// overflow. NoCLI engine flags are excluded because they can never be supplied
// on the command line.
func (w *astCompletionWalker) analysis() CompletionAnalysis {
	values := make(map[*Flag][]string, len(w.flagsSet))
	for flag, flagValues := range w.flagsSet {
		if flag.NoCLI || len(flagValues) == 0 {
			continue
		}

		values[flag] = slices.Clone(flagValues)
	}

	return CompletionAnalysis{
		CommandChain:   slices.Clone(w.cmdChain),
		FlagValues:     values,
		PositionalArgs: slices.Clone(w.analysisPositionals),
		DashArgs:       slices.Clone(w.analysisDashArgs),
		RawArgs:        slices.Clone(w.analysisRawArgs),
	}
}

func (w *astCompletionWalker) resolveTarget(toComplete string) predict.Result {
	compCtx := w.buildCompletionContext(toComplete)

	if w.pendingFlagValue != nil {
		return w.completePendingFlag(toComplete, compCtx)
	}

	if w.rawActive {
		return w.trailingArgsAction(w.currentCmd.RawArgs.Predictor).Invoke(compCtx)
	}

	if w.inTerminator {
		return w.completeTerminatorPositional(compCtx)
	}

	if strings.Contains(toComplete, "=") {
		if invoked, handled := w.completeAttachedFlag(toComplete, compCtx); handled {
			return invoked
		}
	}

	if strings.HasPrefix(toComplete, "-") {
		flagAction := w.completeFlagNamesAction(compCtx.ToComplete)
		if w.canFallbackToRaw() {
			return predict.Batch(flagAction, w.trailingArgsAction(w.currentCmd.RawArgs.Predictor)).Invoke(compCtx)
		}

		return flagAction.Invoke(compCtx)
	}

	cmdOrArgAction := w.completeSubcommandsAndArgsAction(toComplete)
	if w.suppressBareSpaceFlags() {
		return cmdOrArgAction.Invoke(compCtx)
	}

	return predict.Batch(cmdOrArgAction, w.completeFlagNamesAction(toComplete)).Invoke(compCtx)
}

// suppressBareSpaceFlags reports whether flag candidates are withheld at a bare
// space. The engine offers subcommands and positional arguments there and
// reserves flags for the "-" prefix, uniformly across every shell; the
// application-prefixed <APP>_COMPLETION_BARE_SPACE_FLAGS sideband restores the
// flags when a user opts in. A command with no subcommands, positional
// arguments, or raw args has nothing else to offer, so its flags stay visible.
func (w *astCompletionWalker) suppressBareSpaceFlags() bool {
	if wire.BareSpaceFlagsEnabled(w.rootCmd.CompletionEnvPrefix) {
		return false
	}

	return w.currentCmd.HasUserSubcommands() || len(w.currentCmd.Args) > 0 || w.currentCmd.RawArgs != nil
}

func (w *astCompletionWalker) completePendingFlag(toComplete string, compCtx predict.Context) predict.Result {
	baseAction := resolveFlagAction(w.pendingFlagValue, w.opts.RegisteredPredictors, w.rootCmd.WhyFormats())
	previousValues := compCtx.FlagValues(w.pendingFlagValue.Name)
	action := wrapSliceAction(baseAction, w.pendingFlagValue.IsSlice || w.pendingFlagValue.IsMap, w.pendingFlagValue.Sep, previousValues).Tag("values")
	compCtx.ToComplete = toComplete

	return action.Invoke(compCtx)
}

func (w *astCompletionWalker) completeAttachedFlag(toComplete string, compCtx predict.Context) (predict.Result, bool) {
	flagPrefix, valuePart, _ := strings.Cut(toComplete, "=")

	var flag *Flag
	if flagName, cut := strings.CutPrefix(flagPrefix, "--"); cut {
		flag = FindFlagInChain(w.cmdChain, flagName)
		if flag == nil {
			flag = FindNegatableFlagInChain(w.cmdChain, flagName)
		}
	} else if shortName, shortCut := strings.CutPrefix(flagPrefix, "-"); shortCut && len(shortName) == 1 {
		flag = FindShortFlagInChain(w.cmdChain, rune(shortName[0]))
	}

	if flag == nil {
		return predict.Result{}, false
	}

	baseAction := resolveFlagAction(flag, w.opts.RegisteredPredictors, w.rootCmd.WhyFormats())
	previousValues := compCtx.FlagValues(flag.Name)
	action := wrapSliceAction(baseAction, flag.IsSlice || flag.IsMap, flag.Sep, previousValues).Tag("values")

	subCtx := compCtx
	subCtx.ToComplete = valuePart
	invoked := action.Invoke(subCtx)

	for i := range invoked.Items {
		invoked.Items[i].Display = cmp.Or(invoked.Items[i].Display, invoked.Items[i].Value)
		invoked.Items[i].Value = flagPrefix + "=" + invoked.Items[i].Value
	}

	return invoked, true
}

// trailingArgsAction resolves the completion strategy for a trailing
// collector. An untagged collector yields no candidates: the words are opaque,
// so the engine does not guess a domain. Unlike a slice positional, repeated
// words are legitimate forwarded arguments, so no value exclusion applies.
func (w *astCompletionWalker) trailingArgsAction(predictorSpec *PredictorSpec) predict.Predictor {
	if predictorSpec == nil {
		return predict.None().Tag("arguments")
	}

	action, ok := resolvePredictorSpec(predictorSpec, w.opts.RegisteredPredictors)
	if !ok {
		return predict.None().Tag("arguments")
	}

	return action.Tag("arguments")
}

func (w *astCompletionWalker) completeTerminatorPositional(compCtx predict.Context) predict.Result {
	if w.currentCmd.DashArgs != nil {
		return w.trailingArgsAction(w.currentCmd.DashArgs.Predictor).Invoke(compCtx)
	}

	if w.canFallbackToRaw() {
		return w.trailingArgsAction(w.currentCmd.RawArgs.Predictor).Invoke(compCtx)
	}

	posIndex := len(w.posArgs)

	arg := getArgSpec(w.currentCmd, posIndex)
	if arg == nil {
		return predict.None().Invoke(compCtx)
	}

	baseAction := resolveArgAction(arg, w.opts.RegisteredPredictors)
	action := wrapSliceAction(baseAction, arg.IsSlice, arg.Sep, compCtx.Args).Tag("arguments")

	return action.Invoke(compCtx)
}

func (w *astCompletionWalker) completeFlagNamesAction(toComplete string) predict.Predictor {
	var (
		longItems  []predict.Item
		shortItems []predict.Item
	)

	seenLong := make(map[string]bool)
	seenShort := make(map[string]bool)

	for _, cmd := range slices.Backward(w.cmdChain) {
		if w.currentCmd.IsCompletionCmd && cmd != w.currentCmd {
			continue
		}

		for _, flagSpec := range cmd.Flags {
			if !w.isFlagEligible(flagSpec) {
				continue
			}

			w.collectFlagCandidates(&longItems, &shortItems, seenLong, seenShort, flagSpec, toComplete)
		}
	}

	w.appendBuiltinFlags(&longItems, &shortItems, seenLong, seenShort, toComplete)

	var actions []predict.Predictor

	if len(longItems) > 0 {
		actions = append(actions, predict.Items(longItems...).Tag("longhand flags"))
	}

	if len(shortItems) > 0 {
		actions = append(actions, predict.Items(shortItems...).Tag("shorthand flags"))
	}

	if len(actions) == 0 {
		return predict.None()
	}

	return predict.Batch(actions...)
}

func (w *astCompletionWalker) isFlagEligible(flagSpec *Flag) bool {
	if flagSpec.Hidden {
		return false
	}

	if flagSpec.Deprecated != "" {
		return false
	}

	owner := w.flagOwner(flagSpec)

	if flagSpec.XorGroup != "" && w.xorGroupsSet[completionGroupKey{owner: owner, name: flagSpec.XorGroup}] != nil {
		return false
	}

	if flagSpec.OneOfGroup != "" && w.oneOfGroupsSet[completionGroupKey{owner: owner, name: flagSpec.OneOfGroup}] != nil {
		return false
	}

	return !w.nonRepeatableSet[flagSpec]
}

func (w *astCompletionWalker) collectFlagCandidates(longItems, shortItems *[]predict.Item, seenLong, seenShort map[string]bool, flagSpec *Flag, toComplete string) {
	flagStyle := style.Theme.FlagArg
	if flagSpec.IsBool {
		flagStyle = style.Theme.FlagNoArg
	} else if flagSpec.IsSlice || flagSpec.IsMap {
		flagStyle = style.Theme.FlagMultiArg
	}

	longName := "--" + flagSpec.Name
	if !seenLong[longName] {
		seenLong[longName] = true
		*longItems = append(*longItems, predict.Item{
			Value:       longName,
			Display:     longName,
			Description: flagHelpText(flagSpec),
			Style:       flagStyle,
			Tag:         "longhand flags",
		})
	}

	if flagSpec.IsNegatable {
		noName := "--no-" + flagSpec.Name
		if !seenLong[noName] {
			seenLong[noName] = true

			help := flagHelpText(flagSpec)
			if help != "" {
				help = "Disable " + help
			}

			*longItems = append(*longItems, predict.Item{
				Value:       noName,
				Display:     noName,
				Description: help,
				Style:       style.Theme.FlagNoArg,
				Tag:         "longhand flags",
			})
		}
	}

	for _, alias := range flagSpec.Aliases {
		aliasName := "--" + alias
		if !seenLong[aliasName] {
			seenLong[aliasName] = true
			*longItems = append(*longItems, predict.Item{
				Value:       aliasName,
				Display:     aliasName,
				Description: flagHelpText(flagSpec),
				Style:       flagStyle,
				Tag:         "longhand flags",
				Alias:       true,
			})
		}
	}

	if flagSpec.Short != 0 && !strings.HasPrefix(toComplete, "--") {
		shortName := "-" + string(flagSpec.Short)
		if !seenShort[shortName] {
			seenShort[shortName] = true
			*shortItems = append(*shortItems, predict.Item{
				Value:       shortName,
				Display:     shortName,
				Description: flagHelpText(flagSpec),
				Style:       flagStyle,
				Tag:         "shorthand flags",
			})
		}
	}
}

func (w *astCompletionWalker) appendBuiltinFlags(longItems, shortItems *[]predict.Item, seenLong, seenShort map[string]bool, toComplete string) {
	if !w.currentCmd.DisableHelp && !seenLong["--help"] {
		seenLong["--help"] = true

		*longItems = append(*longItems, predict.Item{
			Value:       "--help",
			Display:     "--help",
			Description: BuiltinHelpFlag.Help,
			Style:       style.Theme.FlagNoArg,
			Tag:         "longhand flags",
		})
		w.appendBuiltinShort(shortItems, seenShort, w.rootCmd.HelpShort, BuiltinHelpFlag.Help, toComplete)
	}

	if w.currentCmd.VersionFlagVisible(len(w.cmdChain) == 1) && !seenLong["--version"] {
		seenLong["--version"] = true

		*longItems = append(*longItems, predict.Item{
			Value:       "--version",
			Display:     "--version",
			Description: BuiltinVersionFlag.Help,
			Style:       style.Theme.FlagNoArg,
			Tag:         "longhand flags",
		})
		w.appendBuiltinShort(shortItems, seenShort, w.rootCmd.VersionShort, BuiltinVersionFlag.Help, toComplete)
	}

	if w.currentCmd.WhyFlagVisible() && !seenLong["--why"] {
		seenLong["--why"] = true

		*longItems = append(*longItems, w.builtinWhyItems()...)
	}

	if w.currentCmd.CompletionFlagVisible(len(w.cmdChain) == 1) && !seenLong["--completion"] {
		seenLong["--completion"] = true

		*longItems = append(*longItems, predict.Item{
			Value:       "--completion",
			Display:     "--completion",
			Description: BuiltinCompletionFlag.Help,
			Style:       style.Theme.FlagArg,
			Tag:         "longhand flags",
		})
	}
}

// appendBuiltinShort adds the shorthand candidate for a built-in flag whose
// rune was assigned through app.HelpShortFlag/app.VersionShortFlag, mirroring the
// description and style of the synthesized longhand item. Unassigned runes and
// longhand prefixes contribute nothing.
func (w *astCompletionWalker) appendBuiltinShort(shortItems *[]predict.Item, seenShort map[string]bool, short rune, description string, toComplete string) {
	if short == 0 || strings.HasPrefix(toComplete, "--") {
		return
	}

	shortName := "-" + string(short)
	if seenShort[shortName] {
		return
	}

	seenShort[shortName] = true

	*shortItems = append(*shortItems, predict.Item{
		Value:       shortName,
		Display:     shortName,
		Description: description,
		Style:       style.Theme.FlagNoArg,
		Tag:         "shorthand flags",
	})
}

// builtinWhyItems returns the flag-name-level completion items for the
// built-in --why flag. When the app enabled JSON output, the value-bearing
// --why=<format> forms replace the plain toggle item so shells insert the
// value slot (--why=) and offer the accepted formats; otherwise --why
// completes as a plain toggle.
func (w *astCompletionWalker) builtinWhyItems() []predict.Item {
	formats := w.rootCmd.WhyFormats()
	if len(formats) < 2 {
		return []predict.Item{{
			Value:       "--why",
			Display:     "--why",
			Description: BuiltinWhyFlag.Help,
			Style:       style.Theme.FlagNoArg,
			Tag:         "longhand flags",
		}}
	}

	items := make([]predict.Item, 0, len(formats))
	for _, format := range formats {
		items = append(items, predict.Item{
			Value:       "--why=" + format,
			Display:     "--why=" + format,
			Description: BuiltinWhyFlag.Help,
			Style:       style.Theme.FlagArg,
			Tag:         "longhand flags",
		})
	}

	return items
}

func (w *astCompletionWalker) completeSubcommandsAndArgsAction(toComplete string) predict.Predictor {
	if w.canFallbackToRaw() {
		return w.trailingArgsAction(w.currentCmd.RawArgs.Predictor)
	}

	subCmdItems := w.collectSubcommandCandidates(toComplete)

	posIndex := len(w.posArgs)
	arg := getArgSpec(w.currentCmd, posIndex)

	if len(subCmdItems) > 0 && arg == nil {
		return predict.Items(subCmdItems...)
	}

	if arg != nil {
		argAction := w.resolvePositionalArgAction(arg)
		if len(subCmdItems) > 0 {
			return predict.Batch(predict.Items(subCmdItems...), argAction)
		}

		return argAction
	}

	return predict.None()
}

func (w *astCompletionWalker) resolvePositionalArgAction(arg *Arg) predict.Predictor {
	baseAction := resolveArgAction(arg, w.opts.RegisteredPredictors)

	return wrapSliceAction(baseAction, arg.IsSlice, arg.Sep, nil).Tag("arguments")
}

func (w *astCompletionWalker) isSubcommandEligible(subcommand *Command, toComplete string) bool {
	if subcommand.Hidden || subcommand.IsHelpCmd || subcommand.IsVersionCmd {
		return false
	}

	if subcommand.Deprecated != "" {
		return false
	}

	if subcommand.IsCompletionCmd && !w.currentCmd.HasUserSubcommands() && toComplete == "" {
		return false
	}

	return true
}

func (w *astCompletionWalker) collectSubcommandCandidates(toComplete string) []predict.Item {
	if len(w.currentCmd.Subcommands) == 0 {
		return nil
	}

	sectionIndexMap := make(map[string]int)

	for _, subcommand := range w.currentCmd.Subcommands {
		if subcommand.Section != "" && !subcommand.Hidden && !subcommand.IsHelpCmd && !subcommand.IsVersionCmd && subcommand.Deprecated == "" {
			if _, ok := sectionIndexMap[subcommand.Section]; !ok {
				sectionIndexMap[subcommand.Section] = len(sectionIndexMap)
			}
		}
	}

	var subCmdItems []predict.Item

	for _, subcommand := range w.currentCmd.Subcommands {
		if !w.isSubcommandEligible(subcommand, toComplete) {
			continue
		}

		tag, cmdStyle := subcommandTagAndStyle(subcommand, sectionIndexMap)

		subCmdItems = append(subCmdItems, predict.Item{
			Value:       subcommand.Name,
			Display:     subcommand.Name,
			Description: subcommand.Summary,
			Style:       cmdStyle,
			Tag:         tag,
		})

		for _, alias := range subcommand.Aliases {
			subCmdItems = append(subCmdItems, predict.Item{
				Value:       alias,
				Display:     alias,
				Description: subcommand.Summary,
				Style:       cmdStyle,
				Tag:         tag,
				Alias:       true,
			})
		}
	}

	return subCmdItems
}

// flagOwner returns the command that declares flag within the active chain.
// Flags can only be consumed from the chain and chains only grow, so the
// declaring command is always present. Group suppression is scoped to it.
func (w *astCompletionWalker) flagOwner(flag *Flag) *Command {
	for _, cmd := range slices.Backward(w.cmdChain) {
		if slices.Contains(cmd.Flags, flag) {
			return cmd
		}
	}

	return nil
}

func subcommandTagAndStyle(subcommand *Command, sectionIndexMap map[string]int) (string, string) {
	if subcommand.Section != "" {
		return strings.ToLower(subcommand.Section) + " commands", style.Theme.Highlight(sectionIndexMap[subcommand.Section])
	}

	if len(sectionIndexMap) > 0 {
		return "other commands", style.Default
	}

	return "commands", style.Default
}

func getArgSpec(cmd *Command, posIndex int) *Arg {
	if posIndex < len(cmd.Args) {
		return cmd.Args[posIndex]
	}

	if len(cmd.Args) > 0 && cmd.Args[len(cmd.Args)-1].IsSlice {
		return cmd.Args[len(cmd.Args)-1]
	}

	return nil
}

func resolveArgAction(arg *Arg, registered map[string]predict.Predictor) predict.Predictor {
	if arg.Predictor != nil {
		if act, ok := resolvePredictorSpec(arg.Predictor, registered); ok {
			return act.Tag("arguments")
		}
	}

	if len(arg.Enum) > 0 {
		return predict.Values(arg.Enum...).Tag("arguments")
	}

	cleanName := strings.Trim(arg.Name, "<>")
	helpMsg := fmt.Sprintf("<%s>", cleanName)

	if argDescription := argHelpText(arg); argDescription != "" {
		helpMsg = fmt.Sprintf("<%s>: %s", cleanName, argDescription)
	}

	return predict.None().Usage(helpMsg)
}

func resolveFlagAction(flag *Flag, registered map[string]predict.Predictor, whyFormats []string) predict.Predictor {
	act := resolveBaseFlagAction(flag, registered, whyFormats).Tag("values")
	if description := flagHelpText(flag); description != "" {
		act = act.Usage(description)
	}

	return act
}

// flagHelpText returns the flag's help description for completion candidate
// usage lines, falling back to the tag-aware default contributed by the
// value's domain type.
func flagHelpText(flag *Flag) string {
	if flag.Help != "" {
		return flag.Help
	}

	return DefaultTypeHelp(flag.TargetType, flag.FieldTags)
}

// argHelpText mirrors flagHelpText for positional arguments.
func argHelpText(arg *Arg) string {
	if arg.Help != "" {
		return arg.Help
	}

	return DefaultTypeHelp(arg.TargetType, arg.FieldTags)
}

func resolveBaseFlagAction(flag *Flag, registered map[string]predict.Predictor, whyFormats []string) predict.Predictor {
	if flag == BuiltinWhyFlag {
		return predict.Values(whyFormats...)
	}

	if flag.Predictor != nil {
		if predAct, ok := resolvePredictorSpec(flag.Predictor, registered); ok {
			return predAct
		}
	}

	if len(flag.Enum) > 0 {
		return predict.Values(flag.Enum...)
	}

	if flag.IsBool {
		return predict.Values("true", "false")
	}

	if flag.IsMap {
		return predict.None().Usage("<key>=<value>")
	}

	return predict.None()
}

func resolvePredictorSpec(spec *PredictorSpec, registered map[string]predict.Predictor) (predict.Predictor, bool) {
	switch spec.Kind {
	case PredictorKindPredictable:
		return spec.Predictor, true
	case PredictorKindFile:
		return predict.Files(spec.Patterns...), true
	case PredictorKindCustom:
		if registered != nil {
			if act, ok := registered[spec.Name]; ok {
				return act, true
			}
		}

		return predict.None(), false
	default:
		return resolveBuiltinPredictor(spec.Kind)
	}
}

func resolveBuiltinPredictor(kind PredictorKind) (predict.Predictor, bool) {
	switch kind {
	case PredictorKindDir:
		return predict.Dirs(), true
	case PredictorKindExec:
		return predict.Executables(), true
	case PredictorKindSymlink:
		return predict.Symlinks(), true
	case PredictorKindNone:
		return predict.None(), true
	default:
		return predict.None(), false
	}
}

func wrapSliceAction(base predict.Predictor, isSlice bool, sep string, prevValues []string) predict.Predictor {
	if !isSlice {
		return base
	}

	if sep == "" || sep == "none" {
		if len(prevValues) == 0 {
			return base
		}

		return base.Exclude(prevValues...)
	}

	delim := sep

	return predict.MultiParts(delim, func(sliceCtx predict.Context) predict.Predictor {
		toExclude := make([]string, 0, len(sliceCtx.Parts)+len(prevValues))
		toExclude = append(toExclude, sliceCtx.Parts...)

		for _, prev := range prevValues {
			for subcommand := range strings.SplitSeq(prev, delim) {
				if subcommand != "" {
					toExclude = append(toExclude, subcommand)
				}
			}
		}

		return base.Exclude(toExclude...)
	})
}
