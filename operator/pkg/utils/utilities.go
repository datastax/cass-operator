package utils

// MergeMap will take two maps, merging the entries of the source map into destination map. If both maps share the same key
// then destination's value for that key will be overwritten with what's in source.
func MergeMap(destination *map[string]string, source map[string]string) {
	for k, v := range source {
		(*destination)[k] = v
	}
}
