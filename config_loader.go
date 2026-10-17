package conga

import (
	"errors"
	"os"
	"slices"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
)

// ConfigContext exposes the invocation facts CONGA knows when the
// configuration loader runs: the selected command path, the raw command-line
// values parsed before binding, and the run's environment view. The engine
// supplies the implementation; application tests may implement the interface
// to exercise a loader in isolation.
type ConfigContext interface {
	// CommandPath returns the selected invocation path, for example
	// "cloudctl deploy worker".
	CommandPath() string
	// FlagValue returns the last raw command-line value supplied for the
	// named flag or alias, and whether the flag was supplied at all. Only
	// command-line values are reported; environment and document tiers bind
	// after the loader runs.
	FlagValue(name string) (string, bool)
	// FlagValues returns every raw command-line value supplied for the named
	// flag or alias in order, or nil when the flag was not supplied.
	FlagValues(name string) []string
	// LookupEnv returns the run's environment view (with .env files already
	// applied), like os.LookupEnv.
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
	if d == nil {
		panic("conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)")
	}

	if d.index == nil {
		d.index = make(map[string]int)
	}

	if index, exists := d.index[key]; exists {
		d.entries[index] = documentEntry{key: key, value: value, origin: origin, baseSet: true}

		return d
	}

	d.index[key] = len(d.entries)
	d.entries = append(d.entries, documentEntry{key: key, value: value, origin: origin, baseSet: true})

	return d
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
	if d == nil {
		panic("conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)")
	}

	if d.index == nil {
		d.index = make(map[string]int)
	}

	index, exists := d.index[key]
	if !exists {
		d.index[key] = len(d.entries)
		d.entries = append(d.entries, documentEntry{
			key:      key,
			overlays: []documentMapEntry{{key: entryKey, value: value, origin: origin}},
		})

		return d
	}

	entry := &d.entries[index]
	for overlayIndex := range entry.overlays {
		if entry.overlays[overlayIndex].key == entryKey {
			entry.overlays[overlayIndex] = documentMapEntry{key: entryKey, value: value, origin: origin}

			return d
		}
	}

	entry.overlays = append(entry.overlays, documentMapEntry{key: entryKey, value: value, origin: origin})

	return d
}

// ConfigLoaderFunc supplies the run's configuration document. Returning a nil
// document with a nil error leaves the configuration tier inactive; a returned
// error fails the run and is classified under [ErrConfig].
//
// The function runs at most once per execution, after command-line
// tokenization and before flag binding and feature pruning. Applications that
// enable features may have it run for --help and --why rendering, and shell
// completion invocations run it as well, so it must stay free of side effects,
// prompts, and blocking I/O.
type ConfigLoaderFunc func(ctx ConfigContext) (*ConfigDocument, error)

// ConfigLoader declares the application's programmatic configuration document
// source for [App.ConfigOptions], replacing configuration file resolution
// (decoders, search tiers, and probing) for every run:
//
//	app.ConfigOptions(conga.ConfigLoader(func(ctx conga.ConfigContext) (*conga.ConfigDocument, error) {
//		document := &conga.ConfigDocument{}
//		document.Set("host", "localhost", "embedded defaults")
//
//		if profile, ok := ctx.FlagValue("profile"); ok {
//			document.Set("host", profile+"-host", "flag --profile")
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
func ConfigLoader(load ConfigLoaderFunc) ConfigOption {
	if load == nil {
		panic("conga: app.ConfigOptions(conga.ConfigLoader(...)): the loader function cannot be nil")
	}

	return loaderOption{load: load}
}

// loaderOption is the sealed [ConfigOption] produced by [ConfigLoader]. The
// loader marker rides through schema build in spec.ConfigSettings for the
// conflict guards; the function itself stays on the App, extracted from the
// option list by [App.ConfigOptions].
type loaderOption struct {
	load ConfigLoaderFunc
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
	baseSet  bool
	overlays []documentMapEntry
}

// documentMapEntry is one per-key overlay recorded by [ConfigDocument.SetKey].
type documentMapEntry struct {
	key    string
	value  any
	origin string
}

// loaderContext implements [ConfigContext] over the parsed invocation state.
type loaderContext struct {
	cmdChain   []*spec.Command
	flagValues map[*spec.Flag][]string
}

func (c loaderContext) CommandPath() string {
	return commandChainPath(c.cmdChain)
}

func (c loaderContext) FlagValue(name string) (string, bool) {
	values := c.FlagValues(name)
	if len(values) == 0 {
		return "", false
	}

	return values[len(values)-1], true
}

func (c loaderContext) FlagValues(name string) []string {
	for _, cmd := range c.cmdChain {
		for _, flagSpec := range cmd.Flags {
			if flagSpec.Name != name && !slices.Contains(flagSpec.Aliases, name) {
				continue
			}

			return slices.Clone(c.flagValues[flagSpec])
		}
	}

	return nil
}

func (c loaderContext) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// loaderError classifies a failure returned by the application's configuration
// loader under [ErrConfig] while preserving the cause's configuration-path
// attribution for the tolerant --why report.
type loaderError struct {
	cause error
}

func (e *loaderError) Error() string {
	return "config loader failed: " + e.cause.Error()
}

func (e *loaderError) Unwrap() []error {
	return []error{ErrConfig, e.cause}
}

// validate validates every entry against the command schema, populating dest
// with the resolved values, and returns the distinct non-empty origins in
// first-set order for provenance reporting. On a validation failure the
// origins collected so far are returned alongside the error so partial
// provenance survives.
func (d *ConfigDocument) validate(cmd *spec.Command, dest *configdata.Data) ([]string, error) {
	var origins []string

	for _, entry := range d.entries {
		if entry.baseSet {
			// The write's origin is collected before validation so a failing
			// entry's source still appears in the tolerant --why candidate table.
			if entry.origin != "" {
				origins = append(origins, entry.origin)
			}

			if err := configengine.ValidateDocument(map[string]any{entry.key: entry.value}, cmd, dest, entry.origin); err != nil {
				return slicex.Unique(origins), err
			}
		}

		for _, overlay := range entry.overlays {
			if overlay.origin != "" {
				origins = append(origins, overlay.origin)
			}

			if err := configengine.ValidateDocument(map[string]any{entry.key: map[string]any{overlay.key: overlay.value}}, cmd, dest, overlay.origin); err != nil {
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
// resolved data is returned so the tolerant --why report can still describe
// the values that resolved.
func (a *App[T]) loadDocumentConfigData(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) (*configdata.Data, []string, error) {
	document, err := a.configLoader(loaderContext{cmdChain: cmdChain, flagValues: flagValues})
	if err != nil {
		return nil, nil, &loaderError{cause: err}
	}

	if document == nil {
		return nil, nil, nil
	}

	data := configdata.New()

	origins, err := document.validate(cmd, data)
	if err != nil {
		return data, origins, err
	}

	return data, origins, nil
}

// resolveLoaderConfig resolves the run's programmatic configuration document
// exactly once, memoizing the outcome like the file resolver does. The loaded
// paths report the distinct origins as the loader supplied them, and the
// candidates mirror them so the --why path table shows the document sources.
func (a *App[T]) resolveLoaderConfig(cmd *spec.Command, cmdChain []*spec.Command, flagValues map[*spec.Flag][]string) *configResolution {
	a.configData = configdata.New()

	data, origins, err := a.loadConfigDocument(a, cmd, cmdChain, flagValues)
	if data != nil {
		a.configData = data
	}

	if err != nil {
		// A loader error that attributes itself to a configuration path (the
		// configPathError contract) surfaces on that candidate row in the
		// tolerant --why report.
		if pathErr, ok := errors.AsType[configPathError](err); ok {
			if path := pathErr.ConfigPath(); path != "" && !slices.Contains(origins, path) {
				origins = append(origins, path)
			}
		}

		return &configResolution{err: err, candidates: origins}
	}

	if data == nil {
		return &configResolution{loadedSource: SourceNone}
	}

	return &configResolution{
		loadedPaths:  origins,
		loadedSource: SourceConfig,
		candidates:   origins,
	}
}

// extractConfigLoaderFunc returns the loader declared by the last
// [ConfigLoader] option in call order, or nil when no option declared one.
func extractConfigLoaderFunc(options []ConfigOption) ConfigLoaderFunc {
	var load ConfigLoaderFunc

	for _, option := range options {
		if loader, ok := option.(loaderOption); ok {
			load = loader.load
		}
	}

	return load
}
