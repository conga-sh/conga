package conga

import (
	"reflect"
	"slices"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/spec"
)

// containerSnapshot captures a bootstrap container's value so an
// observation-only completion invocation can restore it.
type containerSnapshot struct {
	handle any
	value  reflect.Value
}

func (s containerSnapshot) restore() {
	reflect.ValueOf(s.handle).Elem().Set(s.value)
}

// selectionSnapshot captures one command container's selection state.
type selectionSnapshot struct {
	handle   any
	selected bool
}

// bindBootstrapFields binds bootstrap-tagged flags, positional arguments, and
// trailing-argument collectors before the configuration loader runs. The
// returned restore function reverts the bound containers and command selection
// for completion invocations; normal executions ignore it because the full
// binding pass rewrites every value.
func (a *App[T]) bindBootstrapFields(invocation parsedInvocation, envLookup func(string) (string, bool), tolerate bool) (func(), error) {
	snapshots, err := a.bindBootstrapContainers(invocation, envLookup, tolerate)
	selections := markSelectedCommands(invocation.cmdChain)

	restore := func() {
		for _, snapshot := range slices.Backward(snapshots) {
			snapshot.restore()
		}

		restoreSelection(selections)
	}

	if err != nil {
		return restore, err
	}

	return restore, nil
}

func (a *App[T]) bindBootstrapContainers(invocation parsedInvocation, envLookup func(string) (string, bool), tolerate bool) ([]containerSnapshot, error) {
	state := &bindingState{
		EnvPrefix:       a.envPrefix,
		EnvLookup:       envLookup,
		TypeParsers:     a.collectTypeParsers(),
		CustomMapMerger: a.customMapMerger,
		Stdin:           a.stdin,
		Stdout:          a.stdout,
	}

	var snapshots []containerSnapshot

	for _, cmd := range invocation.cmdChain {
		for _, flagSpec := range cmd.Flags {
			if !flagSpec.Bootstrap || flagSpec.Handle == nil {
				continue
			}

			snapshot := snapshotContainer(flagSpec.Handle)
			snapshots = append(snapshots, snapshot)

			if err := applyFlagValue(cmd, invocation.cmdChain, flagSpec, invocation.flagValues, state); err != nil {
				snapshot.restore()

				if !tolerate {
					return snapshots, err
				}
			}
		}
	}

	var err error

	snapshots, err = a.bindBootstrapArgs(invocation, state, snapshots, tolerate)
	if err != nil {
		return snapshots, err
	}

	snapshots = bindBootstrapSpecialArgs(invocation.leaf, invocation.dashTokens, invocation.rawTokens, snapshots)

	return snapshots, nil
}

// bindBootstrapArgs maps positional tokens to the declared argument order and
// binds only bootstrap-tagged arguments. Non-bootstrap arguments still advance
// the token position so later tagged arguments resolve correctly.
func (a *App[T]) bindBootstrapArgs(invocation parsedInvocation, state *bindingState, snapshots []containerSnapshot, tolerate bool) ([]containerSnapshot, error) {
	leaf := invocation.leaf
	if leaf == nil {
		return snapshots, nil
	}

	tokenIndex := 0

	for _, arg := range leaf.Args {
		if arg.IsSlice || arg.IsMap {
			var err error

			snapshots, err = a.bindBootstrapTrailingArg(invocation, state, arg, tokenIndex, snapshots, tolerate)
			if err != nil {
				return snapshots, err
			}

			return snapshots, nil
		}

		if tokenIndex < len(invocation.posTokens) {
			var err error

			snapshots, err = a.bindBootstrapTokenArg(invocation, state, arg, tokenIndex, snapshots, tolerate)
			if err != nil {
				return snapshots, err
			}

			tokenIndex++

			continue
		}

		var err error

		snapshots, err = a.bindBootstrapDefaultArg(invocation, state, arg, tokenIndex, snapshots, tolerate)
		if err != nil {
			return snapshots, err
		}
	}

	return snapshots, nil
}

func (a *App[T]) bindBootstrapTrailingArg(invocation parsedInvocation, state *bindingState, arg *spec.Arg, tokenIndex int, snapshots []containerSnapshot, tolerate bool) ([]containerSnapshot, error) {
	if !arg.Bootstrap || arg.Handle == nil {
		return snapshots, nil
	}

	snapshot := snapshotContainer(arg.Handle)
	snapshots = append(snapshots, snapshot)

	var err error

	switch {
	case len(invocation.posTokens[tokenIndex:]) > 0:
		if arg.IsSlice {
			err = resolveSliceArg(invocation.cmdChain, arg, invocation.posTokens[tokenIndex:], state.TypeParsers, coerce.DecodeContext{Stdin: state.Stdin, Stdout: state.Stdout})
		} else {
			err = resolveMapArg(invocation.cmdChain, arg, invocation.posTokens[tokenIndex:], state.TypeParsers)
		}
	case arg.Default != "":
		if arg.IsSlice {
			err = resolveEmptySliceArg(invocation.cmdChain, arg, state.TypeParsers, coerce.DecodeContext{Stdin: state.Stdin, Stdout: state.Stdout})
		} else {
			err = resolveMapArg(invocation.cmdChain, arg, nil, state.TypeParsers)
		}
	}

	return tolerateBootstrapError(snapshots, snapshot, err, tolerate)
}

func (a *App[T]) bindBootstrapTokenArg(invocation parsedInvocation, state *bindingState, arg *spec.Arg, tokenIndex int, snapshots []containerSnapshot, tolerate bool) ([]containerSnapshot, error) {
	if !arg.Bootstrap || arg.Handle == nil {
		return snapshots, nil
	}

	snapshot := snapshotContainer(arg.Handle)
	snapshots = append(snapshots, snapshot)

	_, err := resolveScalarArg(invocation.cmdChain, arg, invocation.posTokens, tokenIndex, state.TypeParsers, coerce.DecodeContext{Stdin: state.Stdin, Stdout: state.Stdout})

	return tolerateBootstrapError(snapshots, snapshot, err, tolerate)
}

func (a *App[T]) bindBootstrapDefaultArg(invocation parsedInvocation, state *bindingState, arg *spec.Arg, tokenIndex int, snapshots []containerSnapshot, tolerate bool) ([]containerSnapshot, error) {
	if !arg.Bootstrap || arg.Handle == nil || arg.Default == "" {
		return snapshots, nil
	}

	snapshot := snapshotContainer(arg.Handle)
	snapshots = append(snapshots, snapshot)

	_, err := resolveScalarArg(invocation.cmdChain, arg, invocation.posTokens, tokenIndex, state.TypeParsers, coerce.DecodeContext{Stdin: state.Stdin, Stdout: state.Stdout})

	return tolerateBootstrapError(snapshots, snapshot, err, tolerate)
}

func snapshotContainer(handle any) containerSnapshot {
	value := reflect.ValueOf(handle).Elem()
	saved := reflect.New(value.Type()).Elem()
	saved.Set(value)

	return containerSnapshot{handle: handle, value: saved}
}

func markSelectedCommands(cmdChain []*spec.Command) []selectionSnapshot {
	if len(cmdChain) < 2 {
		return nil
	}

	snapshots := make([]selectionSnapshot, 0, len(cmdChain))

	for _, cmd := range cmdChain[1:] {
		if cmd.Handle == nil {
			continue
		}

		state := commandStateOf(cmd.Handle)
		snapshots = append(snapshots, selectionSnapshot{handle: cmd.Handle, selected: state.IsSelected()})
		state.markSelected()
	}

	return snapshots
}

func restoreSelection(snapshots []selectionSnapshot) {
	for _, snapshot := range snapshots {
		commandStateOf(snapshot.handle).setSelected(snapshot.selected)
	}
}

func bindBootstrapSpecialArgs(leaf *spec.Command, dashTokens, rawTokens []string, snapshots []containerSnapshot) []containerSnapshot {
	if leaf == nil {
		return snapshots
	}

	if leaf.DashArgs != nil && leaf.DashArgs.Bootstrap && leaf.DashArgs.Handle != nil {
		snapshots = append(snapshots, snapshotContainer(leaf.DashArgs.Handle))

		if dashTokens != nil {
			argsStateOf(leaf.DashArgs.Handle).setArgs(dashTokens)
		}
	}

	if leaf.RawArgs != nil && leaf.RawArgs.Bootstrap && leaf.RawArgs.Handle != nil {
		snapshots = append(snapshots, snapshotContainer(leaf.RawArgs.Handle))

		if len(rawTokens) > 0 {
			argsStateOf(leaf.RawArgs.Handle).setArgs(rawTokens)
		}
	}

	return snapshots
}

func tolerateBootstrapError(snapshots []containerSnapshot, snapshot containerSnapshot, err error, tolerate bool) ([]containerSnapshot, error) {
	if err == nil {
		return snapshots, nil
	}

	snapshot.restore()

	if tolerate {
		return snapshots, nil
	}

	return snapshots, err
}
