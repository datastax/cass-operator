// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/datastax/cass-operator/operator/pkg/utils"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

func (rc *ReconciliationContext) GetPodPVC(podNamespace string, podName string) (*corev1.PersistentVolumeClaim, error) {
	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)

	pvc := &corev1.PersistentVolumeClaim{}
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: podNamespace, Name: pvcFullName}, pvc)
	if err != nil {
		rc.ReqLogger.Error(err, "error retrieving PersistentVolumeClaim")
		return nil, err
	}

	return pvc, nil
}

func (rc *ReconciliationContext) getPodsPVCs(pods []*corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs := []*corev1.PersistentVolumeClaim{}
	for _, pod := range pods {
		pvc, err := rc.GetPodPVC(pod.Namespace, pod.Name)
		if err != nil {
			return nil, err
		}
		pvcs = append(pvcs, pvc)
	}
	return pvcs, nil
}

func (rc *ReconciliationContext) getNodesForNameSet(nodeNameSet map[string]bool) ([]*corev1.Node, error) {
	nodes := []*corev1.Node{}
	for nodeName, _ := range nodeNameSet {
		if nodeName != "" {
			node, err := rc.getNode(nodeName)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

func (rc *ReconciliationContext) getNode(nodeName string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: "", Name: nodeName}, node)
	return node, err
}

func (rc *ReconciliationContext) removePVC(pvc *corev1.PersistentVolumeClaim) error {
	err := rc.Client.Delete(rc.Ctx, pvc)
	if err != nil {
		rc.ReqLogger.Error(err, "error during cassandra pvc delete",
			"pod", pvc.ObjectMeta.Name)
		return err
	}

	return nil
}

func getPVCsNodeNameSet(pvcs []*corev1.PersistentVolumeClaim) map[string]bool {
	nodeNameSet := map[string]bool{}
	for _, pvc := range pvcs {
		nodeName := utils.GetPVCSelectedNodeName(pvc)
		if nodeName != "" {
			nodeNameSet[nodeName] = true
		}
	}
	return nodeNameSet
}

func getPodsNodeNameSet(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		if pod.Spec.NodeName != "" {
			names[pod.Spec.NodeName] = true
		}
	}
	return names
}

func (rc *ReconciliationContext) getDCPodByName(podName string) *corev1.Pod {
	for _, pod := range rc.dcPods {
		if pod.Name == podName {
			return pod
		}
	}
	return nil
}


//
// functions to statisfy EMMSPI interface from psp package
//

func (rc *ReconciliationContext) GetAllNodesInDC() ([]*corev1.Node, error) {
	// Get all nodes for datacenter
	//
	// We need to check taints for all nodes that this datacenter cares about,
	// this includes not just nodes where we have dc pods, but also nodes 
	// where we have PVCs, as PVCs might get separated from their pod when a
	// pod is rescheduled.
	pvcs, err := rc.getPodsPVCs(rc.dcPods)
	if err != nil {
		return nil, err
	}
	nodeNameSet := utils.UnionStringSet(getPVCsNodeNameSet(pvcs), getPodsNodeNameSet(rc.dcPods))
	return rc.getNodesForNameSet(nodeNameSet)
}

func (rc *ReconciliationContext) GetDCPods() []*corev1.Pod {
	return rc.dcPods
}

func (rc *ReconciliationContext) GetNotReadyPodsBootstrappedInDC() []*corev1.Pod {
	return findAllPodsNotReadyAndPotentiallyBootstrapped(rc.dcPods, rc.Datacenter.Status.NodeStatuses)
}

func (rc *ReconciliationContext) GetAllPodsNotReadyInDC() []*corev1.Pod {
	return findAllPodsNotReady(rc.dcPods)
}

func (rc *ReconciliationContext) GetPodPVCs(pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	pvc, err := rc.GetPodPVC(pod.Namespace, pod.Name)
	if err != nil {
		return nil, err
	}
	return []*corev1.PersistentVolumeClaim{pvc}, nil
}

func (rc *ReconciliationContext) StartNodeReplace(podName string) error {
	pod := rc.getDCPodByName(podName)
	if pod == nil {
		return fmt.Errorf("Pod with name '%s' not part of datacenter", podName)
	}

	pvc, err := rc.GetPodPVC(pod.Namespace, pod.Name)
	if err != nil {
		return err
	}
	if pvc == nil {
		return fmt.Errorf("Pod with name '%s' does not have a PVC", podName)
	}

	// Add the cassandra node to replace nodes
	rc.Datacenter.Spec.ReplaceNodes = append(rc.Datacenter.Spec.ReplaceNodes, podName)

	// Update CassandraDatacenter
	if err := rc.Client.Update(rc.Ctx, rc.Datacenter); err != nil {
		rc.ReqLogger.Error(err, "Failed to update CassandraDatacenter with removed finalizers")
		return err
	}

	// delete pod and pvc
	err = rc.removePVC(pvc)
	if err != nil {
		return err
	}

	err = rc.RemovePod(pod)
	if err != nil {
		return err
	}

	return nil
}

func (rc *ReconciliationContext) GetInProgressNodeReplacements() []string {
	return rc.Datacenter.Status.NodeReplacements
}

func (rc *ReconciliationContext) RemovePod(pod *corev1.Pod) error {
	if isMgmtApiRunning(pod) {
		err := rc.NodeMgmtClient.CallDrainEndpoint(pod)
		if err != nil {
			rc.ReqLogger.Error(err, "error during cassandra node drain",
				"pod", pod.ObjectMeta.Name)
			return err
		}
	}

	err := rc.Client.Delete(rc.Ctx, pod)
	if err != nil {
		rc.ReqLogger.Error(err, "error during cassandra node delete",
			"pod", pod.ObjectMeta.Name)
		return err
	}

	return nil
}

func (rc *ReconciliationContext) UpdatePod(pod *corev1.Pod) error {
	return rc.Client.Update(rc.Ctx, pod)
}

func (rc *ReconciliationContext) IsStopped() bool {
	return rc.Datacenter.GetConditionStatus(api.DatacenterStopped) == corev1.ConditionTrue
}

func (rc *ReconciliationContext) IsInitialized() bool {
	return rc.Datacenter.GetConditionStatus(api.DatacenterInitialized) == corev1.ConditionTrue
}
