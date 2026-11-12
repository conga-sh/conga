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
func (w *walker) checkAndCollectFlag(node *commandNode, fieldObject types.Object, tag reflect.StructTag, flagName string, branchNames map[string]spec.FlagOwner, branchShorts map[rune]spec.FlagOwner) {
	shortRune := spec.ParseFlagShort(tag)
	position := w.fieldReportPos(node, fieldObject)
	declared := flagOwner(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, position, declared, owner, "--"+flagName)
	}

	for _, alias := range splitAliases(tag) {
		if owner, ok := branchNames[alias]; ok {
			w.reportFlagShadow(node, position, declared, owner, "alias --"+alias)
		}
	}

	if shortRune != 0 {
		if owner, ok := branchShorts[shortRune]; ok {
			w.reportFlagShadow(node, position, declared, owner, fmt.Sprintf("short flag -%c", shortRune))
		}
	}

	branchNames[flagName] = declared
	for _, alias := range splitAliases(tag) {
		branchNames[alias] = declared
	}

	if shortRune != 0 {
		branchShorts[shortRune] = declared
	}
}

// checkAndCollectSynthesizedFlag folds the name of a synthesized, CLI-exposed
// flag (the Features container's flag tag) into the branch shadow map. The
// engine installs no aliases or short rune for the synthesized flag, so only
// the name participates in ancestor-shadow and sibling-collision reporting.
func (w *walker) checkAndCollectSynthesizedFlag(node *commandNode, fieldObject types.Object, flagName string, branchNames map[string]spec.FlagOwner) {
	position := w.fieldReportPos(node, fieldObject)
	declared := flagOwner(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, position, declared, owner, "--"+flagName)
	}

	branchNames[flagName] = declared
}

// checkVersionJSONShadows reports the synthesized --json flag of each
// VersionCommand field on node colliding with an ancestor flag. The runtime
// validates ancestor shadowing after the whole command tree is built, so this
// check runs after node's field loop: the declaration order of the version
// field and the colliding ancestor flag does not decide the diagnostic.
func (w *walker) checkVersionJSONShadows(node *commandNode, branchNames map[string]spec.FlagOwner) {
	for _, versionField := range node.versionJSONFields {
		owner, ok := branchNames[spec.VersionJSONFlagName]
		if !ok {
			continue
		}

		position := w.fieldReportPos(node, versionField.fieldObject)

		w.report(position, spec.VersionJSONShadowError(flagOwner(node, versionField.fieldObject), owner).Error())
	}
}

// reportFlagShadow reports a flag-shadow collision and marks the command
// rejected when the conflicting declaration belongs to the command itself: the
// runtime's per-command flag tracker rejects a same-command duplicate inside
// the field loop, before the group census runs, so census diagnostics would be
// unreachable. An ancestor collision is validated only after the whole command
// tree is built, so descendant census diagnostics stay reportable. An ancestor
// sharing the exact command name degrades to suppression, which the analyzer
// contract permits: it may miss diagnostics, never invent them.
func (w *walker) reportFlagShadow(node *commandNode, position token.Pos, declared, owner spec.FlagOwner, declaration string) {
	if owner.Command == node.name {
		node.rejected = true

		w.report(position, fmt.Sprintf("command %q: %s: %s (already declared by another field on this command)", node.name, spec.ErrDuplicateFlag, declaration))

		return
	}

	w.report(position, spec.AncestorShadowError(declared, declaration, owner).Error())
}

// flagOwner names the declaration of a flag field on node for shadowing
// diagnostics, in the shape the runtime records on spec.Flag.
func flagOwner(node *commandNode, fieldObject types.Object) spec.FlagOwner {
	return spec.FlagOwner{Command: node.name, Struct: node.named.Obj().Name(), Field: fieldObject.Name()}
}

func splitAliases(tag reflect.StructTag) []string {
	if tag.Get("aliases") == "" {
		return nil
	}

	return spec.SplitTagList(tag, "aliases")
}
