// Package configdata holds the resolved configuration tier consumed by the
// value binder: scalar values and per-key map values keyed by bindable config
// keys. Package configengine populates Data while validating loaded files or
// documents, and package conga folds it into the tier cascade.
package configdata

// Value is a resolved configuration value and the origin that supplied it.
type Value struct {
	// Raw is the stringified value.
	Raw string
	// Origin is the configuration file path that supplied the value;
	// programmatic loaders may supply other source labels.
	Origin string
}

// Data holds the validated configuration tier of one invocation: scalar values
// and per-key map values, keyed by bindable config key. Later stores overwrite
// earlier ones in place as files or documents are read.
type Data struct {
	scalars map[string]Value
	maps    map[string]map[string]Value
}

// New builds an empty configuration data set.
func New() *Data {
	return &Data{
		scalars: make(map[string]Value),
		maps:    make(map[string]map[string]Value),
	}
}

// SetScalar records a scalar value and its origin under key, overwriting any
// value stored by an earlier file.
func (data *Data) SetScalar(key, raw, origin string) {
	data.scalars[key] = Value{Raw: raw, Origin: origin}
}

// EnsureMap returns the shared per-key map for the given alias keys of a map
// flag, creating and binding it across all alias keys if not yet present.
// Calling EnsureMap with an empty map section still records the keys as present
// for HasKey checks.
func (data *Data) EnsureMap(keys []string) map[string]Value {
	var entries map[string]Value

	for _, key := range keys {
		if existing, ok := data.maps[key]; ok {
			entries = existing

			break
		}
	}

	if entries == nil {
		entries = make(map[string]Value)
	}

	for _, key := range keys {
		data.maps[key] = entries
	}

	return entries
}

// Scalar returns the raw scalar string stored under key.
func (data *Data) Scalar(key string) (string, bool) {
	val, exists := data.scalars[key]

	return val.Raw, exists
}

// ScalarValue returns the scalar value and its origin stored under key.
// Consumers that need the provenance — such as the programmatic
// configuration loader — prefer it over Scalar.
func (data *Data) ScalarValue(key string) (Value, bool) {
	val, exists := data.scalars[key]

	return val, exists
}

// Map returns the merged per-key map stored under key, or nil when no map was
// recorded for key.
func (data *Data) Map(key string) map[string]Value {
	return data.maps[key]
}

// HasKey reports whether key was set in the configuration tier (as a scalar or
// as a map section, even an empty one).
func (data *Data) HasKey(key string) bool {
	if _, exists := data.scalars[key]; exists {
		return true
	}

	_, exists := data.maps[key]

	return exists
}
