// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"reflect"
	"testing"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestCassandraDatacenter_buildLabelSelectorForSeedService(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName: "bob",
		},
	}
	want := map[string]string{
		api.ClusterLabel:  "bob",
		api.SeedNodeLabel: "true",
	}

	got := buildLabelSelectorForSeedService(dc)

	if !reflect.DeepEqual(want, got) {
		t.Errorf("buildLabelSelectorForSeedService = %v, want %v", got, want)
	}
}

func Test_calculatePodAntiAffinity(t *testing.T) {
	t.Run("check when we allow more than one server pod per node", func(t *testing.T) {
		paa := calculatePodAntiAffinity(true)
		if paa != nil {
			t.Errorf("calculatePodAntiAffinity() = %v, and we want nil", paa)
		}
	})

	t.Run("check when we do not allow more than one server pod per node", func(t *testing.T) {
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

func Test_newStatefulSetForCassandraDatacenter(t *testing.T) {
	type args struct {
		rackName     string
		dc           *api.CassandraDatacenter
		replicaCount int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test nodeSelector",
			args: args{
				rackName:     "r1",
				replicaCount: 1,
				dc: &api.CassandraDatacenter{
					Spec: api.CassandraDatacenterSpec{
						ClusterName:  "c1",
						NodeSelector: map[string]string{"dedicated": "cassandra"},
						StorageConfig: api.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{},
						},
						ServerType:    "cassandra",
						ServerVersion: "3.11.6",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Log(tt.name)
		got, err := newStatefulSetForCassandraDatacenter(tt.args.rackName, tt.args.dc, tt.args.replicaCount)
		assert.NoError(t, err, "newStatefulSetForCassandraDatacenter should not have errored")
		assert.NotNil(t, got, "newStatefulSetForCassandraDatacenter should not have returned a nil statefulset")
		assert.Equal(t, map[string]string{"dedicated": "cassandra"}, got.Spec.Template.Spec.NodeSelector)
	}
}
