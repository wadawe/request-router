// utils_structs.go
// This file contains utility functions for working with structs

package utils

// Extract all keys from a map
func GetMapKeys[K comparable, V any](m map[K]V) []K {
	mapKeys := make([]K, 0, len(m))
	for k := range m {
		mapKeys = append(mapKeys, k)
	}
	return mapKeys
}
