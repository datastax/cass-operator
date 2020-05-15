// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// This file defines constructors for k8s objects

import (
	"fmt"

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

const pvcName = "server-data"

// Creates a headless service object for the Datacenter, for clients wanting to
// reach out to a ready Server node for either CQL or mgmt API
func newServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	svcName := dc.GetDatacenterServiceName()
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = svcName
	service.Spec.Ports = []corev1.ServicePort{
		// Note: Port Names cannot be more than 15 characters
		{
			Name: "native", Port: 9042, TargetPort: intstr.FromInt(9042),
		},
		{
			Name: "mgmt-api", Port: 8080, TargetPort: intstr.FromInt(8080),
		},
	}
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
	return service
}

// newAllPodsServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// which covers all server pods in the datacenter, whether they are ready or not
func newAllPodsServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetAllPodsServiceName()
	service.Spec.PublishNotReadyAddresses = true
	return service
}

// makeGenericHeadlessService returns a fresh k8s headless (aka ClusterIP equals "None") Service
// struct that has the same namespace as the CassandraDatacenter argument, and proper labels for the DC.
// The caller needs to fill in the ObjectMeta.Name value, at a minimum, before it can be created
// inside the k8s cluster.
func makeGenericHeadlessService(dc *api.CassandraDatacenter) *corev1.Service {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	var service corev1.Service
	service.ObjectMeta.Namespace = dc.Namespace
	service.ObjectMeta.Labels = labels
	service.Spec.Selector = labels
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

// Create a statefulset object for the Datacenter.
func newStatefulSetForCassandraDatacenter(
	rackName string,
	dc *api.CassandraDatacenter,
	replicaCount int) (*appsv1.StatefulSet, error) {

	replicaCountInt32 := int32(replicaCount)

	// see https://github.com/kubernetes/kubernetes/pull/74941
	// pvc labels are ignored before k8s 1.15.0
	pvcLabels := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(pvcLabels)

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
			Name:   pvcName,
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
	addHashAnnotation(result)

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
	addHashAnnotation(pdb)

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
		Name:      pvcName,
		MountPath: "/var/lib/cassandra",
	})
	cassContainer.VolumeMounts = serverVolumeMounts

	// server logger container
	loggerContainer := corev1.Container{}
	loggerContainer.Name = "server-system-logger"
	loggerContainer.Image = "busybox"
	loggerContainer.Args = []string{
		"/bin/sh", "-c", "tail -n+1 -F /var/log/cassandra/system.log",
	}
	loggerContainer.VolumeMounts = []corev1.VolumeMount{cassServerLogsMount}

	return []corev1.Container{cassContainer, loggerContainer}, nil
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

	configData, err := dc.GetConfigAsJSON()
	if err != nil {
		return nil, err
	}
	serverVersion := dc.Spec.ServerVersion
	serverCfg.Env = []corev1.EnvVar{
		{Name: "CONFIG_FILE_DATA", Value: configData},
		{Name: "POD_IP", ValueFrom: selectorFromFieldPath("status.podIP")},
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

	// volumes
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

	volumes := []corev1.Volume{vServerConfig, vServerLogs}
	baseTemplate.Spec.Volumes = append(baseTemplate.Spec.Volumes, volumes...)

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

	return baseTemplate, nil
}
