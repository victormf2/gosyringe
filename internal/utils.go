package internal

import (
	"slices"
	"sync"
)

func Map[T any, U any](values []T, mapFn func(value T) U) []U {
	result := make([]U, len(values))
	for i, value := range values {
		result[i] = mapFn(value)
	}
	return result
}

func Flat[T any](values [][]T) []T {
	flatten := []T{}
	for _, row := range values {
		flatten = append(flatten, row...)
	}
	return flatten
}

type SyncMap[K comparable, V any] struct {
	mu      sync.RWMutex
	Parent  *SyncMap[K, V]
	backMap map[K]V
}
type KeyValue[K comparable, V any] struct {
	Key   K
	Value V
}

func NewSyncMap[K comparable, V any](initialKeyValues ...KeyValue[K, V]) *SyncMap[K, V] {
	syncMap := &SyncMap[K, V]{
		backMap: map[K]V{},
	}

	for _, kv := range initialKeyValues {
		syncMap.Store(kv.Key, kv.Value)
	}

	return syncMap
}

// Stores shallow
func (m *SyncMap[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backMap[key] = value
}

// Load search on Parent as well
func (m *SyncMap[K, V]) Load(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, found := m.backMap[key]
	if found {
		return value, true
	}
	if m.Parent != nil {
		return m.Parent.Load(key)
	}
	var zero V
	return zero, false
}

// LoadOrStores shallow
func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	presentValue, found := m.backMap[key]
	if found {
		return presentValue, true
	}

	m.backMap[key] = value
	return value, false
}

type SyncSlice[T any] struct {
	mu        sync.RWMutex
	backSlice []T
}

func NewSyncSlice[T any]() *SyncSlice[T] {
	return &SyncSlice[T]{
		backSlice: []T{},
	}
}
func (s *SyncSlice[T]) Append(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.backSlice = append(s.backSlice, value)
}
func (s *SyncSlice[T]) Snapshot() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Clone(s.backSlice)
}

type RLockValue[T any] struct {
	Value T
	lock  sync.RWMutex
}

func NewRLockValue[T any](initialValue T) *RLockValue[T] {
	return &RLockValue[T]{
		Value: initialValue,
	}
}

func (v *RLockValue[T]) Load() (T, func()) {
	v.lock.RLock()
	return v.Value, v.lock.RUnlock
}

func (v *RLockValue[T]) Store(value T) {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.Value = value
}
