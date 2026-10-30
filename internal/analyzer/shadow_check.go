package analyzer

import (
	"fmt"
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

	if ancestorCommand, ok := branchNames[flagName]; ok {
		w.report(position, fmt.Sprintf("command %q: %s: --%s (already declared by ancestor command %q)", node.name, spec.ErrDuplicateFlag, flagName, ancestorCommand))
	}

	for _, alias := range splitAliases(tag) {
		if ancestorCommand, ok := branchNames[alias]; ok {
			w.report(position, fmt.Sprintf("command %q: %s: alias --%s (already declared by ancestor command %q)", node.name, spec.ErrDuplicateFlag, alias, ancestorCommand))
		}
	}

	if shortRune != 0 {
		if ancestorCommand, ok := branchShorts[shortRune]; ok {
			w.report(position, fmt.Sprintf("command %q: %s: short flag -%c (already declared by ancestor command %q)", node.name, spec.ErrDuplicateFlag, shortRune, ancestorCommand))
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

	if ancestorCommand, ok := branchNames[flagName]; ok {
		w.report(position, fmt.Sprintf("command %q: %s: --%s (already declared by ancestor command %q)", node.name, spec.ErrDuplicateFlag, flagName, ancestorCommand))
	}

	branchNames[flagName] = node.name
}

func splitAliases(tag reflect.StructTag) []string {
	if tag.Get("aliases") == "" {
		return nil
	}

	return spec.SplitTagList(tag, "aliases")
}
