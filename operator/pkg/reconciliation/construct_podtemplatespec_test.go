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
		na := calculateNodeAffinity(map[string]string{})
		if na != nil {
			t.Errorf("calculateNodeAffinity() = %v, and we want nil", na)
		}
	})

	t.Run("check when we do not allow more than one dse pod per node", func(t *testing.T) {
		na := calculateNodeAffinity(map[string]string{zoneLabel: "thezone"})
		if na == nil ||
			na.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			t.Errorf("calculateNodeAffinity() = %v, and we want a non-nil RequiredDuringSchedulingIgnoredDuringExecution", na)
		}
	})
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

	podTemplateSpec := corev1.PodTemplateSpec{}
	err := buildInitContainers(dc, "testRack", &podTemplateSpec)
	initContainers := podTemplateSpec.Spec.InitContainers
	assert.NotNil(t, initContainers, "Unexpected init containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, initContainers, 1, "Unexpected number of init containers returned")
	if !reflect.DeepEqual(dc.Spec.ConfigBuilderResources, initContainers[0].Resources) {
		t.Errorf("system-config-init container resources not correctly set")
		t.Errorf("got: %v", initContainers[0])
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

	podTemplateSpec := corev1.PodTemplateSpec{}
	err := buildInitContainers(dc, "testRack", &podTemplateSpec)
	initContainers := podTemplateSpec.Spec.InitContainers
	assert.NotNil(t, initContainers, "Unexpected init containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, initContainers, 1, "Unexpected number of init containers returned")
	if !reflect.DeepEqual(initContainers[0].Resources, DefaultsConfigInitContainer) {
		t.Error("Unexpected default resources allocated for the init container.")
	}
}

func TestCassandraDatacenter_buildInitContainer_with_overrides(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				corev1.Container{
					Name: ServerConfigContainerName,
					Env: []corev1.EnvVar{
						corev1.EnvVar{
							Name:  "k1",
							Value: "v1",
						},
					},
				},
			},
		},
	}

	err := buildInitContainers(dc, "testRack", podTemplateSpec)
	initContainers := podTemplateSpec.Spec.InitContainers
	assert.NotNil(t, initContainers, "Unexpected init containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, initContainers, 1, "Unexpected number of init containers returned")
	if !reflect.DeepEqual(initContainers[0].Resources, DefaultsConfigInitContainer) {
		t.Error("Unexpected default resources allocated for the init container.")
	}
	if !reflect.DeepEqual(initContainers[0].Env[0],
		corev1.EnvVar{
			Name:  "k1",
			Value: "v1",
		}) {
		t.Errorf("Unexpected env vars allocated for the init container: %v", initContainers[0].Env)
	}
	if !reflect.DeepEqual(initContainers[0].Env[4],
		corev1.EnvVar{
			Name:  "USE_HOST_IP_FOR_BROADCAST",
			Value: "false",
		}) {
		t.Errorf("Unexpected env vars allocated for the init container: %v", initContainers[0].Env)
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

	podTemplateSpec := &corev1.PodTemplateSpec{}
	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
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

	podTemplateSpec := &corev1.PodTemplateSpec{}
	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
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

	podTemplateSpec := &corev1.PodTemplateSpec{}
	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
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

	podTemplateSpec := &corev1.PodTemplateSpec{}
	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 3, "Unexpected number of containers containers returned")
	if !reflect.DeepEqual(containers[2].Resources, DefaultsReaperContainer) {
		t.Error("reaper container resources are not set to the default values.")
	}
}

func TestCassandraDatacenter_buildContainers_use_cassandra_settings(t *testing.T) {
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

	cassContainer := corev1.Container{
		Name: "cassandra",
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "k1",
				Value: "v1",
			},
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{}
	podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, cassContainer)

	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 3, "Unexpected number of containers containers returned")
	if !reflect.DeepEqual(containers[2].Resources, DefaultsReaperContainer) {
		t.Error("reaper container resources are not set to the default values.")
	}

	if !reflect.DeepEqual(containers[0].Env[0].Name, "k1") {
		t.Errorf("Unexpected env vars allocated for the cassandra container: %v", containers[0].Env)
	}
}

func TestCassandraDatacenter_buildContainers_override_other_containers(t *testing.T) {
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

	podTemplateSpec := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				corev1.Container{
					Name: SystemLoggerContainerName,
					VolumeMounts: []corev1.VolumeMount{
						corev1.VolumeMount{
							Name:      "extra",
							MountPath: "/extra",
						},
					},
				},
			},
		},
	}

	err := buildContainers(dc, podTemplateSpec)
	containers := podTemplateSpec.Spec.Containers
	assert.NotNil(t, containers, "Unexpected containers containers received")
	assert.Nil(t, err, "Unexpected error encountered")

	assert.Len(t, containers, 3, "Unexpected number of containers containers returned")
	if !reflect.DeepEqual(containers[2].Resources, DefaultsReaperContainer) {
		t.Error("reaper container resources are not set to the default values.")
	}

	if !reflect.DeepEqual(containers[0].VolumeMounts,
		[]corev1.VolumeMount{
			corev1.VolumeMount{
				Name:      "extra",
				MountPath: "/extra",
			},
			corev1.VolumeMount{
				Name:      "server-logs",
				MountPath: "/var/log/cassandra",
			},
		}) {
		t.Errorf("Unexpected volume mounts for the logger container: %v", containers[0].VolumeMounts)
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
	got, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")
	assert.Equal(t, 3, len(got.Spec.Containers))
	if !reflect.DeepEqual(testContainer, got.Spec.Containers[0]) {
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
	got, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")
	assert.Equal(t, 2, len(got.Spec.InitContainers))
	if !reflect.DeepEqual(testContainer, got.Spec.InitContainers[0]) {
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

	spec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")
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

// A volume added to ServerConfigContainerName should be added to all built-in containers
func TestCassandraDatacenter_buildPodTemplateSpec_propagate_volumes(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: ServerConfigContainerName,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "extra",
									MountPath: "/extra",
								},
							},
						},
					},
				},
			},
		},
	}

	spec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")
	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")

	if !reflect.DeepEqual(spec.Spec.InitContainers[0].VolumeMounts,
		[]corev1.VolumeMount{
			{
				Name:      "extra",
				MountPath: "/extra",
			},
			{
				Name:      "server-config",
				MountPath: "/config",
			},
		}) {
		t.Errorf("Unexpected volume mounts for the init config container: %v", spec.Spec.InitContainers[0].VolumeMounts)
	}

	if !reflect.DeepEqual(spec.Spec.Containers[0].VolumeMounts,
		[]corev1.VolumeMount{
			{
				Name:      "server-logs",
				MountPath: "/var/log/cassandra",
			},
			{
				Name:      "server-data",
				MountPath: "/var/lib/cassandra",
			},
			{
				Name:      "encryption-cred-storage",
				MountPath: "/etc/encryption/",
			},
			{
				Name:      "extra",
				MountPath: "/extra",
			},
			{
				Name:      "server-config",
				MountPath: "/config",
			},
		}) {
		t.Errorf("Unexpected volume mounts for the cassandra container: %v", spec.Spec.Containers[0].VolumeMounts)
	}

	// Logger just gets the logs
	if !reflect.DeepEqual(spec.Spec.Containers[1].VolumeMounts,
		[]corev1.VolumeMount{
			{
				Name:      "server-logs",
				MountPath: "/var/log/cassandra",
			},
		}) {
		t.Errorf("Unexpected volume mounts for the logger container: %v", spec.Spec.Containers[1].VolumeMounts)
	}
}

func TestCassandraDatacenter_buildContainers_DisableSystemLoggerSidecar(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:                "bob",
			ServerType:                 "cassandra",
			ServerVersion:              "3.11.7",
			PodTemplateSpec:            nil,
			DisableSystemLoggerSidecar: true,
			SystemLoggerImage:          "alpine",
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{}

	err := buildContainers(dc, podTemplateSpec)

	assert.NoError(t, err, "should not have gotten error from calling buildContainers()")

	assert.Len(t, podTemplateSpec.Spec.Containers, 1, "should have one container in the podTemplateSpec")
	assert.Equal(t, "cassandra", podTemplateSpec.Spec.Containers[0].Name)
}

func TestCassandraDatacenter_buildContainers_EnableSystemLoggerSidecar_CustomImage(t *testing.T) {
	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:                "bob",
			ServerType:                 "cassandra",
			ServerVersion:              "3.11.7",
			PodTemplateSpec:            nil,
			DisableSystemLoggerSidecar: false,
			SystemLoggerImage:          "alpine",
		},
	}

	podTemplateSpec := &corev1.PodTemplateSpec{}

	err := buildContainers(dc, podTemplateSpec)

	assert.NoError(t, err, "should not have gotten error from calling buildContainers()")

	assert.Len(t, podTemplateSpec.Spec.Containers, 2, "should have two containers in the podTemplateSpec")
	assert.Equal(t, "cassandra", podTemplateSpec.Spec.Containers[0].Name)
	assert.Equal(t, "server-system-logger", podTemplateSpec.Spec.Containers[1].Name)

	assert.Equal(t, "alpine", podTemplateSpec.Spec.Containers[1].Image)
}
