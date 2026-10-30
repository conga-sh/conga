package analyzer

import (
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/conga-sh/conga/internal/spec"
)

// checkFeatureEnvSurfaceConflicts delegates to the shared runtime validator.
// Both diagnostics report at the Features container field, matching the
// runtime's field-context wrapping. The result reports whether a conflict was
// found, so a broken declaration records no collision bindings (the runtime
// aborts before the collision validator runs).
func (w *walker) checkFeatureEnvSurfaceConflicts(node *commandNode, fieldObject types.Object, tag reflect.StructTag, envDeclarer string) bool {
	if err := spec.ValidateFeatureEnvSurfaces(tag, envDeclarer); err != nil {
		w.reportFieldError(node, fieldObject, err)

		return true
	}

	return false
}

// recordFeatureEnvBindings resolves the Features container's environment
// surface the way the runtime schema builder does and records it for the env
// collision check: explicit env names bind the combined map variable, and
// each feature field resolves an explicit env tag or a name derived from the
// container's envprefix.
func (w *walker) recordFeatureEnvBindings(node *commandNode, fieldObject types.Object, tag reflect.StructTag, structType *types.Struct, flagName string) {
	w.featureEnvNode = node
	w.featureEnvFieldObject = fieldObject

	if envVars, envExcluded := spec.ParseFeatureContainerEnv(tag); !envExcluded {
		for _, envVar := range envVars {
			w.featureEnvBindings = append(w.featureEnvBindings, envBinding{
				envVar: envVar,
				owner:  "the feature map flag --" + flagName,
			})
		}
	}

	envPrefix := strings.TrimSpace(tag.Get("envprefix"))

	for index := range structType.NumFields() {
		innerField := structType.Field(index)
		if !innerField.Exported() {
			continue
		}

		innerTag := reflect.StructTag(structType.Tag(index))

		name, err := spec.FeatureFieldName(innerField.Name(), innerTag)
		if err != nil {
			continue
		}

		envVars, _ := spec.ResolveFlagEnvVars(innerTag, name, envPrefix)
		for _, envVar := range envVars {
			w.featureEnvBindings = append(w.featureEnvBindings, envBinding{
				envVar: envVar,
				owner:  "feature " + strconv.Quote(name),
			})
		}
	}
}

// recordFlagEnvOwners resolves a flag field's environment variable names the
// way the runtime schema builder does — explicit env tag names, else a name
// derived from the command's env prefix (config-file flags derive
// <PREFIX>CONFIG) — and records them for the feature env collision check.
// Derived names are skipped when the effective prefix is unknown, so the
// check may only miss collisions, never invent them.
func (w *walker) recordFlagEnvOwners(node *commandNode, tag reflect.StructTag, flagName string, configFile bool) {
	effectivePrefix := ""
	if node.envPrefixKnown {
		effectivePrefix = node.envPrefix
	}

	derivedName := flagName
	if configFile {
		derivedName = "config"
	}

	envVars, excluded := spec.ResolveFlagEnvVars(tag, derivedName, effectivePrefix)
	if excluded {
		return
	}

	for _, envVar := range envVars {
		w.recordFlagEnvOwner(envVar, flagName)
	}
}

func (w *walker) recordFlagEnvOwner(envVar string, flagName string) {
	w.flagEnvBindings = append(w.flagEnvBindings, envBinding{
		envVar: envVar,
		owner:  "flag --" + flagName,
	})
}

// checkFeatureEnvCollisions mirrors the runtime feature env uniqueness
// validator after the whole tree is walked: feature-to-feature duplicates
// report first, then feature-to-flag collisions in flag declaration order.
// Like the runtime, the first collision found is reported and the check stops.
func (w *walker) checkFeatureEnvCollisions() {
	if w.featureEnvNode == nil || len(w.featureEnvBindings) == 0 {
		return
	}

	featureOwners := make(map[string]string, len(w.featureEnvBindings))

	for _, binding := range w.featureEnvBindings {
		if previous, ok := featureOwners[binding.envVar]; ok {
			w.reportFieldError(w.featureEnvNode, w.featureEnvFieldObject, spec.FeatureEnvCollisionError(binding.envVar, previous, binding.owner))

			return
		}

		featureOwners[binding.envVar] = binding.owner
	}

	for _, binding := range w.flagEnvBindings {
		if featureOwner, ok := featureOwners[binding.envVar]; ok {
			w.reportFieldError(w.featureEnvNode, w.featureEnvFieldObject, spec.FeatureEnvCollisionError(binding.envVar, featureOwner, binding.owner))

			return
		}
	}
}
