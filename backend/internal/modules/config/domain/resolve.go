package domain

import (
	"fmt"
	"sort"
	"time"
)

// Resolved is the effective configuration for one placement.
//
// It always contains every registered key, because every key has a default.
// That is what lets the accessors return a value rather than a value and an
// "is it there" flag: a caller asking for the delivery base fee in an area we
// have never configured gets the global default, not a zero.
type Resolved struct {
	placement Placement
	values    map[Key]Value
	sources   map[Key]Scope
}

// Resolve applies overrides to defaults in Appendix B order.
//
// Overrides are consulted area → district → division → global, and the first
// match wins. An override for an unknown key is ignored rather than fatal: keys
// are removed from the registry over time and a stale row in the database
// should not take the whole config load down with it.
func Resolve(placement Placement, overrides []Override) Resolved {
	chain := placement.Chain()
	rank := make(map[Scope]int, len(chain))
	for i, scope := range chain {
		rank[scope] = i
	}

	best := make(map[Key]int, len(registry))
	values := make(map[Key]Value, len(registry))
	sources := make(map[Key]Scope, len(registry))

	for _, d := range definitions {
		values[d.Key] = d.Default
		sources[d.Key] = GlobalScope
		best[d.Key] = len(chain) // worse than any real scope
	}

	for _, o := range overrides {
		d, ok := registry[o.Key]
		if !ok {
			continue // a key that no longer exists
		}
		// The immutable variable ignores every override, wherever it came from.
		// Enforcing it here as well as at write time means a row inserted
		// directly into the database — by a migration, a console, or a bug —
		// still cannot turn off the division ceiling.
		if d.Immutable {
			continue
		}
		if o.Value.Kind() != d.Kind {
			continue // a row whose type no longer matches the definition
		}
		position, applies := rank[o.Scope]
		if !applies || position >= best[o.Key] {
			continue
		}
		best[o.Key] = position
		values[o.Key] = o.Value
		sources[o.Key] = o.Scope
	}

	return Resolved{placement: placement, values: values, sources: sources}
}

// Placement returns where this configuration was resolved for.
func (r Resolved) Placement() Placement { return r.placement }

// Value returns the effective value for a key.
func (r Resolved) Value(key Key) (Value, error) {
	v, ok := r.values[key]
	if !ok {
		return Value{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return v, nil
}

// Source returns the scope the effective value came from, which is what an
// admin needs to see to understand why a value is what it is.
func (r Resolved) Source(key Key) (Scope, error) {
	s, ok := r.sources[key]
	if !ok {
		return Scope{}, fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return s, nil
}

// Int returns a whole-number value.
func (r Resolved) Int(key Key) (int64, error) {
	v, err := r.Value(key)
	if err != nil {
		return 0, err
	}
	return v.Int()
}

// Bool returns a switch value.
func (r Resolved) Bool(key Key) (bool, error) {
	v, err := r.Value(key)
	if err != nil {
		return false, err
	}
	return v.Bool()
}

// Ratio returns a multiplier value.
func (r Resolved) Ratio(key Key) (float64, error) {
	v, err := r.Value(key)
	if err != nil {
		return 0, err
	}
	return v.Ratio()
}

// Entry is one resolved setting together with where it came from.
type Entry struct {
	Key    Key
	Value  Value
	Source Scope
}

// Entries returns every setting in key order, each with its source.
//
// Value and source come back together rather than through two lookups, so a
// caller listing the snapshot has no error path to handle. Resolve populated
// both maps from the same registry in the same pass, so a key present in one
// and absent from the other cannot happen — and code defending against it would
// be untestable by construction.
func (r Resolved) Entries() []Entry {
	keys := r.Keys()
	out := make([]Entry, 0, len(keys))
	for _, key := range keys {
		out = append(out, Entry{
			Key:    key,
			Value:  r.values[key],
			Source: r.sources[key],
		})
	}
	return out
}

// Keys returns every key in the snapshot, sorted.
func (r Resolved) Keys() []Key {
	keys := make([]Key, 0, len(r.values))
	for k := range r.values {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Actor is who made a change. Every audit entry has one.
type Actor struct {
	// Kind is "admin" or "auto_tuner".
	Kind string
	// ID identifies the admin. Empty for the tuner, which is not a person.
	ID string
}

// Actor kinds.
const (
	ActorAdmin     = "admin"
	ActorAutoTuner = "auto_tuner"
)

// AdminActor builds an actor for a named admin.
func AdminActor(id string) Actor { return Actor{Kind: ActorAdmin, ID: id} }

// TunerActor builds the auto-tuner's actor.
func TunerActor() Actor { return Actor{Kind: ActorAutoTuner} }

// Change is one entry in the configuration audit log.
//
// D5 requires that every change is recorded with its reasoning. The old value
// is part of the record because "what was it before" is the first question
// anyone asks when a fee changes unexpectedly, and reconstructing it from the
// previous entry fails the moment one entry is missing.
type Change struct {
	ID       string
	Key      Key
	Scope    Scope
	OldValue Value
	NewValue Value
	Actor    Actor
	// Reason is free text from an admin, or the tuner's stated justification.
	Reason string
	At     time.Time
}
