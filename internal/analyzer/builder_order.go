package analyzer

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	// builderOrderChainGroup is the fluent chain attached to conga.New.
	builderOrderChainGroup builderOrderGroup = iota

	// builderOrderStatementGroup is the straight-line statement run on the app
	// variable bound by the construction assignment.
	builderOrderStatementGroup

	// builderOrderGroupCount bounds the builderOrderGroup enum.
	builderOrderGroupCount
)

// errBuilderOrderSpec marks malformed -builder-order-spec values detected while
// parsing the configured canonical builder call order.
var errBuilderOrderSpec = errors.New("invalid builder order spec")

// builderOrderGroup identifies one textually interchangeable call group at a
// construction site: the fluent chain attached to conga.New, or the
// straight-line statements on the app variable. Calls only swap with calls of
// the same group.
type builderOrderGroup int

// builderOrderSpec is the flag.Value behind -builder-order-spec: it validates
// and normalizes the spec at flag-parse time, so a malformed configuration
// fails before any package is analyzed.
type builderOrderSpec struct {
	value string
}

// Set validates the spec and records its normalized form.
func (s *builderOrderSpec) Set(value string) error {
	order, err := parseBuilderCallOrder(value)
	if err != nil {
		return err
	}

	s.value = strings.Join(order, ",")

	return nil
}

// String returns the normalized spec, empty when the flag was not provided.
func (s *builderOrderSpec) String() string {
	return s.value
}

// builderOrderSlot is one *conga.App[T] method call participating in the
// call-order check. start and end bound the swappable source span, while
// argumentStart and argumentEnd bound the argument list, whose comments travel
// safely with the call text. rank is the 1-based canonical position (0 when
// the active order leaves the method unconstrained) and barrier marks calls
// that execute user code and therefore never move.
type builderOrderSlot struct {
	name           string
	start          token.Pos
	end            token.Pos
	argumentStart  token.Pos
	argumentEnd    token.Pos
	reportPosition token.Pos
	rank           int
	barrier        bool
	group          builderOrderGroup
}

// swappable reports whether the slot text may travel during a reorder:
// unconstrained and barrier calls stay pinned in place.
func (s builderOrderSlot) swappable() bool {
	return s.rank > 0 && !s.barrier
}

// builderOrderSite captures one conga.New construction site: the fluent chain
// calls plus the builder statements observed on the bound app variable.
type builderOrderSite struct {
	slots        []builderOrderSlot
	groupFixable [builderOrderGroupCount]bool
}

// violations scans the site's slots in source order, recording every call that
// sits below a higher-ranked predecessor.
func (s *builderOrderSite) violations() []builderOrderViolation {
	var violations []builderOrderViolation

	var maximum *builderOrderSlot

	for index := range s.slots {
		slot := &s.slots[index]
		if slot.rank == 0 {
			continue
		}

		if maximum != nil && slot.rank < maximum.rank {
			violations = append(violations, builderOrderViolation{
				later:   *slot,
				earlier: *maximum,
				fixable: s.pairFixable(*maximum, *slot),
			})

			continue
		}

		if maximum == nil || slot.rank > maximum.rank {
			maximum = slot
		}
	}

	return violations
}

// pairFixable reports whether text reordering can repair an inversion: both
// calls must share one reorderable group, neither may be a barrier call, and
// the group must not be pinned by comments or an unswappable statement shape.
func (s *builderOrderSite) pairFixable(earlier builderOrderSlot, later builderOrderSlot) bool {
	if earlier.barrier || later.barrier {
		return false
	}

	if earlier.group != later.group {
		return false
	}

	return s.groupFixable[earlier.group]
}

// groupSlots returns the source-ordered slots of one group.
func (s *builderOrderSite) groupSlots(group builderOrderGroup) []builderOrderSlot {
	var slots []builderOrderSlot

	for _, slot := range s.slots {
		if slot.group == group {
			slots = append(slots, slot)
		}
	}

	return slots
}

// builderOrderViolation records one call that appears after a call it must
// precede, together with whether text reordering can repair the inversion.
type builderOrderViolation struct {
	later   builderOrderSlot
	earlier builderOrderSlot
	fixable bool
}

// builderOrderTextSwap pairs a destination slot with the slot whose text must
// move into it.
type builderOrderTextSwap struct {
	slot   builderOrderSlot
	source builderOrderSlot
}

// checkBuilderCallOrder enforces the configured builder call order at every
// conga.New construction site. The check is opt-in via -builder-order and runs
// independently of the schema hierarchy walk.
func checkBuilderCallOrder(pass *analysis.Pass) error {
	if !isFlagEnabled(pass, "builder-order") {
		return nil
	}

	order, err := activeBuilderCallOrder(pass)
	if err != nil {
		return err
	}

	ranks := builderOrderRanks(order)

	for _, file := range pass.Files {
		if isTestFile(pass.Fset, file) || ast.IsGenerated(file) {
			continue
		}

		diagnostic, found := builderOrderFileDiagnostic(pass, file, ranks)
		if !found {
			continue
		}

		pass.Report(diagnostic)
	}

	return nil
}

// builderOrderFileDiagnostic aggregates one file's order violations into a
// single diagnostic, carrying the reordering fix when every violation in the
// file is auto-fixable.
func builderOrderFileDiagnostic(pass *analysis.Pass, file *ast.File, ranks map[string]int) (analysis.Diagnostic, bool) {
	sites := collectBuilderOrderSites(pass, file, ranks)
	if len(sites) == 0 {
		return analysis.Diagnostic{}, false
	}

	pinBuilderOrderCommentRegions(pass.Fset, file, sites)

	var violations []builderOrderViolation

	for index := range sites {
		violations = append(violations, sites[index].violations()...)
	}

	if len(violations) == 0 {
		return analysis.Diagnostic{}, false
	}

	slices.SortFunc(violations, func(first builderOrderViolation, second builderOrderViolation) int {
		return cmp.Compare(first.later.reportPosition, second.later.reportPosition)
	})

	fixable := true

	for _, violation := range violations {
		if !violation.fixable {
			fixable = false

			break
		}
	}

	suggestedFixes := builderOrderSuggestedFixes(pass, file, sites, fixable)
	if fixable && len(suggestedFixes) == 0 {
		fixable = false
	}

	return analysis.Diagnostic{
		Pos:            violations[0].later.reportPosition,
		Message:        builderOrderMessage(violations, fixable),
		SuggestedFixes: suggestedFixes,
	}, true
}

// builderOrderSuggestedFixes converts the site plans into one suggested fix,
// or nil when the file is not auto-fixable.
func builderOrderSuggestedFixes(pass *analysis.Pass, file *ast.File, sites []builderOrderSite, fixable bool) []analysis.SuggestedFix {
	if !fixable {
		return nil
	}

	edits, ok := builderOrderFileEdits(pass, file, sites)
	if !ok || len(edits) == 0 {
		return nil
	}

	return []analysis.SuggestedFix{{
		Message:   "reorder builder calls to the configured order",
		TextEdits: edits,
	}}
}

// collectBuilderOrderSites discovers the conga.New construction sites in one
// file and captures the fluent chain calls plus the straight-line builder
// statements observed on each bound app variable.
func collectBuilderOrderSites(pass *analysis.Pass, file *ast.File, ranks map[string]int) []builderOrderSite {
	parents := nodeParents(file)

	var sites []builderOrderSite

	for _, call := range anchorCallNodes(file) {
		if anchorFromCall(pass, call) == nil {
			continue
		}

		chainEnd := chainEndNode(parents, call)

		site := builderOrderSite{groupFixable: [builderOrderGroupCount]bool{true, true}}

		chainSlots, _ := fluentChainSegments(pass, chainEnd, ranks)
		site.slots = append(site.slots, chainSlots...)

		if variable, block, index := boundAppVariable(pass, parents, chainEnd); variable != nil {
			statementSlots, statementFixable := collectBuilderOrderStatementSlots(pass, block, index, variable, ranks)

			site.slots = append(site.slots, statementSlots...)
			site.groupFixable[builderOrderStatementGroup] = statementFixable
		}

		if len(site.slots) == 0 {
			continue
		}

		sites = append(sites, site)
	}

	return sites
}

// collectBuilderOrderStatementSlots gathers the builder calls of the
// straight-line statements following a construction assignment. The run stops
// at the first execution call and at the first statement that touches the app
// value in any way other than a pure builder chain.
func collectBuilderOrderStatementSlots(pass *analysis.Pass, block *ast.BlockStmt, startIndex int, variable *types.Var, ranks map[string]int) ([]builderOrderSlot, bool) {
	var slots []builderOrderSlot

	fixable := true

	for _, statement := range block.List[startIndex+1:] {
		chainSlots, rootExpression := fluentChainSegments(pass, statementExpression(statement), ranks)

		if len(chainSlots) > 0 && isAppVariableExpression(pass, rootExpression, variable) {
			executionIndex := slices.IndexFunc(chainSlots, func(slot builderOrderSlot) bool {
				return isBuilderOrderExecution(slot.name)
			})

			switch {
			case executionIndex == 0:
				return slots, fixable
			case executionIndex > 0:
				slots = append(slots, regroupBuilderOrderSlots(chainSlots[:executionIndex])...)

				return slots, false
			case len(chainSlots) == 1:
				slots = append(slots, statementBuilderOrderSlot(statement, chainSlots[0]))
			default:
				fixable = false

				slots = append(slots, regroupBuilderOrderSlots(chainSlots)...)
			}

			continue
		}

		if statementReferencesVariable(pass, statement, variable) {
			break
		}
	}

	return slots, fixable
}

// fluentChainSegments walks a fluent selector chain from its outermost call
// inward, returning every *conga.App[T] method call in source order together
// with the chain's root expression: the conga.New call, the app variable, or
// whichever inner expression ends the chain.
func fluentChainSegments(pass *analysis.Pass, outermost ast.Expr, ranks map[string]int) ([]builderOrderSlot, ast.Expr) {
	var reversed []builderOrderSlot

	current := outermost

	for {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			break
		}

		selector, ok := methodSelector(call.Fun)
		if !ok {
			break
		}

		method, ok := pass.TypesInfo.Uses[selector.Sel].(*types.Func)
		if !ok || !isCongaAppMethod(method) {
			break
		}

		name := selector.Sel.Name

		reversed = append(reversed, builderOrderSlot{
			name:           name,
			start:          selector.X.End(),
			end:            call.End(),
			argumentStart:  call.Lparen,
			argumentEnd:    call.Rparen,
			reportPosition: selector.Sel.Pos(),
			rank:           ranks[name],
			barrier:        isBuilderOrderBarrier(name),
			group:          builderOrderChainGroup,
		})

		current = selector.X
	}

	slices.Reverse(reversed)

	return reversed, current
}

// chainEndNode climbs from a conga.New call to the outermost node of its
// fluent selector chain, unwrapping generic instantiations and parentheses.
func chainEndNode(parents map[ast.Node]ast.Node, call *ast.CallExpr) ast.Expr {
	var current ast.Expr = call

	for {
		parent, ok := parents[current]
		if !ok {
			return current
		}

		switch parentNode := parent.(type) {
		case *ast.SelectorExpr:
			if parentNode.X != current {
				return current
			}

			current = parentNode
		case *ast.IndexExpr:
			if parentNode.X != current {
				return current
			}

			current = parentNode
		case *ast.IndexListExpr:
			if parentNode.X != current {
				return current
			}

			current = parentNode
		case *ast.ParenExpr:
			current = parentNode
		case *ast.CallExpr:
			if parentNode.Fun != current {
				return current
			}

			current = parentNode
		default:
			return current
		}
	}
}

// boundAppVariable resolves the app variable an assignment binds to the
// construction chain, along with its enclosing block and statement index.
func boundAppVariable(pass *analysis.Pass, parents map[ast.Node]ast.Node, chainEnd ast.Expr) (*types.Var, *ast.BlockStmt, int) {
	assignment, ok := parents[chainEnd].(*ast.AssignStmt)
	if !ok || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || assignment.Rhs[0] != chainEnd {
		return nil, nil, -1
	}

	identifier, ok := assignment.Lhs[0].(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return nil, nil, -1
	}

	object := pass.TypesInfo.Defs[identifier]
	if object == nil {
		object = pass.TypesInfo.Uses[identifier]
	}

	variable, ok := object.(*types.Var)
	if !ok {
		return nil, nil, -1
	}

	block, ok := parents[assignment].(*ast.BlockStmt)
	if !ok {
		return nil, nil, -1
	}

	index := slices.IndexFunc(block.List, func(statement ast.Stmt) bool {
		return statement == assignment
	})
	if index < 0 {
		return nil, nil, -1
	}

	return variable, block, index
}

// methodSelector unwraps the callee selector of a method call, covering direct
// calls and explicit generic instantiations such as app.RegisterType[T](...).
func methodSelector(function ast.Expr) (*ast.SelectorExpr, bool) {
	switch expression := function.(type) {
	case *ast.SelectorExpr:
		return expression, true
	case *ast.IndexExpr:
		selector, ok := expression.X.(*ast.SelectorExpr)

		return selector, ok
	case *ast.IndexListExpr:
		selector, ok := expression.X.(*ast.SelectorExpr)

		return selector, ok
	default:
		return nil, false
	}
}

// statementExpression returns the expression of an expression statement, or
// nil for every other statement shape.
func statementExpression(statement ast.Stmt) ast.Expr {
	expressionStatement, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil
	}

	return expressionStatement.X
}

// statementReferencesVariable reports whether the statement mentions the app
// variable anywhere, marking the end of the builder statement run.
func statementReferencesVariable(pass *analysis.Pass, statement ast.Stmt, variable *types.Var) bool {
	found := false

	ast.Inspect(statement, func(node ast.Node) bool {
		if found {
			return false
		}

		identifier, ok := node.(*ast.Ident)
		if ok && pass.TypesInfo.Uses[identifier] == variable {
			found = true

			return false
		}

		return true
	})

	return found
}

// isAppVariableExpression reports whether the expression is exactly the app
// variable bound by the construction assignment.
func isAppVariableExpression(pass *analysis.Pass, expression ast.Expr, variable *types.Var) bool {
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}

	return pass.TypesInfo.Uses[identifier] == variable
}

// regroupBuilderOrderSlots retags chain slots as statement-group calls for
// statements whose text cannot be permuted as a whole.
func regroupBuilderOrderSlots(slots []builderOrderSlot) []builderOrderSlot {
	regrouped := make([]builderOrderSlot, 0, len(slots))

	for _, slot := range slots {
		slot.group = builderOrderStatementGroup
		regrouped = append(regrouped, slot)
	}

	return regrouped
}

// statementBuilderOrderSlot turns a single-call statement into a swappable
// statement slot: the whole statement text moves as one unit.
func statementBuilderOrderSlot(statement ast.Stmt, call builderOrderSlot) builderOrderSlot {
	return builderOrderSlot{
		name:           call.name,
		start:          statement.Pos(),
		end:            statement.End(),
		argumentStart:  call.argumentStart,
		argumentEnd:    call.argumentEnd,
		reportPosition: statement.Pos(),
		rank:           call.rank,
		barrier:        call.barrier,
		group:          builderOrderStatementGroup,
	}
}

// nodeParents maps every node in the file to its immediate parent, enabling
// upward navigation of fluent chains and statement context.
func nodeParents(file *ast.File) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)

	var stack []ast.Node

	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]

			return false
		}

		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}

		stack = append(stack, node)

		return true
	})

	return parents
}

// pinBuilderOrderCommentRegions disables auto-reordering for groups whose text
// range is touched by comments that a permutation would misattach.
func pinBuilderOrderCommentRegions(fileSet *token.FileSet, file *ast.File, sites []builderOrderSite) {
	for index := range sites {
		for group := range builderOrderGroupCount {
			if !sites[index].groupFixable[group] {
				continue
			}

			if builderOrderGroupHasCommentConflict(fileSet, file, sites[index].groupSlots(group)) {
				sites[index].groupFixable[group] = false
			}
		}
	}
}

// builderOrderGroupHasCommentConflict reports whether permuting the group's
// slots would misattach a comment: comments inside a call's argument list
// travel with the call text, every other comment touching the group's text
// range pins the group in place.
func builderOrderGroupHasCommentConflict(fileSet *token.FileSet, file *ast.File, slots []builderOrderSlot) bool {
	if len(slots) == 0 {
		return false
	}

	first := slots[0].start
	last := slots[len(slots)-1].end

	for _, commentGroup := range file.Comments {
		for _, comment := range commentGroup.List {
			if builderOrderCommentInsideArguments(comment, slots) {
				continue
			}

			if comment.Pos() >= first && comment.End() <= last {
				return true
			}

			if comment.Pos() > last && sameBuilderOrderLine(fileSet, comment.Pos(), last) {
				return true
			}

			if comment.End() < first && sameBuilderOrderLine(fileSet, comment.End(), first) {
				return true
			}
		}
	}

	return false
}

// builderOrderCommentInsideArguments reports whether a comment sits inside a
// slot's argument list, where a text swap carries it along safely.
func builderOrderCommentInsideArguments(comment *ast.Comment, slots []builderOrderSlot) bool {
	for _, slot := range slots {
		if comment.Pos() >= slot.argumentStart && comment.End() <= slot.argumentEnd {
			return true
		}
	}

	return false
}

// sameBuilderOrderLine reports whether both positions sit on the same line.
func sameBuilderOrderLine(fileSet *token.FileSet, first token.Pos, second token.Pos) bool {
	return fileSet.Position(first).Line == fileSet.Position(second).Line
}

// builderOrderFileEdits plans the text swaps for every violated group and
// converts them into suggested-fix edits over the file's original bytes.
func builderOrderFileEdits(pass *analysis.Pass, file *ast.File, sites []builderOrderSite) ([]analysis.TextEdit, bool) {
	tokenFile := pass.Fset.File(file.FileStart)
	if tokenFile == nil {
		return nil, false
	}

	source, ok := builderOrderSource(pass, tokenFile.Name())
	if !ok {
		return nil, false
	}

	var edits []analysis.TextEdit

	for index := range sites {
		for group := range builderOrderGroupCount {
			for _, swap := range builderOrderGroupSwaps(sites[index].groupSlots(group)) {
				edits = append(edits, analysis.TextEdit{
					Pos:     swap.slot.start,
					End:     swap.slot.end,
					NewText: source[tokenFile.Offset(swap.source.start):tokenFile.Offset(swap.source.end)],
				})
			}
		}
	}

	return edits, true
}

// builderOrderSource reads the analyzed file's bytes for fix construction,
// preferring the driver-provided reader over direct filesystem access.
func builderOrderSource(pass *analysis.Pass, filename string) ([]byte, bool) {
	if pass.ReadFile != nil {
		source, err := pass.ReadFile(filename)
		if err == nil {
			return source, true
		}
	}

	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, false
	}

	return source, true
}

// builderOrderGroupSwaps plans the text swaps that restore the configured
// order within one group: barrier calls pin their position, so participants
// only permute within the partition delimited by barriers.
func builderOrderGroupSwaps(slots []builderOrderSlot) []builderOrderTextSwap {
	var swaps []builderOrderTextSwap

	for _, partition := range builderOrderPartitions(slots) {
		var participants []builderOrderSlot

		for _, slot := range partition {
			if slot.swappable() {
				participants = append(participants, slot)
			}
		}

		sorted := slices.Clone(participants)

		slices.SortStableFunc(sorted, func(first builderOrderSlot, second builderOrderSlot) int {
			return cmp.Compare(first.rank, second.rank)
		})

		for index, slot := range participants {
			if slot.start == sorted[index].start {
				continue
			}

			swaps = append(swaps, builderOrderTextSwap{slot: slot, source: sorted[index]})
		}
	}

	return swaps
}

// builderOrderPartitions splits a group's slots at barrier calls, which pin
// their position and cannot be crossed by a reorder.
func builderOrderPartitions(slots []builderOrderSlot) [][]builderOrderSlot {
	var partitions [][]builderOrderSlot

	start := 0

	for index, slot := range slots {
		if slot.barrier {
			if index > start {
				partitions = append(partitions, slots[start:index])
			}

			start = index + 1
		}
	}

	if start < len(slots) {
		partitions = append(partitions, slots[start:])
	}

	return partitions
}

// builderOrderMessage renders the aggregated order violations, naming the
// first offender and counting the rest.
func builderOrderMessage(violations []builderOrderViolation, fixable bool) string {
	first := violations[0]

	message := fmt.Sprintf(
		"builder call order violation: %s (rank %d) called after %s (rank %d)",
		first.later.name,
		first.later.rank,
		first.earlier.name,
		first.earlier.rank,
	)

	if remaining := len(violations) - 1; remaining == 1 {
		message += "; 1 more violation"
	} else if remaining > 1 {
		message += fmt.Sprintf("; %d more violations", remaining)
	}

	if fixable {
		return message + "; use -fix to reorder"
	}

	return message + "; reorder manually (not auto-fixable)"
}

// activeBuilderCallOrder resolves the effective method order for a pass: the
// -builder-order-spec override when set, the compiled-in default otherwise.
func activeBuilderCallOrder(pass *analysis.Pass) ([]string, error) {
	registered := pass.Analyzer.Flags.Lookup("builder-order-spec")
	if registered == nil {
		return defaultBuilderCallOrder(), nil
	}

	return parseBuilderCallOrder(registered.Value.String())
}

// builderOrderRanks indexes a configured method order, mapping each method name
// to its 1-based rank; methods absent from the order stay unconstrained.
func builderOrderRanks(order []string) map[string]int {
	ranks := make(map[string]int, len(order))

	for index, name := range order {
		ranks[name] = index + 1
	}

	return ranks
}

// parseBuilderCallOrder parses a -builder-order-spec value: a comma-, newline-,
// or whitespace-separated method list, or "@path" to read one method name per
// line from a file (# comments and blank lines are ignored). An empty value
// selects the compiled-in default order; names must be known App methods and
// unique.
func parseBuilderCallOrder(specValue string) ([]string, error) {
	trimmed := strings.TrimSpace(specValue)
	if trimmed == "" {
		return defaultBuilderCallOrder(), nil
	}

	var names []string

	if filename, isFile := strings.CutPrefix(trimmed, "@"); isFile {
		content, err := os.ReadFile(strings.TrimSpace(filename))
		if err != nil {
			return nil, fmt.Errorf("%w: reading order file: %w", errBuilderOrderSpec, err)
		}

		names = extractBuilderOrderNames(string(content))
	} else {
		names = extractBuilderOrderNames(trimmed)
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("%w: no method names listed", errBuilderOrderSpec)
	}

	known := defaultBuilderCallOrder()
	seen := make(map[string]bool, len(names))

	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("%w: duplicate App method %q", errBuilderOrderSpec, name)
		}

		seen[name] = true

		if !slices.Contains(known, name) {
			return nil, fmt.Errorf("%w: unknown App method %q (known methods: %s)", errBuilderOrderSpec, name, strings.Join(known, ", "))
		}
	}

	return names, nil
}

// extractBuilderOrderNames splits a spec body into method names: entries are
// separated by commas, whitespace, or newlines, and "#" starts a comment that
// runs to the end of the line.
func extractBuilderOrderNames(specBody string) []string {
	var names []string

	for line := range strings.SplitSeq(specBody, "\n") {
		if commentIndex := strings.IndexByte(line, '#'); commentIndex >= 0 {
			line = line[:commentIndex]
		}

		for entry := range strings.SplitSeq(line, ",") {
			for name := range strings.FieldsSeq(entry) {
				names = append(names, name)
			}
		}
	}

	return names
}

// defaultBuilderCallOrder returns the canonical builder call order for the
// *conga.App[T] fluent API: identity and documentation first, the
// Enable*/Disable* toggles alphabetically, behaviour calls grouped by concern,
// and the execution methods last.
func defaultBuilderCallOrder() []string {
	return []string{
		"Name",
		"Description",
		"Version",
		"Examples",
		"HelpFooter",
		"Deprecated",
		"DisableHelpFlag",
		"DisableVersionFlag",
		"EnableCompletion",
		"EnableFeatures",
		"EnableWhy",
		"EnvPrefix",
		"Dotenv",
		"RegisterDecoder",
		"ConfigOptions",
		"AllowTags",
		"Predictor",
		"RegisterType",
		"RegisterMap",
		"Use",
		"UsageOnError",
		"Args",
		"Stdin",
		"Stdout",
		"Stderr",
		"SetenvFunc",
		"ExitFunc",
		"Run",
		"RunContext",
		"Exit",
	}
}

// isBuilderOrderBarrier reports whether calling the method executes user code
// (binding functions), pinning the call in place for automatic reordering.
func isBuilderOrderBarrier(name string) bool {
	return name == "Use"
}

// isBuilderOrderExecution reports whether the method executes the app; the
// check stops at the first execution call.
func isBuilderOrderExecution(name string) bool {
	switch name {
	case "Run", "RunContext", "Exit":
		return true
	default:
		return false
	}
}
