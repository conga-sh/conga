package analyzer

import (
	"go/token"
	"go/types"
	"maps"
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"
)

// wrapperContainerContracts maps each wrapper type the analyzer classifies to
// the engine classification kind its containerKind marker declares and the
// unexported engine state methods the binding helpers rely on. The engine owns
// container behavior, so the contract is pinned by method presence and
// signature rather than by interface implementation. The key set must stay
// identical to congaWrapperRoles.
//
//nolint:gochecknoglobals // the contract table is inherently a package-level fixture
var wrapperContainerContracts = map[string]struct {
	kind         string
	stateMethods []string
}{
	"Command":           {kind: "KindCommand", stateMethods: []string{"innerValue", "markSelected"}},
	"HelpCommand":       {kind: "KindHelpCommand"},
	"VersionCommand":    {kind: "KindVersionCommand"},
	"CompletionCommand": {kind: "KindCompletionCommand"},
	"ConfigFile":        {kind: "KindConfigFile", stateMethods: []string{"getValue", "setValue"}},
	"Features":          {kind: "KindFeatures", stateMethods: []string{"getValue", "setValue", "featureProvided", "applyFeatures", "enforceCLIRestrictions", "enforceConfigRestrictions", "prepareForCompletion"}},
	"DashArgs":          {kind: "KindDashArgs", stateMethods: []string{"setArgs"}},
	"RawArgs":           {kind: "KindRawArgs", stateMethods: []string{"setArgs"}},
	"OptionalFlag":      {kind: "KindFlag", stateMethods: []string{"getValue", "setValue"}},
	"RequiredFlag":      {kind: "KindFlag", stateMethods: []string{"getValue", "setValue"}},
	"Counter":           {kind: "KindCounter", stateMethods: []string{"getValue", "setValue"}},
	"OptionalArg":       {kind: "KindArg", stateMethods: []string{"getValue", "setValue"}},
	"RequiredArg":       {kind: "KindArg", stateMethods: []string{"getValue", "setValue"}},
}

// TestWrapperContract pins the analyzer's identity-based classification to the
// engine's zero-reflection classifier contract: every classified conga wrapper
// must expose the unexported containerKind marker returning the engine's
// Classification plus the engine state methods its binding path needs, the
// classification table must stay identical to congaWrapperRoles, no other
// exported conga type may declare the marker, and the deleted adapter
// interfaces must not reappear in internal/container. A failure here means the
// classification table or the runtime classifier drifted and the analyzer
// would silently misclassify command struct fields.
func TestWrapperContract(t *testing.T) {
	t.Parallel()

	loaded, err := packages.Load(&packages.Config{
		Mode:  packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
		Tests: false,
	}, congaPackagePath, congaPackagePath+"/internal/container")
	if err != nil {
		t.Fatalf("loading runtime packages: %v", err)
	}

	runtimePackages := make(map[string]*types.Package)

	for _, loadedPackage := range loaded {
		if len(loadedPackage.Errors) > 0 {
			t.Fatalf("loading %s: %v", loadedPackage.PkgPath, loadedPackage.Errors)
		}

		if loadedPackage.Types != nil {
			runtimePackages[loadedPackage.PkgPath] = loadedPackage.Types
		}
	}

	congaPackage := runtimePackages[congaPackagePath]
	containerPackage := runtimePackages[congaPackagePath+"/internal/container"]

	if congaPackage == nil || containerPackage == nil {
		t.Fatal("conga runtime packages missing from load result")
	}

	classificationType := containerNamedType(t, containerPackage, "Classification")
	if _, ok := classificationType.Underlying().(*types.Struct); !ok {
		t.Fatalf("container.Classification is %s, want a struct", classificationType.Underlying())
	}

	assertClassificationFields(t, classificationType)

	classifiedWrappers := slices.Sorted(maps.Keys(congaWrapperRoles))
	contractWrappers := slices.Sorted(maps.Keys(wrapperContainerContracts))

	if !slices.Equal(classifiedWrappers, contractWrappers) {
		t.Fatalf("classification table drifted from the contract table:\nclassified: %v\ncontract:   %v", classifiedWrappers, contractWrappers)
	}

	for wrapperName, contract := range wrapperContainerContracts {
		wrapper := congaPackage.Scope().Lookup(wrapperName)

		named, ok := wrapper.Type().(*types.Named)
		if !ok {
			t.Fatalf("conga.%s is %s, want a named type", wrapperName, wrapper.Type())
		}

		containerNamedType(t, containerPackage, contract.kind)

		addressable := addressableWrapperType(t, named)

		assertMarkerMethod(t, wrapperName, addressable, classificationType)

		for _, stateMethod := range contract.stateMethods {
			if methodByName(addressable, stateMethod) == nil {
				t.Errorf("conga.%s has no %s() engine state method; update the contract table and the runtime classifier", wrapperName, stateMethod)
			}
		}
	}

	assertNoUnclassifiedMarkers(t, congaPackage)

	for _, removed := range []string{
		"Flag", "ConfigFile", "Counter", "Arg", "Command",
		"DashArgs", "RawArgs", "HelpCommand", "VersionCommand", "CompletionCommand",
	} {
		if containerPackage.Scope().Lookup(removed) != nil {
			t.Errorf("container.%s must stay deleted; the engine observes containers through plain data and opaque handles", removed)
		}
	}
}

// assertClassificationFields pins the engine classification payload the schema
// builder consumes.
func assertClassificationFields(t *testing.T, classificationType *types.Named) {
	t.Helper()

	structType, ok := classificationType.Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("container.Classification is %s, want a struct", classificationType.Underlying())
	}

	for _, fieldName := range []string{"Kind", "Required", "Target"} {
		found := false

		for field := range structType.Fields() {
			if field.Name() == fieldName {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("container.Classification must keep the %s field", fieldName)
		}
	}
}

// assertMarkerMethod asserts the wrapper exposes containerKind() Classification.
func assertMarkerMethod(t *testing.T, wrapperName string, addressable types.Type, classificationType *types.Named) {
	t.Helper()

	marker := methodByName(addressable, "containerKind")
	if marker == nil {
		t.Errorf("conga.%s has no containerKind() marker; update the contract table and the runtime classifier", wrapperName)

		return
	}

	signature, ok := marker.Type().(*types.Signature)
	if !ok || signature.Params().Len() != 0 || signature.Results().Len() != 1 {
		t.Errorf("conga.%s.containerKind must take no parameters and return exactly one value", wrapperName)

		return
	}

	if !types.Identical(signature.Results().At(0).Type(), classificationType) {
		t.Errorf("conga.%s.containerKind must return container.Classification, got %s", wrapperName, signature.Results().At(0).Type())
	}
}

// assertNoUnclassifiedMarkers asserts the unexported marker seals the wrapper
// set: no other exported conga type may declare containerKind.
func assertNoUnclassifiedMarkers(t *testing.T, congaPackage *types.Package) {
	t.Helper()

	for _, scopeName := range congaPackage.Scope().Names() {
		if !token.IsExported(scopeName) || congaWrapperRoles[scopeName] != "" {
			continue
		}

		named, ok := congaPackage.Scope().Lookup(scopeName).Type().(*types.Named)
		if !ok {
			continue
		}

		if methodByName(addressableWrapperType(t, named), "containerKind") != nil {
			t.Errorf("conga.%s declares containerKind but is not classified; add it to congaWrapperRoles and checkField", scopeName)
		}
	}
}

// methodByName resolves an unexported method by name on a type.
func methodByName(targetType types.Type, methodName string) *types.Func {
	for method := range types.NewMethodSet(targetType).Methods() {
		function, ok := method.Obj().(*types.Func)
		if ok && function.Name() == methodName {
			return function
		}
	}

	return nil
}

// containerNamedType resolves a named declaration from the internal/container
// runtime scope.
func containerNamedType(t *testing.T, containerPackage *types.Package, typeName string) *types.Named {
	t.Helper()

	object := containerPackage.Scope().Lookup(typeName)
	if object == nil {
		t.Fatalf("container.%s missing from the runtime scope", typeName)
	}

	named, ok := object.Type().(*types.Named)
	if !ok {
		t.Fatalf("container.%s is %s, want a named type", typeName, object.Type())
	}

	return named
}

// addressableWrapperType returns the pointer type whose method set the engine
// classifier exposes: instantiated for generic wrappers, plain otherwise.
func addressableWrapperType(t *testing.T, named *types.Named) types.Type {
	t.Helper()

	if named.TypeParams().Len() == 0 {
		return types.NewPointer(named)
	}

	typeArguments := make([]types.Type, named.TypeParams().Len())
	for index := range typeArguments {
		typeArguments[index] = types.Typ[types.String]
	}

	instantiated, err := types.Instantiate(nil, named, typeArguments, false)
	if err != nil {
		t.Fatalf("instantiating %s: %v", named, err)
	}

	return types.NewPointer(instantiated)
}
