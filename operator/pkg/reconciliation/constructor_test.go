// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/stretchr/testify/assert"
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

func TestCassandraDatacenter_buildInitContainer_resources_set(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			ConfigBuilderResources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
					"memory": *resource.NewScaledQuantity(1, resource.Giga),
				},
				Requests: corev1.ResourceList{
					"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
					"memory": *resource.NewScaledQuantity(1, resource.Giga),
				},
			},
		},
	}

	initContainers, err := buildInitContainers(dc, "testRack")
	assert.NotNil(t, initContainers, "Unexpected init containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, initContainers, 1, "Unexpected number of init containers returned")
	if !reflect.DeepEqual(dc.Spec.ConfigBuilderResources, initContainers[0].Resources) {
		t.Errorf("system-config-init container resources not correctly set")
	}
}

func TestCassandraDatacenter_buildInitContainer_resources_set_when_not_specified(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
		},
	}

	initContainers, err := buildInitContainers(dc, "testRack")
	assert.NotNil(t, initContainers, "Unexpected init containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, initContainers, 1, "Unexpected number of init containers returned")
	if !reflect.DeepEqual(initContainers[0].Resources, DefaultsConfigInitContainer) {
		t.Error("Unexpected default resources allocated for the init container.")
	}
}

func TestCassandraDatacenter_buildContainers_systemlogger_resources_set(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			SystemLoggerResources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
					"memory": *resource.NewScaledQuantity(1, resource.Giga),
				},
				Requests: corev1.ResourceList{
					"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
					"memory": *resource.NewScaledQuantity(1, resource.Giga),
				},
			},
		},
	}

	containers, err := buildContainers(dc, []corev1.VolumeMount{})
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 2, "Unexpected number of containers containers returned")
	assert.Equal(t, containers[1].Resources, dc.Spec.SystemLoggerResources,
		"server-system-logger container resources are unexpected")
}

func TestCassandraDatacenter_buildContainers_systemlogger_resources_set_when_not_specified(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
		},
	}

	containers, err := buildContainers(dc, []corev1.VolumeMount{})
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 2, "Unexpected number of containers containers returned")
	if !reflect.DeepEqual(containers[1].Resources, DefaultsLoggerContainer) {
		t.Error("server-system-logger container resources are not set to default values.")
	}
}

func TestCassandraDatacenter_buildContainers_reaper_resources(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			Reaper: &api.ReaperConfig{
				Enabled: true,
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
						"memory": *resource.NewScaledQuantity(1, resource.Giga),
					},
					Requests: corev1.ResourceList{
						"cpu":    *resource.NewMilliQuantity(1, resource.DecimalSI),
						"memory": *resource.NewScaledQuantity(1, resource.Giga),
					},
				},
			},
		},
	}

	containers, err := buildContainers(dc, []corev1.VolumeMount{})
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 3, "Unexpected number of containers containers returned")
	assert.Equal(t, containers[2].Resources, dc.Spec.Reaper.Resources,
		"reaper container resources have unexpected values.")
}

func TestCassandraDatacenter_buildContainers_reaper_resources_set_when_not_specified(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			Reaper: &api.ReaperConfig{
				Enabled: true,
			},
		},
	}

	containers, err := buildContainers(dc, []corev1.VolumeMount{})
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 3, "Unexpected number of containers containers returned")
	if !reflect.DeepEqual(containers[2].Resources, DefaultsReaperContainer) {
		t.Error("reaper container resources are not set to the default values.")
	}
}

func TestCassandraDatacenter_buildPodTemplateSpec_containers_merge(t *testing.T) {
	testContainer := corev1.Container{}
	testContainer.Name = "test-container"
	testContainer.Image = "test-image"
	testContainer.Env = []corev1.EnvVar{
		{Name: "TEST_VAL", Value: "TEST"},
	}

	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{testContainer}},
			},
		},
	}
	got, err := buildPodTemplateSpec(dc, "testzone", "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")
	assert.Equal(t, 3, len(got.Spec.Containers))
	if !reflect.DeepEqual(testContainer, got.Spec.Containers[2]) {
		t.Errorf("third container = %v, want %v", got, testContainer)
	}
}

func TestCassandraDatacenter_buildPodTemplateSpec_initcontainers_merge(t *testing.T) {
	testContainer := corev1.Container{}
	testContainer.Name = "test-container-init"
	testContainer.Image = "test-image-init"
	testContainer.Env = []corev1.EnvVar{
		{Name: "TEST_VAL", Value: "TEST"},
	}

	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{testContainer}},
			},
			ConfigBuilderResources: testContainer.Resources,
		},
	}
	got, err := buildPodTemplateSpec(dc, "testzone", "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")
	assert.Equal(t, 2, len(got.Spec.InitContainers))
	if !reflect.DeepEqual(testContainer, got.Spec.InitContainers[1]) {
		t.Errorf("second init container = %v, want %v", got, testContainer)
	}
}

func TestCassandraDatacenter_buildPodTemplateSpec_labels_merge(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:     "bob",
			ServerType:      "cassandra",
			ServerVersion:   "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{},
		},
	}
	dc.Spec.PodTemplateSpec.Labels = map[string]string{"abc": "123"}

	spec, err := buildPodTemplateSpec(dc, "testzone", "testrack")
	got := spec.Labels

	expected := dc.GetRackLabels("testrack")
	expected[api.CassNodeState] = stateReadyToStart
	expected["app.kubernetes.io/managed-by"] = oplabels.ManagedByLabelValue
	expected["abc"] = "123"

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("labels = %v, want %v", got, expected)
	}
}
