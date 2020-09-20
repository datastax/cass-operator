// Copyright DataStax, Inc.
// Please see the included license file for details.

package utils

import (
	"os"
	"strings"
	"reflect"
	"math"
)

func IsPSPEnabled() bool {
	value, exists := os.LookupEnv("ENABLE_VMWARE_PSP")
	return exists && "true" == strings.TrimSpace(value)
}

func RangeInt(min, max, step int) []int {
	size := int(math.Ceil(float64((max - min)) / float64(step)))
	l := make([]int, size)
	for i := 0; i < size; i++ {
		l[i] = min + i * step
	}
	return l
}

func isArrayOrSlice(a interface{}) bool {
	t := reflect.TypeOf(a)
	k := t.Kind()
	return k == reflect.Slice || k == reflect.Array
}

func DeepEqualArrayIgnoreOrder(a interface{}, b interface{}) bool {
	if !isArrayOrSlice(a) || !isArrayOrSlice(b) {
		return false
	}

	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)

	if aValue.Len() != bValue.Len() {
		return false
	}

	idxs := RangeInt(0, bValue.Len(), 1)

	for i := 0; i < aValue.Len(); i++ {
		e1 := aValue.Index(i)

		foundIndex := -1
		for k, bIndex := range idxs {
			e2 := bValue.Index(bIndex)
			if reflect.DeepEqual(e1.Interface(), e2.Interface()) {
				foundIndex = k
				break
			}
		}
		if foundIndex > -1 {
			idxs = append(idxs[:foundIndex], idxs[foundIndex+1:]...)
		} else {
			return false
		}
	}
	return true
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