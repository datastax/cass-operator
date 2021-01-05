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
				{
					Name: ServerConfigContainerName,
					Env: []corev1.EnvVar{
						{
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

func TestCassandraDatacenter_buildPodTemplateSpec_add_initContainer_after_config_initContainer(t *testing.T) {
	// When adding an initContainer with podTemplate spec it will run before
	// the server config initContainer by default. This test demonstrates and
	// verifies how to specify the initContainer to run after the server config
	// initContainer.

	initContainer := corev1.Container{
		Name: "test-container",
		Image: "test-image",
	}

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
						},
						initContainer,
					},
				},
			},
		},
	}

	podTemplateSpec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")

	initContainers := podTemplateSpec.Spec.InitContainers
	assert.Equal(t, 2, len(initContainers))
	assert.Equal(t, initContainers[0].Name, ServerConfigContainerName)
	assert.Equal(t, initContainers[1].Name, initContainer.Name)
}

func TestCassandraDatacenter_buildPodTemplateSpec_add_initContainer_with_volumes(t *testing.T) {
	// This test adds an initContainer, a new volume, a volume mount for the
	// new volume, and mounts for existing volumes. Not only does the test
	// verify that the initContainer has the correct volumes, but it also
	// verifies that the "built-in" containers have the correct mounts.

	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name: "test",
							Image: "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "server-data",
									MountPath: "/var/lib/cassandra",
								},
								{
									Name: "server-config",
									MountPath: "/config",
								},
								{
									Name: "test-data",
									MountPath: "/test",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test-data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	podTemplateSpec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")

	initContainers := podTemplateSpec.Spec.InitContainers

	assert.Equal(t, 2, len(initContainers))
	assert.Equal(t, "test", initContainers[0].Name)
	assert.Equal(t, 3, len(initContainers[0].VolumeMounts))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumeMountsContains(initContainers[0].VolumeMounts, volumeMountNameMatcher(PvcName)))
	assert.True(t, volumeMountsContains(initContainers[0].VolumeMounts, volumeMountNameMatcher("test-data")))
	assert.True(t, volumeMountsContains(initContainers[0].VolumeMounts, volumeMountNameMatcher("server-config")))

	assert.Equal(t, ServerConfigContainerName, initContainers[1].Name)
	assert.Equal(t, 1, len(initContainers[1].VolumeMounts))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumeMountsContains(initContainers[1].VolumeMounts, volumeMountNameMatcher("server-config")))

	volumes := podTemplateSpec.Spec.Volumes
	assert.Equal(t, 4, len(volumes))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumesContains(volumes, volumeNameMatcher("server-config")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("test-data")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("server-logs")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("encryption-cred-storage")))

	containers := podTemplateSpec.Spec.Containers
	assert.Equal(t, 2, len(containers))

	cassandraContainer := findContainer(containers, CassandraContainerName)
	assert.NotNil(t, cassandraContainer)

	cassandraVolumeMounts := cassandraContainer.VolumeMounts
	assert.Equal(t, 4, len(cassandraVolumeMounts))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-config")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-logs")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("encryption-cred-storage")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-data")))

	loggerContainer := findContainer(containers, SystemLoggerContainerName)
	assert.NotNil(t, loggerContainer)

	loggerVolumeMounts := loggerContainer.VolumeMounts
	assert.Equal(t, 1, len(loggerVolumeMounts))
	assert.True(t, volumeMountsContains(loggerVolumeMounts, volumeMountNameMatcher("server-logs")))
}

func TestCassandraDatacenter_buildPodTemplateSpec_add_container_with_volumes(t *testing.T) {
	// This test adds a container, a new volume, a volume mount for the
	// new volume, and mounts for existing volumes. Not only does the test
	// verify that the container has the correct volumes, but it also verifies
	// that the "built-in" containers have the correct mounts.

	dc := &api.CassandraDatacenter{
		Spec: api.CassandraDatacenterSpec{
			ClusterName:   "bob",
			ServerType:    "cassandra",
			ServerVersion: "3.11.7",
			PodTemplateSpec: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Image: "test",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "server-data",
									MountPath: "/var/lib/cassandra",
								},
								{
									Name: "server-config",
									MountPath: "/config",
								},
								{
									Name: "test-data",
									MountPath: "/test",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test-data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	podTemplateSpec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")

	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")

	initContainers := podTemplateSpec.Spec.InitContainers

	assert.Equal(t, 1, len(initContainers))
	assert.Equal(t, ServerConfigContainerName, initContainers[0].Name)

	serverConfigInitContainer := initContainers[0]
	assert.Equal(t, 1, len(serverConfigInitContainer.VolumeMounts))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumeMountsContains(serverConfigInitContainer.VolumeMounts, volumeMountNameMatcher("server-config")))

	volumes := podTemplateSpec.Spec.Volumes
	assert.Equal(t, 4, len(volumes))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumesContains(volumes, volumeNameMatcher("server-config")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("test-data")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("server-logs")))
	assert.True(t, volumesContains(volumes, volumeNameMatcher("encryption-cred-storage")))

	containers := podTemplateSpec.Spec.Containers
	assert.Equal(t, 3, len(containers))

	testContainer := findContainer(containers, "test")
	assert.NotNil(t, testContainer)

	assert.Equal(t, 3, len(testContainer.VolumeMounts))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumeMountsContains(testContainer.VolumeMounts, volumeMountNameMatcher(PvcName)))
	assert.True(t, volumeMountsContains(testContainer.VolumeMounts, volumeMountNameMatcher("test-data")))
	assert.True(t, volumeMountsContains(testContainer.VolumeMounts, volumeMountNameMatcher("server-config")))

	cassandraContainer := findContainer(containers, CassandraContainerName)
	assert.NotNil(t, cassandraContainer)

	cassandraVolumeMounts := cassandraContainer.VolumeMounts
	assert.Equal(t, 4, len(cassandraVolumeMounts))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-config")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-logs")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("encryption-cred-storage")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-data")))

	loggerContainer := findContainer(containers, SystemLoggerContainerName)
	assert.NotNil(t, loggerContainer)

	loggerVolumeMounts := loggerContainer.VolumeMounts
	assert.Equal(t, 1, len(loggerVolumeMounts))
	assert.True(t, volumeMountsContains(loggerVolumeMounts, volumeMountNameMatcher("server-logs")))
}

type VolumeMountMatcher func(volumeMount corev1.VolumeMount) bool

type VolumeMatcher func(volume corev1.Volume) bool

func volumeNameMatcher(name string) VolumeMatcher {
	return func(volume corev1.Volume) bool {
		return volume.Name == name
	}
}

func volumeMountNameMatcher(name string) VolumeMountMatcher {
	return func(volumeMount corev1.VolumeMount) bool {
		return volumeMount.Name == name
	}
}

func volumeMountsContains(volumeMounts []corev1.VolumeMount, matcher VolumeMountMatcher) bool {
	for _, mount := range volumeMounts {
		if matcher(mount) {
			return true
		}
	}
	return false
}

func volumesContains(volumes []corev1.Volume, matcher VolumeMatcher) bool {
	for _, volume := range volumes {
		if matcher(volume) {
			return true
		}
	}
	return false
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for _, container := range containers {
		if container.Name == name {
			return &container
		}
	}
	return nil
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

func TestCassandraDatacenter_buildPodTemplateSpec_do_not_propagate_volumes(t *testing.T) {
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
					Volumes: []corev1.Volume{
						{
							Name: "extra",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	spec, err := buildPodTemplateSpec(dc, map[string]string{zoneLabel: "testzone"}, "testrack")
	assert.NoError(t, err, "should not have gotten error when building podTemplateSpec")

	initContainers := spec.Spec.InitContainers

	assert.Equal(t, 1, len(initContainers))
	assert.Equal(t, ServerConfigContainerName, initContainers[0].Name)

	serverConfigInitContainer := initContainers[0]
	assert.Equal(t, 2, len(serverConfigInitContainer.VolumeMounts))
	// We use a contains check here because the ordering is not important
	assert.True(t, volumeMountsContains(serverConfigInitContainer.VolumeMounts, volumeMountNameMatcher("server-config")))
	assert.True(t, volumeMountsContains(serverConfigInitContainer.VolumeMounts, volumeMountNameMatcher("extra")))


	containers := spec.Spec.Containers
	cassandraContainer := findContainer(containers, CassandraContainerName)
	assert.NotNil(t, cassandraContainer)

	cassandraVolumeMounts := cassandraContainer.VolumeMounts
	assert.Equal(t, 4, len(cassandraVolumeMounts))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-config")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-logs")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("encryption-cred-storage")))
	assert.True(t, volumeMountsContains(cassandraVolumeMounts, volumeMountNameMatcher("server-data")))

	systemLoggerContainer := findContainer(containers, SystemLoggerContainerName)
	assert.NotNil(t, systemLoggerContainer)

	systemLoggerVolumeMounts := systemLoggerContainer.VolumeMounts
	assert.Equal(t, 1, len(systemLoggerVolumeMounts))
	assert.True(t, volumeMountsContains(systemLoggerVolumeMounts, volumeMountNameMatcher("server-logs")))
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
