// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// This file defines constructors for k8s objects

import (
	"fmt"
	"os"

	v1 "k8s.io/api/batch/v1"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const PvcName = "server-data"

// Creates a headless service object for the Datacenter, for clients wanting to
// reach out to a ready Server node for either CQL or mgmt API
func newServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	svcName := dc.GetDatacenterServiceName()
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = svcName

	nativePort := api.DefaultNativePort
	if dc.IsNodePortEnabled() {
		nativePort = dc.GetNodePortNativePort()
	}

	service.Spec.Ports = []corev1.ServicePort{
		{
			Name: "native", Port: int32(nativePort), TargetPort: intstr.FromInt(nativePort),
		},
		{
			Name: "mgmt-api", Port: 8080, TargetPort: intstr.FromInt(8080),
		},
		{
			Name: "prometheus", Port: 9103, TargetPort: intstr.FromInt(9103),
		},
	}

	utils.AddHashAnnotation(service)

	return service
}

func buildLabelSelectorForSeedService(dc *api.CassandraDatacenter) map[string]string {
	labels := dc.GetClusterLabels()

	// narrow selection to just the seed nodes
	labels[api.SeedNodeLabel] = "true"

	return labels
}

// newSeedServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter which will attach to all seed
// nodes in the cluster
func newSeedServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetSeedServiceName()

	labels := dc.GetClusterLabels()
	oplabels.AddManagedByLabel(labels)
	service.ObjectMeta.Labels = labels

	service.Spec.Selector = buildLabelSelectorForSeedService(dc)
	service.Spec.PublishNotReadyAddresses = true

	utils.AddHashAnnotation(service)

	return service
}

// newAdditionalSeedServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// whether the additional seed pods are ready or not
func newAdditionalSeedServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	var service corev1.Service
	service.ObjectMeta.Name = dc.GetAdditionalSeedsServiceName()
	service.ObjectMeta.Namespace = dc.Namespace
	service.ObjectMeta.Labels = labels
	// We omit the label selector because we will create the endpoints manually
	service.Spec.Type = "ClusterIP"
	service.Spec.ClusterIP = "None"
	service.Spec.PublishNotReadyAddresses = true

	utils.AddHashAnnotation(&service)

	return &service
}

func newEndpointsForAdditionalSeeds(dc *api.CassandraDatacenter) *corev1.Endpoints {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	var endpoints corev1.Endpoints
	endpoints.ObjectMeta.Name = dc.GetAdditionalSeedsServiceName()
	endpoints.ObjectMeta.Namespace = dc.Namespace
	endpoints.ObjectMeta.Labels = labels

	var addresses []corev1.EndpointAddress

	for seedIdx := range dc.Spec.AdditionalSeeds {
		additionalSeed := dc.Spec.AdditionalSeeds[seedIdx]
		addresses = append(addresses, corev1.EndpointAddress{
			IP: additionalSeed,
		})
	}

	// See: https://godoc.org/k8s.io/api/core/v1#Endpoints
	endpoints.Subsets = []corev1.EndpointSubset{
		{
			Addresses: addresses,
		},
	}

	utils.AddHashAnnotation(&endpoints)

	return &endpoints
}

// newNodePortServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// that preserves the client source IPs
func newNodePortServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetNodePortServiceName()

	service.Spec.Type = "NodePort"
	// Note: ClusterIp = "None" is not valid for NodePort
	service.Spec.ClusterIP = ""
	service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal

	nativePort := dc.GetNodePortNativePort()
	internodePort := dc.GetNodePortInternodePort()

	service.Spec.Ports = []corev1.ServicePort{
		// Note: Port Names cannot be more than 15 characters
		{
			Name:       "internode",
			Port:       int32(internodePort),
			NodePort:   int32(internodePort),
			TargetPort: intstr.FromInt(internodePort),
		},
		{
			Name:       "native",
			Port:       int32(nativePort),
			NodePort:   int32(nativePort),
			TargetPort: intstr.FromInt(nativePort),
		},
	}

	return service
}

// newAllPodsServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// which covers all server pods in the datacenter, whether they are ready or not
func newAllPodsServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetAllPodsServiceName()
	service.Spec.PublishNotReadyAddresses = true

	nativePort := api.DefaultNativePort
	if dc.IsNodePortEnabled() {
		nativePort = dc.GetNodePortNativePort()
	}

	service.Spec.Ports = []corev1.ServicePort{
		{
			Name: "native", Port: int32(nativePort), TargetPort: intstr.FromInt(nativePort),
		},
		{
			Name: "mgmt-api", Port: 8080, TargetPort: intstr.FromInt(8080),
		},
		{
			Name: "prometheus", Port: 9103, TargetPort: intstr.FromInt(9103),
		},
	}

	utils.AddHashAnnotation(service)

	return service
}

// makeGenericHeadlessService returns a fresh k8s headless (aka ClusterIP equals "None") Service
// struct that has the same namespace as the CassandraDatacenter argument, and proper labels for the DC.
// The caller needs to fill in the ObjectMeta.Name value, at a minimum, before it can be created
// inside the k8s cluster.
func makeGenericHeadlessService(dc *api.CassandraDatacenter) *corev1.Service {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	selector := dc.GetDatacenterLabels()
	var service corev1.Service
	service.ObjectMeta.Namespace = dc.Namespace
	service.ObjectMeta.Labels = labels
	service.Spec.Selector = selector
	service.Spec.Type = "ClusterIP"
	service.Spec.ClusterIP = "None"
	return &service
}

func newNamespacedNameForStatefulSet(
	dc *api.CassandraDatacenter,
	rackName string) types.NamespacedName {

	name := dc.Spec.ClusterName + "-" + dc.Name + "-" + rackName + "-sts"
	ns := dc.Namespace

	return types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
}

// We have to account for the fact that they might use the old managed-by label value
// (oplabels.ManagedByLabelDefunctValue) for CassandraDatacenters originally
// created in version 1.1.0 or earlier.
func newStatefulSetForCassandraDatacenterWithDefunctPvcManagedBy(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int) (*appsv1.StatefulSet, error) {

	return newStatefulSetForCassandraDatacenterHelper(rackName, dc, replicaCount, true)
}

func usesDefunctPvcManagedByLabel(sts *appsv1.StatefulSet) bool {
	usesDefunct := false
	for _, pvc := range sts.Spec.VolumeClaimTemplates {
		value, ok := pvc.Labels[oplabels.ManagedByLabel]
		if ok && value == oplabels.ManagedByLabelDefunctValue {
			usesDefunct = true
			break
		}
	}

	return usesDefunct
}

func newStatefulSetForCassandraDatacenter(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int) (*appsv1.StatefulSet, error) {

	return newStatefulSetForCassandraDatacenterHelper(rackName, dc, replicaCount, false)
}

// Create a statefulset object for the Datacenter.
func newStatefulSetForCassandraDatacenterHelper(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int,
	useDefunctManagedByForPvc bool) (*appsv1.StatefulSet, error) {

	replicaCountInt32 := int32(replicaCount)

	// see https://github.com/kubernetes/kubernetes/pull/74941
	// pvc labels are ignored before k8s 1.15.0
	pvcLabels := dc.GetRackLabels(rackName)
	if useDefunctManagedByForPvc {
		oplabels.AddDefunctManagedByLabel(pvcLabels)
	} else {
		oplabels.AddManagedByLabel(pvcLabels)
	}

	statefulSetLabels := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(statefulSetLabels)

	statefulSetSelectorLabels := dc.GetRackLabels(rackName)

	var volumeClaimTemplates []corev1.PersistentVolumeClaim

	racks := dc.GetRacks()
	var zone string
	for _, rack := range racks {
		if rack.Name == rackName {
			zone = rack.Zone
			break
		}
	}

	// Add storage
	if dc.Spec.StorageConfig.CassandraDataVolumeClaimSpec == nil {
		err := fmt.Errorf("StorageConfig.cassandraDataVolumeClaimSpec is required")
		return nil, err
	}

	volumeClaimTemplates = []corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Labels: pvcLabels,
			Name:   PvcName,
		},
		Spec: *dc.Spec.StorageConfig.CassandraDataVolumeClaimSpec,
	}}

	nsName := newNamespacedNameForStatefulSet(dc, rackName)

	template, err := buildPodTemplateSpec(dc, zone, rackName)
	if err != nil {
		return nil, err
	}

	// if the dc.Spec has a nodeSelector map, copy it into each sts pod template
	if len(dc.Spec.NodeSelector) > 0 {
		template.Spec.NodeSelector = utils.MergeMap(map[string]string{}, dc.Spec.NodeSelector)
	}

	// workaround for https://cloud.google.com/kubernetes-engine/docs/security-bulletins#may-31-2019
	if dc.Spec.ServerType == "dse" {
		var userID int64 = 999
		template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser:  &userID,
			RunAsGroup: &userID,
			FSGroup:    &userID,
		}
	}

	_ = httphelper.AddManagementApiServerSecurity(dc, template)

	result := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nsName.Name,
			Namespace: nsName.Namespace,
			Labels:    statefulSetLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: statefulSetSelectorLabels,
			},
			Replicas:             &replicaCountInt32,
			ServiceName:          dc.GetDatacenterServiceName(),
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			Template:             *template,
			VolumeClaimTemplates: volumeClaimTemplates,
		},
	}
	result.Annotations = map[string]string{}

	// add a hash here to facilitate checking if updates are needed
	utils.AddHashAnnotation(result)

	return result, nil
}

// Create a PodDisruptionBudget object for the Datacenter
func newPodDisruptionBudgetForDatacenter(dc *api.CassandraDatacenter) *policyv1beta1.PodDisruptionBudget {
	minAvailable := intstr.FromInt(int(dc.Spec.Size - 1))
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	selectorLabels := dc.GetDatacenterLabels()
	pdb := &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dc.Name + "-pdb",
			Namespace:   dc.Namespace,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			MinAvailable: &minAvailable,
		},
	}

	// add a hash here to facilitate checking if updates are needed
	utils.AddHashAnnotation(pdb)

	return pdb
}

func setOperatorProgressStatus(rc *ReconciliationContext, newState api.ProgressState) error {
	currentState := rc.Datacenter.Status.CassandraOperatorProgress
	if currentState == newState {
		// early return, no need to ping k8s
		return nil
	}

	patch := client.MergeFrom(rc.Datacenter.DeepCopy())
	rc.Datacenter.Status.CassandraOperatorProgress = newState
	// TODO there may be a better place to push status.observedGeneration in the reconcile loop
	if newState == api.ProgressReady {
		rc.Datacenter.Status.ObservedGeneration = rc.Datacenter.Generation
	}
	if err := rc.Client.Status().Patch(rc.Ctx, rc.Datacenter, patch); err != nil {
		rc.ReqLogger.Error(err, "error updating the Cassandra Operator Progress state")
		return err
	}

	return nil
}

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

func buildContainers(dc *api.CassandraDatacenter, serverVolumeMounts []corev1.VolumeMount) ([]corev1.Container, error) {
	// cassandra container
	cassContainer := corev1.Container{}
	cassContainer.Name = "cassandra"

	serverImage, err := dc.GetServerImage()
	if err != nil {
		return nil, err
	}

	cassContainer.Image = serverImage
	cassContainer.Resources = dc.Spec.Resources
	cassContainer.Env = []corev1.EnvVar{
		{Name: "DS_LICENSE", Value: "accept"},
		{Name: "DSE_AUTO_CONF_OFF", Value: "all"},
		{Name: "USE_MGMT_API", Value: "true"},
		{Name: "MGMT_API_EXPLICIT_START", Value: "true"},
		// TODO remove this post 1.0
		{Name: "DSE_MGMT_EXPLICIT_START", Value: "true"},
	}

	if dc.Spec.ServerType == "dse" && dc.Spec.DseWorkloads != nil {
		cassContainer.Env = append(
			cassContainer.Env,
			corev1.EnvVar{Name: "JVM_EXTRA_OPTS", Value: getJvmExtraOpts(dc)})
	}

	ports, err := dc.GetContainerPorts()
	if err != nil {
		return nil, err
	}

	cassContainer.Ports = ports
	cassContainer.LivenessProbe = probe(8080, "/api/v0/probes/liveness", 15, 15)
	cassContainer.ReadinessProbe = probe(8080, "/api/v0/probes/readiness", 20, 10)

	cassServerLogsMount := corev1.VolumeMount{
		Name:      "server-logs",
		MountPath: "/var/log/cassandra",
	}
	serverVolumeMounts = append(serverVolumeMounts, cassServerLogsMount)
	serverVolumeMounts = append(serverVolumeMounts, corev1.VolumeMount{
		Name:      PvcName,
		MountPath: "/var/lib/cassandra",
	})
	serverVolumeMounts = append(serverVolumeMounts, corev1.VolumeMount{
		Name:      "encryption-cred-storage",
		MountPath: "/etc/encryption/",
	})
	cassContainer.VolumeMounts = serverVolumeMounts

	// server logger container
	loggerContainer := corev1.Container{}
	loggerContainer.Name = "server-system-logger"
	if baseImageOs := os.Getenv(api.EnvBaseImageOs); baseImageOs != "" {
		loggerContainer.Image = baseImageOs
	} else {
		loggerContainer.Image = "busybox"
	}
	loggerContainer.Args = []string{
		"/bin/sh", "-c", "tail -n+1 -F /var/log/cassandra/system.log",
	}
	loggerContainer.VolumeMounts = []corev1.VolumeMount{cassServerLogsMount}
	loggerContainer.Resources = *getResourcesOrDefault(&dc.Spec.SystemLoggerResources, &DefaultsLoggerContainer)

	containers := []corev1.Container{cassContainer, loggerContainer}
	if dc.Spec.Reaper != nil && dc.Spec.Reaper.Enabled && dc.Spec.ServerType == "cassandra" {
		reaperContainer := buildReaperContainer(dc)
		containers = append(containers, reaperContainer)
	}

	return containers, nil
}

func buildInitContainers(dc *api.CassandraDatacenter, rackName string) ([]corev1.Container, error) {
	serverCfg := corev1.Container{}
	serverCfg.Name = "server-config-init"
	serverCfg.Image = dc.GetConfigBuilderImage()
	serverCfgMount := corev1.VolumeMount{
		Name:      "server-config",
		MountPath: "/config",
	}
	serverCfg.VolumeMounts = []corev1.VolumeMount{serverCfgMount}
	serverCfg.Resources = *getResourcesOrDefault(&dc.Spec.ConfigBuilderResources, &DefaultsConfigInitContainer)

	// Convert the bool to a string for the env var setting
	useHostIpForBroadcast := "false"
	if dc.IsNodePortEnabled() {
		useHostIpForBroadcast = "true"
	}

	configData, err := dc.GetConfigAsJSON()
	if err != nil {
		return nil, err
	}
	serverVersion := dc.Spec.ServerVersion
	serverCfg.Env = []corev1.EnvVar{
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

	return []corev1.Container{serverCfg}, nil
}

func buildPodTemplateSpec(dc *api.CassandraDatacenter, zone string, rackName string) (*corev1.PodTemplateSpec, error) {
	baseTemplate := dc.Spec.PodTemplateSpec.DeepCopy()

	if baseTemplate == nil {
		baseTemplate = &corev1.PodTemplateSpec{}
	}

	podLabels := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(podLabels)
	podLabels[api.CassNodeState] = stateReadyToStart

	if baseTemplate.Labels == nil {
		baseTemplate.Labels = make(map[string]string)
	}
	baseTemplate.Labels = utils.MergeMap(baseTemplate.Labels, podLabels)

	// affinity
	affinity := &corev1.Affinity{}
	affinity.NodeAffinity = calculateNodeAffinity(zone)
	affinity.PodAntiAffinity = calculatePodAntiAffinity(dc.Spec.AllowMultipleNodesPerWorker)
	baseTemplate.Spec.Affinity = affinity

	addVolumes(dc, baseTemplate)

	serviceAccount := "default"
	if dc.Spec.ServiceAccount != "" {
		serviceAccount = dc.Spec.ServiceAccount
	}
	baseTemplate.Spec.ServiceAccountName = serviceAccount

	// init containers
	initContainers, err := buildInitContainers(dc, rackName)
	if err != nil {
		return nil, err
	}
	baseTemplate.Spec.InitContainers = append(initContainers, baseTemplate.Spec.InitContainers...)

	var serverVolumeMounts []corev1.VolumeMount
	for _, c := range initContainers {
		serverVolumeMounts = append(serverVolumeMounts, c.VolumeMounts...)
	}

	// containers
	containers, err := buildContainers(dc, serverVolumeMounts)
	if err != nil {
		return nil, err
	}

	baseTemplate.Spec.Containers = append(containers, baseTemplate.Spec.Containers...)

	// host networking
	if dc.IsHostNetworkEnabled() {
		baseTemplate.Spec.HostNetwork = true
		baseTemplate.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	return baseTemplate, nil
}

func buildInitReaperSchemaJob(dc *api.CassandraDatacenter) *v1.Job {
	return &v1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dc.Namespace,
			Name:      getReaperSchemaInitJobName(dc),
			Labels:    dc.GetDatacenterLabels(),
		},
		Spec: v1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:            getReaperSchemaInitJobName(dc),
							Image:           ReaperSchemaInitJobImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name:  "KEYSPACE",
									Value: ReaperKeyspace,
								},
								{
									Name:  "CONTACT_POINTS",
									Value: dc.GetSeedServiceName(),
								},
								{
									Name:  "REPLICATION",
									Value: getReaperReplication(dc),
								},
							},
						},
					},
				},
			},
		},
	}
}

func addVolumes(dc *api.CassandraDatacenter, baseTemplate *corev1.PodTemplateSpec) []corev1.Volume {
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
	baseTemplate.Spec.Volumes = append(baseTemplate.Spec.Volumes, volumes...)
	return volumes

}
