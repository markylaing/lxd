package datastructures

import (
	"errors"
	"fmt"
)

// MapToSliceFilter filters and transforms the map into a slice. The include argument is a filter.
// It should return true to include the transformed key-pair in the output slice.
// If the include argument is nil, all key-pairs will be included in the output slice.
// The transform argument transforms the key-pair into a single element. It cannot be nil.
// This function returns an error if and only if either the include or transform functions return an error.
func MapToSliceFilter[M map[K]V, K comparable, V any, E any](m M, include func(K, V) (bool, error), transform func(K, V) (E, error)) ([]E, error) {
	s := make([]E, 0, len(m))
	if transform == nil {
		return nil, errors.New("Transform function cannot be nil")
	}

	for k, v := range m {
		if include != nil {
			keep, err := include(k, v)
			if err != nil {
				return nil, fmt.Errorf("Failed to include map entry: %w", err)
			}

			if !keep {
				continue
			}
		}

		e, err := transform(k, v)
		if err != nil {
			return nil, fmt.Errorf("Failed to transform map entry: %w", err)
		}

		s = append(s, e)
	}

	return s, nil
}

// MapToSlice transforms the map into a slice.
// The transform argument transforms the key-pair into a single element. It cannot be nil.
// This function returns an error if and only if the transform function returns an error.
func MapToSlice[M map[K]V, K comparable, V any, E any](m M, t func(K, V) (E, error)) ([]E, error) {
	return MapToSliceFilter(m, nil, t)
}

// MapToMapFilter filters and transforms the map into a different map. The include argument is a filter.
// It should return true to include the transformed key-pair in the output map.
// If the include argument is nil, all key-pairs will be included in the output map.
// The transform argument transforms the key-pair into a new key-pair. It cannot be nil.
// This function returns an error if and only if either the include or transform functions return an error.
func MapToMapFilter[M map[K]V, K comparable, V any, K2 comparable, V2 any](m M, include func(K, V) (bool, error), transform func(K, V) (K2, V2, error)) (map[K2]V2, error) {
	m2 := make(map[K2]V2, len(m))
	if transform == nil {
		return nil, errors.New("Transform function cannot be nil")
	}

	for k, v := range m {
		if include != nil {
			keep, err := include(k, v)
			if err != nil {
				return nil, fmt.Errorf("Failed to filter map entry: %w", err)
			}

			if !keep {
				continue
			}
		}

		k2, v2, err := transform(k, v)
		if err != nil {
			return nil, fmt.Errorf("Failed to transform map entry: %w", err)
		}

		m2[k2] = v2
	}

	return m2, nil
}

func MapFilter[M map[K]V, K comparable, V any](m M, include func(K, V) (bool, error)) (M, error) {
	return MapToMapFilter(m, include, func(k K, v V) (K, V, error) {
		return k, v, nil
	})
}

// SliceToSliceFilter filters and transforms the slice into a different slice. The include argument is a filter.
// It should return true to include the transformed element in the output slice.
// If the include argument is nil, all elements will be included in the output slice.
// The transform argument transforms the element into a new element. It cannot be nil.
// This function returns an error if and only if either the include or transform functions return an error.
func SliceToSliceFilter[S []E, E any, E2 any](s S, include func(i int, e E) (bool, error), transform func(i int, e E) (E2, error)) ([]E2, error) {
	s2 := make([]E2, 0, len(s))
	if transform == nil {
		return nil, errors.New("Transform function cannot be nil")
	}

	for i, e := range s {
		if include != nil {
			keep, err := include(i, e)
			if err != nil {
				return nil, fmt.Errorf("Failed to filter slice element: %w", err)
			}

			if !keep {
				continue
			}
		}

		e2, err := transform(i, e)
		if err != nil {
			return nil, fmt.Errorf("Failed to transform slice element: %w", err)
		}

		s2 = append(s2, e2)
	}

	return s2, nil
}

// SliceToMapFilter filters and transforms the slice into a map. The include argument is a filter.
// It should return true to include the transformed element in the output map.
// If the include argument is nil, all elements will be included in the output map.
// The transform argument transforms the element into a key-pair. It cannot be nil.
// This function returns an error if and only if either the include or transform functions return an error.
func SliceToMapFilter[S []E, E any, K comparable, V any](s S, include func(i int, e E) (bool, error), transform func(i int, e E) (K, V, error)) (map[K]V, error) {
	m := make(map[K]V, len(s))
	if transform == nil {
		return nil, errors.New("Transform function cannot be nil")
	}

	for i, e := range s {
		if include != nil {
			keep, err := include(i, e)
			if err != nil {
				return nil, fmt.Errorf("Failed to filter slice element: %w", err)
			}

			if !keep {
				continue
			}
		}

		k, v, err := transform(i, e)
		if err != nil {
			return nil, fmt.Errorf("Failed to transform slice element: %w", err)
		}

		m[k] = v
	}

	return m, nil
}

// SliceFilter calls include on each element of the input slice and includes them in the output slice if true.
// This function returns an error if and only if the include function returns an error.
func SliceFilter[S []E, E any](s S, include func(i int, e E) (bool, error)) ([]E, error) {
	return SliceToSliceFilter(s, include, func(i int, e E) (E, error) {
		return e, nil
	})
}

// SliceToSlice calls transform on each element of the input slice and appends the result to the output slice.
// This function returns an error if and only if the include function returns an error.
func SliceToSlice[S []E, E any, E2 any](s S, t func(i int, e E) (E2, error)) ([]E2, error) {
	return SliceToSliceFilter(s, nil, t)
}

// SliceToMap calls transform on each element of the input slice and sets the resultant key-pair in the output map.
// This function returns an error if and only if the include function returns an error.
func SliceToMap[S []E, E any, K comparable, V any](s S, t func(i int, e E) (K, V, error)) (map[K]V, error) {
	return SliceToMapFilter(s, nil, t)
}
