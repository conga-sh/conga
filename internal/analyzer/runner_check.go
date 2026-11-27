package analyzer

import (
	"go/types"

	"github.com/conga-sh/conga/internal/spec"
)

const (
	embeddedHookNone embeddedHookKind = iota
	embeddedHookPointer
	embeddedHookInterface
)

type embeddedHookKind uint8

type embeddedHookOrigin struct {
	kind embeddedHookKind
	name string
}

// checkLifecycleReceivers rejects command structs implementing any lifecycle
// hook on the value type rather than the pointer type, distinguishing value
// receivers, embedded pointers, and embedded interfaces. Value-embedded mixins
// with pointer-receiver hooks only implement the hook on *T and stay allowed.
func (w *walker) checkLifecycleReceivers(node *commandNode) {
	if node.named == nil {
		return
	}

	hooks := []struct {
		iface  *types.Interface
		method string
	}{
		{iface: w.conga.runner, method: "Run"},
		{iface: w.conga.bootstrapper, method: "Bootstrap"},
		{iface: w.conga.validator, method: "Validate"},
		{iface: w.conga.preRunner, method: "PreRun"},
		{iface: w.conga.postRunner, method: "PostRun"},
		{iface: w.conga.finalizer, method: "Finally"},
	}

	position := node.reportPos
	if node.inPackage {
		position = node.named.Obj().Pos()
	}

	structName := node.named.Obj().Name()

	for _, hook := range hooks {
		if !w.implements(node.named, hook.iface) {
			continue
		}

		object, index, _ := types.LookupFieldOrMethod(node.named, false, nil, hook.method)

		method, ok := object.(*types.Func)
		if !ok {
			continue
		}

		signature, ok := method.Type().(*types.Signature)
		if !ok || signature.Recv() == nil {
			continue
		}

		receiverType := types.Unalias(signature.Recv().Type())
		hookErr := classifyLifecycleHookError(node.named, structName, hook.method, receiverType, index)

		w.reportError(position, hookErr)
	}
}

// checkRunner enforces the leaf command contract: a command struct without
// user subcommand fields must implement conga.Runner, mirroring the runtime
// bind-time check (noRunnerError) that otherwise only surfaces on first
// execution.
func (w *walker) checkRunner(node *commandNode, subcommandCount int) {
	if subcommandCount > 0 || node.named == nil {
		return
	}

	if w.implements(types.NewPointer(node.named), w.conga.runner) {
		return
	}

	position := node.reportPos
	if node.inPackage {
		position = node.named.Obj().Pos()
	}

	w.reportError(position, spec.NoRunnerError(node.named.Obj().Name()))
}

// classifyLifecycleHookError maps a value-type lifecycle hook lookup to one of
// the three shared spec error constructors. When the promotion path crosses an
// embedded pointer or interface, that error takes precedence so the user is
// not sent through a two-step hint.
func classifyLifecycleHookError(root *types.Named, structName, methodName string, receiverType types.Type, index []int) error {
	switch origin := findEmbeddedHookOrigin(root, index); origin.kind {
	case embeddedHookPointer:
		return spec.EmbeddedPointerHookError(structName, origin.name, methodName)
	case embeddedHookInterface:
		return spec.EmbeddedInterfaceHookError(structName, origin.name, methodName)
	case embeddedHookNone:
	}

	receiverName := structName
	if named, ok := receiverType.(*types.Named); ok {
		receiverName = named.Obj().Name()
	}

	return spec.ValueReceiverHookError(structName, receiverName, methodName)
}

func findEmbeddedHookOrigin(root *types.Named, index []int) embeddedHookOrigin {
	if len(index) <= 1 {
		return embeddedHookOrigin{}
	}

	var current types.Type = root

	for _, fieldIndex := range index[:len(index)-1] {
		unaliased := types.Unalias(current)
		if ptr, ok := unaliased.(*types.Pointer); ok {
			unaliased = types.Unalias(ptr.Elem())
		}

		structType, ok := unaliased.Underlying().(*types.Struct)
		if !ok || fieldIndex < 0 || fieldIndex >= structType.NumFields() {
			break
		}

		fieldType := types.Unalias(structType.Field(fieldIndex).Type())

		switch typ := fieldType.(type) {
		case *types.Pointer:
			elemName := ""
			if named, ok := types.Unalias(typ.Elem()).(*types.Named); ok {
				elemName = named.Obj().Name()
			}

			return embeddedHookOrigin{kind: embeddedHookPointer, name: elemName}
		case *types.Named:
			if _, ok := typ.Underlying().(*types.Interface); ok {
				return embeddedHookOrigin{kind: embeddedHookInterface, name: typ.Obj().Name()}
			}
		case *types.Interface:
			return embeddedHookOrigin{kind: embeddedHookInterface}
		}

		current = fieldType
	}

	return embeddedHookOrigin{}
}
