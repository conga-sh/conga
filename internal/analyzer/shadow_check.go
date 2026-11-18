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
func (w *walker) checkAndCollectFlag(node *commandNode, fieldObject types.Object, tag reflect.StructTag, flagName string, branchNames map[string]shadowOwner, branchShorts map[rune]shadowOwner) {
	shortRune := spec.ParseFlagShort(tag)
	position := w.fieldReportPos(node, fieldObject)
	declared := flagOwner(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, fieldObject, position, declared, owner, "--"+flagName)
	}

	for _, alias := range splitAliases(tag) {
		if owner, ok := branchNames[alias]; ok {
			w.reportFlagShadow(node, fieldObject, position, declared, owner, "alias --"+alias)
		}
	}

	if shortRune != 0 {
		if owner, ok := branchShorts[shortRune]; ok {
			w.reportFlagShadow(node, fieldObject, position, declared, owner, fmt.Sprintf("short flag -%c", shortRune))
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
func (w *walker) checkAndCollectSynthesizedFlag(node *commandNode, fieldObject types.Object, flagName string, branchNames map[string]shadowOwner) {
	position := w.fieldReportPos(node, fieldObject)
	declared := flagOwner(node, fieldObject)

	if owner, ok := branchNames[flagName]; ok {
		w.reportFlagShadow(node, fieldObject, position, declared, owner, "--"+flagName)
	}

	branchNames[flagName] = declared
}

// checkVersionJSONShadows reports the synthesized --json flag of each
// VersionCommand field on node colliding with an ancestor flag. The runtime
// validates ancestor shadowing after the whole command tree is built, so this
// check runs after node's field loop: the declaration order of the version
// field and the colliding ancestor flag does not decide the diagnostic.
func (w *walker) checkVersionJSONShadows(node *commandNode, branchNames map[string]shadowOwner) {
	for _, versionField := range node.versionJSONFields {
		owner, ok := branchNames[spec.VersionJSONFlagName]
		if !ok {
			continue
		}

		position := w.fieldReportPos(node, versionField)

		w.report(position, spec.VersionJSONShadowError(flagOwner(node, versionField).FlagOwner, owner.FlagOwner).Error())
	}
}

// shadowOwner is a flag declaration in the shadow maps: the struct and field
// the message names, and the command node that declares it, so a same-command
// duplicate is told apart from an ancestor by node, never by command name.
type shadowOwner struct {
	spec.FlagOwner

	node *commandNode
}

// reportFlagShadow reports a flag-shadow collision and marks the command
// rejected when the conflicting declaration belongs to the command itself: the
// runtime's per-command flag tracker rejects a same-command duplicate inside
// the field loop, before the group census runs, so census diagnostics would be
// unreachable. An ancestor collision is validated only after the whole command
// tree is built, so descendant census diagnostics stay reportable.
func (w *walker) reportFlagShadow(node *commandNode, fieldObject types.Object, position token.Pos, declared, owner shadowOwner, declaration string) {
	if owner.node == node {
		w.reportFieldError(node, fieldObject, spec.DuplicateFlagError(declaration, owner.Field))

		return
	}

	w.report(position, spec.AncestorShadowError(declared.FlagOwner, declaration, owner.FlagOwner).Error())
}

// flagOwner names the declaration of a flag field on node for shadowing
// diagnostics, in the shape the runtime records on spec.Flag.
func flagOwner(node *commandNode, fieldObject types.Object) shadowOwner {
	return shadowOwner{
		FlagOwner: spec.FlagOwner{Struct: node.named.Obj().Name(), Field: fieldObject.Name()},
		node:      node,
	}
}

func splitAliases(tag reflect.StructTag) []string {
	if tag.Get("aliases") == "" {
		return nil
	}

	return spec.SplitTagList(tag, "aliases")
}
