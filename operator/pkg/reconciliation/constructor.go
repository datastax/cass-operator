package reconciliation

//
// This module defines constructors for k8s objects
//

import (
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
		"app": dseDatacenter.Name,
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

// Create a statefulset object for the DSE Datacenter.
func newStatefulSetForDseDatacenter(
	rackName string,
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	service *corev1.Service,
	replicaCount int) *appsv1.StatefulSet {
	replicaCountInt32 := int32(replicaCount)
	labels := map[string]string{
		"app":  dseDatacenter.Name,
		"rack": rackName,
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
					Containers: []corev1.Container{{
						// TODO FIXME
						Name:  "dse",
						Image: dseDatacenter.GetServerImage(),
						Env: []corev1.EnvVar{
							{
								Name:  "DS_LICENSE",
								Value: "accept",
							},
							// TODO FIXME - use custom seed handling
							{
								Name:  "SEEDS",
								Value: "example-dsedatacenter-default-stateful-set-0.example-dsedatacenter-service.default.svc.cluster.local",
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
					}},
					// TODO We can document that the user installing the operator put the imagePullSecret
					// into the service account, and the process for that is documented here:
					// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account
					// ImagePullSecrets: []corev1.LocalObjectReference{{}},
				},
			},
		},
	}
}

// Create a statefulset object for the DSE Datacenter.
func newPodDisruptionBudgetForStatefulSet(
	dseDatacenter *datastaxv1alpha1.DseDatacenter,
	statefulSet *appsv1.StatefulSet,
	service *corev1.Service) *policyv1beta1.PodDisruptionBudget {
	// Right now, we will just have maxUnavailable at 1
	maxUnavailable := intstr.FromInt(1)
	labels := map[string]string{
		"app": dseDatacenter.Name,
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
