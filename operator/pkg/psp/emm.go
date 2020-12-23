// Copyright DataStax, Inc.
// Please see the included license file for details.

// PSP EMM operations are triggered by a user through a UI and appear to the
// operator as taints on k8s nodes. To allow the EMM operation to proceed, all
// pods must be removed from the tainted node. EMM can be cancelled by adding
// a failure annotation to pods on the node.
//
// The way we have implemented EMM here is to only allow an EMM operation to
// proceed if it does not compromise availability. We assume RF of 3 with 3
// racks, meaning we will never allow more than 1 rack to have bootstrapped
// pods that are not ready. We effectively ignore pods that are not ready and
// represent cassandra nodes that were never bootstrapped, since their being
// up or down has no meaningful impact on the availability of the cassandra
// datacenter (since they don't belong to the ring yet). We do allow an EMM
// operation to proceed, subject to the above constraints, when there are
// not ready bootstrapped pods on the rack the operation is being performed
// on, as the operation might be being performed to resolve the problem
// causing the pods to lose readiness. For example, a node might be
// temporarily taken offline to replace defective memory which was causing
// cassandra to crash.

package psp

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/datastax/cass-operator/operator/pkg/utils"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

const (
	EMMFailureAnnotation   = "appplatform.vmware.com/emm-failure"
	VolumeHealthAnnotation = "volumehealth.storage.kubernetes.io/health"
	EMMTaintKey            = "node.vmware.com/drain"
)

type EMMTaintValue string

const (
	EvacuateAllData EMMTaintValue = "drain"
	PlannedDowntime               = "planned-downtime"
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

// This interface is what, in practice, the ReconciliationContext will need to
// implement. Its intention is to keep coupling with the reconciliation package
// low and also to keep this package relatively sheltered from certain thorny
// details, like determining which pods represent cassandra nodes that have
// bootstrapped at some point.
type EMMSPI interface {
	GetAllNodesInDC() ([]*corev1.Node, error)
	GetDCPods() []*corev1.Pod
	GetNotReadyPodsBootstrappedInDC() []*corev1.Pod
	GetAllPodsNotReadyInDC() []*corev1.Pod
	GetPodPVCs(pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error)
	StartNodeReplace(podName string) error
	GetInProgressNodeReplacements() []string
	RemovePod(pod *corev1.Pod) error
	UpdatePod(pod *corev1.Pod) error
	IsStopped() bool
	IsInitialized() bool
	GetLogger() logr.Logger
	GetAllNodes() ([]*corev1.Node, error)
}

type EMMChecks interface {
	getPodPVCSelectedNodeName(podName string) (string, error)
	getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (utils.StringSet, error)
	getRacksWithNotReadyPodsBootstrapped() []string
	getRackNodeNameSet(rackName string) utils.StringSet
	getInProgressNodeReplacements() []string
	IsStopped() bool
	IsInitialized() bool
	getNodeNameSet() (utils.StringSet, error)
	getPodNameSet() utils.StringSet
}

type EMMOperations interface {
	cleanupEMMAnnotations() (bool, error)
	getPlannedDownTimeNodeNameSet() (utils.StringSet, error)
	getEvacuateAllDataNodeNameSet() (utils.StringSet, error)
	removeAllNotReadyPodsOnEMMNodes() (bool, error)
	failEMM(nodeName string, failure EMMFailure) (bool, error)
	performEvacuateDataPodReplace() (bool, error)
	removeNextPodFromEvacuateDataNode() (bool, error)
	removeAllPodsFromOnePlannedDowntimeNode() (bool, error)
	startNodeReplace(podName string) error
	emmFailureStillProcessing() (bool, error)
}

type EMMService interface {
	EMMOperations
	EMMChecks
	getLogger() logr.Logger
}

//
// Util
//
func getPodsRackNameSet(pods []*corev1.Pod) utils.StringSet {
	names := utils.StringSet{}
	for _, pod := range pods {
		names[pod.Labels[api.RackLabel]] = true
	}
	return names
}

func filterPodsByRackName(pods []*corev1.Pod, rackName string) []*corev1.Pod {
	return utils.FilterPodsWithLabel(pods, api.RackLabel, rackName)
}

//
// EMMOperations impl
//
type EMMServiceImpl struct {
	EMMSPI
}

func (impl *EMMServiceImpl) getPodPVCSelectedNodeName(podName string) (string, error) {
	matchingPods := utils.FilterPodsWithFn(impl.GetDCPods(), func(pod *corev1.Pod) bool {
		return pod != nil && pod.Name == podName
	})

	if len(matchingPods) == 0 {
		return "", nil
	}

	pod := matchingPods[0]

	pvcs, err := impl.GetPodPVCs(pod)
	if err != nil {
		return "", err
	}

	for _, pvc := range pvcs {
		nodeName := utils.GetPVCSelectedNodeName(pvc)
		if nodeName != "" {
			return nodeName, nil
		}
	}

	return "", nil
}

func (impl *EMMServiceImpl) cleanupEMMAnnotations() (bool, error) {
	nodes, err := impl.getEvacuateAllDataNodeNameSet()
	if err != nil {
		return false, err
	}
	nodes2, err := impl.getPlannedDownTimeNodeNameSet()
	if err != nil {
		return false, err
	}
	nodes = utils.UnionStringSet(nodes, nodes2)

	// Strip EMM failure annotation from pods where node is no longer tainted
	podsFailedEmm := impl.getPodsWithAnnotationKey(EMMFailureAnnotation)
	nodesWithPodsFailedEMM := utils.GetPodNodeNameSet(podsFailedEmm)
	nodesNoLongerEMM := utils.SubtractStringSet(
		nodesWithPodsFailedEMM,
		nodes)

	podsNoLongerFailed := utils.FilterPodsWithNodeInNameSet(podsFailedEmm, nodesNoLongerEMM)
	didUpdate := false
	for _, pod := range podsNoLongerFailed {
		err := impl.removePodAnnotation(pod, EMMFailureAnnotation)
		if err != nil {
			return false, err
		}
		didUpdate = true
	}

	return didUpdate, nil
}

func (impl *EMMServiceImpl) emmFailureStillProcessing() (bool, error) {
	nodes, err := impl.getEvacuateAllDataNodeNameSet()
	if err != nil {
		return false, err
	}
	nodes2, err := impl.getPlannedDownTimeNodeNameSet()
	if err != nil {
		return false, err
	}
	nodes = utils.UnionStringSet(nodes, nodes2)

	// Strip EMM failure annotation from pods where node is no longer tainted
	podsFailedEmm := impl.getPodsWithAnnotationKey(EMMFailureAnnotation)
	nodesWithPodsFailedEMM := utils.GetPodNodeNameSet(podsFailedEmm)

	return len(utils.IntersectionStringSet(nodes, nodesWithPodsFailedEMM)) > 0, nil
}

func (impl *EMMServiceImpl) getPlannedDownTimeNodeNameSet() (utils.StringSet, error) {
	nodes, err := impl.getNodesWithTaintKeyValueEffect(EMMTaintKey, string(PlannedDowntime), corev1.TaintEffectNoSchedule)
	if err != nil {
		return nil, err
	}
	return utils.GetNodeNameSet(nodes), nil
}

func (impl *EMMServiceImpl) getEvacuateAllDataNodeNameSet() (utils.StringSet, error) {
	nodes, err := impl.getNodesWithTaintKeyValueEffect(EMMTaintKey, string(EvacuateAllData), corev1.TaintEffectNoSchedule)
	if err != nil {
		return nil, err
	}
	return utils.GetNodeNameSet(nodes), nil
}

func (impl *EMMServiceImpl) removeAllNotReadyPodsOnEMMNodes() (bool, error) {
	podsNotReady := impl.GetAllPodsNotReadyInDC()
	plannedDownNameSet, err := impl.getPlannedDownTimeNodeNameSet()
	if err != nil {
		return false, err
	}

	evacuateDataNameSet, err := impl.getEvacuateAllDataNodeNameSet()
	if err != nil {
		return false, err
	}

	taintedNodesNameSet := utils.UnionStringSet(plannedDownNameSet, evacuateDataNameSet)
	podsNotReadyOnTaintedNodes := utils.FilterPodsWithNodeInNameSet(podsNotReady, taintedNodesNameSet)

	if len(podsNotReadyOnTaintedNodes) > 0 {
		for _, pod := range podsNotReadyOnTaintedNodes {
			err := impl.RemovePod(pod)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}

	return false, nil
}

func (impl *EMMServiceImpl) failEMM(nodeName string, failure EMMFailure) (bool, error) {
	pods := impl.getPodsForNodeName(nodeName)
	didUpdate := false
	for _, pod := range pods {
		added, err := impl.addPodAnnotation(pod, EMMFailureAnnotation, string(failure))
		if err != nil {
			return false, err
		}
		didUpdate = didUpdate || added
	}
	return didUpdate, nil
}

func (impl *EMMServiceImpl) getNodeNameSet() (utils.StringSet, error) {
	nodes, err := impl.EMMSPI.GetAllNodes()
	if err != nil {
		return nil, err
	}

	return utils.GetNodeNameSet(nodes), nil
}

func (impl *EMMServiceImpl) getPodNameSet() utils.StringSet {
	return utils.GetPodNameSet(impl.EMMSPI.GetDCPods())
}

func (impl *EMMServiceImpl) performEvacuateDataPodReplace() (bool, error) {
	downPods := impl.GetNotReadyPodsBootstrappedInDC()
	if len(downPods) > 0 {
		evacuateAllDataNameSet, err := impl.getEvacuateAllDataNodeNameSet()
		if err != nil {
			return false, err
		}

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
			if utils.IsPodUnschedulable(pod) {

				// NOTE: There isn't a great machine readable way to know why
				// the pod is unschedulable. The reasons are, unfortunately,
				// buried within human readable explanation text. As a result,
				// a pod might not get scheduled due to no nodes having
				// sufficent memory, and then we delete a PVC thinking that
				// the PVC was causing scheduling to fail even though it
				// wasn't.

				pvcNode, err := impl.getPodPVCSelectedNodeName(pod.Name)
				if err != nil {
					return false, err
				}
				if pvcNode != "" && pod.Spec.NodeName != pvcNode {
					if evacuateAllDataNameSet[pvcNode] {
						deletedPodsOrPVCs = true

						// set pod to be replaced
						err := impl.StartNodeReplace(pod.Name)
						if err != nil {
							return false, err
						}
					}
				}
			}
		}

		return deletedPodsOrPVCs, nil
	}
	return false, nil
}

func (impl *EMMServiceImpl) removeNextPodFromEvacuateDataNode() (bool, error) {
	nodeNameSet, err := impl.getEvacuateAllDataNodeNameSet()
	if err != nil {
		return false, err
	}

	for name, _ := range nodeNameSet {
		for _, pod := range impl.getPodsForNodeName(name) {
			err := impl.RemovePod(pod)
			if err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
}

func (impl *EMMServiceImpl) removeAllPodsFromOnePlannedDowntimeNode() (bool, error) {
	nodeNameSet, err := impl.getPlannedDownTimeNodeNameSet()
	for nodeName, _ := range nodeNameSet {
		pods := impl.getPodsForNodeName(nodeName)
		if len(pods) > 0 {
			for _, pod := range pods {
				err := impl.RemovePod(pod)
				if err != nil {
					return false, err
				}
			}
			return true, err
		}
	}
	return false, nil
}

func (impl *EMMServiceImpl) startNodeReplace(podName string) error {
	return impl.StartNodeReplace(podName)
}

func (impl *EMMServiceImpl) getLogger() logr.Logger {
	return impl.GetLogger()
}

//
// EMMChecks impl
//
func (impl *EMMServiceImpl) getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (utils.StringSet, error) {
	pods := impl.GetDCPods()
	result := []*corev1.Pod{}
	for _, pod := range pods {
		pvcs, err := impl.GetPodPVCs(pod)
		if err != nil {
			return nil, err
		}

		pvcs = utils.FilterPVCsWithFn(pvcs, func(pvc *corev1.PersistentVolumeClaim) bool {
			return pvc.Annotations != nil && pvc.Annotations[VolumeHealthAnnotation] == string(VolumeHealthInaccessible)
		})

		if len(pvcs) > 0 {
			result = append(result, pod)
		}
	}

	if rackName != "" {
		return utils.GetPodNameSet(filterPodsByRackName(result, rackName)), nil
	} else {
		return utils.GetPodNameSet(result), nil
	}
}

func (impl *EMMServiceImpl) getRacksWithNotReadyPodsBootstrapped() []string {
	pods := impl.GetNotReadyPodsBootstrappedInDC()
	rackNameSet := getPodsRackNameSet(pods)
	rackNames := []string{}
	for rackName, _ := range rackNameSet {
		rackNames = append(rackNames, rackName)
	}
	return rackNames
}

func (impl *EMMServiceImpl) getRackNodeNameSet(rackName string) utils.StringSet {
	podsForDownRack := utils.FilterPodsWithLabel(impl.GetDCPods(), api.RackLabel, rackName)
	nodeNameSetForRack := utils.GetPodNodeNameSet(podsForDownRack)
	return nodeNameSetForRack
}

func (impl *EMMServiceImpl) getInProgressNodeReplacements() []string {
	return impl.GetInProgressNodeReplacements()
}

//
// Helper methods
//
func (impl *EMMServiceImpl) getNodesWithTaintKeyValueEffect(taintKey, value string, effect corev1.TaintEffect) ([]*corev1.Node, error) {
	nodes, err := impl.GetAllNodesInDC()
	if err != nil {
		return nil, err
	}
	return utils.FilterNodesWithTaintKeyValueEffect(nodes, taintKey, value, effect), nil
}

func (impl *EMMServiceImpl) getPodsForNodeName(nodeName string) []*corev1.Pod {
	return utils.FilterPodsWithNodeInNameSet(impl.GetDCPods(), utils.StringSet{nodeName: true})
}

func (impl *EMMServiceImpl) getPodsWithAnnotationKey(key string) []*corev1.Pod {
	pods := impl.GetDCPods()
	return utils.FilterPodsWithAnnotationKey(pods, key)
}

func (impl *EMMServiceImpl) addPodAnnotation(pod *corev1.Pod, key, value string) (bool, error) {
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	} else if currentValue, found := pod.Annotations[key]; found {
		//check if the value matches, if so then the work is already done
		return currentValue == value, nil
	}

	pod.Annotations[key] = value
	return true, impl.UpdatePod(pod)
}

func (impl *EMMServiceImpl) removePodAnnotation(pod *corev1.Pod, key string) error {
	if pod.ObjectMeta.Annotations != nil {
		delete(pod.ObjectMeta.Annotations, key)
		return impl.UpdatePod(pod)
	}
	return nil
}

// Check nodes for vmware PSP draining taints. This function embodies the
// business logic around when EMM operations are executed.
func checkNodeEMM(provider EMMService) result.ReconcileResult {
	logger := provider.getLogger()

	// Strip EMM failure annotation from pods where node is no longer tainted
	didUpdate, err := provider.cleanupEMMAnnotations()
	if err != nil {
		logger.Error(err, "Encountered an error while cleaning up defunct EMM failure annotations")
		return result.Error(err)
	}
	if didUpdate {
		logger.Info("Cleaned up defunct EMM failure annotations")
		return result.RequeueSoon(2)
	}

	// Check if there are still pods annotated with EMM failure.
	// If there are then that means vmware has not removed the node
	// taint in response to a failure. We can requeue or stop
	// reconciliation in this situation.
	stillProcessing, err := provider.emmFailureStillProcessing()
	if err != nil {
		logger.Error(err, "Failed to check if EMM failures are still processing")
		return result.Error(err)
	}
	if stillProcessing {
		return result.RequeueSoon(2)
	}

	// Do not perform EMM operations while the datacenter is initializing
	if !provider.IsInitialized() {
		logger.Info("Skipping EMM check as the cluster is not yet initialized")
		return result.Continue()
	}

	// Find tainted nodes
	//
	// We may have some tainted nodes where we had previously failed the EMM
	// operation, so be sure to filter those out.
	plannedDownNodeNameSet, err := provider.getPlannedDownTimeNodeNameSet()
	if err != nil {
		return result.Error(err)
	}
	evacuateDataNodeNameSet, err := provider.getEvacuateAllDataNodeNameSet()
	if err != nil {
		return result.Error(err)
	}

	allNodes, err := provider.getNodeNameSet()
	if err != nil {
		logger.Error(err, "Failed to get node name set")
		return result.Error(err)
	}

	// Fail EMM operations if insufficient nodes for pods
	//
	// VMWare requires that we perform some kind of check to ensure that there is at least hope that any
	// impacted pods from an EMM operation will get rescheduled successfully to some other node. Here
	// we do a basic check to ensure that there are at least as many nodes available as we have cassandra
	// pods
	unavailableNodes := utils.UnionStringSet(plannedDownNodeNameSet, evacuateDataNodeNameSet)
	availableNodes := utils.SubtractStringSet(allNodes, unavailableNodes)

	if len(provider.getPodNameSet()) > len(availableNodes) {
		anyUpdated := false
		updated := false
		for node := range unavailableNodes {
			if updated, err = provider.failEMM(node, NotEnoughResources); err != nil {
				logger.Error(err, "Failed to add "+EMMFailureAnnotation, "Node", node)
				return result.Error(err)
			}
			anyUpdated = anyUpdated || updated
		}
		if anyUpdated {
			return result.RequeueSoon(10)
		}
	}

	// Fail any evacuate data EMM operations if the datacenter is stopped
	//
	// Cassandra must be up and running to rebuild cassandra nodes. Since
	// evacuating may entail deleting PVCs, we need to fail these operations
	// as we are unlikely to be able to carry them out successfully.
	if provider.IsStopped() {
		didUpdate := false
		for nodeName, _ := range evacuateDataNodeNameSet {
			podsUpdated, err := provider.failEMM(nodeName, GenericFailure)
			if err != nil {
				return result.Error(err)
			}
			logger.Info("Failed EMM operation for evacuate all data as the datacenter is currently stopped", "node", nodeName)
			didUpdate = didUpdate || podsUpdated
		}

		if didUpdate {
			return result.RequeueSoon(2)
		}
	}

	// NOTE: There might be pods that aren't ready for a variety of reasons,
	// however, with respect to data availability, we really only care if a
	// pod representing a cassandra node that is _currently_ bootstrapped to
	// the is down. We do not care if a pod is down for a cassandra node
	// that was never bootstrapped (for example, maybe we are in the middle of
	// scaling up), as such pods are not part of the cluster presently and
	// their being down has no impact on data availability. Also, simply
	// looking at the pod state label is insufficient here. The pod might be
	// brand new, but represents a cassandra node that is already part of the
	// cluster.
	racksWithDownPods := provider.getRacksWithNotReadyPodsBootstrapped()

	// If we have multipe racks with down pods we will need to fail any
	// EMM operation as cluster availability is already compromised.
	if len(racksWithDownPods) > 1 {
		allTaintedNameSet := utils.UnionStringSet(plannedDownNodeNameSet, evacuateDataNodeNameSet)
		didUpdate := false
		for nodeName, _ := range allTaintedNameSet {
			didUpdatePods, err := provider.failEMM(nodeName, TooManyExistingFailures)
			if err != nil {
				return result.Error(err)
			}
			logger.Info("Failed EMM operation as availability is already compromised due to multiple racks having not ready pods", "node", nodeName)
			didUpdate = didUpdate || didUpdatePods
		}
		if didUpdate {
			return result.RequeueSoon(2)
		}
	}

	// Remember the down rack as we'll need to fail any EMM operations outside
	// of this rack.
	downRack := ""
	for _, rackName := range racksWithDownPods {
		downRack = rackName
		break
	}

	// Fail EMM operations for nodes that do not have pods for the down
	// rack
	if downRack != "" {
		nodeNameSetForDownRack := provider.getRackNodeNameSet(downRack)

		nodeNameSetForNoPodsInRack := utils.SubtractStringSet(
			utils.UnionStringSet(
				plannedDownNodeNameSet,
				evacuateDataNodeNameSet),
			nodeNameSetForDownRack)

		if len(nodeNameSetForNoPodsInRack) > 0 {
			didUpdate := false
			for nodeName, _ := range nodeNameSetForNoPodsInRack {
				podsUpdated, err := provider.failEMM(nodeName, TooManyExistingFailures)
				if err != nil {
					return result.Error(err)
				}
				logger.Info("Failed EMM operation as it did not contain any pods for the currently down rack", "node", nodeName)
				didUpdate = didUpdate || podsUpdated
			}
			if didUpdate {
				return result.RequeueSoon(2)
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
	// so we just delete all the not ready pods.
	//
	// Note that, due to earlier checks, we know the only not-ready pods
	// are those that have not bootstrapped (so it doesn't matter if we delete
	// them) or pods that all belong to the same rack. Consequently, we can
	// delete all such pods on the tainted node without impacting
	// availability.
	didUpdate, err = provider.removeAllNotReadyPodsOnEMMNodes()
	if err != nil {
		return result.Error(err)
	}
	if didUpdate {
		return result.RequeueSoon(2)
	}

	// At this point we know there are no not-ready pods on the tainted nodes,
	// and that all pods that are down belong to the same rack as the tainted
	// nodes.

	// Wait for pods not on tainted nodes to become ready
	//
	// If we have pods down (for cassandra nodes that have potentially
	// bootstrapped) that are not on the tainted nodes, these are likely pods
	// we previously deleted due to the taints. We try to move these pods
	// one-at-a-time to spare ourselves unnecessary rebuilds if
	// the EMM operation should fail, so we wait for them to become ready.
	if downRack != "" {
		didUpdate, err = provider.performEvacuateDataPodReplace()
		if err != nil {
			return result.Error(err)
		}
		if didUpdate {
			return result.RequeueSoon(2)
		}

		// Pods are not ready (because downRack isn't the empty string) and
		// there aren't any pods stuck in an unscheduable state with PVCs on
		// on nodes marked for evacuate all data, so continue to allow
		// cassandra a chance to start on the not ready pods. These not ready
		// pods are likely ones we deleted previously when moving them off of
		// the tainted node.
		//
		// TODO: Some of these pods might be from a planned-downtime EMM
		// operation and so will not become ready until their node comes back
		// online. With the way this logic works, if two nodes are marked for
		// planned-downtime, only one node will have its pods deleted, and the
		// other will effectively be ignored until the other node is back
		// online, even if both nodes belong to the same rack. Revisit whether
		// this behaviour is desirable.
		return result.Continue()
	}

	// At this point, we know there are no not-ready pods on the tainted
	// nodes and we know there are no down pods that are bootstrapped, so we
	// can delete any pod we like without impacting availability.

	// Delete a pod for an evacuate data tainted node
	//
	// We give preference to nodes tainted to evacuate all data mainly to
	// ensure some level of determinism. We could give preference to
	// planned downtime taints. In an ideal world, we'd address tainted
	// nodes in chronilogical order of when they received a taint, but the
	// operator doesn't track that information (and I'm not inclined to do
	// more book keeping) and the node doesn't have this information as
	// far as I'm aware.
	didUpdate, err = provider.removeNextPodFromEvacuateDataNode()
	if err != nil {
		return result.Error(err)
	}
	if didUpdate {
		return result.RequeueSoon(2)
	}

	// Delete all pods for a planned down time tainted node
	//
	// For planned-downtime we will not migrate data to new volumes, so we
	// just delete the pods and leave it at that.
	didUpdate, err = provider.removeAllPodsFromOnePlannedDowntimeNode()
	if err != nil {
		return result.Error(err)
	}
	if didUpdate {
		return result.RequeueSoon(2)
	}

	// At this point we know that no nodes are tainted, so we can go ahead and
	// continue
	return result.Continue()
}

// Check for PVCs with health inaccessible. This function embodies the
// business logic around when inaccessible PVCs are replaced.
func checkPVCHealth(provider EMMService) result.ReconcileResult {
	logger := provider.getLogger()

	podNameSetWithInaccessible, err := provider.getPodNameSetWithVolumeHealthInaccessiblePVC("")
	if err != nil {
		return result.Error(err)
	}

	if len(podNameSetWithInaccessible) == 0 {
		// nothing to do
		return result.Continue()
	}

	racksWithDownPods := provider.getRacksWithNotReadyPodsBootstrapped()

	if len(racksWithDownPods) > 1 {
		logger.Info("Found PVCs marked inaccessible but ignoring due to availability compromised by multiple racks having pods not ready", "racks", racksWithDownPods)
		return result.Continue()
	}

	if len(racksWithDownPods) == 1 {
		podNameSetWithInaccessible, err = provider.getPodNameSetWithVolumeHealthInaccessiblePVC(racksWithDownPods[0])
		if err != nil {
			return result.Error(err)
		}

		if len(podNameSetWithInaccessible) == 0 {
			logger.Info("Found PVCs marked inaccessible but ignoring due to a different rack with pods not ready", "rack", racksWithDownPods[0])
			return result.Continue()
		}
	}

	if len(provider.getInProgressNodeReplacements()) > 0 {
		logger.Info("Found PVCs marked inaccessible but ignore due to an ongoing node replace")
		return result.Continue()
	}

	for podName, _ := range podNameSetWithInaccessible {
		err := provider.startNodeReplace(podName)
		if err != nil {
			logger.Error(err, "Failed to start node replacement for pod with inaccessible PVC", "pod", podName)
			return result.Error(err)
		}
		return result.RequeueSoon(2)
	}

	// Should not be possible to get here, but just in case
	return result.Continue()
}

func CheckPVCHealth(spi EMMSPI) result.ReconcileResult {
	service := &EMMServiceImpl{EMMSPI: spi}
	logger := service.getLogger()
	logger.Info("psp::CheckPVCHealth")
	return checkPVCHealth(service)
}

func CheckEMM(spi EMMSPI) result.ReconcileResult {
	service := &EMMServiceImpl{EMMSPI: spi}
	logger := service.getLogger()
	logger.Info("psp::CheckEMM")
	return checkNodeEMM(service)
}
