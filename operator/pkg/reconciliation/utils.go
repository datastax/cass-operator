package reconciliation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Builds the resource requirements given the default values for cpu and memory.
func buildResourceRequirements(cpuMillis int64, memoryMB int64) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"cpu": *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			"memory": *resource.NewScaledQuantity(memoryMB, resource.Mega),
		},
		Limits: corev1.ResourceList{
			"cpu": *resource.NewMilliQuantity(cpuMillis, resource.DecimalSI),
			"memory": *resource.NewScaledQuantity(memoryMB, resource.Mega),
		},
	}
}

// Determines if the given resource requirementw are specifed or not.
func isResourceRequirementsNotSpecified(res *corev1.ResourceRequirements) bool {
	if res.Limits == nil && res.Requests == nil {
		return false
	}

	return true
}

// Returns the default resources in case the given resources are not configured.
func getResourcesOrDefault(res *corev1.ResourceRequirements,
		defaultRes *corev1.ResourceRequirements) *corev1.ResourceRequirements {
	if !isResourceRequirementsNotSpecified(res) {
		return defaultRes
	}

	return res
}