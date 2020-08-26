package utils

import (
	"testing"
	appsv1 "k8s.io/api/apps/v1"
)


func Test_deepHashString(t *testing.T) {

	t.Run("test hash behavior", func(t *testing.T) {
		var ss1 appsv1.StatefulSet
		var ss2 appsv1.StatefulSet

		ss1.Labels = map[string]string{"abc": "123"}
		ss2.Labels = map[string]string{"def": "456"}

		hash1 := deepHashString(&ss1)
		hash2 := deepHashString(&ss2)

		if hash1 == hash2 {
			t.Errorf("deepHash did not produce different hashes %s %s", hash1, hash2)
		}

		var d1 appsv1.Deployment

		hash3 := deepHashString(&d1)

		if hash1 == hash3 {
			t.Errorf("deepHash did not produce different hashes %s %s", hash1, hash3)
		}

		ss1.Labels["def"] = "456"
		ss2.Labels["abc"] = "123"

		hash4 := deepHashString(&ss1)
		hash5 := deepHashString(&ss2)

		if hash4 != hash5 {
			t.Errorf("deepHash should have produced the same hash %s %s", hash4, hash5)
		}
	})
}