// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// This file defines constructors for k8s objects

import (
	"fmt"
	"reflect"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/images"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultTerminationGracePeriodSeconds = 120
	ServerConfigContainerName            = "server-config-init"
	CassandraContainerName               = "cassandra"
	PvcName                              = "server-data"
	SystemLoggerContainerName            = "server-system-logger"
)

// calculateNodeAffinity provides a way to pin all pods within a statefulset to the same zone
func calculateNodeAffinity(zone string) *corev1.NodeAffinity {
	if zone == "" {
		return nil
	}
	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      "failure-domain.beta.kubernetes.io/zone",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{zone},
						},
					},
				},
			},
		},
	}
}

// calculatePodAntiAffinity provides a way to keep the db pods of a statefulset away from other db pods
func calculatePodAntiAffinity(allowMultipleNodesPerWorker bool) *corev1.PodAntiAffinity {
	if allowMultipleNodesPerWorker {
		return nil
	}
	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      api.ClusterLabel,
							Operator: metav1.LabelSelectorOpExists,
						},
						{
							Key:      api.DatacenterLabel,
							Operator: metav1.LabelSelectorOpExists,
						},
						{
							Key:      api.RackLabel,
							Operator: metav1.LabelSelectorOpExists,
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
}

func selectorFromFieldPath(fieldPath string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: fieldPath,
		},
	}
}

func probe(port int, path string, initDelay int, period int) *corev1.Probe {
	return &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(port),
				Path: path,
			},
		},
		InitialDelaySeconds: int32(initDelay),
		PeriodSeconds:       int32(period),
	}
}

func getJvmExtraOpts(dc *api.CassandraDatacenter) string {
	flags := ""

	if dc.Spec.DseWorkloads.AnalyticsEnabled == true {
		flags += "-Dspark-trackers=true "
	}
	if dc.Spec.DseWorkloads.GraphEnabled == true {
		flags += "-Dgraph-enabled=true "
	}
	if dc.Spec.DseWorkloads.SearchEnabled == true {
		flags += "-Dsearch-service=true"
	}
	return flags
}

func combineVolumeMountSlices(defaults []corev1.VolumeMount, overrides []corev1.VolumeMount) []corev1.VolumeMount {
	out := append([]corev1.VolumeMount{}, overrides...)
outerLoop:
	// Only add the defaults that don't have an override
	for _, volumeDefault := range defaults {
		for _, volumeOverride := range overrides {
			if volumeDefault.Name == volumeOverride.Name {
				continue outerLoop
			}
		}
		out = append(out, volumeDefault)
	}
	return out
}

func combineVolumeSlices(defaults []corev1.Volume, overrides []corev1.Volume) []corev1.Volume {
	out := append([]corev1.Volume{}, overrides...)
outerLoop:
	// Only add the defaults that don't have an override
	for _, volumeDefault := range defaults {
		for _, volumeOverride := range overrides {
			if volumeDefault.Name == volumeOverride.Name {
				continue outerLoop
			}
		}
		out = append(out, volumeDefault)
	}
	return out
}

func combinePortSlices(defaults []corev1.ContainerPort, overrides []corev1.ContainerPort) []corev1.ContainerPort {
	out := append([]corev1.ContainerPort{}, overrides...)
outerLoop:
	// Only add the defaults that don't have an override
	for _, portDefault := range defaults {
		for _, portOverride := range overrides {
			if portDefault.Name == portOverride.Name {
				continue outerLoop
			}
		}
		out = append(out, portDefault)
	}
	return out
}

func combineEnvSlices(defaults []corev1.EnvVar, overrides []corev1.EnvVar) []corev1.EnvVar {
	out := append([]corev1.EnvVar{}, overrides...)
outerLoop:
	// Only add the defaults that don't have an override
	for _, envDefault := range defaults {
		for _, envOverride := range overrides {
			if envDefault.Name == envOverride.Name {
				continue outerLoop
			}
		}
		out = append(out, envDefault)
	}
	return out
}

func addVolumes(dc *api.CassandraDatacenter, baseTemplate *corev1.PodTemplateSpec) {
	vServerConfig := corev1.Volume{}
	vServerConfig.Name = "server-config"
	vServerConfig.VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}

	vServerLogs := corev1.Volume{}
	vServerLogs.Name = "server-logs"
	vServerLogs.VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}

	vServerEncryption := corev1.Volume{}
	vServerEncryption.Name = "encryption-cred-storage"
	vServerEncryption.VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName: fmt.Sprintf("%s-keystore", dc.Name)},
	}

	volumes := []corev1.Volume{vServerConfig, vServerLogs, vServerEncryption}

	baseTemplate.Spec.Volumes = combineVolumeSlices(
		volumes, baseTemplate.Spec.Volumes)
}

// This ensure that the server-config-builder init container is properly configured.
func buildInitContainers(dc *api.CassandraDatacenter, rackName string, baseTemplate *corev1.PodTemplateSpec) error {

	serverCfg := &corev1.Container{}
	foundOverrides := false

	for i, c := range baseTemplate.Spec.InitContainers {
		if c.Name == ServerConfigContainerName {
			// Modify the existing container
			foundOverrides = true
			serverCfg = &baseTemplate.Spec.InitContainers[i]
			break
		}
	}

	serverCfg.Name = ServerConfigContainerName

	if serverCfg.Image == "" {
		serverCfg.Image = dc.GetConfigBuilderImage()
	}

	serverCfgMount := corev1.VolumeMount{
		Name:      "server-config",
		MountPath: "/config",
	}

	serverCfg.VolumeMounts = combineVolumeMountSlices([]corev1.VolumeMount{serverCfgMount}, serverCfg.VolumeMounts)

	serverCfg.Resources = *getResourcesOrDefault(&dc.Spec.ConfigBuilderResources, &DefaultsConfigInitContainer)

	// Convert the bool to a string for the env var setting
	useHostIpForBroadcast := "false"
	if dc.IsNodePortEnabled() {
		useHostIpForBroadcast = "true"
	}

	configData, err := dc.GetConfigAsJSON()
	if err != nil {
		return err
	}

	serverVersion := dc.Spec.ServerVersion

	envDefaults := []corev1.EnvVar{
		{Name: "CONFIG_FILE_DATA", Value: configData},
		{Name: "POD_IP", ValueFrom: selectorFromFieldPath("status.podIP")},
		{Name: "HOST_IP", ValueFrom: selectorFromFieldPath("status.hostIP")},
		{Name: "USE_HOST_IP_FOR_BROADCAST", Value: useHostIpForBroadcast},
		{Name: "RACK_NAME", Value: rackName},
		{Name: "PRODUCT_VERSION", Value: serverVersion},
		{Name: "PRODUCT_NAME", Value: dc.Spec.ServerType},
		// TODO remove this post 1.0
		{Name: "DSE_VERSION", Value: serverVersion},
	}

	serverCfg.Env = combineEnvSlices(envDefaults, serverCfg.Env)

	if !foundOverrides {
		// Note that append makes a copy, so we must do this after
		// serverCfg has been properly set up.
		baseTemplate.Spec.InitContainers = append(baseTemplate.Spec.InitContainers, *serverCfg)
	}

	return nil
}

// If values are provided in the matching containers in the
// PodTemplateSpec field of the dc, they will override defaults.
func buildContainers(dc *api.CassandraDatacenter, baseTemplate *corev1.PodTemplateSpec) error {

	// Create new Container structs or get references to existing ones

	cassContainer := &corev1.Container{}
	loggerContainer := &corev1.Container{}
	reaperContainer := &corev1.Container{}

	foundCass := false
	foundLogger := false
	foundReaper := false
	for i, c := range baseTemplate.Spec.Containers {
		if c.Name == CassandraContainerName {
			foundCass = true
			cassContainer = &baseTemplate.Spec.Containers[i]
		} else if c.Name == SystemLoggerContainerName {
			foundLogger = true
			loggerContainer = &baseTemplate.Spec.Containers[i]
		} else if c.Name == ReaperContainerName {
			foundReaper = true
			reaperContainer = &baseTemplate.Spec.Containers[i]
		}
	}

	// Cassandra container

	cassContainer.Name = CassandraContainerName
	if cassContainer.Image == "" {
		serverImage, err := dc.GetServerImage()
		if err != nil {
			return err
		}

		cassContainer.Image = serverImage
	}

	if reflect.DeepEqual(cassContainer.Resources, corev1.ResourceRequirements{}) {
		cassContainer.Resources = dc.Spec.Resources
	}

	if cassContainer.LivenessProbe == nil {
		cassContainer.LivenessProbe = probe(8080, "/api/v0/probes/liveness", 15, 15)
	}

	if cassContainer.ReadinessProbe == nil {
		cassContainer.ReadinessProbe = probe(8080, "/api/v0/probes/readiness", 20, 10)
	}

	if cassContainer.Lifecycle == nil {
		cassContainer.Lifecycle = &corev1.Lifecycle{}
	}

	if cassContainer.Lifecycle.PreStop == nil {
		action, err := httphelper.GetMgmtApiWgetPostAction(dc, httphelper.WgetNodeDrainEndpoint, "")
		if err != nil {
			return err
		}
		cassContainer.Lifecycle.PreStop = &corev1.Handler{
			Exec: action,
		}
	}

	// Combine env vars

	envDefaults := []corev1.EnvVar{
		{Name: "DS_LICENSE", Value: "accept"},
		{Name: "DSE_AUTO_CONF_OFF", Value: "all"},
		{Name: "USE_MGMT_API", Value: "true"},
		{Name: "MGMT_API_EXPLICIT_START", Value: "true"},
		// TODO remove this post 1.0
		{Name: "DSE_MGMT_EXPLICIT_START", Value: "true"},
	}

	if dc.Spec.ServerType == "dse" && dc.Spec.DseWorkloads != nil {
		envDefaults = append(
			envDefaults,
			corev1.EnvVar{Name: "JVM_EXTRA_OPTS", Value: getJvmExtraOpts(dc)})
	}

	cassContainer.Env = combineEnvSlices(envDefaults, cassContainer.Env)

	// Combine ports

	portDefaults, err := dc.GetContainerPorts()
	if err != nil {
		return err
	}

	cassContainer.Ports = combinePortSlices(portDefaults, cassContainer.Ports)

	// Combine volumes

	var volumeDefaults []corev1.VolumeMount
	for _, c := range baseTemplate.Spec.InitContainers {
		volumeDefaults = combineVolumeMountSlices(volumeDefaults, c.VolumeMounts)
	}

	cassServerLogsMount := corev1.VolumeMount{
		Name:      "server-logs",
		MountPath: "/var/log/cassandra",
	}
	volumeDefaults = append(volumeDefaults, cassServerLogsMount)

	volumeDefaults = append(volumeDefaults, corev1.VolumeMount{
		Name:      PvcName,
		MountPath: "/var/lib/cassandra",
	})

	volumeDefaults = append(volumeDefaults, corev1.VolumeMount{
		Name:      "encryption-cred-storage",
		MountPath: "/etc/encryption/",
	})

	cassContainer.VolumeMounts = combineVolumeMountSlices(volumeDefaults, cassContainer.VolumeMounts)

	// Server Logger Container

	loggerContainer.Name = SystemLoggerContainerName

	if loggerContainer.Image == "" {
		loggerContainer.Image = images.GetSystemLoggerImage()
	}

	if len(loggerContainer.Args) == 0 {
		loggerContainer.Args = []string{
			"/bin/sh", "-c", "tail -n+1 -F /var/log/cassandra/system.log",
		}
	}

	loggerContainer.VolumeMounts = combineVolumeMountSlices(
		[]corev1.VolumeMount{cassServerLogsMount}, loggerContainer.VolumeMounts)

	loggerContainer.Resources = *getResourcesOrDefault(&dc.Spec.SystemLoggerResources, &DefaultsLoggerContainer)

	// Reaper Container

	if dc.IsReaperEnabled() {
		buildReaperContainer(dc, reaperContainer)
	}

	// Note that append() can make copies of each element,
	// so we call it after modifying any existing elements.

	if !foundCass {
		baseTemplate.Spec.Containers = append(baseTemplate.Spec.Containers, *cassContainer)
	}

	if !foundLogger {
		baseTemplate.Spec.Containers = append(baseTemplate.Spec.Containers, *loggerContainer)
	}

	if dc.IsReaperEnabled() {
		if !foundReaper {
			baseTemplate.Spec.Containers = append(baseTemplate.Spec.Containers, *reaperContainer)
		}
	}

	return nil
}

func buildPodTemplateSpec(dc *api.CassandraDatacenter, zone string, rackName string) (*corev1.PodTemplateSpec, error) {

	baseTemplate := dc.Spec.PodTemplateSpec.DeepCopy()

	if baseTemplate == nil {
		baseTemplate = &corev1.PodTemplateSpec{}
	}

	// Service Account

	serviceAccount := "default"
	if dc.Spec.ServiceAccount != "" {
		serviceAccount = dc.Spec.ServiceAccount
	}
	baseTemplate.Spec.ServiceAccountName = serviceAccount

	// Host networking

	if dc.IsHostNetworkEnabled() {
		baseTemplate.Spec.HostNetwork = true
		baseTemplate.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	if baseTemplate.Spec.TerminationGracePeriodSeconds == nil {
		// Note: we cannot take the address of a constant
		gracePeriodSeconds := int64(DefaultTerminationGracePeriodSeconds)
		baseTemplate.Spec.TerminationGracePeriodSeconds = &gracePeriodSeconds
	}

	// Adds custom registry pull secret if needed

	_ = images.AddDefaultRegistryImagePullSecrets(&baseTemplate.Spec)

	// Labels

	podLabels := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(podLabels)
	podLabels[api.CassNodeState] = stateReadyToStart

	if baseTemplate.Labels == nil {
		baseTemplate.Labels = make(map[string]string)
	}
	baseTemplate.Labels = utils.MergeMap(baseTemplate.Labels, podLabels)

	// Annotations

	podAnnotations := map[string]string{}

	if baseTemplate.Annotations == nil {
		baseTemplate.Annotations = make(map[string]string)
	}
	baseTemplate.Annotations = utils.MergeMap(baseTemplate.Annotations, podAnnotations)

	// Affinity

	affinity := &corev1.Affinity{}
	affinity.NodeAffinity = calculateNodeAffinity(zone)
	affinity.PodAntiAffinity = calculatePodAntiAffinity(dc.Spec.AllowMultipleNodesPerWorker)
	baseTemplate.Spec.Affinity = affinity

	// Volumes

	addVolumes(dc, baseTemplate)

	// Init Containers

	err := buildInitContainers(dc, rackName, baseTemplate)
	if err != nil {
		return nil, err
	}

	// Containers

	err = buildContainers(dc, baseTemplate)
	if err != nil {
		return nil, err
	}

	return baseTemplate, nil
}
