package restate

import "github.com/restatedev/sdk-go/internal/stringmap"

// StringMap is a read-only, deterministically-ordered view over string key/value pairs.
//
// Iterating a StringMap is deterministic (key-sorted), unlike ranging over a Go map. Use
// ToMap when you need a plain map to hand to external code.
type StringMap = stringmap.Map
