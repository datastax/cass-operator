// Copyright DataStax, Inc.
// Please see the included license file for details.

package utils

import (
	"os"
	"strings"
)

func IsPSPEnabled() bool {
	value, exists := os.LookupEnv("ENABLE_VMWARE_PSP")
	return exists && "true" == strings.TrimSpace(value)
}

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

func IndexOfString(a []string, v string) int {
	foundIdx := -1
	for idx, item := range a {
		if item == v {
			foundIdx = idx
			break
		}
	}

	return foundIdx
}

func RemoveValueFromStringArray(a []string, v string) []string {
	foundIdx := IndexOfString(a, v)

	if foundIdx > -1 {
		copy(a[foundIdx:], a[foundIdx+1:])
		a[len(a)-1] = ""
		a = a[:len(a)-1]
	}
	return a
}

func AppendValuesToStringArrayIfNotPresent(a []string, values ...string) []string {
	for _, v := range values {
		idx := IndexOfString(a, v)
		if idx < 0 {
			// array does not contain this value, so add it
			a = append(a, v)
		}
	}
	return a
}