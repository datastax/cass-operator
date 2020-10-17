// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"sync"
	"time"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	volumeutil "k8s.io/kubernetes/pkg/controller/volume/persistentvolume/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"github.com/datastax/cass-operator/operator/internal/result"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

const (
	EMMFailureAnnotation string    = "appplatform.vmware.com/emm-failure"
	VolumeHealthAnnotations string = "volumehealth.storage.kubernetes.io/health"
)

type VolumeHealth string

const (
	VolumeHealthInaccessible VolumeHealth = "inaccessible"
)

type EMMFailure string

const (
	GenericFailure          EMMFailure = "GenericFailure"
	NotEnoughResources      EMMFailure = "NotEnoughResources"
	TooManyExistingFailures EMMFailure = "TooManyExistingFailures"
)

func getPVCNameForPod(podName string) string {
	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)
	return pvcFullName
}

func getPodNameForPVC(pvcName string) string {
	return strings.TrimPrefix(pvcName, fmt.Sprintf("%s-", PvcName))
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

func (rc *ReconciliationContext) DeletePvcIgnoreFinalizers(podNamespace string, podName string) (*corev1.PersistentVolumeClaim, error) {
	var wg sync.WaitGroup

	wg.Add(1)

	var goRoutineError *error = nil

	pvcFullName := fmt.Sprintf("%s-%s", PvcName, podName)

	pvc, err := rc.GetPVCForPod(podNamespace, podName)
	if err != nil {
		return nil, err
	}

	// Delete might hang due to a finalizer such as kubernetes.io/pvc-protection
	// so we run it asynchronously and then remove any finalizers to unblock it.
	go func() {
		defer wg.Done()
		rc.ReqLogger.Info("goroutine to delete pvc started")

		// If we don't grab a new copy of the pvc, the deletion could fail because the update has
		// changed the pvc and the delete fails because there is a newer version

		pvcToDelete := &corev1.PersistentVolumeClaim{}
		err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: podNamespace, Name: pvcFullName}, pvcToDelete)
		if err != nil {
			rc.ReqLogger.Info("goroutine to delete pvc: error found in get")
			rc.ReqLogger.Error(err, "error retrieving PersistentVolumeClaim for deletion")
			goRoutineError = &err
		}

		rc.ReqLogger.Info("goroutine to delete pvc: no error found in get")

		err = rc.Client.Delete(rc.Ctx, pvcToDelete)
		if err != nil {
			rc.ReqLogger.Info("goroutine to delete pvc: error found in delete")
			rc.ReqLogger.Error(err, "error removing PersistentVolumeClaim",
				"name", pvcFullName)
			goRoutineError = &err
		}
		rc.ReqLogger.Info("goroutine to delete pvc: no error found in delete")
		rc.ReqLogger.Info("goroutine to delete pvc: end of goroutine")
	}()

	// Give the resource a second to get to a terminating state. Note that this
	// may not be reflected in the resource's status... hence the sleep here as
	// opposed to checking the status.
	time.Sleep(5 * time.Second)

	// In the case of PVCs at least, finalizers removed before deletion can be
	// automatically added back. Consequently, we delete the resource first,
	// then remove any finalizers while it is terminating.

	pvc.ObjectMeta.Finalizers = []string{}

	err = rc.Client.Update(rc.Ctx, pvc)
	if err != nil {
		rc.ReqLogger.Info("ignoring error removing finalizer from PersistentVolumeClaim",
			"name", pvcFullName,
			"err", err.Error())

		// Ignore some errors as this may fail due to the resource already having been
		// deleted (which is what we want).
	}

	rc.ReqLogger.Info("before wg.Wait()")

	// Wait for the delete to finish, which should have been unblocked by
	// removing the finalizers.
	wg.Wait()
	rc.ReqLogger.Info("after wg.Wait()")

	// We can't dereference a nil, so check if we have one
	if goRoutineError == nil {
		return pvc, nil
	}
	return nil, *goRoutineError
}

func (rc *ReconciliationContext) removePvcAndPod(pod corev1.Pod) error {

	// Drain the cassandra node

	if isMgmtApiRunning(&pod) {
		err := rc.NodeMgmtClient.CallDrainEndpoint(&pod)
		if err != nil {
			rc.ReqLogger.Error(err, "error during cassandra node drain",
				"pod", pod.ObjectMeta.Name)
			return err
		}
	}

	// Add the cassandra node to replace nodes

	rc.Datacenter.Spec.ReplaceNodes = append(rc.Datacenter.Spec.ReplaceNodes, pod.ObjectMeta.Name)

	// Update CassandraDatacenter
	if err := rc.Client.Update(rc.Ctx, rc.Datacenter); err != nil {
		rc.ReqLogger.Error(err, "Failed to update CassandraDatacenter with removed finalizers")
		return err
	}

	// Remove the pvc

	_, err := rc.DeletePvcIgnoreFinalizers(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	if err != nil {
		rc.ReqLogger.Error(err, "error during PersistentVolume delete",
			"pod", pod.ObjectMeta.Name)
		return err
	}

	// Remove the pod

	err = rc.Client.Delete(rc.Ctx, &pod)
	if err != nil {
		rc.ReqLogger.Error(err, "error during cassandra node delete",
			"pod", pod.ObjectMeta.Name)
		return err
	}

	return nil
}

func getNodeNameSetForPods(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Spec.NodeName] = true
	}
	return names
}

func getRackNameSetForPods(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Labels[api.RackLabel]] = true
	}
	return names
}

func (rc *ReconciliationContext) getNode(nodeName string) (*corev1.Node, error) {
	node := &corev1.Node{}
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: "", Name: nodeName}, node)
	return node, err
}

func (rc *ReconciliationContext) getNodesForNameSet(nodeNameSet map[string]bool) ([]*corev1.Node, error) {
	for nodeName, _ := range nodeNameSet {
		node, err := rc.getNode(nodeName)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (rc *ReconciliationContext) getNodesForPods(pods []*corev1.Pod) ([]*corev1.Node, error) {
	nodeNameSet := getNodeNameSetForPods(pods)
	return getNodesForNameSet(nodeNameSet)
}

func hasTaint(node *corev1.Node, taintKey, value string, effect corev1.TaintEffect) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == taintKey && taint.Effect == effect {
			if taint.Value == value {
				return true
			}
		}
	}
	return false
}

func filterNodesWithFn(nodes []*corev1.Node, fn func(*corev1.Node) bool) []*corev1.Node {
	result := []*corev1.Node{}
	for _, node := range nodes {
		if fn(node) {
			result = append(result, node)
		}
	}
	return result
}

func filterNodesWithTaintKeyValueEffect(nodes []*corev1.Node, taintKey, value string, effect corev1.TaintEffect) []*corev1.Node {
	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
		return hasTaint(node, taintKey, value, effect)
	})
}

func filterNodesForEvacuateAllDataTaint(nodes []*corev1.Node) []*corev1.Node {
	return filterNodesWithTaintKeyValueEffect(nodes, "node.vmware.com/drain", "drain", corev1.TaintEffectNoSchedule)
}

func filterNodesForPlannedDownTimeTaint(nodes []*corev1.Node) []*corev1.Node {
	return filterNodesWithTaintKeyValueEffect(nodes, "node.vmware.com/drain", "planned-downtime", corev1.TaintEffectNoSchedule)
}

func filterNodesInNameSet(nodes []*corev1.Node, nameSet map[string]bool) []*corev1.Node {
	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
		return nameSet[node.Name]
	})
}

func filterNodesNotInNameSet(nodes []*corev1.Node, nameSet map[string]bool) []*corev1.Node {
	return filterNodesWithFn(nodes, func(node *corev1.Node) bool {
		return !nameSet[node.Name]
	})
}

func filterPodsWithFn(pods []*corev1.Pod, fn func(*corev1.Pod)bool) []*corev1.Pod {
	result := []*corev1.Pod{}
	for _, pod := range pods {
		result = append(result, pod)
	}
	return result
}

func filterPodsWithNodeInNameSet(pods []*corev1.Pod, nameSet map[string]bool) []*corev1.Pod {
	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
		return nameSet[pod.Spec.NodeName]
	})
}

func filterPodsFailedEMM(pods []*corev1.Pod) []*corev1.Pod {
	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
		return nameSet[pod.Spec.NodeName]
	})
}

func getNameSetForNodes(nodes []*corev1.Node) map[string]bool {
	result := map[string]bool{}
	for _, node := range nodes {
		result[node.Name] = true
	}
	return result
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

func (rc *ReconciliationContext) removePod(pod *corev1.Pod) error {
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

func (rc *ReconciliationContext) removePods(pods []*corev1.Pod) error {
	for _, pod := range pods {
		err := rc.removePod(pod)
		if err != nil {
			return err
		}
	}
	return nil
}

func isPodUnschedulable(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Reason == corev1.PodReasonUnschedulable && 
			condition.Type == corev1.PodScheduled &&
			condition.Status == corev1.ConditionFalse {
				return true
			}
	}
	return false
}

func getSelectedNodeNameForPVC(pvc *corev1.PersistentVolumeClaim) string {
	annos := pvc.Annotations
	if annos == nil {
		annos = map[string]string{}
	}
	pvcNode := annos[volumeutil.AnnSelectedNode]
	return pvcNode
}

func (rc *ReconciliationContext) getDCPodByName(podName) *corev1.Pod {
	pods := filterPodsWithFn(rc.dcPods, func(pod *corev1.Pod) bool {
		return pod.Name == podName
	})

	if len(pods) == 0 {
		return nil
	}

	return pods[0]
}

func (rc *ReconciliationContext) startNodeReplace(podName string) error {
	pod := getDCPodByName(podName)
	if pod == nil {
		return fmt.Errorf("Pod with name '%s' not part of datacenter", podName)
	}

	pvc := rc.GetPVCForPod(pod)
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

	err = rc.removePod(pod)
	if err != nil {
		return err
	}

	return nil
}

func (rc *ReconciliationContext) getPVCsForPods(pods []*corev1.Pod) []*corev1.PersistentVolumeClaim {
	pvcs := []*corev1.PersistentVolumeClaim{}
	for _, pod := pods {
		pvc := rc.GetPVCForPod(pod.Namespace, pod.Name)
		pvcs := append(pvcs, pvc)
	}
	return pvcs
}

func getNodeNameSetForPVCs(pvcs []*corev1.PersistentVolumeClaim) map[string]bool {
	nodeNameSet = map[string]bool{}
	for _, pvc := range pvcs {
		nodeName := getSelectedNodeNameForPVC(pvc)
		if nodeName != "" {
			nodeNameSet[nodeName] = true
		}
	}
	return nodeNameSet
}

func getPVCNameSetForPods(pods []*corev1.Pod) map[string]bool {
	nameSet := map[string]bool{}
	for _, pod := range pods {
		nameSet[getPVCNameForPod(pod)] = true
	}
	return nameSet
}

func filterPVCsWithFn(pvcs []*corev1.PersistentVolumeClaim, fn func(*corev1.PersistentVolumeClaim) bool) {
	result := []*corev1.PersistentVolumeClaim{}
	for _, pvc := range pvcs {
		if fn(pvc) {
			result := append(result, pvc)
		}
	}
	return result
}

func filterPVCsInNameSet(pvcs []*corev1.PersistentVolumeClaim, nameSet map[string]bool) []*corev1.PersistentVolumeClaim {
	return filterPVCsWithFn(
		pvcs,
		func(pvc *corev1.PersistentVolumeClaim) bool {
			return nameSet[pvc.Name]
		})
}

func filterPodsThatHaveEMMFailureAnnotation(pods []*corev1.Pod) []*corev1.Pod {
	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
		annos = pod.Annotations
		if annos == nil {
			return false
		}
		_, ok := annos[EMMFailureAnnotation]
		return ok
	})
}

func (rc *ReconciliationContext) checkPVCHealth() {
	logger := rc.ReqLogger
	rc.ReqLogger.Info("reconciler::checkPVCHealth")


}

func (rc *ReconciliationContext) updatePod(pod *corev1.Pod) {
	return rc.Client.Update(rc.Ctx, pod)
}

func (rc *ReconciliationContext) failEMMForPods(pods []*corev1.Pod, reason EMMFailure) (bool, error) {
	updatedAny := false
	for _, pod := range pods {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		val, _ := pod.Annotations[EMMFailureAnnotation]
		if reason != val {
			pod.Annotations[EMMFailureAnnotation] = reason
			updatedAny = true
			err := rc.updatePod(pod)
			if err != nil {
				return false, err
			}
		}
	}
	return updatedAny, err
}

func (rc *ReconciliationContext) removeEMMFailureAnnotations(pods []*corev1.Pod) (bool, error) {
	updatedAny := false
	for _, pod := range pods {
		if pod.Annotations != nil {
			_, ok := pod.Annotations[EMMFailureAnnotation]
			if ok {
				delete(pod.Annotations, EMMFailureAnnotation)
				updatedAny = true
				err := rc.updatePod(pod)
				if err != nil {
					return false, err
				}
			}
		}
	}
	return updatedAny, nil
}

func filterPVCsWithVolumeHealth(pvcs []*corev1.PersistentVolumeClaim, health VolumeHealth) []*corev1.PersistentVolumeClaim {
	return filterPVCsWithFn(
		pvcs, 
		func(pvc *corev1.PersistentVolumeClaim) {
			annos := pvc.Annotations
			return annos != nil && annos[VolumeHealthAnnotations] == VolumeHealthInaccessible})
}

type Foo interface {
	getInaccessiblePVCs()
	getNotReadyPodsJoined()
	startNodeReplace(podName string)
	getInProgressNodeReplacements() []string
}

func (rc *ReconciliationContext) checkPVCHealth() result.ReconcileResult {
	inaccessiblePVCs := filterPVCsWithVolumeHealth(
		rc.getPVCsForPods(rc.dcPods), 
		VolumeHealthInaccessible)

	if len(inaccessiblePVCs) == 0 {
		// nothing to do
		return result.Continue()
	}
	
	downPods := findAllPodsNotReadyAndPotentiallyJoined(rc.dcPods, rc.Datacenter.Status.NodeStatuses)
	racksWithDownPods := getRackNameSetForPods(downPods)

	if len(racksWithDownPods) > 1 {
		// Availability is currently compromised, don't do anything
		return result.Continue()
	}

	if len(racksWithDownPods) == 1 {
		inaccessiblePVCs = filterPVCsWithVolumeHealth(
			downPods, 
			VolumeHealthInaccessible)

		if len(inaccessiblePVCs) == 0 {
			// there are inaccessible PVCs, but none on the rack that is currently
			// compromised
			return result.Continue()
		}
	}

	if len(rc.Datacenter.Status.NodeReplacements) > 0 {
		// there are inaccessible PVCs, but we are currently replacing some
		// other node in the cluster
		return result.Continue()
	}

	for _, inaccessiblePVCs := range inaccessiblePVCs {
		err := rc.startNodeReplace(getPodNameForPVC(inaccessiblePVCs))
		if err != nil {
			return result.Error(err)
		}
		return result.Requeue(2)
	}

	// Should not be possible to get here, but just in case
	return result.Continue()
}

func (rc *ReconciliationContext) getDCNodes() ([]*corev1.Node, error) {
	pvcs := rc.getPVCsForPods(rc.dcPods)
	nodeNameSet := unionStringSet(getNodeNameSetForPVCs(pvcs), getNodeNameSetForPods(rc.dcPods))
	return rc.getNodesForNameSet(nodeNameSet)
}

// NOTE: This check has to come before CheckPodsReady() because we will tolerate
// some pods to be down so long as they are on the tainted node.

// Check nodes for vmware PSP draining taints
// and check PVCs for vmware PSP failure annotations
func (rc *ReconciliationContext) checkNodeAndPvcTaints() result.ReconcileResult {
	logger := rc.ReqLogger
	rc.ReqLogger.Info("reconciler::checkNodesTaints")

	// Get all nodes for datacenter
	//
	// We need to check taints for all nodes that this datacenter cares about,
	// this includes not just nodes where we have dc pods, but also nodes 
	// where we have PVCs, as PVCs might get separated from their pod when a
	// pod is rescheduled.
	pvcs := rc.getPVCsForPods(rc.dcPods)
	nodeNameSet := unionStringSet(getNodeNameSetForPVCs(pvcs), getNodeNameSetForPods(rc.dcPods))
	nodes, err := rc.getNodesForNameSet(nodeNameSet)

	// Bail if we cannot list the nodes for some reason
	if err != nil {
		return result.Error(err)
	}

	// Find tainted nodes
	//
	// We may have some tainted nodes where we had previously failed the EMM
	// operation, so be sure to filter those out.
	plannedDownNodes := filterNodesForPlannedDownTimeTaint(nodes)
	evacuateDataNodes := filterNodesForEvacuateAllDataTaint(nodes)

	// Strip EMM failure annotation from pods where node is no longer tainted
	podsFailedEmm := filterPodsThatHaveEMMFailureAnnotation(rc.dcPods)
	nodesWithPodsFailedEMM := getNodeNameSetForPods(podsFailEmm)
	nodesNoLongerEMM := subtranctStringSet(
		nodesWithPodsFailedEMM,
		unionStringSet(
			getNameSetForNodes(plannedDownNodes),
			getNameSetForNodes(evacuateDataNodes)))
	didUpdate, err := rc.removeEMMFailureAnnotations(filterPodsWithNodeInNameSet(rc.dcPods, nodesNoLongerEMM))
	if err != nil {
		return result.Error(err)
	}
	if didUpdate {
		return result.Requeue(2)
	}

	// Account for EMM operations previously failed
	plannedDownNodes = filterNodesNotInNameSet(nodesWithPodsFailedEMM)
	evacuateDataNodes = filterNodesNotInNameSet(nodesWithPodsFailedEMM)

	// NOTE: There might be pods that aren't ready for a variety of reasons,
	// however, with respect to data availability, we really only care if a
	// pod representing a cassandra node that is _currently_ joined to the 
	// cluster is down. We do not care if a pod is down for a cassandra node
    // that was never joined (for example, maybe we are in the middle of scaling
	// up), as such pods are not part of the cluster presently and their being
	// down has no impact on data availability. Also, simply looking at the pod
	// state label is insufficient here. The pod might be brand new, but 
	// represents a cassandra node that is already part of the cluster.
	downPods := findAllPodsNotReadyAndPotentiallyJoined(rc.dcPods, rc.Datacenter.Status.NodeStatuses)
	racksWithDownPods := getRackNameSetForPods(downPods)

	// If we have multipe racks with down pods we will need to fail any
	// EMM operation as cluster availability is already compromised.
	if len(racksWithDownPods) > 1 {
		nodeNameSetToFail := unionStringSet(
			getNameSetForNodes(plannedDownNodes), 
			getNameSetForNodes(evacuateDataNodes))
		podsFailEMM := filterPodsWithNodeInNameSet(rc.dcPods, nodeNameSetToFail)
		if len(failEMMForPods) > 0 {
			err := failEMMForPods(podsFailEMM, TooManyExistingFailures)
			if err != nil {
				return result.Error(err)
			}
			return result.Requeue(2)
		}
	}

	// Remember the down rack as we'll need to fail any EMM operations outside
	// of this rack.
	downRack := ""
	for rackName := range racksWithDownPods {
		downRack = rackName
		break
	}

	// Fail EMM operations for nodes that do not have pods for the down
	// rack
	if downRack != "" {
		podsForDownRack := FilterPodListByLabels(rc.dcPods, map[string]string{api.RackLabel: downRack})
		nodeNameSetForRack := getNodeNameSetForPods(podsForDownRack)
		nodeNameSetForNoPodsInRack := subtractStringSet(
			unionStringSet(
				getNameSetForNodes(plannedDownNodes), 
				getNameSetForNodes(evacuateDataNodes)),
			nodeNameSetForRack)

		if len(nodeNameSetForNoPodsInRack) > 0 {
			// fail the EMM and requeue
			podsFailEMM := filterPodsWithNodeInNameSet(rc.dcPods, nodeNameSetForNoPodsInRack)
			didUpdate, err := failEMMForPods(podsFailEMM, TooManyExistingFailures)
			if err != nil {
				return result.Error(err)
			}
			if didUpdate {
				return result.Requeue(2)	
			}
		}
	}

	// Delete any not ready pods from the tainted nodes
	//
	// This is necessary as CheckPodsReady() might not start Cassandra on
	// any pods we reschedule if there are other pods down at the time.
	// For example, if we have a pod on the tainted node that is stuck in
	// "Starting", no other pods will have Cassandra started until that is
	// resolved. Admittedly, CheckPodsReady() will permit some pods to be
	// not ready before starting, but we don't want these two functions 
	// to become deeply coupled as it makes testing nightmarishly difficult,
	// so we just delete all the errored pods.
	//
	// Note that, due to earlier checks, we know the only not-ready pods
	// are those that have not joined the cluster (so it doesn't matter
	// if we delete them) or pods that all belong to the same rack. 
	// Consequently, we can delete all such pods on the tainted node without
	// impacting availability.
	podsNotReady := findAllPodsNotReady(rc.dcPods)
	taintedNodesNameSet := unionStringSet(
		getNameSetForNodes(plannedDownNodes),
		getNameSetForNodes(evacuateDataNodes))
	podsNotReadyOnTaintedNodes := filterPodsWithNodeInNameSet(podsNotReady, taintedNodesNameSet)
	
	if len(podsNotReadyOnTaintedNodes) > 1 {
		err := rc.removePods(podsNotReadyOnTaintedNodes)
		if err != nil {
			return result.Error(err)
		}
		return result.Requeue(2)
	}

	// At this point we know there are no not-ready pods on the tainted nodes,
	// and that all pods that are down belong to the same rack as the tainted
	// nodes.

	// Wait for pods not on tainted nodes to become ready
	//
	// If we have pods down (for cassandra nodes that have potentially joined
	// the cluster) that are not on the tainted nodes, these are likely pods 
	// we previously deleted due to the taints. We try to move these pods 
	// one-at-a-time to spare ourselves unnecessary rebuilds if
	// the EMM operation should fail, so we wait for them to become ready.
	if len(downPods) > 0 {

		// Check if any of these pods are stuck due to PVC associated to a 
		// tainted node for evacuate all data. This would happen, for example,
		// in the case of local persistent volumes where the volume cannot
		// move with the pod.
		//
		// NOTE: This arguably belongs in our check for stuck pods that
		// CheckPodsReady() does. Keeping all the logic together for the
		// time being since it is all pretty specific to PSP at the moment.
		deletedPodsOrPVCs := false
		for _, pod := range downPods {
			if isPodUnschedulable(pod) {

				// NOTE: There isn't a great machine readable way to know why
				// the pod is unschedulable. The reasons are, unfortunately,
				// buried within human readable explanation text. As a result,
				// a pod might not get scheduled due to no nodes having 
				// sufficent memory, and then we delete a PVC thinking that
				// the PVC was causing scheduling to fail even though it 
				// wasn't.
				pvc, err := rc.GetPVCForPod(pod.Namespace, pod.Name)
				if err != nil {
					if errors.IsNotFound(err) {
						// let CheckPodsReady() figure this error out
						continue
					} else {
						// requeue, try this again later
						return result.Error(err)
					}
				}
				pvcNode := getSelectedNodeNameForPVC(pvc)
				if pvcNode != "" && pod.Spec.NodeName != pvcNode{
					if getNameSetForNodes(evacuateDataNodes)[pvcNode] {
						deletedPodsOrPVCs = true

						// set pod to be replaced
						err := rc.startNodeReplace(pod.Name)
						if err != nil {
							return result.Error(err)
						}

						// delete pod and pvc
						err = rc.removePVC(pvc)
						if err != nil {
							return result.Error(err)
						}

						err = rc.removePod(pod)
						if err != nil {
							return result.Error(err)
						}
					}
				}
			}
		}

		if deletedPodsOrPVCs {
			// requeue
			return result.Requeue(2)
		}
		
		// Allow CheckPodsReady() to start cassandra on any down nodes
		return result.Continue()
	}

	// At this point, we know there are no not-ready pods on the tainted 
	// nodes and we know there are no down pods that are joined to the 
	// cluster, so we can delete any pod we like without impacting
	// availability.

	// Sanity check that we do not have a down rack
	if downRack != "" {
		// log an error
		// and requeue
		// return nil
	}

	// Delete a pod for an evacuate data tainted node
	//
	// We give preference to nodes tainted to evacuate all data mainly to
	// ensure some level of determinism. We could give preference to 
	// planned downtime taints. In an ideal world, we'd address tainted
	// nodes in chronilogical order of when they received a taint, but the
	// operator doesn't track that information (and I'm not inclined to do
	// more book keeping) and the node doesn't have this information as 
	// far as I'm aware.
	podsForEvacuateData := filterPodsWithNodeInNameSet(rc.dcPods, getNodeNameSet(evacuateDataNodes))
	for _, pod := range podsForEvacuateData {
		err := rc.deletePod(pod)
		if err {
			return result.Error(err)
		}
		return result.Requeue(2)
	}

	// Delete all pods for a planned down time tainted node
	//
	// For planned-downtime we will not migrate data to new volumes, so we
	// just delete the pods and leave it at that.
	for _, node := range plannedDownNodes {
		podsOnPlannedDown := filterPodWithNodeInNameSet(rc.dcPods, map[string]bool {node.Name: true})
		if len(podsOnPlannedDown) > 0 {
			for _, pod := range podsOnPlannedDown {
				err := rc.deletePod(pod)
				if err != nil {
					return result.Error(err)
				}
			}
			return result.Requeue(2)
		}
	}

	// At this point we know that no nodes are tainted, so we can go ahead and
	// continue
	return result.Continue()

	// // Get the pods

	// podList, err := rc.listPods(rc.Datacenter.GetClusterLabels())
	// if err != nil {
	// 	logger.Error(err, "error listing all pods in the cluster")
	// }

	// rc.clusterPods = PodPtrsFromPodList(podList)

	// for _, pod := range podList.Items {

	// 	// Check the related node for taints
	// 	node := &corev1.Node{}
	// 	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: "", Name: pod.Spec.NodeName}, node)
	// 	if err != nil {
	// 		logger.Error(err, "error retrieving node for pod for node taint check")
	// 		return err
	// 	}

	// 	rc.ReqLogger.Info(fmt.Sprintf("node %s has %d taints", node.ObjectMeta.Name, len(node.Spec.Taints)))

	// 	for _, taint := range node.Spec.Taints {
	// 		if taint.Key == "node.vmware.com/drain" && taint.Effect == "NoSchedule" {
	// 			if taint.Value == "planned-downtime" || taint.Value == "drain" {
	// 				rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found.  draining and deleting pod",
	// 					"pod", pod.Name)

	// 				err = rc.removePvcAndPod(pod)
	// 				if err != nil {
	// 					rc.ReqLogger.Error(err, "error during cassandra node drain",
	// 						"pod", pod.Name)
	// 					return err
	// 				}
	// 			}
	// 		}
	// 	}

	// 	// Get the name of the pvc

	// 	pvcName := ""
	// 	for _, vol := range pod.Spec.Volumes {
	// 		if vol.Name == PvcName {
	// 			pvcName = vol.PersistentVolumeClaim.ClaimName
	// 		}
	// 	}

	// 	// Check the related PVCs for annotation

	// 	pvc := &corev1.PersistentVolumeClaim{}
	// 	err = rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: pod.ObjectMeta.Namespace, Name: pvcName}, pvc)
	// 	if err != nil {
	// 		logger.Error(err, "error retrieving PersistentVolumeClaim for pod for pvc annotation check")
	// 		return err
	// 	}

	// 	rc.ReqLogger.Info(fmt.Sprintf("pvc %s has %d annotations", pvc.ObjectMeta.Name, len(pvc.ObjectMeta.Annotations)))

	// 	// “volumehealth.storage.kubernetes.io/health”: “inaccessible”

	// 	for k, v := range pvc.ObjectMeta.Annotations {
	// 		if k == "volumehealth.storage.kubernetes.io/health" && v == "inaccessible" {
	// 			rc.ReqLogger.Info("vmware pvc inaccessible annotation found.  draining and deleting pod",
	// 				"pod", pod.Name)

	// 			err = rc.removePvcAndPod(pod)
	// 			if err != nil {
	// 				rc.ReqLogger.Error(err, "error during cassandra node drain",
	// 					"pod", pod.Name)
	// 				return err
	// 			}
	// 		}
	// 	}
	// }

	// return nil
}
