package conga

import (
	"fmt"
	"os"
	"reflect"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
)

// ConfigContext exposes the invocation facts CONGA knows when the
// configuration loader runs: the selected command path and the run's
// environment view. Bootstrap-tagged fields are bound on the root struct
// before the loader runs, so the loader reads their typed containers directly.
//
// The engine supplies the implementation; application tests may implement the
// interface to exercise a loader in isolation.
type ConfigContext interface {
	// CommandPath returns the selected invocation path, for example
	// "cloudctl deploy worker".
	CommandPath() string
	// LookupEnv returns the environment visible to this invocation, like
	// os.LookupEnv. Normal executions include .env values because dotenv runs
	// before binding; completion uses the shell/process environment and the
	// injected completion lookup, and does not load .env files.
	LookupEnv(key string) (string, bool)
}

// ConfigDocument is a programmatically built configuration document: values in
// the same key space a configuration decoder produces, each remembering the
// origin it was set from. The zero value is ready to use.
//
// Keys address top-level document entries — a flag name, a config key, a
// subcommand section, or a namespace section — and values accept the shapes a
// decoder returns:
//
//	document := &conga.ConfigDocument{}
//	document.Set("port", 8080, "embedded defaults")
//	document.SetKey("headers", "X-Region", "eu", "taskrc")
//	document.SetScoped("log-level", "debug", "pipeline.yaml", "pipeline deploy")
//	document.Set("deploy", map[string]any{"replicas": 3}, "taskrc")
//
// Scalar values stringify the same way decoded-file values do, and list values
// comma-join before binding; supply a pre-joined string when the target flag
// declares a custom sep. Map flags accept a nested map[string]any or
// map[string]string section, a []string or []any of key=value tokens, or a
// flat token string. [ConfigDocument.SetKey] records a single map entry and
// merges it key by key with earlier writes.
//
// The origin is a free-form label CONGA never parses; it surfaces in the --why
// report and attributes each merged map key to the source that last wrote it.
// [ConfigDocument.SetScoped] and [ConfigDocument.SetKeyScoped] add an optional
// scope within the origin; value attribution renders it as origin › scope while
// the Config paths table lists the origin path once.
//
// Do not set nil values: like a null in a decoded file, a nil value stringifies
// rather than being dropped.
type ConfigDocument struct {
	entries []documentEntry
	index   map[string]int
}

// Set records value under key with the given origin and returns the document
// for chaining. A later Set of the same key replaces the earlier entry,
// including every [ConfigDocument.SetKey] contribution recorded for it.
//
// Panics:
//   - If the receiver is nil; mutate the zero value instead.
func (d *ConfigDocument) Set(key string, value any, origin string) *ConfigDocument {
	return d.set(key, value, origin, "")
}

// SetScoped records value under key with the given origin and scope and
// returns the document for chaining. It behaves like [ConfigDocument.Set]; the
// scope names a region within the origin, such as a section of a configuration
// file. Value attribution in the --why report renders the pair as
// origin › scope, while the Config paths table lists the origin once.
//
// An empty scope is equivalent to [ConfigDocument.Set]. An empty origin
// records no provenance detail and ignores the scope.
//
// Panics:
//   - If the receiver is nil; mutate the zero value instead.
func (d *ConfigDocument) SetScoped(key string, value any, origin, scope string) *ConfigDocument {
	return d.set(key, value, origin, scope)
}

// SetKey records value under entryKey within the map entry named key with the
// given origin and returns the document for chaining. It is the per-key
// counterpart of [ConfigDocument.Set]: a later layer overlays a single map
// entry without re-supplying the whole map.
//
// A SetKey entry merges key by key with a map recorded by Set: the later call
// wins for the entry keys it names, while keys set by an earlier call survive.
// A later Set of the same key replaces the whole entry and clears every
// SetKey contribution recorded for it. Each entry key keeps the origin it was
// last written with, so the --why report attributes map keys individually.
//
// Panics:
//   - If the receiver is nil; mutate the zero value instead.
func (d *ConfigDocument) SetKey(key, entryKey string, value any, origin string) *ConfigDocument {
	return d.setKey(key, entryKey, value, origin, "")
}

// SetKeyScoped records value under entryKey within the map entry named key with
// the given origin and scope and returns the document for chaining. It behaves
// like [ConfigDocument.SetKey]; value attribution in the --why report renders
// the pair as origin › scope, while the Config paths table lists the origin
// once.
//
// An empty scope is equivalent to [ConfigDocument.SetKey]. An empty origin
// records no provenance detail and ignores the scope.
//
// Panics:
//   - If the receiver is nil; mutate the zero value instead.
func (d *ConfigDocument) SetKeyScoped(key, entryKey string, value any, origin, scope string) *ConfigDocument {
	return d.setKey(key, entryKey, value, origin, scope)
}

// ConfigLoaderFunc supplies the run's configuration document. Returning a nil
// document with a nil error leaves the configuration tier inactive; a returned
// error fails the run and is classified under [ErrConfig].
//
// The function runs at most once per execution, after command-line
// tokenization and bootstrap-field binding, and before configuration
// resolution, full flag binding, and feature pruning. Fields declared with the
// bootstrap tag are already bound on root; every other field holds its zero or
// caller-seeded value. Loader applications that enable features may have it
// run for --help and --why rendering, and shell completion invocations run it
// as well, so it must be deterministic and may perform only bounded local I/O
// and updates to app-owned non-schema state.
type ConfigLoaderFunc[T any] func(ctx ConfigContext, root *T) (*ConfigDocument, error)

// ConfigLoader declares the application's programmatic configuration document
// source for [App.ConfigOptions], replacing configuration file resolution
// (decoders, search tiers, and probing) for every run:
//
//	type ServerCLI struct {
//		Profile conga.OptionalFlag[string] `bootstrap:"true"`
//		Host    conga.OptionalFlag[string]
//	}
//
//	app.ConfigOptions(conga.ConfigLoader(func(ctx conga.ConfigContext, root *ServerCLI) (*conga.ConfigDocument, error) {
//		document := &conga.ConfigDocument{}
//		document.Set("host", "localhost", "embedded defaults")
//
//		if root.Profile.IsSet() {
//			document.Set("host", root.Profile.Get()+"-host", "flag --profile")
//		}
//
//		return document, nil
//	}))
//
// The document is validated exactly like a decoded configuration file: unknown
// keys fail with [ErrConfig] and a did-you-mean hint, config:"-" exclusions are
// honored, and nested sections, subcommand namespaces, and map flags resolve
// the same way. Values keep the configuration tier rank (CLI > environment >
// document > default), and [OptionalFlag.Source] reports [SourceConfig] for
// document-supplied values. The application-supplied origins surface in the
// --why report.
//
// A loader application must not register decoders ([App.RegisterDecoder]),
// declare file resolution options ([ConfigPaths], [ConfigExplicit],
// [ConfigCascading]), or declare a root [ConfigFile] field: schema build
// rejects the combination with an [ErrSchema] error.
//
// Panics:
//   - If load is nil.
func ConfigLoader[T any](load ConfigLoaderFunc[T]) ConfigOption {
	if load == nil {
		panic("conga: app.ConfigOptions(conga.ConfigLoader(...)): the loader function cannot be nil")
	}

	return loaderOption{load: load}
}

// loaderOption is the sealed [ConfigOption] produced by [ConfigLoader]. The
// loader marker rides through schema build in spec.ConfigSettings for the
// conflict guards; the typed function itself stays on the App, extracted from
// the option list by [App.ConfigOptions].
type loaderOption struct {
	load any
}

func (option loaderOption) applyTo(settings *spec.ConfigSettings) {
	settings.Loader = true
}

// documentEntry is one top-level configuration document entry: an optional
// whole-key write and the ordered per-entry-key overlays recorded under it.
type documentEntry struct {
	key      string
	value    any
	origin   string
	scope    string
	baseSet  bool
	overlays []documentMapEntry
}

// documentMapEntry is one per-key overlay recorded by [ConfigDocument.SetKey].
type documentMapEntry struct {
	key    string
	value  any
	origin string
	scope  string
}

// parsedInvocation carries the parsed command line facts shared by
// configuration resolution, bootstrap binding, and completion analysis.
type parsedInvocation struct {
	root       *spec.Command
	leaf       *spec.Command
	cmdChain   []*spec.Command
	flagValues map[*spec.Flag][]string
	posTokens  []string
	dashTokens []string
	rawTokens  []string
}

// loaderRequest carries the parsed invocation into the configuration loader,
// together with the environment view and the bootstrap error policy.
type loaderRequest struct {
	invocation        parsedInvocation
	envLookup         func(string) (string, bool)
	tolerateBootstrap bool
}

// loaderResult is the outcome of one programmatic configuration load. restore
// reverts bootstrap-bound containers and command selection after an
// observation-only completion invocation; it is nil when nothing was bound.
type loaderResult struct {
	data    *configdata.Data
	origins []string
	restore func()
	err     error
}

// loaderContext implements [ConfigContext] over the parsed invocation state.
type loaderContext struct {
	cmdChain  []*spec.Command
	envLookup func(string) (string, bool)
}

func (c loaderContext) CommandPath() string {
	return commandChainPath(c.cmdChain)
}

func (c loaderContext) LookupEnv(key string) (string, bool) {
	if c.envLookup != nil {
		return c.envLookup(key)
	}

	return os.LookupEnv(key)
}

// loaderError classifies a failure returned by the application's configuration
// loader under [ErrConfig].
type loaderError struct {
	cause error
}

func (e *loaderError) Error() string {
	return "config loader failed: " + e.cause.Error()
}

func (e *loaderError) Unwrap() []error {
	return []error{ErrConfig, e.cause}
}

func (d *ConfigDocument) set(key string, value any, origin, scope string) *ConfigDocument {
	if d == nil {
		panic("conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)")
	}

	if d.index == nil {
		d.index = make(map[string]int)
	}

	entry := documentEntry{key: key, value: value, origin: origin, scope: scope, baseSet: true}
	if index, exists := d.index[key]; exists {
		d.entries[index] = entry

		return d
	}

	d.index[key] = len(d.entries)
	d.entries = append(d.entries, entry)

	return d
}

func (d *ConfigDocument) setKey(key, entryKey string, value any, origin, scope string) *ConfigDocument {
	if d == nil {
		panic("conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)")
	}

	if d.index == nil {
		d.index = make(map[string]int)
	}

	overlay := documentMapEntry{key: entryKey, value: value, origin: origin, scope: scope}

	index, exists := d.index[key]
	if !exists {
		d.index[key] = len(d.entries)
		d.entries = append(d.entries, documentEntry{
			key:      key,
			overlays: []documentMapEntry{overlay},
		})

		return d
	}

	entry := &d.entries[index]
	for overlayIndex := range entry.overlays {
		if entry.overlays[overlayIndex].key == entryKey {
			entry.overlays[overlayIndex] = overlay

			return d
		}
	}

	entry.overlays = append(entry.overlays, overlay)

	return d
}

// validate validates every entry against the command schema, populating dest
// with the resolved values, and returns the distinct non-empty origins in
// first-set order for provenance reporting. Origins are reported without their
// scopes, so the Config paths table lists each source once. On a validation
// failure the origins collected so far are returned alongside the error so the
// resolution can still name what it accepted.
func (d *ConfigDocument) validate(cmd *spec.Command, dest *configdata.Data) ([]string, error) {
	var origins []string

	record := func(origin string) {
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	for _, entry := range d.entries {
		if entry.baseSet {
			record(entry.origin)

			if err := configengine.ValidateDocument(map[string]any{entry.key: entry.value}, cmd, dest, entry.origin, entry.scope); err != nil {
				return slicex.Unique(origins), err
			}
		}

		for _, overlay := range entry.overlays {
			record(overlay.origin)

			if err := configengine.ValidateDocument(map[string]any{entry.key: map[string]any{overlay.key: overlay.value}}, cmd, dest, overlay.origin, overlay.scope); err != nil {
				return slicex.Unique(origins), err
			}
		}
	}

	return slicex.Unique(origins), nil
}

// loadDocumentConfigData runs the application's configuration loader and
// validates the returned document. It is installed on the App only when
// ConfigOptions declares a loader, so applications without one keep the
// document validation pipeline unreferenced at link time. A nil document
// (nil data) leaves the tier inactive; on a validation failure the partially
// resolved data and the origins recorded so far are returned so the resolution
// can report them.
func (a *App[T]) loadDocumentConfigData(request loaderRequest) loaderResult {
	restore, err := a.bindBootstrapFields(request.invocation, request.envLookup, request.tolerateBootstrap)
	if err != nil {
		if restore != nil {
			restore()
		}

		return loaderResult{err: err}
	}

	result := loaderResult{restore: restore}

	document, err := a.configLoader(loaderContext{cmdChain: request.invocation.cmdChain, envLookup: request.envLookup}, a.rootTarget)
	if err != nil {
		result.err = &loaderError{cause: err}

		return result
	}

	if document == nil {
		return result
	}

	data := configdata.New()

	origins, err := document.validate(request.invocation.root, data)
	result.data = data
	result.origins = origins
	result.err = err

	return result
}

// resolveLoaderConfig resolves the run's programmatic configuration document
// exactly once, memoizing the outcome like the file resolver does. Loaded
// paths and candidates report the distinct origins without their scopes; the
// per-value scopes stay on the stored configuration values.
func (a *App[T]) resolveLoaderConfig(request loaderRequest) *configResolution {
	a.configData = configdata.New()

	result := a.loadConfigDocument(a, request)
	if result.data != nil {
		a.configData = result.data
	}

	if result.err != nil {
		return &configResolution{err: result.err, candidates: result.origins}
	}

	if result.data == nil {
		return &configResolution{loadedSource: SourceNone}
	}

	return &configResolution{
		loadedPaths:  result.origins,
		loadedSource: SourceConfig,
		candidates:   result.origins,
	}
}

// extractConfigLoaderFunc returns the loader declared by the last
// [ConfigLoader] option in call order, or nil when no option declared one.
func extractConfigLoaderFunc[T any](options []ConfigOption) ConfigLoaderFunc[T] {
	var load ConfigLoaderFunc[T]

	for _, option := range options {
		loader, ok := option.(loaderOption)
		if !ok {
			continue
		}

		typed, ok := loader.load.(ConfigLoaderFunc[T])
		if !ok {
			panic(fmt.Sprintf(
				"conga: app.ConfigOptions(conga.ConfigLoader(...)): loader root type %s does not match application root type %s",
				reflect.TypeOf(loader.load).In(1),
				reflect.PointerTo(reflect.TypeFor[T]()),
			))
		}

		load = typed
	}

	return load
}
