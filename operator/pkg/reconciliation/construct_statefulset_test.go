// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"testing"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

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
						ServerVersion: "3.11.7",
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

func Test_newStatefulSetForCassandraDatacenter_rackNodeAffinitylabels(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:     "bob",
			ServerType:      "cassandra",
			ServerVersion:   "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{},
			NodeAffinityLabels: map[string]string{"dclabel1": "dcvalue1", "dclabel2": "dcvalue2"},
			Racks: []api.Rack{
				{
					Name: "rack1",
					Zone: "z1",
					NodeAffinityLabels: map[string]string{"r1label1": "r1value1", "r1label2": "r1value2"},
				},
			},
		},
	}
	var nodeAffinityLabels map[string]string
	var nodeAffinityLabelsConfigurationError error

	nodeAffinityLabels, nodeAffinityLabelsConfigurationError = rackNodeAffinitylabels(dc, "rack1")

	assert.NoError(t, nodeAffinityLabelsConfigurationError,
		"should not have gotten error when getting NodeAffinitylabels of rack rack1")

	expected := map[string]string {
		"dclabel1": "dcvalue1",
		"dclabel2": "dcvalue2",
		"r1label1": "r1value1",
		"r1label2": "r1value2",
		zoneLabel:  "z1",
	}

	assert.Equal(t, expected, nodeAffinityLabels)
}

func Test_newStatefulSetForCassandraDatacenterWithAdditionalVolumes(t *testing.T) {
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
						ClusterName: "c1",
						StorageConfig: api.StorageConfig{
							CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{},
							AdditionalVolumes: api.AdditionalVolumesSlice{
								api.AdditionalVolumes{
									MountPath: "/var/log/cassandra",
									Name:      "cassandra-logs",
									PVCSpec:   &corev1.PersistentVolumeClaimSpec{},
								},
								api.AdditionalVolumes{
									MountPath: "/var/lib/cassandra/commitlog",
									Name:      "cassandra-commitlogs",
									PVCSpec:   &corev1.PersistentVolumeClaimSpec{},
								},
							},
						},
						ServerType:    "cassandra",
						ServerVersion: "3.11.7",
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
		assert.Equal(t, 3, len(got.Spec.VolumeClaimTemplates))
	}
}