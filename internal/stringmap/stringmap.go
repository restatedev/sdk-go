// Package stringmap provides a read-only, deterministically-ordered view over a set
// of string key/value pairs. It is the type the SDK hands back to users for things
// like incoming request headers and TerminalError metadata, where exposing a plain
// map would leak Go's intentionally-randomized map iteration order.
package stringmap

import (
	"encoding/json"
	"iter"
	"sort"
)

// Map is a read-only view over string key/value pairs with deterministic
// (key-sorted) iteration order.
type Map interface {
	// Get returns the value associated with key, or "" if there is none.
	Get(key string) string
	// Iter iterates the pairs in a deterministic, key-sorted order.
	Iter() iter.Seq2[string, string]
	// ToMap returns the pairs as a new plain map, for interop with external code.
	// The returned map is a copy: mutating it does not affect the Map.
	ToMap() map[string]string

	json.Marshaler
}

// New wraps m as a read-only [Map]. The map must not be mutated afterwards;
// callers of the returned Map cannot mutate m, as ToMap returns a copy.
func New(m map[string]string) Map {
	return stringMap{m: m}
}

type stringMap struct {
	m map[string]string
}

func (s stringMap) Get(key string) string { return s.m[key] }

func (s stringMap) Iter() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		keys := make([]string, 0, len(s.m))
		for k := range s.m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !yield(k, s.m[k]) {
				return
			}
		}
	}
}

func (s stringMap) ToMap() map[string]string {
	out := make(map[string]string, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

// MarshalJSON serialises the pairs as a JSON object, so a Map can be embedded in user
// structs and stored/serialised. encoding/json emits map keys in sorted order, matching
// Iter; a nil Map serialises as null, like a nil map.
func (s stringMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.m)
}
