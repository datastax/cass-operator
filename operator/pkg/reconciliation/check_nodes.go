// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	// "sync"
	// "time"
	// "strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	volumeutil "k8s.io/kubernetes/pkg/controller/volume/persistentvolume/util"
	"k8s.io/apimachinery/pkg/api/errors"
	// "github.com/datastax/cass-operator/operator/internal/result"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

func (rc *ReconciliationContext) GetAllNodesInDC() ([]*corev1.Node, error) {
	// Get all nodes for datacenter
	//
	// We need to check taints for all nodes that this datacenter cares about,
	// this includes not just nodes where we have dc pods, but also nodes 
	// where we have PVCs, as PVCs might get separated from their pod when a
	// pod is rescheduled.
	pvcs, err := rc.getPVCsForPods(rc.dcPods)
	if err != nil {
		return nil, err
	}
	nodeNameSet := unionStringSet(getNodeNameSetForPVCs(pvcs), getNodeNameSetForPods(rc.dcPods))
	return rc.getNodesForNameSet(nodeNameSet)
}

func (rc *ReconciliationContext) GetPVCsForPod(pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
	if err != nil {
		return nil, err
	}
	return []*corev1.PersistentVolumeClaim{pvc}, nil
}

func (rc *ReconciliationContext) GetDCPods() []*corev1.Pod {
	return rc.dcPods
}

func (rc *ReconciliationContext) GetNotReadyPodsJoinedInDC() []*corev1.Pod {
	return findAllPodsNotReadyAndPotentiallyJoined(rc.dcPods, rc.Datacenter.Status.NodeStatuses)
}

func (rc *ReconciliationContext) GetAllPodsNotReadyInDC() []*corev1.Pod {
	return findAllPodsNotReady(rc.dcPods)
}

func (rc *ReconciliationContext) GetSelectedNodeNameForPodPVC(pod *corev1.Pod) (string, error) {
	pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil
		} else {
			// requeue, try this again later
			return "", err
		}
	}
	pvcNode := getSelectedNodeNameForPVC(pvc)
	return pvcNode, nil
}

func (rc *ReconciliationContext) StartNodeReplace(podName string) error {
	pod := rc.getDCPodByName(podName)
	if pod == nil {
		return fmt.Errorf("Pod with name '%s' not part of datacenter", podName)
	}

	pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
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

func (rc *ReconciliationContext) GetLogger() logr.Logger {
	return rc.ReqLogger
}

func getSelectedNodeNameForPVC(pvc *corev1.PersistentVolumeClaim) string {
	annos := pvc.Annotations
	if annos == nil {
		annos = map[string]string{}
	}
	pvcNode := annos[volumeutil.AnnSelectedNode]
	return pvcNode
}

func (rc *ReconciliationContext) getPVCsForPods(pods []*corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	pvcs := []*corev1.PersistentVolumeClaim{}
	for _, pod := range pods {
		pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
		if err != nil {
			return nil, err
		}
		pvcs = append(pvcs, pvc)
	}
	return pvcs, nil
}

func (rc *ReconciliationContext) GetPVCForPod(podNamespace string, podName string) (*corev1.PersistentVolumeClaim, error) {
	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)

	pvc := &corev1.PersistentVolumeClaim{}
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: podNamespace, Name: pvcFullName}, pvc)
	if err != nil {
		rc.ReqLogger.Error(err, "error retrieving PersistentVolumeClaim")
		return nil, err
	}

	return pvc, nil
}

func (rc *ReconciliationContext) getNodesForNameSet(nodeNameSet map[string]bool) ([]*corev1.Node, error) {
	nodes := []*corev1.Node{}
	for nodeName, _ := range nodeNameSet {
		node, err := rc.getNode(nodeName)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
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

func unionStringSet(a, b map[string]bool) map[string]bool {
	result := map[string]bool{}
	for _, m := range []map[string]bool{a, b} {
		for k := range m {
			result[k] = true
		}
	}
	return result
}

func getNodeNameSetForPVCs(pvcs []*corev1.PersistentVolumeClaim) map[string]bool {
	nodeNameSet := map[string]bool{}
	for _, pvc := range pvcs {
		nodeName := getSelectedNodeNameForPVC(pvc)
		if nodeName != "" {
			nodeNameSet[nodeName] = true
		}
	}
	return nodeNameSet
}

func getNodeNameSetForPods(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Spec.NodeName] = true
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

// const (
// 	EMMFailureAnnotation string    = "appplatform.vmware.com/emm-failure"
// 	VolumeHealthAnnotations string = "volumehealth.storage.kubernetes.io/health"
// )

// type VolumeHealth string

// const (
// 	VolumeHealthInaccessible VolumeHealth = "inaccessible"
// )

// type EMMFailure string

// const (
// 	GenericFailure          EMMFailure = "GenericFailure"
// 	NotEnoughResources      EMMFailure = "NotEnoughResources"
// 	TooManyExistingFailures EMMFailure = "TooManyExistingFailures"
// )

// func getPVCNameForPod(podName string) string {
// 	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)
// 	return pvcFullName
// }

// func getPodNameForPVC(pvcName string) string {
// 	return strings.TrimPrefix(pvcName, fmt.Sprintf("%s-", PvcName))
// }



// func (rc *ReconciliationContext) DeletePvcIgnoreFinalizers(podNamespace string, podName string) (*corev1.PersistentVolumeClaim, error) {
// 	var wg sync.WaitGroup

// 	wg.Add(1)

// 	var goRoutineError *error = nil

// 	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)

// 	pvc, err := rc.GetPVCForPod(podNamespace, podName)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Delete might hang due to a finalizer such as kubernetes.io/pvc-protection
// 	// so we run it asynchronously and then remove any finalizers to unblock it.
// 	go func() {
// 		defer wg.Done()
// 		rc.ReqLogger.Info("goroutine to delete pvc started")

// 		// If we don't grab a new copy of the pvc, the deletion could fail because the update has
// 		// changed the pvc and the delete fails because there is a newer version

// 		pvcToDelete := &corev1.PersistentVolumeClaim{}
// 		err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: podNamespace, Name: pvcFullName}, pvcToDelete)
// 		if err != nil {
// 			rc.ReqLogger.Info("goroutine to delete pvc: error found in get")
// 			rc.ReqLogger.Error(err, "error retrieving PersistentVolumeClaim for deletion")
// 			goRoutineError = &err
// 		}

// 		rc.ReqLogger.Info("goroutine to delete pvc: no error found in get")

// 		err = rc.Client.Delete(rc.Ctx, pvcToDelete)
// 		if err != nil {
// 			rc.ReqLogger.Info("goroutine to delete pvc: error found in delete")
// 			rc.ReqLogger.Error(err, "error removing PersistentVolumeClaim",
// 				"name", pvcFullName)
// 			goRoutineError = &err
// 		}
// 		rc.ReqLogger.Info("goroutine to delete pvc: no error found in delete")
// 		rc.ReqLogger.Info("goroutine to delete pvc: end of goroutine")
// 	}()

// 	// Give the resource a second to get to a terminating state. Note that this
// 	// may not be reflected in the resource's status... hence the sleep here as
// 	// opposed to checking the status.
// 	time.Sleep(5 * time.Second)

// 	// In the case of PVCs at least, finalizers removed before deletion can be
// 	// automatically added back. Consequently, we delete the resource first,
// 	// then remove any finalizers while it is terminating.

// 	pvc.ObjectMeta.Finalizers = []string{}

// 	err = rc.Client.Update(rc.Ctx, pvc)
// 	if err != nil {
// 		rc.ReqLogger.Info("ignoring error removing finalizer from PersistentVolumeClaim",
// 			"name", pvcFullName,
// 			"err", err.Error())

// 		// Ignore some errors as this may fail due to the resource already having been
// 		// deleted (which is what we want).
// 	}

// 	rc.ReqLogger.Info("before wg.Wait()")

// 	// Wait for the delete to finish, which should have been unblocked by
// 	// removing the finalizers.
// 	wg.Wait()
// 	rc.ReqLogger.Info("after wg.Wait()")

// 	// We can't dereference a nil, so check if we have one
// 	if goRoutineError == nil {
// 		return pvc, nil
// 	}
// 	return nil, *goRoutineError
// }

// func (rc *ReconciliationContext) removePvcAndPod(pod corev1.Pod) error {

// 	// Drain the cassandra node

// 	if isMgmtApiRunning(&pod) {
// 		err := rc.NodeMgmtClient.CallDrainEndpoint(&pod)
// 		if err != nil {
// 			rc.ReqLogger.Error(err, "error during cassandra node drain",
// 				"pod", pod.ObjectMeta.Name)
// 			return err
// 		}
// 	}

// 	// Add the cassandra node to replace nodes

// 	rc.Datacenter.Spec.ReplaceNodes = append(rc.Datacenter.Spec.ReplaceNodes, pod.ObjectMeta.Name)

// 	// Update CassandraDatacenter
// 	if err := rc.Client.Update(rc.Ctx, rc.Datacenter); err != nil {
// 		rc.ReqLogger.Error(err, "Failed to update CassandraDatacenter with removed finalizers")
// 		return err
// 	}

// 	// Remove the pvc

// 	_, err := rc.DeletePvcIgnoreFinalizers(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
// 	if err != nil {
// 		rc.ReqLogger.Error(err, "error during PersistentVolume delete",
// 			"pod", pod.ObjectMeta.Name)
// 		return err
// 	}

// 	// Remove the pod

// 	err = rc.Client.Delete(rc.Ctx, &pod)
// 	if err != nil {
// 		rc.ReqLogger.Error(err, "error during cassandra node delete",
// 			"pod", pod.ObjectMeta.Name)
// 		return err
// 	}

// 	return nil
// }



// func getRackNameSetForPods(pods []*corev1.Pod) map[string]bool {
// 	names := map[string]bool{}
// 	for _, pod := range pods {
// 		names[pod.Labels[api.RackLabel]] = true
// 	}
// 	return names
// }





// func (rc *ReconciliationContext) getNodesForPods(pods []*corev1.Pod) ([]*corev1.Node, error) {
// 	nodeNameSet := getNodeNameSetForPods(pods)
// 	return rc.getNodesForNameSet(nodeNameSet)
// }

// func hasTaint(node *corev1.Node, taintKey, value string, effect corev1.TaintEffect) bool {
// 	for _, taint := range node.Spec.Taints {
// 		if taint.Key == taintKey && taint.Effect == effect {
// 			if taint.Value == value {
// 				return true
// 			}
// 		}
// 	}
// 	return false
// }



// func (rc *ReconciliationContext) getNodesWithTaintKeyValueEffect(taintKey, value string, effect corev1.TaintEffect) ([]*corev1.Node, error) {
// 	nodes, err := rc.getAllNodesInDc()
// 	if err != nil {
// 		return nil, err
// 	}

// 	nodesWithTaint := filterNodesWithFn(nodes, func(node *corev1.Node) bool {
// 		return hasTaint(node, taintKey, value, effect)
// 	})

// 	return nodesWithTaint, nil
// }

// func filterNodesWithFn(nodes []*corev1.Node, fn func(*corev1.Node) bool) []*corev1.Node {
// 	result := []*corev1.Node{}
// 	for _, node := range nodes {
// 		if fn(node) {
// 			result = append(result, node)
// 		}
// 	}
// 	return result
// }

// func filterNodesWithTaintKeyValueEffect(nodes []*corev1.Node, taintKey, value string, effect corev1.TaintEffect) []*corev1.Node {
// 	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
// 		return hasTaint(node, taintKey, value, effect)
// 	})
// }

// func filterNodesForEvacuateAllDataTaint(nodes []*corev1.Node) []*corev1.Node {
// 	return filterNodesWithTaintKeyValueEffect(nodes, "node.vmware.com/drain", "drain", corev1.TaintEffectNoSchedule)
// }

// func filterNodesForPlannedDownTimeTaint(nodes []*corev1.Node) []*corev1.Node {
// 	return filterNodesWithTaintKeyValueEffect(nodes, "node.vmware.com/drain", "planned-downtime", corev1.TaintEffectNoSchedule)
// }

// func filterNodesInNameSet(nodes []*corev1.Node, nameSet map[string]bool) []*corev1.Node {
// 	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
// 		return nameSet[node.Name]
// 	})
// }

// func filterNodesNotInNameSet(nodes []*corev1.Node, nameSet map[string]bool) []*corev1.Node {
// 	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
// 		return !nameSet[node.Name]
// 	})
// }

// func filterPodsWithFn(pods []*corev1.Pod, fn func(*corev1.Pod)bool) []*corev1.Pod {
// 	result := []*corev1.Pod{}
// 	for _, pod := range pods {
// 		result = append(result, pod)
// 	}
// 	return result
// }

// func filterPodsWithNodeInNameSet(pods []*corev1.Pod, nameSet map[string]bool) []*corev1.Pod {
// 	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
// 		return nameSet[pod.Spec.NodeName]
// 	})
// }

// func filterPodsFailedEMM(pods []*corev1.Pod) []*corev1.Pod {
// 	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
// 		annos := pod.Annotations
// 		if annos == nil {
// 			return false
// 		}
// 		_, ok := annos[EMMFailureAnnotation]
// 		return ok
// 	})
// }

// func getNameSetForNodes(nodes []*corev1.Node) map[string]bool {
// 	result := map[string]bool{}
// 	for _, node := range nodes {
// 		result[node.Name] = true
// 	}
// 	return result
// }



// func (rc *ReconciliationContext) removePods(pods []*corev1.Pod) error {
// 	for _, pod := range pods {
// 		err := rc.RemovePod(pod)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func isPodUnschedulable(pod *corev1.Pod) bool {
// 	for _, condition := range pod.Status.Conditions {
// 		if condition.Reason == corev1.PodReasonUnschedulable && 
// 			condition.Type == corev1.PodScheduled &&
// 			condition.Status == corev1.ConditionFalse {
// 				return true
// 			}
// 	}
// 	return false
// }

// func getSelectedNodeNameForPVC(pvc *corev1.PersistentVolumeClaim) string {
// 	annos := pvc.Annotations
// 	if annos == nil {
// 		annos = map[string]string{}
// 	}
// 	pvcNode := annos[volumeutil.AnnSelectedNode]
// 	return pvcNode
// }



// func (rc *ReconciliationContext) removePVC(pvc *corev1.PersistentVolumeClaim) error {
// 	err := rc.Client.Delete(rc.Ctx, pvc)
// 	if err != nil {
// 		rc.ReqLogger.Error(err, "error during cassandra pvc delete",
// 			"pod", pvc.ObjectMeta.Name)
// 		return err
// 	}

// 	return nil
// }



// func getPVCNameSetForPods(pods []*corev1.Pod) map[string]bool {
// 	nameSet := map[string]bool{}
// 	for _, pod := range pods {
// 		nameSet[getPVCNameForPod(pod.Name)] = true
// 	}
// 	return nameSet
// }

// func filterPVCsWithFn(pvcs []*corev1.PersistentVolumeClaim, fn func(*corev1.PersistentVolumeClaim) bool) []*corev1.PersistentVolumeClaim {
// 	result := []*corev1.PersistentVolumeClaim{}
// 	for _, pvc := range pvcs {
// 		if fn(pvc) {
// 			result = append(result, pvc)
// 		}
// 	}
// 	return result
// }

// func filterPVCsInNameSet(pvcs []*corev1.PersistentVolumeClaim, nameSet map[string]bool) []*corev1.PersistentVolumeClaim {
// 	return filterPVCsWithFn(
// 		pvcs,
// 		func(pvc *corev1.PersistentVolumeClaim) bool {
// 			return nameSet[pvc.Name]
// 		})
// }

// func filterPodsThatHaveEMMFailureAnnotation(pods []*corev1.Pod) []*corev1.Pod {
// 	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
// 		annos := pod.Annotations
// 		if annos == nil {
// 			return false
// 		}
// 		_, ok := annos[EMMFailureAnnotation]
// 		return ok
// 	})
// }

// func (rc *ReconciliationContext) failEMMForPods(pods []*corev1.Pod, reason EMMFailure) (bool, error) {
// 	updatedAny := false
// 	for _, pod := range pods {
// 		if pod.Annotations == nil {
// 			pod.Annotations = map[string]string{}
// 		}
// 		val, _ := pod.Annotations[EMMFailureAnnotation]
// 		if string(reason) != val {
// 			pod.Annotations[EMMFailureAnnotation] = string(reason)
// 			updatedAny = true
// 			err := rc.updatePod(pod)
// 			if err != nil {
// 				return false, err
// 			}
// 		}
// 	}
// 	return updatedAny, nil
// }

// func filterPVCsWithVolumeHealth(pvcs []*corev1.PersistentVolumeClaim, health VolumeHealth) []*corev1.PersistentVolumeClaim {
// 	return filterPVCsWithFn(
// 		pvcs, 
// 		func(pvc *corev1.PersistentVolumeClaim) bool {
// 			annos := pvc.Annotations
// 			return annos != nil && annos[VolumeHealthAnnotations] == string(VolumeHealthInaccessible)})
// }

// func filterPodsInNameSet(pods []*corev1.Pod, nameSet map[string]bool) []*corev1.Pod {
// 	return filterPodsWithFn(
// 		pods,
// 		func(pod *corev1.Pod) bool {
// 			return nameSet[pod.Name]
// 		})
// }

// func (rc *ReconciliationContext) getPodsWithVolumeHealthInaccessiblePVC(rack string) ([]*corev1.Pod, error) {
// 	pods := rc.dcPods

// 	pvcs, err := rc.getPVCsForPods(pods)
// 	if err != nil {
// 		return nil, err
// 	}

// 	inaccessiblePVCs := filterPVCsWithVolumeHealth(
// 		pvcs, 
// 		VolumeHealthInaccessible)
	
// 	podNameSet := map[string]bool{}
// 	for _, pvc := range inaccessiblePVCs {
// 		podNameSet[getPodNameForPVC(pvc.Name)] = true
// 	}

// 	return filterPodsInNameSet(rc.dcPods, podNameSet), nil
// }

// type PVCHealthSPI interface {
// 	getPodsWithVolumeHealthInaccessiblePVC() ([]*corev1.Pod, error)
// 	getRacksWithNotReadyPodsJoined() []string
// 	startNodeReplace(podName string) error
// 	getInProgressNodeReplacements() []string
// 	getLogger() logr.Logger
// }

// func filterPodsByRackName(pods []*corev1.Pod, rackName string) []*corev1.Pod {
// 	return FilterPodListByLabel(pods, api.RackLabel, rackName)
// }

// func checkPVCHealth(provider PVCHealthSPI) result.ReconcileResult {
// 	logger := provider.getLogger()

// 	podsWithInaccessible, err := provider.getPodsWithVolumeHealthInaccessiblePVC()
// 	if err != nil {
// 		return result.Error(err)
// 	}

// 	if len(podsWithInaccessible) == 0 {
// 		// nothing to do
// 		return result.Continue()
// 	}
	
// 	racksWithDownPods := provider.getRacksWithNotReadyPodsJoined()

// 	if len(racksWithDownPods) > 1 {
// 		logger.Info("Found PVCs marked inaccessible but ignoring due to availability compromised by multiple racks having pods not ready", "racks", racksWithDownPods)
// 		return result.Continue()
// 	}

// 	if len(racksWithDownPods) == 1 {
// 		podsWithInaccessible = filterPodsByRackName(podsWithInaccessible, racksWithDownPods[0])

// 		if len(podsWithInaccessible) == 0 {
// 			logger.Info("Found PVCs marked inaccessible but ignoring due to a different rack with pods not ready", "rack", racksWithDownPods[0])
// 			return result.Continue()
// 		}
// 	}

// 	if len(provider.getInProgressNodeReplacements()) > 0 {
// 		logger.Info("Found PVCs marked inaccessible but ignore due to an ongoing node replace")
// 		return result.Continue()
// 	}

// 	for _, pod := range podsWithInaccessible {
// 		err := provider.startNodeReplace(pod.Name)
// 		if err != nil {
// 			logger.Error(err, "Failed to start node replacement for pod with inaccessible PVC", "pod", pod.Name)
// 			return result.Error(err)
// 		}
// 		return result.RequeueSoon(2)
// 	}

// 	// Should not be possible to get here, but just in case
// 	return result.Continue()
// }

// func getNodeNameSet(nodes []*corev1.Node) map[string]bool {
// 	nameSet := map[string]bool{}
// 	for _, node := range nodes {
// 		nameSet[node.Name] = true
// 	}
// 	return nameSet
// }

// func (rc *ReconciliationContext) getDCNodes() ([]*corev1.Node, error) {
// 	// We need all nodes that this datacenter cares about, this includes not
// 	// just nodes where we have dc pods, but also nodes where we have PVCs, 
// 	// as PVCs might get separated from their pod when a pod is rescheduled.
// 	pvcs, err := rc.getPVCsForPods(rc.dcPods)
// 	if err != nil {
// 		return nil, err
// 	}
// 	nodeNameSet := unionStringSet(getNodeNameSetForPVCs(pvcs), getNodeNameSetForPods(rc.dcPods))
// 	return rc.getNodesForNameSet(nodeNameSet)
// }

// func (rc *ReconciliationContext) getPodsForNodeName(nodeName string) []*corev1.Pod {
// 	return filterPodsWithNodeInNameSet(rc.dcPods, map[string]bool{nodeName: true})
// }

// func (rc *ReconciliationContext) getSelectedNodeNameForPodPvc(pod *corev1.Pod) (string, error) {
// 	pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
// 	if err != nil {
// 		if errors.IsNotFound(err) {
// 			return "", nil
// 		} else {
// 			// requeue, try this again later
// 			return "", err
// 		}
// 	}
// 	pvcNode := getSelectedNodeNameForPVC(pvc)
// 	return pvcNode, nil
// }

// func (rc *ReconciliationContext) getNodeNameSetForRack(rackName string) map[string]bool {
// 	podsForDownRack := FilterPodListByLabels(rc.dcPods, map[string]string{api.RackLabel: rackName})
// 	nodeNameSetForRack := getNodeNameSetForPods(podsForDownRack)
// 	return nodeNameSetForRack
// }


// type EMMSPI interface {
// 	startNodeReplace(podName string) error
// 	getInProgressNodeReplacements() []string

// 	getRacksWithNotReadyPodsJoined() []string

// 	getNodesWithTaintKeyValueEffect(taintKey, value string, effect corev1.TaintEffect) ([]*corev1.Node, error)

// 	getNodeNameSetForRack(rackName string) map[string]bool

// 	getPodsWithAnnotationKey(key string) []*corev1.Pod
// 	getAllPodsNotReady() []*corev1.Pod
// 	getAllPodsNotReadyJoined() []*corev1.Pod
// 	getPodsForNodeName(nodeName string) []*corev1.Pod
// 	removePod(pod *corev1.Pod) error
// 	removePodAnnotation(pod *corev1.Pod, annoKey string) error
// 	addPodAnnotation(pod *corev1.Pod, key, value string) error

// 	getSelectedNodeNameForPodPvc(pod *corev1.Pod) (string, error)

// 	getLogger() logr.Logger
// }

// func failEMMOperation(provider EMMSPI, nodeName string, failure EMMFailure) (bool, error) {
// 	pods := provider.getPodsForNodeName(nodeName)
// 	didUpdate := false
// 	for _, pod := range pods {
// 		err := provider.addPodAnnotation(pod, EMMFailureAnnotation, string(failure))
// 		if err != nil {
// 			return false, err
// 		}
// 	}
// 	return didUpdate, nil
// }

// func getNodesForEvacuateAllData(provider EMMSPI) ([]*corev1.Node, error) {
// 	return provider.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "drain", corev1.TaintEffectNoSchedule)
// }

// func getNodesForPlannedDownTime(provider EMMSPI) ([]*corev1.Node, error) {
// 	return provider.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "planned-downtime", corev1.TaintEffectNoSchedule)
// }

// func cleanupEMMAnnotations(provider EMMSPI) (bool, error) {
// 	nodes, err := getNodesForEvacuateAllData(provider)
// 	if err != nil {
// 		return false, err
// 	}
// 	nodes2, err := getNodesForPlannedDownTime(provider)
// 	if err != nil {
// 		return false, err
// 	}
// 	nodes = append(nodes, nodes2...)

// 	// Strip EMM failure annotation from pods where node is no longer tainted
// 	podsFailedEmm := provider.getPodsWithAnnotationKey(EMMFailureAnnotation)
// 	nodesWithPodsFailedEMM := getNodeNameSetForPods(podsFailedEmm)
// 	nodesNoLongerEMM := subtractStringSet(
// 		nodesWithPodsFailedEMM,
// 		getNameSetForNodes(nodes))
	
// 	podsNoLongerFailed := filterPodsWithNodeInNameSet(podsFailedEmm, nodesNoLongerEMM)
// 	didUpdate := false
// 	for _, pod := range podsNoLongerFailed {
// 		err := provider.removePodAnnotation(pod, EMMFailureAnnotation)
// 		if err != nil {
// 			return false, err
// 		}
// 		didUpdate = true
// 	}

// 	return didUpdate, nil
// }

// // NOTE: This check has to come before CheckPodsReady() because we will tolerate
// // some pods to be down so long as they are on the tainted node.

// // cleanup no longer failed pods
// // fail operation for nodes multiple racks down
// // fail operation for nodes not on down rack
// // remove not ready pods from tainted nodes
// // remove a pod from evacuate data node
// // remove all pods from planned downtime node

// type EMMChecks interface {
// 	getRacksWithNotReadyPodsJoined() []string
// 	getNodeNameSetForRack(rackName string) map[string]bool
// 	IsStopped() bool
// 	IsInitialized() bool
// }

// type EMMOperations interface {
// 	cleanupEMMAnnotations() (bool, error)
// 	getNodeNameSetForPlannedDownTime() (map[string]bool, error)
// 	getNodeNameSetForEvacuateAllData() (map[string]bool, error)
// 	removeAllNotReadyPodsOnEMMNodes() (bool, error)
// 	failEMM(nodeName string, failure EMMFailure) (bool, error)
// 	performPodReplaceForEvacuateData() (bool, error)
// 	removeNextPodFromEvacuateDataNode() (bool, error)
// 	removeAllPodsFromPlannedDowntimeNode() (bool, error)
// }

// type EMMService interface {
// 	EMMOperations
// 	EMMChecks
// }

// type DAO interface {
// 	GetAllNodesInDC() ([]*corev1.Node, error)
// 	GetDCPods() []*corev1.Pod
// 	GetNotReadyPodsJoinedInDC() []*corev1.Pod
// 	GetAllPodsNotReadyInDC() []*corev1.Pod
// 	GetSelectedNodeNameForPodPVC(pod *corev1.Pod) (string, error)
// 	StartNodeReplace(podName string) error
// 	RemovePod(pod *corev1.Pod) error
// 	UpdatePod(pod *corev1.Pod) error
// 	IsStopped() bool
// 	IsInitialized() bool
// }

// type EMMServiceImpl struct {
// 	DAO
// }

// func filterPodsWithAnnotationKey(pods []*corev1.Pod, key string) []*corev1.Pod {
// 	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
// 		annos := pod.ObjectMeta.Annotations
// 		if annos != nil {
// 			_, ok := annos[key]
// 			return ok
// 		}
// 		return false
// 	})
// }

// func (impl *EMMServiceImpl) removeAllPodsFromPlannedDowntimeNode() (bool, error) {
// 	nodeNameSet, err := impl.getNodeNameSetForPlannedDownTime()
// 	for nodeName, _ := range nodeNameSet {
// 		pods := impl.getPodsForNodeName(nodeName)
// 		if len(pods) > 0 {
// 			for _, pod := range pods {
// 				err := impl.RemovePod(pod)
// 				if err != nil {
// 					return false, err
// 				}
// 			}
// 			return true, err
// 		}
// 	}
// 	return false, nil
// }

// func (impl *EMMServiceImpl) removeNextPodFromEvacuateDataNode() (bool, error) {
// 	nodeNameSet, err := impl.getNodeNameSetForEvacuateAllData()
// 	if err != nil {
// 		return false, err
// 	}

// 	for name, _ := range nodeNameSet {
// 		for _, pod := range impl.getPodsForNodeName(name) {
// 			err := impl.RemovePod(pod)
// 			if err != nil {
// 				return false, err
// 			}
// 			return true, nil
// 		}
// 	}

// 	return false, nil
// }

// func (impl *EMMServiceImpl) performPodReplaceForEvacuateData() (bool, error) {
// 	downPods := impl.GetNotReadyPodsJoinedInDC()
// 	if len(downPods) > 0 {
// 		evacuateAllDataNameSet, err := impl.getNodeNameSetForEvacuateAllData()
// 		if err != nil {
// 			return false, err
// 		}

// 		// Check if any of these pods are stuck due to PVC associated to a 
// 		// tainted node for evacuate all data. This would happen, for example,
// 		// in the case of local persistent volumes where the volume cannot
// 		// move with the pod.
// 		//
// 		// NOTE: This arguably belongs in our check for stuck pods that
// 		// CheckPodsReady() does. Keeping all the logic together for the
// 		// time being since it is all pretty specific to PSP at the moment.
// 		deletedPodsOrPVCs := false
// 		for _, pod := range downPods {
// 			if isPodUnschedulable(pod) {

// 				// NOTE: There isn't a great machine readable way to know why
// 				// the pod is unschedulable. The reasons are, unfortunately,
// 				// buried within human readable explanation text. As a result,
// 				// a pod might not get scheduled due to no nodes having 
// 				// sufficent memory, and then we delete a PVC thinking that
// 				// the PVC was causing scheduling to fail even though it 
// 				// wasn't.
				
// 				pvcNode, err := impl.GetSelectedNodeNameForPodPVC(pod)
// 				if err != nil {
// 					return false, err
// 				}
// 				if pvcNode != "" && pod.Spec.NodeName != pvcNode{
// 					if evacuateAllDataNameSet[pvcNode] {
// 						deletedPodsOrPVCs = true

// 						// set pod to be replaced
// 						err := impl.StartNodeReplace(pod.Name)
// 						if err != nil {
// 							return false, err
// 						}
// 					}
// 				}
// 			}
// 		}

// 		return deletedPodsOrPVCs, nil
// 	}
// 	return false, nil
// }

// func (impl *EMMServiceImpl) removeAllNotReadyPodsOnEMMNodes() (bool, error) {
// 	podsNotReady := impl.GetAllPodsNotReadyInDC()
// 	plannedDownNameSet, err := impl.getNodeNameSetForPlannedDownTime()
// 	if err != nil {
// 		return false, err
// 	}

// 	evacuateDataNameSet, err := impl.getNodeNameSetForEvacuateAllData()
// 	if err != nil {
// 		return false, err
// 	}

// 	taintedNodesNameSet := unionStringSet(plannedDownNameSet, evacuateDataNameSet)
// 	podsNotReadyOnTaintedNodes := filterPodsWithNodeInNameSet(podsNotReady, taintedNodesNameSet)
	
// 	if len(podsNotReadyOnTaintedNodes) > 1 {
// 		for _, pod := range podsNotReadyOnTaintedNodes {
// 			err := impl.RemovePod(pod)
// 			if err != nil {
// 				return false, err
// 			}
// 		}
// 		return true, nil
// 	}

// 	return false, nil
// }

// func (impl *EMMServiceImpl) getPodsForNodeName(nodeName string) []*corev1.Pod {
// 	return filterPodsWithNodeInNameSet(impl.GetDCPods(), map[string]bool{nodeName: true})
// }

// func (impl *EMMServiceImpl) getRacksWithNotReadyPodsJoined() []string {
// 	pods := impl.GetNotReadyPodsJoinedInDC()
// 	rackNameSet := getRackNameSetForPods(pods)
// 	rackNames := []string{}
// 	for rackName, _ := range rackNameSet {
// 		rackNames = append(rackNames, rackName)
// 	}
// 	return rackNames
// }

// func (impl *EMMServiceImpl) addPodAnnotation(pod *corev1.Pod, key, value string) error {
// 	if pod.ObjectMeta.Annotations == nil {
// 		pod.ObjectMeta.Annotations = map[string]string{}
// 	}

// 	pod.Annotations[key] = value
// 	return impl.UpdatePod(pod)
// }

// func (impl *EMMServiceImpl) failEMM(nodeName string, failure EMMFailure) (bool, error) {
// 	pods := impl.getPodsForNodeName(nodeName)
// 	didUpdate := false
// 	for _, pod := range pods {
// 		err := impl.addPodAnnotation(pod, EMMFailureAnnotation, string(failure))
// 		if err != nil {
// 			return false, err
// 		}
// 	}
// 	return didUpdate, nil
// }

// func (impl *EMMServiceImpl) getNodeNameSetForEvacuateAllData() (map[string]bool, error) {
// 	nodes, err := impl.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "drain", corev1.TaintEffectNoSchedule)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return getNameSetForNodes(nodes), nil
// }

// func (impl *EMMServiceImpl) getNodeNameSetForPlannedDownTime() (map[string]bool, error) {
// 	nodes, err := impl.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "planned-downtime", corev1.TaintEffectNoSchedule)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return getNameSetForNodes(nodes), nil
// }

// func (impl *EMMServiceImpl) getNodesWithTaintKeyValueEffect(taintKey, value string, effect corev1.TaintEffect) ([]*corev1.Node, error) {
// 	nodes, err := impl.GetAllNodesInDC()
// 	if err != nil {
// 		return nil, err
// 	}
// 	return filterNodesWithTaintKeyValueEffect(nodes, taintKey, value, effect), nil
// }

// func (impl *EMMServiceImpl) getPodsWithAnnotationKey(key string) []*corev1.Pod {
// 	pods := impl.GetDCPods()
// 	return filterPodsWithAnnotationKey(pods, key)
// }

// func (impl *EMMServiceImpl) removePodAnnotation(pod *corev1.Pod, key string) error {
// 	if pod.ObjectMeta.Annotations != nil {
// 		delete(pod.ObjectMeta.Annotations, key)
// 		return impl.UpdatePod(pod)
// 	}
// 	return nil
// }

// func (impl *EMMServiceImpl) cleanupEMMAnnotations() (bool, error) {
// 	nodes, err := impl.getNodeNameSetForEvacuateAllData()
// 	if err != nil {
// 		return false, err
// 	}
// 	nodes2, err := impl.getNodeNameSetForPlannedDownTime()
// 	if err != nil {
// 		return false, err
// 	}
// 	nodes = unionStringSet(nodes, nodes2)

// 	// Strip EMM failure annotation from pods where node is no longer tainted
// 	podsFailedEmm := impl.getPodsWithAnnotationKey(EMMFailureAnnotation)
// 	nodesWithPodsFailedEMM := getNodeNameSetForPods(podsFailedEmm)
// 	nodesNoLongerEMM := subtractStringSet(
// 		nodesWithPodsFailedEMM,
// 		nodes)
	
// 	podsNoLongerFailed := filterPodsWithNodeInNameSet(podsFailedEmm, nodesNoLongerEMM)
// 	didUpdate := false
// 	for _, pod := range podsNoLongerFailed {
// 		err := impl.removePodAnnotation(pod, EMMFailureAnnotation)
// 		if err != nil {
// 			return false, err
// 		}
// 		didUpdate = true
// 	}

// 	return didUpdate, nil
// }

// // Check nodes for vmware PSP draining taints
// // and check PVCs for vmware PSP failure annotations
// func checkNodeEMM(provider EMMService) result.ReconcileResult {
// 	// logger := rc.ReqLogger
// 	// rc.ReqLogger.Info("reconciler::checkNodesTaints")

// 	// Strip EMM failure annotation from pods where node is no longer tainted
// 	didUpdate, err := provider.cleanupEMMAnnotations()
// 	if err != nil {
// 		return result.Error(err)
// 	}
// 	if didUpdate {
// 		return result.RequeueSoon(2)
// 	}

// 	// Do not perform EMM operations while the datacenter is initializing
// 	if !provider.IsInitialized() {
// 		return result.Continue()
// 	}

// 	// Find tainted nodes
// 	//
// 	// We may have some tainted nodes where we had previously failed the EMM
// 	// operation, so be sure to filter those out.
// 	plannedDownNodeNameSet, err := provider.getNodeNameSetForPlannedDownTime()
// 	if err != nil {
// 		return result.Error(err)
// 	}
// 	evacuateDataNodeNameSet, err := provider.getNodeNameSetForEvacuateAllData()
// 	if err != nil {
// 		return result.Error(err)
// 	}

// 	// Fail any evacuate data EMM operations if the datacenter is stopped
// 	//
// 	// Cassandra must be up and running to rebuild cassandra nodes. Since 
// 	// evacuating may entail deleting PVCs, we need to fail these operations
// 	// as we are unlikely to be able to carry them out successfully.
// 	if provider.IsStopped() {
// 		didUpdate := false
// 		for nodeName, _ := range evacuateDataNodeNameSet {
// 			podsUpdated, err := provider.failEMM(nodeName, GenericFailure)
// 			if err != nil {
// 				return result.Error(err)
// 			}
// 			didUpdate = didUpdate || podsUpdated
// 		}

// 		if didUpdate {
// 			return result.RequeueSoon(2)
// 		}
// 	}

// 	// NOTE: There might be pods that aren't ready for a variety of reasons,
// 	// however, with respect to data availability, we really only care if a
// 	// pod representing a cassandra node that is _currently_ joined to the 
// 	// cluster is down. We do not care if a pod is down for a cassandra node
//     // that was never joined (for example, maybe we are in the middle of scaling
// 	// up), as such pods are not part of the cluster presently and their being
// 	// down has no impact on data availability. Also, simply looking at the pod
// 	// state label is insufficient here. The pod might be brand new, but 
// 	// represents a cassandra node that is already part of the cluster.
// 	racksWithDownPods := provider.getRacksWithNotReadyPodsJoined()

// 	// If we have multipe racks with down pods we will need to fail any
// 	// EMM operation as cluster availability is already compromised.
// 	if len(racksWithDownPods) > 1 {
// 		allTaintedNameSet := unionStringSet(plannedDownNodeNameSet, evacuateDataNodeNameSet)
// 		didUpdate := false
// 		for nodeName, _ := range allTaintedNameSet {
// 			didUpdatePods, err := provider.failEMM(nodeName, TooManyExistingFailures)
// 			if err != nil {
// 				return result.Error(err)
// 			}
// 			didUpdate = didUpdate || didUpdatePods
// 		}
// 		if didUpdate {
// 			return result.RequeueSoon(2)
// 		}
// 	}

// 	// Remember the down rack as we'll need to fail any EMM operations outside
// 	// of this rack.
// 	downRack := ""
// 	for _, rackName := range racksWithDownPods {
// 		downRack = rackName
// 		break
// 	}

// 	// Fail EMM operations for nodes that do not have pods for the down
// 	// rack
// 	if downRack != "" {
// 		nodeNameSetForDownRack := provider.getNodeNameSetForRack(downRack)

// 		nodeNameSetForNoPodsInRack := subtractStringSet(
// 			unionStringSet(
// 				plannedDownNodeNameSet, 
// 				evacuateDataNodeNameSet),
// 			nodeNameSetForDownRack)

// 		if len(nodeNameSetForNoPodsInRack) > 0 {
// 			didUpdate := false
// 			for nodeName, _ := range nodeNameSetForNoPodsInRack {
// 				podsUpdated, err := provider.failEMM(nodeName, TooManyExistingFailures)
// 				if err != nil {
// 					return result.Error(err)
// 				}
// 				didUpdate = didUpdate || podsUpdated
// 			}
// 			if didUpdate {
// 				return result.RequeueSoon(2)	
// 			}
// 		}
// 	}

// 	// Delete any not ready pods from the tainted nodes
// 	//
// 	// This is necessary as CheckPodsReady() might not start Cassandra on
// 	// any pods we reschedule if there are other pods down at the time.
// 	// For example, if we have a pod on the tainted node that is stuck in
// 	// "Starting", no other pods will have Cassandra started until that is
// 	// resolved. Admittedly, CheckPodsReady() will permit some pods to be
// 	// not ready before starting, but we don't want these two functions 
// 	// to become deeply coupled as it makes testing nightmarishly difficult,
// 	// so we just delete all the errored pods.
// 	//
// 	// Note that, due to earlier checks, we know the only not-ready pods
// 	// are those that have not joined the cluster (so it doesn't matter
// 	// if we delete them) or pods that all belong to the same rack. 
// 	// Consequently, we can delete all such pods on the tainted node without
// 	// impacting availability.
// 	didUpdate, err = provider.removeAllNotReadyPodsOnEMMNodes()
// 	if err != nil {
// 		return result.Error(err)
// 	}
// 	if didUpdate {
// 		return result.RequeueSoon(2)
// 	}

// 	// At this point we know there are no not-ready pods on the tainted nodes,
// 	// and that all pods that are down belong to the same rack as the tainted
// 	// nodes.

// 	// Wait for pods not on tainted nodes to become ready
// 	//
// 	// If we have pods down (for cassandra nodes that have potentially joined
// 	// the cluster) that are not on the tainted nodes, these are likely pods 
// 	// we previously deleted due to the taints. We try to move these pods 
// 	// one-at-a-time to spare ourselves unnecessary rebuilds if
// 	// the EMM operation should fail, so we wait for them to become ready.
// 	if downRack != "" {
// 		didUpdate, err = provider.performPodReplaceForEvacuateData()
// 		if err != nil {
// 			return result.Error(err)
// 		}
// 		if didUpdate {
// 			return result.RequeueSoon(2)
// 		}

// 		// Pods are not ready (because downRack isn't the empty string) and 
// 		// there aren't any pods stuck in an unscheduable state with PVCs on
// 		// on nodes marked for evacuate all data, so continue to allow 
// 		// cassandra a chance to start on the not ready pods. These not ready
// 		// pods are likely ones we deleted previously when moving them off of 
// 		// the tainted node.
// 		//
// 		// TODO: Some of these pods might be from a planned-downtime EMM 
// 		// operation and so will not become ready until their node comes back
// 		// online. With the way this logic works, if two nodes are marked for
// 		// planned-downtime, only one node will have its pods deleted, and the
// 		// other will effectively be ignored until the other node is back 
// 		// online, even if both nodes belong to the same rack. Revisit whether
// 		// this behaviour is desirable.
// 		return result.Continue()
// 	}

// 	// At this point, we know there are no not-ready pods on the tainted 
// 	// nodes and we know there are no down pods that are joined to the 
// 	// cluster, so we can delete any pod we like without impacting
// 	// availability.

// 	// Sanity check that we do not have a down rack
// 	if downRack != "" {
// 		// log an error
// 		// and requeue
// 		// return nil
// 	}

// 	// Delete a pod for an evacuate data tainted node
// 	//
// 	// We give preference to nodes tainted to evacuate all data mainly to
// 	// ensure some level of determinism. We could give preference to 
// 	// planned downtime taints. In an ideal world, we'd address tainted
// 	// nodes in chronilogical order of when they received a taint, but the
// 	// operator doesn't track that information (and I'm not inclined to do
// 	// more book keeping) and the node doesn't have this information as 
// 	// far as I'm aware.
// 	didUpdate, err = provider.removeNextPodFromEvacuateDataNode()
// 	if err != nil {
// 		return result.Error(err)
// 	}
// 	if didUpdate {
// 		return result.RequeueSoon(2)
// 	}

// 	// Delete all pods for a planned down time tainted node
// 	//
// 	// For planned-downtime we will not migrate data to new volumes, so we
// 	// just delete the pods and leave it at that.
// 	didUpdate, err = provider.removeAllPodsFromPlannedDowntimeNode()
// 	if err != nil {
// 		return result.Error(err)
// 	}
// 	if didUpdate {
// 		return result.RequeueSoon(2)
// 	}

// 	// At this point we know that no nodes are tainted, so we can go ahead and
// 	// continue
// 	return result.Continue()
// }
