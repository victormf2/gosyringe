package internal

import "sync"

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
	mu      sync.Mutex
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

func (m *SyncMap[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backMap[key] = value
}
func (m *SyncMap[K, V]) Load(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	value, found := m.backMap[key]
	if !found {
		var zero V
		return zero, false
	}

	return value, true

}
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
