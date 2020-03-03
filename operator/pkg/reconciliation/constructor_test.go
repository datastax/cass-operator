package reconciliation

import (
	"reflect"
	"testing"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

func TestDseDatacenter_buildLabelSelectorForSeedService(t *testing.T) {
	dc := &datastaxv1alpha1.DseDatacenter{
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			DseClusterName: "bob",
		},
	}
	want := map[string]string{
		datastaxv1alpha1.ClusterLabel:  "bob",
		datastaxv1alpha1.SeedNodeLabel: "true",
	}

	got := buildLabelSelectorForSeedService(dc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("buildLabelSelectorForSeedService = %v, want %v", got, want)
	}
}

func Test_calculatePodAntiAffinity(t *testing.T) {
	t.Run("check when we allow more than one dse pod per node", func(t *testing.T) {
		paa := calculatePodAntiAffinity(true)
		if paa != nil {
			t.Errorf("calculatePodAntiAffinity() = %v, and we want nil", paa)
		}
	})

	t.Run("check when we do not allow more than one dse pod per node", func(t *testing.T) {
		paa := calculatePodAntiAffinity(false)
		if paa == nil ||
			len(paa.RequiredDuringSchedulingIgnoredDuringExecution) != 1 {
			t.Errorf("calculatePodAntiAffinity() = %v, and we want one element in RequiredDuringSchedulingIgnoredDuringExecution", paa)
		}
	})
}

func Test_calculateNodeAffinity(t *testing.T) {
	t.Run("check when we dont have a zone we want to use", func(t *testing.T) {
		na := calculateNodeAffinity("")
		if na != nil {
			t.Errorf("calculateNodeAffinity() = %v, and we want nil", na)
		}
	})

	t.Run("check when we do not allow more than one dse pod per node", func(t *testing.T) {
		na := calculateNodeAffinity("thezone")
		if na == nil ||
			na.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			t.Errorf("calculateNodeAffinity() = %v, and we want a non-nil RequiredDuringSchedulingIgnoredDuringExecution", na)
		}
	})
}

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
