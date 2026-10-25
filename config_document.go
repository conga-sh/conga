package conga

import (
	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/configengine"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
)

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

// configRequest carries the parsed invocation into configuration resolution,
// together with the environment view and the bootstrap error policy.
type configRequest struct {
	invocation        parsedInvocation
	envLookup         func(string) (string, bool)
	tolerateBootstrap bool
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
