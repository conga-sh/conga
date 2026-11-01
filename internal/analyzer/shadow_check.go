package analyzer

import (
	"fmt"
	"go/token"
	"go/types"
	"reflect"

	"github.com/conga-sh/conga/internal/spec"
)

// checkAndCollectFlag mirrors the runtime ancestor shadowing contract: the
// flag's name, aliases, and short rune are checked against every ancestor
// declaration, then folded into the branch maps for descendants.
func (w *walker) checkAndCollectFlag(node *commandNode, fieldObject types.Object, tag reflect.StructTag, flagName string, branchNames map[string]string, branchShorts map[rune]string) {
	shortRune := spec.ParseFlagShort(tag)
	position := w.fieldReportPos(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, position, owner, "--"+flagName)
	}

	for _, alias := range splitAliases(tag) {
		if owner, ok := branchNames[alias]; ok {
			w.reportFlagShadow(node, position, owner, "alias --"+alias)
		}
	}

	if shortRune != 0 {
		if owner, ok := branchShorts[shortRune]; ok {
			w.reportFlagShadow(node, position, owner, fmt.Sprintf("short flag -%c", shortRune))
		}
	}

	branchNames[flagName] = node.name
	for _, alias := range splitAliases(tag) {
		branchNames[alias] = node.name
	}

	if shortRune != 0 {
		branchShorts[shortRune] = node.name
	}
}

// checkAndCollectSynthesizedFlag folds the name of a synthesized, CLI-exposed
// flag (the Features container's flag tag) into the branch shadow map. The
// engine installs no aliases or short rune for the synthesized flag, so only
// the name participates in ancestor-shadow and sibling-collision reporting.
func (w *walker) checkAndCollectSynthesizedFlag(node *commandNode, fieldObject types.Object, flagName string, branchNames map[string]string) {
	position := w.fieldReportPos(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, position, owner, "--"+flagName)
	}

	branchNames[flagName] = node.name
}

// reportFlagShadow reports a flag-shadow collision and marks the command
// rejected when the conflicting declaration belongs to the command itself: the
// runtime's per-command flag tracker rejects a same-command duplicate inside
// the field loop, before the group census runs, so census diagnostics would be
// unreachable. An ancestor collision is validated only after the whole command
// tree is built, so descendant census diagnostics stay reportable. An ancestor
// sharing the exact command name degrades to suppression, which the analyzer
// contract permits: it may miss diagnostics, never invent them.
func (w *walker) reportFlagShadow(node *commandNode, position token.Pos, owner, declaration string) {
	location := fmt.Sprintf("ancestor command %q", owner)

	if owner == node.name {
		node.rejected = true
		location = "another field on this command"
	}

	w.report(position, fmt.Sprintf("command %q: %s: %s (already declared by %s)", node.name, spec.ErrDuplicateFlag, declaration, location))
}

func splitAliases(tag reflect.StructTag) []string {
	if tag.Get("aliases") == "" {
		return nil
	}

	return spec.SplitTagList(tag, "aliases")
}
