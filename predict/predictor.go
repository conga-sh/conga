package predict

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/conga-sh/conga/internal/slicex"
)

// DefaultTimeout is the default execution timeout applied to dynamic predictor callbacks.
const DefaultTimeout = 250 * time.Millisecond

// Result represents the evaluated result of a Predictor against a Context.
type Result struct {
	// Items holds the completion candidates. Their order is significant when
	// Directive includes DirectiveKeepOrder: the wire layer forwards the
	// keeporder instruction and the shell preserves the engine order.
	Items []Item

	// Directive is the bitmask of shell instructions produced by the
	// predictor and its modifiers.
	Directive Directive

	// Usage is an ActiveHelp/tooltip hint accompanying the candidates, not
	// syntax help. When Directive includes DirectiveError, the wire layer
	// also uses it as the error message for shells with a message channel
	// (an empty Usage renders the literal "ERROR").
	Usage string
}

// Predictor represents a completion candidate generator or combinator.
type Predictor struct {
	callback   func(ctx Context) Predictor
	timeoutAlt *Predictor
	batch      []Predictor
	multiparts *multiPartsConfig
	filterText string
	prefix     string
	suffix     string
	style      string
	tag        string
	usage      string
	items      []Item
	raw        []string
	timeout    time.Duration
	directive  Directive
	noSpace    bool
	keepOrder  bool
}

// Exclude returns a new Predictor that filters out candidates whose Value or Display matches any of the specified values.
func (p Predictor) Exclude(values ...string) Predictor {
	if len(values) == 0 {
		return p
	}

	excluded := make(map[string]bool, len(values))
	for _, v := range values {
		excluded[v] = true
	}

	filtered := p
	if len(p.items) > 0 {
		filtered.items = slicex.Filter(p.items, func(item Item) bool {
			return !excluded[item.Value] && !excluded[item.Display]
		})
	}

	if len(p.raw) > 0 {
		filtered.raw = slicex.Filter(p.raw, func(r string) bool {
			return !excluded[r]
		})
	}

	if p.callback != nil {
		origCb := p.callback
		filtered.callback = func(ctx Context) Predictor {
			return origCb(ctx).Exclude(values...)
		}
	}

	return filtered
}

// FilterText filters candidates by matching against the given query prefix.
// The argument replaces the implicit ctx.ToComplete term used during
// filtering; FilterText("") restores the implicit term. There is no "disable
// filtering" spelling. This method filters by query match — unlike the
// predicate-based FilesFiltered and PathsFiltered constructors, whose filter
// argument is a predicate over directory entries.
func (p Predictor) FilterText(toComplete string) Predictor {
	p.filterText = toComplete

	return p
}

// Invoke evaluates the Predictor against the given Context and returns the resolved Result.
func (p Predictor) Invoke(ctx Context) Result {
	base, hasCustomSubcommandCompletion := p.evaluateBase(ctx)

	return p.applyModifiers(base, ctx, hasCustomSubcommandCompletion)
}

// KeepOrder marks the predictor to instruct the shell to preserve candidate order.
func (p Predictor) KeepOrder() Predictor {
	p.keepOrder = true

	return p
}

// NoSpace marks the predictor to instruct the shell not to append a space after completion.
func (p Predictor) NoSpace() Predictor {
	p.noSpace = true

	return p
}

// Prefix prepends a prefix string to all candidate values.
func (p Predictor) Prefix(prefix string) Predictor {
	p.prefix += prefix

	return p
}

// Raw returns the statically stored completion candidate values: the raw
// strings recorded by constructors such as Values, Items, and Map, or the
// Value fields of the stored Items when no raw slice was recorded. It is a
// snapshot, not an evaluation — callbacks, batches, and MultiParts are not
// invoked (a callback-only predictor returns an empty slice), and while
// Exclude narrows the stored set, Prefix, Suffix, Style, and Tag are ignored
// because they apply to Result.Items only. The returned slice is always a
// fresh copy.
func (p Predictor) Raw() []string {
	if len(p.raw) > 0 {
		return slices.Clone(p.raw)
	}

	return slicex.Map(p.items, func(item Item) string {
		return item.Value
	})
}

// Style sets a default style for candidates in this predictor that do not have their own style.
func (p Predictor) Style(style string) Predictor {
	p.style = style

	return p
}

// Suffix appends a suffix string to all candidate values.
func (p Predictor) Suffix(suffix string) Predictor {
	p.suffix += suffix

	return p
}

// Tag sets a default group tag for candidates in this predictor that do not have their own tag.
func (p Predictor) Tag(tag string) Predictor {
	p.tag = tag

	return p
}

// Timeout sets an execution deadline for the predictor with an optional fallback predictor.
// Passing d < 0 disables timeout enforcement for this predictor.
// Passing d == 0 (the default) applies DefaultTimeout.
// If more than one alternative predictor is passed, Timeout panics.
func (p Predictor) Timeout(d time.Duration, alternative ...Predictor) Predictor {
	if len(alternative) > 1 {
		panic("predict: Timeout accepts at most 1 alternative predictor")
	}

	p.timeout = d

	if len(alternative) == 1 {
		alt := alternative[0]
		p.timeoutAlt = &alt
	}

	return p
}

// Usage attaches an ActiveHelp/tooltip hint message to the predictor. This is
// completion-time guidance, not the root package's syntax-help usage summary
// (Session.Usage and App.UsageOnError).
func (p Predictor) Usage(message string) Predictor {
	p.usage = message

	return p
}

// Usagef attaches a formatted ActiveHelp/tooltip hint message to the predictor
// using fmt.Sprintf. See Usage for the distinction from the root package's
// syntax-help usage summary.
func (p Predictor) Usagef(format string, a ...any) Predictor {
	p.usage = fmt.Sprintf(format, a...)

	return p
}

// Batch combines multiple predictors into a single aggregated predictor.
func Batch(predictors ...Predictor) Predictor {
	return Predictor{
		batch: predictors,
	}
}

// Callback defers candidate computation until invocation time using the provided function.
func Callback(fn func(c Context) Predictor) Predictor {
	return Predictor{
		callback: fn,
	}
}

// Items creates a Predictor with the specified completion items.
func Items(items ...Item) Predictor {
	copied := slicex.Map(items, func(item Item) Item {
		item.Display = cmp.Or(item.Display, item.Value)

		return item
	})
	raw := slicex.Map(items, func(item Item) string { return item.Value })

	return Predictor{
		items:     copied,
		directive: DirectiveNoFileComp,
		raw:       raw,
	}
}

// Map creates a Predictor from a map of completion candidate values to descriptions, sorted by key.
func Map(items map[string]string) Predictor {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	completionItems := make([]Item, 0, len(keys))
	for _, k := range keys {
		completionItems = append(completionItems, Item{
			Value:       k,
			Display:     k,
			Description: items[k],
		})
	}

	return Predictor{
		items:     completionItems,
		directive: DirectiveNoFileComp,
		raw:       keys,
	}
}

// MapValues creates a segmented KEY=VALUE Predictor from a map of keys to allowed value slices.
func MapValues(spec map[string][]string) Predictor {
	return MultiParts("=", func(compCtx Context) Predictor {
		existingEntries := slices.Clone(compCtx.ParentParts)

		for _, values := range compCtx.FlagMultiValues {
			existingEntries = append(existingEntries, values...)
		}

		usedKeys, usedKeyValues := parseUsedMapEntries(existingEntries)

		if len(compCtx.Parts) == 0 {
			keys := make([]string, 0, len(spec))

			for k := range spec {
				if !usedKeys[k] {
					keys = append(keys, k+"=")
				}
			}

			slices.Sort(keys)

			return Values(keys...).NoSpace()
		}

		key := compCtx.Parts[0]
		if values, ok := spec[key]; ok {
			allowed := make([]string, 0, len(values))

			for _, value := range values {
				if usedKeyValues[key] == nil || !usedKeyValues[key][value] {
					allowed = append(allowed, value)
				}
			}

			return Values(allowed...)
		}

		return None()
	})
}

// MultiParts completes segmented tokens by splitting on a delimiter.
func MultiParts(delimiter string, fn func(c Context) Predictor) Predictor {
	return Predictor{
		multiparts: &multiPartsConfig{
			delimiter: delimiter,
			fn:        fn,
		},
	}
}

// None creates a Predictor that suppresses completion and file fallbacks.
func None() Predictor {
	return Predictor{
		directive: DirectiveNoFileComp,
	}
}

// Values creates a Predictor from static candidate value strings.
func Values(values ...string) Predictor {
	items := slicex.Map(values, func(value string) Item {
		return Item{Value: value, Display: value}
	})

	return Predictor{
		items:     items,
		directive: DirectiveNoFileComp,
		raw:       slices.Clone(values),
	}
}

type multiPartsConfig struct {
	delimiter string
	fn        func(ctx Context) Predictor
}

func (p Predictor) applyModifiers(base Result, ctx Context, hasCustomSubcommandCompletion bool) Result {
	items := slices.Clone(base.Items)
	directive := base.Directive
	usage := base.Usage

	if p.prefix != "" {
		for i := range items {
			items[i].Value = p.prefix + items[i].Value
		}
	}

	if p.suffix != "" {
		for i := range items {
			items[i].Value += p.suffix
		}
	}

	if p.style != "" {
		for i := range items {
			items[i].Style = cmp.Or(items[i].Style, p.style)
		}
	}

	if p.tag != "" {
		for i := range items {
			items[i].Tag = cmp.Or(items[i].Tag, p.tag)
		}
	}

	usage = cmp.Or(p.usage, usage)

	if p.noSpace {
		directive |= DirectiveNoSpace
	}

	if p.keepOrder {
		directive |= DirectiveKeepOrder
	}

	items = p.filterItems(items, ctx, hasCustomSubcommandCompletion)

	return Result{
		Items:     items,
		Directive: directive,
		Usage:     usage,
	}
}

func (p Predictor) evaluateBase(ctx Context) (Result, bool) {
	if p.callback == nil && len(p.batch) == 0 && (p.multiparts == nil || p.multiparts.delimiter == "") {
		return p.evaluateBaseRaw(ctx)
	}

	timeout := p.timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	if timeout > 0 {
		return p.evaluateBaseWithTimeout(ctx, timeout)
	}

	return p.evaluateBaseRaw(ctx)
}

func (p Predictor) evaluateBaseWithTimeout(ctx Context, timeout time.Duration) (Result, bool) {
	type evalResult struct {
		result           Result
		customSubcommand bool
		panicValue       any
	}

	resultChan := make(chan evalResult, 1)

	go func() {
		defer func() {
			// A panicking callback must not crash the process from this
			// side goroutine: forward the panic value to the consumer,
			// which re-panics on the main goroutine so the completion
			// dispatcher's recovery contract applies.
			if recovered := recover(); recovered != nil {
				resultChan <- evalResult{panicValue: recovered}
			}
		}()

		base, hasCustomSubcommand := p.evaluateBaseRaw(ctx)
		resultChan <- evalResult{result: base, customSubcommand: hasCustomSubcommand}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case evaluated := <-resultChan:
		if evaluated.panicValue != nil {
			panic(evaluated.panicValue)
		}

		return evaluated.result, evaluated.customSubcommand
	case <-timer.C:
		if p.timeoutAlt != nil {
			return p.timeoutAlt.evaluateBase(ctx)
		}

		return Result{
			Directive: DirectiveNoFileComp,
		}, false
	}
}

func (p Predictor) evaluateBaseRaw(ctx Context) (Result, bool) {
	if p.callback != nil {
		return p.invokeCallback(ctx), false
	}

	if len(p.batch) > 0 {
		return p.invokeBatch(ctx), false
	}

	if p.multiparts != nil && p.multiparts.delimiter != "" {
		items, directive, usage := p.invokeMultiParts(ctx)

		return Result{
			Items:     items,
			Directive: directive,
			Usage:     usage,
		}, true
	}

	return Result{
		Items:     slices.Clone(p.items),
		Directive: p.directive,
		Usage:     p.usage,
	}, false
}

func (p Predictor) filterItems(items []Item, ctx Context, hasCustomSubcommandCompletion bool) []Item {
	filterTerm := p.filterText
	if filterTerm == "" && !hasCustomSubcommandCompletion && ctx.ToComplete != "" {
		filterTerm = ctx.ToComplete
	}

	if filterTerm == "" {
		return items
	}

	cleanTerm := strings.Trim(filterTerm, "'\"")
	if cleanTerm == "" {
		return items
	}

	lowerTerm := strings.ToLower(cleanTerm)

	return slicex.Filter(items, func(item Item) bool {
		return matchesSmartCase(item.Value, cleanTerm, lowerTerm)
	})
}

func (p Predictor) invokeBatch(ctx Context) Result {
	var (
		items     []Item
		usage     string
		directive = p.directive
		seen      = make(map[string]bool)
	)

	for _, subPred := range p.batch {
		subRes := subPred.Invoke(ctx)
		directive |= subRes.Directive

		if subRes.Usage != "" && usage == "" {
			usage = subRes.Usage
		}

		for _, item := range subRes.Items {
			if !seen[item.Value] {
				seen[item.Value] = true
				items = append(items, item)
			}
		}
	}

	return Result{
		Items:     items,
		Directive: directive,
		Usage:     usage,
	}
}

func (p Predictor) invokeCallback(ctx Context) Result {
	child := p.callback(ctx)
	childRes := child.Invoke(ctx)

	return Result{
		Items:     childRes.Items,
		Directive: childRes.Directive | p.directive,
		Usage:     childRes.Usage,
	}
}

func (p Predictor) invokeMultiParts(ctx Context) ([]Item, Directive, string) {
	var (
		prefix         string
		subToComplete  string
		completedParts []string
	)

	delim := p.multiparts.delimiter
	lastIdx := strings.LastIndex(ctx.ToComplete, delim)

	if lastIdx >= 0 {
		prefix = ctx.ToComplete[:lastIdx+len(delim)]
		subToComplete = ctx.ToComplete[lastIdx+len(delim):]
		partsStr := ctx.ToComplete[:lastIdx]

		if partsStr != "" {
			completedParts = strings.Split(partsStr, delim)
		}
	} else {
		subToComplete = ctx.ToComplete
	}

	subCtx := ctx
	subCtx.ToComplete = subToComplete

	if len(ctx.Parts) > 0 {
		subCtx.ParentParts = append(slices.Clone(ctx.ParentParts), ctx.Parts...)
	}

	subCtx.Parts = slices.Clone(completedParts)

	childPred := p.multiparts.fn(subCtx)
	childRes := childPred.Invoke(subCtx)

	items := slicex.Map(childRes.Items, func(item Item) Item {
		item.Value = prefix + item.Value

		return item
	})

	directive := childRes.Directive | p.directive
	if lastIdx >= 0 {
		directive |= DirectiveNoSpace
	}

	return items, directive, childRes.Usage
}

func matchesSmartCase(candidate, term, lowerTerm string) bool {
	if term == lowerTerm {
		return strings.HasPrefix(strings.ToLower(candidate), lowerTerm)
	}

	return strings.HasPrefix(candidate, term)
}

func parseUsedMapEntries(existingEntries []string) (map[string]bool, map[string]map[string]bool) {
	usedKeys := make(map[string]bool)
	usedKeyValues := make(map[string]map[string]bool)

	for _, entry := range existingEntries {
		for _, subEntry := range slicex.SplitAndTrim(entry, ",") {
			entryKey, entryValue, found := strings.Cut(subEntry, "=")
			if found && entryKey != "" {
				usedKeys[entryKey] = true

				if usedKeyValues[entryKey] == nil {
					usedKeyValues[entryKey] = make(map[string]bool)
				}

				if entryValue != "" {
					usedKeyValues[entryKey][entryValue] = true
				}
			}
		}
	}

	return usedKeys, usedKeyValues
}
