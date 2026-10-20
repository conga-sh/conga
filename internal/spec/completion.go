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
type CompleteOptions struct {
	RegisteredPredictors map[string]predict.Predictor
	Env                  map[string]string
	EnvLookup            func(string) (string, bool)
	Dir                  string
	TargetShell          string
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

type astCompletionWalker struct {
	rootCmd          *Command
	opts             CompleteOptions
	currentCmd       *Command
	cmdChain         []*Command
	flagsSet         map[*Flag][]string
	xorGroupsSet     map[string]*Flag
	nonRepeatableSet map[*Flag]bool
	posArgs          []string
	inTerminator     bool
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
		xorGroupsSet:     make(map[string]*Flag),
		nonRepeatableSet: make(map[*Flag]bool),
		posArgs:          make([]string, 0),
	}
}

func (w *astCompletionWalker) walkTokens(toks []tokenizer.Token) {
	for i := range toks {
		w.consumeToken(toks[i])
	}
}

func (w *astCompletionWalker) consumeToken(tok tokenizer.Token) {
	if w.pendingFlagValue != nil {
		w.consumePendingFlagValue(tok.Raw)

		return
	}

	matchedSubcommand := w.matchSubcommand(tok)

	w.trackAnalysisToken(tok, matchedSubcommand)

	if w.inTerminator {
		w.posArgs = append(w.posArgs, tok.Raw)

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

// matchSubcommand resolves a positional token against the current command's
// subcommands. Tokens after the -- terminator never route to a subcommand.
func (w *astCompletionWalker) matchSubcommand(tok tokenizer.Token) *Command {
	if tok.Type != tokenizer.TokenTypePositional || w.inTerminator {
		return nil
	}

	return w.currentCmd.FindSubcommand(tok.Raw)
}

// trackAnalysisToken mirrors the runtime token parser's positional, dash, and
// raw attribution for the loader's completion-time view. The candidate walker
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

// registerXorGroup records the flag's xor-group as used, when it declares one.
func (w *astCompletionWalker) registerXorGroup(flag *Flag) {
	if flag.XorGroup != "" {
		w.xorGroupsSet[flag.XorGroup] = flag
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
	w.registerXorGroup(flag)
	w.markNonRepeatable(flag)

	w.pendingFlagValue = nil
}

func (w *astCompletionWalker) markFlagConsumed(flag *Flag, hasAttached bool, attachedValue string, negated bool) {
	w.registerXorGroup(flag)

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

func (w *astCompletionWalker) consumeLongFlag(tok tokenizer.Token) {
	flag := FindFlagInChain(w.cmdChain, tok.Name)

	negated := false

	if flag == nil {
		flag = FindNegatableFlagInChain(w.cmdChain, tok.Name)
		negated = flag != nil
	}

	if flag == nil {
		return
	}

	w.markFlagConsumed(flag, tok.HasAttachedValue, tok.Value, negated)
}

func (w *astCompletionWalker) consumeShortFlag(tok tokenizer.Token) {
	runes := tok.ShortRunes

	for index := range runes {
		flag := FindShortFlagInChain(w.cmdChain, runes[index])
		if flag == nil {
			return
		}

		w.registerXorGroup(flag)

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
	} else {
		w.posArgs = append(w.posArgs, tok.Raw)
	}
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

// analysis returns the parsed invocation facts for the engine's early loader
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

	if strings.Contains(toComplete, "=") {
		if invoked, handled := w.completeAttachedFlag(toComplete, compCtx); handled {
			return invoked
		}
	}

	if w.inTerminator {
		return w.completeTerminatorPositional(compCtx)
	}

	if strings.HasPrefix(toComplete, "-") {
		return w.completeFlagNamesAction(compCtx.ToComplete).Invoke(compCtx)
	}

	cmdOrArgAction := w.completeSubcommandsAndArgsAction(toComplete)
	if w.isFlatShell() {
		if w.currentCmd.HasUserSubcommands() || len(w.currentCmd.Args) > 0 || w.currentCmd.RawArgs != nil {
			return cmdOrArgAction.Invoke(compCtx)
		}
	}

	flagAction := w.completeFlagNamesAction(toComplete)

	return predict.Batch(cmdOrArgAction, flagAction).Invoke(compCtx)
}

func (w *astCompletionWalker) completePendingFlag(toComplete string, compCtx predict.Context) predict.Result {
	baseAction := resolveFlagAction(w.pendingFlagValue, w.opts.RegisteredPredictors)
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

	baseAction := resolveFlagAction(flag, w.opts.RegisteredPredictors)
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

func (w *astCompletionWalker) completeTerminatorPositional(compCtx predict.Context) predict.Result {
	posIndex := len(w.posArgs)
	arg := getArgSpec(w.currentCmd, posIndex)

	if arg != nil {
		baseAction := resolveArgAction(arg, w.opts.RegisteredPredictors)
		action := wrapSliceAction(baseAction, arg.IsSlice, arg.Sep, compCtx.Args).Tag("arguments")

		return action.Invoke(compCtx)
	}

	return predict.Files().Invoke(compCtx)
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

	w.appendBuiltinFlags(&longItems, seenLong)

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

func (w *astCompletionWalker) isFlatShell() bool {
	shell := strings.ToLower(strings.TrimSpace(w.opts.TargetShell))
	if shell == "" {
		shell = strings.ToLower(strings.TrimSpace(wire.Shell()))
	}

	if shell == "" || shell == "zsh" || shell == "export" {
		return false
	}

	return true
}

func (w *astCompletionWalker) isFlagEligible(flagSpec *Flag) bool {
	if flagSpec.Hidden {
		return false
	}

	if flagSpec.Deprecated != "" {
		return false
	}

	if flagSpec.XorGroup != "" && w.xorGroupsSet[flagSpec.XorGroup] != nil {
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

func (w *astCompletionWalker) appendBuiltinFlags(longItems *[]predict.Item, seenLong map[string]bool) {
	if !w.currentCmd.DisableHelp && !seenLong["--help"] {
		seenLong["--help"] = true

		*longItems = append(*longItems, predict.Item{
			Value:       "--help",
			Display:     "--help",
			Description: BuiltinHelpFlag.Help,
			Style:       style.Theme.FlagNoArg,
			Tag:         "longhand flags",
		})
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
	}

	if w.currentCmd.WhyFlagVisible() && !seenLong["--why"] {
		seenLong["--why"] = true

		*longItems = append(*longItems, predict.Item{
			Value:       "--why",
			Display:     "--why",
			Description: BuiltinWhyFlag.Help,
			Style:       style.Theme.FlagNoArg,
			Tag:         "longhand flags",
		})
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

func (w *astCompletionWalker) completeSubcommandsAndArgsAction(toComplete string) predict.Predictor {
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

	groupIndexMap := make(map[string]int)

	for _, subcommand := range w.currentCmd.Subcommands {
		if subcommand.Group != "" && !subcommand.Hidden && !subcommand.IsHelpCmd && !subcommand.IsVersionCmd && subcommand.Deprecated == "" {
			if _, ok := groupIndexMap[subcommand.Group]; !ok {
				groupIndexMap[subcommand.Group] = len(groupIndexMap)
			}
		}
	}

	var subCmdItems []predict.Item

	for _, subcommand := range w.currentCmd.Subcommands {
		if !w.isSubcommandEligible(subcommand, toComplete) {
			continue
		}

		tag, cmdStyle := subcommandTagAndStyle(subcommand, groupIndexMap)

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
			})
		}
	}

	return subCmdItems
}

func subcommandTagAndStyle(subcommand *Command, groupIndexMap map[string]int) (string, string) {
	if subcommand.Group != "" {
		return strings.ToLower(subcommand.Group) + " commands", style.Theme.Highlight(groupIndexMap[subcommand.Group])
	}

	if len(groupIndexMap) > 0 {
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

func resolveFlagAction(flag *Flag, registered map[string]predict.Predictor) predict.Predictor {
	act := resolveBaseFlagAction(flag, registered).Tag("values")
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

func resolveBaseFlagAction(flag *Flag, registered map[string]predict.Predictor) predict.Predictor {
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
