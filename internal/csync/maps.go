package csync

import (
	"encoding/json"
	"iter"
	"maps"
	"sync"
)

type inflightCall[V any] struct {
	wg  sync.WaitGroup
	val V
}

// Map is a concurrent map implementation that provides thread-safe access.
type Map[K comparable, V any] struct {
	inner    map[K]V
	inflight map[K]*inflightCall[V]
	mu       sync.RWMutex
}

// NewMap creates a new thread-safe map with the specified key and value types.
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		inner: make(map[K]V),
	}
}

// NewMapFrom creates a new thread-safe map from an existing map.
func NewMapFrom[K comparable, V any](m map[K]V) *Map[K, V] {
	return &Map[K, V]{
		inner: m,
	}
}

// NewLazyMap creates a new lazy-loaded map. The provided load function is
// executed in a separate goroutine to populate the map.
func NewLazyMap[K comparable, V any](load func() map[K]V) *Map[K, V] {
	m := &Map[K, V]{}
	m.mu.Lock()
	go func() {
		defer m.mu.Unlock()
		m.inner = load()
	}()
	return m
}

// Reset replaces the inner map with the new one.
func (m *Map[K, V]) Reset(input map[K]V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner = input
	m.inflight = nil
}

// Set sets the value for the specified key in the map.
func (m *Map[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner[key] = value
	if m.inflight != nil {
		delete(m.inflight, key)
	}
}

// Del deletes the specified key from the map.
func (m *Map[K, V]) Del(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inner, key)
	if m.inflight != nil {
		delete(m.inflight, key)
	}
}

// CompareAndDelete deletes the key only if the current value matches the
// expected pointer. Returns true if the deletion occurred. This is the
// ABA-safe cleanup primitive: it prevents a deferred cleanup from removing
// a value that was replaced by a newer writer in the window between the
// explicit Del and the deferred Del.
func (m *Map[K, V]) CompareAndDelete(key K, expected any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.inner[key]
	if !ok {
		return false
	}
	if any(current) != expected {
		return false
	}
	delete(m.inner, key)
	return true
}

// Get gets the value for the specified key from the map.
func (m *Map[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.inner[key]
	return v, ok
}

// Len returns the number of items in the map.
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inner)
}

// GetOrSet gets and returns the key if it exists, otherwise, it executes the
// given function, set its return value for the given key, and returns it.
func (m *Map[K, V]) GetOrSet(key K, fn func() V) V {
	got, ok := m.Get(key)
	if ok {
		return got
	}
	value := fn()
	m.Set(key, value)
	return value
}

// GetOrSetCoalesced returns the cached value for key or executes fn once
// without holding the map mutex, sharing the result with concurrent callers.
func (m *Map[K, V]) GetOrSetCoalesced(key K, fn func() V) V {
	m.mu.RLock()
	if got, ok := m.inner[key]; ok {
		m.mu.RUnlock()
		return got
	}
	m.mu.RUnlock()

	m.mu.Lock()
	if got, ok := m.inner[key]; ok {
		m.mu.Unlock()
		return got
	}
	if call, ok := m.inflight[key]; ok {
		m.mu.Unlock()
		call.wg.Wait()
		return call.val
	}
	call := &inflightCall[V]{}
	call.wg.Add(1)
	if m.inflight == nil {
		m.inflight = make(map[K]*inflightCall[V])
	}
	m.inflight[key] = call
	m.mu.Unlock()

	completed := false
	defer func() {
		if !completed {
			m.mu.Lock()
			if m.inflight != nil && m.inflight[key] == call {
				delete(m.inflight, key)
			}
			m.mu.Unlock()
			call.wg.Done()
		}
	}()

	value := fn()
	completed = true

	m.mu.Lock()
	call.val = value
	if m.inflight != nil && m.inflight[key] == call {
		if m.inner == nil {
			m.inner = make(map[K]V)
		}
		m.inner[key] = value
		delete(m.inflight, key)
	}
	m.mu.Unlock()
	call.wg.Done()
	return value
}

// Take gets an item and then deletes it.
func (m *Map[K, V]) Take(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.inner[key]
	delete(m.inner, key)
	if m.inflight != nil {
		delete(m.inflight, key)
	}
	return v, ok
}

// Copy returns a copy of the inner map.
func (m *Map[K, V]) Copy() map[K]V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.inner)
}

// Seq2 returns an iter.Seq2 that yields key-value pairs from the map.
func (m *Map[K, V]) Seq2() iter.Seq2[K, V] {
	dst := m.Copy()
	return func(yield func(K, V) bool) {
		for k, v := range dst {
			if !yield(k, v) {
				return
			}
		}
	}
}

// Seq returns an iter.Seq that yields values from the map.
func (m *Map[K, V]) Seq() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range m.Seq2() {
			if !yield(v) {
				return
			}
		}
	}
}

var (
	_ json.Unmarshaler = &Map[string, any]{}
	_ json.Marshaler   = &Map[string, any]{}
)

// JSONSchemaAlias returns the underlying map type for JSON schema generation.
// Value receiver is required because github.com/SpherePrime/CLI/vendordeps/invopop/jsonschema checks
// interface satisfaction on the non-pointer type after stripping pointers.
func (Map[K, V]) JSONSchemaAlias() any { //nolint
	m := map[K]V{}
	return m
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner = make(map[K]V)
	m.inflight = nil
	return json.Unmarshal(data, &m.inner)
}

// MarshalJSON implements json.Marshaler.
func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.Marshal(m.inner)
}
