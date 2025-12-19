package datastructures

import (
	"maps"
	"sync"
	"sync/atomic"
)

// SyncMap is a map that is safe for concurrent use. It just uses a [sync.RWMutex] to achieve this.
// The zero value can be used directly.
//
// [sync.Map] is great, but it is optimised for read-heavy use cases where the caller reads a single key-pair.
// It does not provide a method to read all key-pairs from a map while under a lock.
type SyncMap[K comparable, V any] struct {
	locked atomic.Bool
	m      map[K]V
	mu     sync.RWMutex
}

// Lock obtains a read lock on the SyncMap. Subsequent reads or writes will block unless called with lock=false.
func (s *SyncMap[K, V]) Lock() {
	s.mu.Lock()
}

// Unlock releases the map write lock.
func (s *SyncMap[K, V]) Unlock() {
	s.mu.Unlock()
}

// Len returns the length of the map.
func (s *SyncMap[K, V]) Len(lock bool) int {
	if s.m == nil {
		return 0
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	return len(s.m)
}

// Set sets a single key-pair in the SyncMap.
func (s *SyncMap[K, V]) Set(lock bool, k K, v V) {
	if lock {
		s.mu.Lock()
		defer s.mu.Unlock()
	}

	if s.m == nil {
		s.m = map[K]V{k: v}
		return
	}

	s.m[k] = v
}

// Delete deletes a single key-pair from the SyncMap.
func (s *SyncMap[K, V]) Delete(lock bool, k K) {
	if lock {
		s.mu.Lock()
		defer s.mu.Unlock()
	}

	if s.m == nil {
		return
	}

	delete(s.m, k)
}

// Patch adds all the key-pairs from the input map to the SyncMap.
func (s *SyncMap[K, V]) Patch(lock bool, m map[K]V) {
	if lock {
		s.mu.Lock()
		defer s.mu.Unlock()
	}

	if s.m == nil {
		s.m = m
		return
	}

	for k, v := range m {
		s.m[k] = v
	}
}

// Replace replaces the key-pairs in the SyncMap with the key-pairs from the input map.
func (s *SyncMap[K, V]) Replace(lock bool, m map[K]V) {
	if lock {
		s.mu.Lock()
		defer s.mu.Unlock()
	}

	s.m = m
}

// Get gets a single value from the map with the given key.
func (s *SyncMap[K, V]) Get(lock bool, k K) (V, bool) {
	if s.m == nil {
		return *(new(V)), false
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	v, ok := s.m[k]
	return v, ok
}

// Clone returns a clone of the SyncMap as a standard map.
func (s *SyncMap[K, V]) Clone(lock bool) map[K]V {
	if s.m == nil {
		return make(map[K]V)
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	return maps.Clone(s.m)
}

func (s *SyncMap[K, V]) Range(lock bool, do func(k K, v V) error) error {
	if s.m == nil {
		return nil
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	for k, v := range s.m {
		err := do(k, v)
		if err != nil {
			return err
		}
	}

	return nil
}

// Filter calls MapFilter on the SyncMap, returning a copy.
func (s *SyncMap[K, V]) Filter(lock bool, include func(k K, v V) (bool, error)) (map[K]V, error) {
	if s.m == nil {
		return make(map[K]V), nil
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	return MapFilter(s.m, include)
}

// SyncMapToSliceFilter calls MapToSliceFilter on the SyncMap while it is under a read lock.
func SyncMapToSliceFilter[K comparable, V any, E any](s *SyncMap[K, V], lock bool, include func(K, V) (bool, error), transform func(K, V) (E, error)) ([]E, error) {
	if s.m == nil {
		return make([]E, 0), nil
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	return MapToSliceFilter(s.m, include, transform)
}

// SyncMapToSlice calls MapToSlice on the SyncMap while it is under a read lock.
func SyncMapToSlice[K comparable, V any, E any](s *SyncMap[K, V], lock bool, transform func(K, V) (E, error)) ([]E, error) {
	if s.m == nil {
		return make([]E, 0), nil
	}

	if lock {
		s.mu.RLock()
		defer s.mu.RUnlock()
	}

	return MapToSlice(s.m, transform)
}
