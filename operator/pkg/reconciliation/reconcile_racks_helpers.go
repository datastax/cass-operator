// Copyright DataStax, Inc.
// Please see the included license file for details.
package reconciliation

import (
	"fmt"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	corev1 "k8s.io/api/core/v1"
)

func mapContains(base map[string]string, submap map[string]string) bool {
	for k, v := range submap {
		if val, ok := base[k]; !ok || val != v {
			return false
		}
	}
	return true
}

// Takes a list of *Pod and filters down to only the pods that
// match every label/val in the provided label map.
func FilterPodListByLabels(pods []*corev1.Pod, labelMap map[string]string) []*corev1.Pod {
	filtered := []*corev1.Pod{}
	for _, p := range pods {
		if mapContains(p.Labels, labelMap) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func FilterPodListByLabel(pods []*corev1.Pod, labelName string, labelVal string) []*corev1.Pod {
	labels := map[string]string{
		labelName: labelVal,
	}
	return FilterPodListByLabels(pods, labels)
}

func FilterPodListByCassNodeState(pods []*corev1.Pod, state string) []*corev1.Pod {
	filtered := []*corev1.Pod{}
	for _, p := range pods {
		if val := p.Labels[api.CassNodeState]; val == state {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func ListAllStartedPods(pods []*corev1.Pod) []*corev1.Pod {
	return FilterPodListByCassNodeState(pods, stateStarted)
}

func FindIpForHostId(endpointData httphelper.CassMetadataEndpoints, hostId string) (string, error) {
	// If there are no nodes to ask, then of course we will not find an IP. We
	// treat this as an error since we have not way to determine the mapping.
	if len(endpointData.Entity) < 1 {
		return "", fmt.Errorf("No pods available to ask for the IP address of %s", hostId)
	}

	// Search for a cassandra node that knows about the given hostId
	for _, ep := range endpointData.Entity {
		if ep.HostID == hostId && len(ep.GetRpcAddress()) > 0 {
			return ep.GetRpcAddress(), nil
		}
	}

	// This indicates the cassandra node with the given hostId never
	// actually joined the ring
	return "", nil
}

func PodPtrsFromPodList(podList *corev1.PodList) []*corev1.Pod {
	var pods []*corev1.Pod
	for idx := range podList.Items {
		pod := &podList.Items[idx]
		pods = append(pods, pod)
	}
	return pods
}
