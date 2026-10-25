package conga

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/conga-sh/conga/internal/coerce"
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/dotenv"
	"github.com/conga-sh/conga/internal/spec"
)

// Bootstrap is the handle passed to [Bootstrapper.Bootstrap]. It exposes the
// invocation facts and the bootstrap-phase operations: loading environment
// files and supplying the run's programmatic configuration document. The
// engine constructs it; applications only read from it.
type Bootstrap struct {
	setenvFunc    func(key, value string)
	dotenvOrigins map[string]string
	envLookup     func(string) (string, bool)
	cmdChain      []*spec.Command
	document      *ConfigDocument
	configSet     bool
	validateDoc   func(*ConfigDocument, *spec.Command, *configdata.Data) ([]string, error)
}

// Path returns the selected invocation path, for example
// "cloudctl deploy worker".
func (b *Bootstrap) Path() string {
	return commandChainPath(b.cmdChain)
}

// Name returns the matched leaf command name.
func (b *Bootstrap) Name() string {
	if len(b.cmdChain) == 0 {
		return ""
	}

	return b.cmdChain[len(b.cmdChain)-1].Name
}

// LookupEnv returns the environment visible to this invocation, like
// os.LookupEnv. Completion invocations see the shell-injected view; real runs
// see the process environment, including variables loaded by earlier
// [Bootstrap.Dotenv] calls.
func (b *Bootstrap) LookupEnv(key string) (string, bool) {
	if b.envLookup != nil {
		return b.envLookup(key)
	}

	return os.LookupEnv(key)
}

// Dotenv loads each existing .env file in order through the configured setenv
// function. Missing files are ignored; malformed or oversized (>16MB) files
// return an error wrapping [ErrConfig]. When called without paths it probes
// .env followed by .env.local in the current working directory, loading both if
// present, mirroring [App.Dotenv]. Variables already present in the host
// process environment are never overwritten, while a file loaded here overrides
// a key an earlier .env file set in the same run.
func (b *Bootstrap) Dotenv(paths ...string) error {
	err := dotenv.LoadWithOrigins(b.setenvFunc, b.dotenvOrigins, false, paths...)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}

	return nil
}

// DotenvExplicit loads each named .env file through the configured setenv
// function, returning an error wrapping [ErrConfig] when a file is missing,
// malformed, or oversized (>16MB). It is the strict counterpart of
// [Bootstrap.Dotenv] for paths supplied by a flag such as --env-file.
func (b *Bootstrap) DotenvExplicit(paths ...string) error {
	err := dotenv.LoadWithOrigins(b.setenvFunc, b.dotenvOrigins, true, paths...)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConfig, err)
	}

	return nil
}

// SetConfig supplies the run's programmatic configuration document. The
// document overlays the file-resolved configuration tier key by key: it wins
// the keys it names, while file keys it does not name survive. It therefore
// composes with [ConfigPaths], [ConfigExplicit], [ConfigCascading], registered
// decoders, and a root [ConfigFile] field. Passing nil leaves the document
// contribution inactive; a later call replaces an earlier one.
func (b *Bootstrap) SetConfig(doc *ConfigDocument) {
	b.document = doc
	b.configSet = true
	b.validateDoc = (*ConfigDocument).validate
}

// bootstrapResult is the outcome of the bootstrap phase. restore reverts
// bootstrap-bound containers and command selection after an observation-only
// completion invocation; it is nil when nothing was bound.
type bootstrapResult struct {
	document    *ConfigDocument
	validateDoc func(*ConfigDocument, *spec.Command, *configdata.Data) ([]string, error)
	restore     func()
	configSet   bool
	err         error
}

// bootstrapError classifies a failure returned by a [Bootstrapper] under
// [ErrConfig].
type bootstrapError struct {
	cause error
}

func (e *bootstrapError) Error() string {
	return "bootstrap failed: " + e.cause.Error()
}

func (e *bootstrapError) Unwrap() []error {
	return []error{ErrConfig, e.cause}
}

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

// runBootstrap binds bootstrap-tagged fields and runs every [Bootstrapper] in
// the selected command chain top-down, then validates the document supplied via
// [Bootstrap.SetConfig]. It reports false when no command in the chain
// implements [Bootstrapper], leaving the configuration tier to file
// resolution. It is pure: it never mutates App state, so completion can run it
// as an observation.
func (a *App[T]) runBootstrap(ctx context.Context, request configRequest) (bootstrapResult, bool) {
	handle, ok := a.newBootstrap(request)
	if !ok {
		return bootstrapResult{}, false
	}

	restore, err := a.bindBootstrapFields(request.invocation, request.envLookup, request.tolerateBootstrap)

	result := bootstrapResult{restore: restore}
	if err != nil {
		result.err = classifyBootstrapError(err)

		return result, true
	}

	var bootstrapErr error

	for _, cmd := range request.invocation.cmdChain {
		bootstrapper, ok := getCmdTarget(cmd).(Bootstrapper)
		if !ok {
			continue
		}

		if err := bootstrapper.Bootstrap(ctx, handle); err != nil {
			bootstrapErr = classifyBootstrapError(err)

			break
		}
	}

	result.configSet = handle.configSet
	result.document = handle.document
	result.validateDoc = handle.validateDoc

	if bootstrapErr != nil {
		result.err = bootstrapErr

		return result, true
	}

	return result, true
}

// newBootstrap builds the Bootstrap handle when any command in the chain
// implements [Bootstrapper], sharing the App's dotenv origins map so loaded
// keys stay attributed across phases and invocations.
func (a *App[T]) newBootstrap(request configRequest) (*Bootstrap, bool) {
	if !chainImplementsBootstrapper(request.invocation.cmdChain) {
		return nil, false
	}

	return &Bootstrap{
		setenvFunc:    a.setenvFunc,
		dotenvOrigins: a.dotenvOriginMap(),
		envLookup:     request.envLookup,
		cmdChain:      request.invocation.cmdChain,
	}, true
}

// bindBootstrapFields binds bootstrap-tagged flags, positional arguments, and
// trailing-argument collectors in the bootstrap pass, before configuration
// resolution runs. The returned restore function reverts the bound containers
// and command selection for completion invocations; normal executions ignore it
// because the normal binding pass rewrites every value.
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

// overlayBootstrapDocument validates the document supplied via
// [Bootstrap.SetConfig] into data, which may already hold file-resolved values,
// so the document wins the keys it names key by key. It returns the document
// origins in first-set order, or nil when no document was supplied or the
// bootstrap phase already failed.
func overlayBootstrapDocument(result bootstrapResult, cmd *spec.Command, data *configdata.Data) ([]string, error) {
	if !result.configSet || result.document == nil {
		return nil, nil
	}

	if result.err != nil {
		return nil, result.err
	}

	return result.validateDoc(result.document, cmd, data)
}

func chainImplementsBootstrapper(cmdChain []*spec.Command) bool {
	for _, cmd := range cmdChain {
		if _, ok := getCmdTarget(cmd).(Bootstrapper); ok {
			return true
		}
	}

	return false
}

func classifyBootstrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, ErrConfig) {
		return err
	}

	return &bootstrapError{cause: err}
}

// isBootstrapper reports whether a command struct value implements
// [Bootstrapper]; it backs spec.Options.IsBootstrapper during schema build.
func isBootstrapper(target any) bool {
	_, ok := target.(Bootstrapper)

	return ok
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
