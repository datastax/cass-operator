package reconciliation

//
// This module defines constructors for k8s objects
//

import (
	"strings"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Creates a headless service object for the DSE Datacenter.
func newServiceForDseDatacenter(
	dseDatacenter *datastaxv1alpha1.DseDatacenter) *corev1.Service {
	// TODO adjust labels
	labels := map[string]string{
		datastaxv1alpha1.DATACENTER_LABEL: dseDatacenter.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dseDatacenter.Name + "-service",
			Namespace: dseDatacenter.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			// This MUST match a template pod label in the statefulset
			Selector:  labels,
			Type:      "ClusterIP",
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				// Note: Port Names cannot be more than 15 characters
				{
					Name:       "native",
					Port:       9042,
					TargetPort: intstr.FromInt(9042),
				},
				{
					Name:       "inter-node-msg",
					Port:       8609,
					TargetPort: intstr.FromInt(8609),
				},
				{
					Name:       "intra-node",
					Port:       7000,
					TargetPort: intstr.FromInt(7000),
				},
				{
					Name:       "tls-intra-node",
					Port:       7001,
					TargetPort: intstr.FromInt(7001),
				},
				{
					Name:       "jmx",
					Port:       7199,
					TargetPort: intstr.FromInt(7199),
				},
			},
		},
	}
}

// newSeedServiceForDseDatacenter creates a headless service owned by the DseDatacenter which will attach to all seed
// nodes in the cluster
func newSeedServiceForDseDatacenter(
	dseDatacenter *datastaxv1alpha1.DseDatacenter) *corev1.Service {
	// TODO adjust labels
	labels := map[string]string{
		datastaxv1alpha1.CLUSTER_LABEL: dseDatacenter.Name, // FIXME: this will need to be adjusted once we start to extract cluster name from dc name
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dseDatacenter.Name + "-seed-service",
			Namespace: dseDatacenter.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: nil,
			// This MUST match a template pod label in the statefulset
			Selector:  labels,
			ClusterIP: "None",
			Type:      "ClusterIP",
			// Make sure addresses are provided from the beginning so that we don't have to go back and reload seeds
			PublishNotReadyAddresses: true,
			SessionAffinityConfig: &corev1.SessionAffinityConfig{
				ClientIP: &corev1.ClientIPConfig{
					TimeoutSeconds: nil,
				},
			},
		},
	}
}

// Create a statefulset object for the DSE Datacenter.
func newStatefulSetForDseDatacenter(
	rackName string,
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	service *corev1.Service,
	replicaCount int) *appsv1.StatefulSet {
	replicaCountInt32 := int32(replicaCount)
	labels := map[string]string{
		datastaxv1alpha1.DATACENTER_LABEL: dseDatacenter.Name,
		datastaxv1alpha1.RACK_LABEL:       rackName,
	}

	seeds := dseDatacenter.GetSeedList()

	var userID int64 = 999
	var volumeCaimTemplates []corev1.PersistentVolumeClaim
	var volumeMounts []corev1.VolumeMount

	// Add storage if storage claim defined
	if nil != dseDatacenter.Spec.StorageClaim {
		pvName := "dse-data"
		storageClaim := dseDatacenter.Spec.StorageClaim
		volumeMounts = []corev1.VolumeMount {
			{
				Name: pvName,
				MountPath: "/var/lib/cassandra",
			},
		}
		volumeCaimTemplates = []corev1.PersistentVolumeClaim {{
			ObjectMeta: metav1.ObjectMeta{
				Labels: labels,
				Name: pvName,
			},
			Spec: corev1.PersistentVolumeClaimSpec {
				AccessModes: []corev1.PersistentVolumeAccessMode {
					corev1.ReadWriteOnce,
				},
				Resources: storageClaim.Resources,
				StorageClassName: &(storageClaim.StorageClassName),
			},
		}}
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dseDatacenter.Name + "-" + rackName + "-stateful-set",
			Namespace: dseDatacenter.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			// TODO adjust this
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:            &replicaCountInt32,
			ServiceName:         service.Name,
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					// Keep the pods of a statefulset within the same zone but off the same node. We don't want multiple pods
					// on the same node just in case something goes wrong but we do want to contain all pods within a statefulset
					// to the same zone to limit the need for cross zone communication.
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 100,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											datastaxv1alpha1.RACK_LABEL: rackName,
										},
									},
									TopologyKey: "failure-domain.beta.kubernetes.io/zone",
								},
							}},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
								Weight: 90,
								PodAffinityTerm: corev1.PodAffinityTerm{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											datastaxv1alpha1.DATACENTER_LABEL: dseDatacenter.Name,
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							}},
						},
					},
					// workaround for https://cloud.google.com/kubernetes-engine/docs/security-bulletins#may-31-2019
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:  &userID,
						RunAsGroup: &userID,
						FSGroup:    &userID,
					},
					Containers: []corev1.Container{{
						// TODO FIXME
						Name:  "dse",
						Image: dseDatacenter.GetServerImage(),
						Env: []corev1.EnvVar{
							{
								Name:  "DS_LICENSE",
								Value: "accept",
							},
							{
								Name:  "SEEDS",
								Value: strings.Join(seeds, ","),
							},
							{
								Name:  "NUM_TOKENS",
								Value: "32",
							},
						},
						Ports: []corev1.ContainerPort{
							// Note: Port Names cannot be more than 15 characters
							{
								Name:          "native",
								ContainerPort: 9042,
							},
							{
								Name:          "inter-node-msg",
								ContainerPort: 8609,
							},
							{
								Name:          "intra-node",
								ContainerPort: 7000,
							},
							{
								Name:          "tls-intra-node",
								ContainerPort: 7001,
							},
							{
								Name:          "jmx",
								ContainerPort: 7199,
							},
						},
						VolumeMounts: volumeMounts,
					}},
					// TODO We can document that the user installing the operator put the imagePullSecret
					// into the service account, and the process for that is documented here:
					// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
					// ImagePullSecrets: []corev1.LocalObjectReference{{}},
				},
			},
			VolumeClaimTemplates: volumeCaimTemplates,
		},
	}
}

// Create a statefulset object for the DSE Datacenter.
func newPodDisruptionBudgetForStatefulSet(
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	statefulSet *appsv1.StatefulSet) *policyv1beta1.PodDisruptionBudget {
	// Right now, we will just have maxUnavailable at 1
	maxUnavailable := intstr.FromInt(1)
	labels := map[string]string{
		datastaxv1alpha1.DATACENTER_LABEL: dseDatacenter.Name,
	}
	return &policyv1beta1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      statefulSet.Name + "-pdb",
			Namespace: statefulSet.Namespace,
			Labels:    labels,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			// TODO figure selector policy this out
			// right now this is matching ALL pods for a given datacenter
			// across all racks
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			MaxUnavailable: &maxUnavailable,
		},
	}
}
