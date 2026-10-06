// Package configengine implements the configuration-file resolution engine
// shared by the root conga package and the public conga/config façade: decoder
// registration, search-tier probing, strict content validation, and the
// per-invocation Resolver that turns those pieces into loaded files.
package configengine

import (
	"maps"
	"slices"
	"strings"
)

// Decoder parses raw configuration bytes into the generic key-value map validated against the Command AST during configuration resolution.
type Decoder func(data []byte) (map[string]any, error)

// Registry holds one application's locally registered configuration decoders.
type Registry struct {
	decoders map[string]Decoder
}

// NewRegistry returns an empty per-application decoder registry.
func NewRegistry() *Registry {
	return &Registry{
		decoders: make(map[string]Decoder),
	}
}

// Register adds or replaces the decoder for a file extension (e.g. ".json", ".toml").
func (r *Registry) Register(extension string, decoder Decoder) {
	if r == nil {
		return
	}

	r.decoders[normalizeExtension(extension)] = decoder
}

// Decoder returns the registered decoder for a file extension, or nil if none
// is registered.
func (r *Registry) Decoder(extension string) Decoder {
	if r == nil {
		return nil
	}

	return r.decoders[normalizeExtension(extension)]
}

// Extensions returns the sorted decoder file extensions registered in the registry.
func (r *Registry) Extensions() []string {
	if r == nil {
		return nil
	}

	extensions := slices.Collect(maps.Keys(r.decoders))
	slices.Sort(extensions)

	return extensions
}

func normalizeExtension(extension string) string {
	extension = strings.ToLower(strings.TrimSpace(extension))

	extension = strings.TrimPrefix(extension, "*")
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}

	return extension
}
