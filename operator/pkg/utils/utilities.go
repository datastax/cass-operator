// Copyright DataStax, Inc.
// Please see the included license file for details.

package utils

// MergeMap will take two maps, merging the entries of the source map into destination map. If both maps share the same key
// then destination's value for that key will be overwritten with what's in source.
func MergeMap(destination map[string]string, sources ...map[string]string) map[string]string {
	for _, source := range sources {
		for k, v := range source {
			destination[k] = v
		}
	}

	return destination
}

// SearchMap will recursively search a map looking for a key with a value of another map
func SearchMap(mapToSearch map[string]interface{}, key string) map[string]interface{} {

	if v, ok := mapToSearch[key]; ok {
		return v.(map[string]interface{})
	}

	for _, v := range mapToSearch {
		switch v.(type) {
		case map[string]interface{}:

			if foundMap := SearchMap(v.(map[string]interface{}), key); len(foundMap) != 0 {
				return foundMap
			}
		}
	}

	return make(map[string]interface{})
}
