package psp

import (
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/types"
	// volumeutil "k8s.io/kubernetes/pkg/controller/volume/persistentvolume/util"
	// "k8s.io/apimachinery/pkg/api/errors"
	// "github.com/datastax/cass-operator/operator/internal/result"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

const (
	EMMFailureAnnotation string    = "appplatform.vmware.com/emm-failure"
	VolumeHealthAnnotation string  = "volumehealth.storage.kubernetes.io/health"
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
// joined the cluster at some point.
type EMMSPI interface {
	GetAllNodesInDC() ([]*corev1.Node, error)
	GetDCPods() []*corev1.Pod
	GetNotReadyPodsJoinedInDC() []*corev1.Pod
	GetAllPodsNotReadyInDC() []*corev1.Pod
	GetSelectedNodeNameForPodPVC(pod *corev1.Pod) (string, error)
	GetPVCsForPod(pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error)
	StartNodeReplace(podName string) error
	GetInProgressNodeReplacements() []string
	RemovePod(pod *corev1.Pod) error
	UpdatePod(pod *corev1.Pod) error
	IsStopped() bool
	IsInitialized() bool
	GetLogger() logr.Logger
}

type EMMChecks interface {
	getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (map[string]bool, error)
	getRacksWithNotReadyPodsJoined() []string
	getNodeNameSetForRack(rackName string) map[string]bool
	getInProgressNodeReplacements() []string
	IsStopped() bool
	IsInitialized() bool
}

type EMMOperations interface {
	cleanupEMMAnnotations() (bool, error)
	getNodeNameSetForPlannedDownTime() (map[string]bool, error)
	getNodeNameSetForEvacuateAllData() (map[string]bool, error)
	removeAllNotReadyPodsOnEMMNodes() (bool, error)
	failEMM(nodeName string, failure EMMFailure) (bool, error)
	performPodReplaceForEvacuateData() (bool, error)
	removeNextPodFromEvacuateDataNode() (bool, error)
	removeAllPodsFromPlannedDowntimeNode() (bool, error)
	startNodeReplace(podName string) error
}

type EMMService interface {
	EMMOperations
	EMMChecks
	getLogger() logr.Logger
}


//
// StringSet helper functions
//
func unionStringSet(a, b map[string]bool) map[string]bool {
	result := map[string]bool{}
	for _, m := range []map[string]bool{a, b} {
		for k := range m {
			result[k] = true
		}
	}
	return result
}

func subtractStringSet(a, b map[string]bool) map[string]bool {
	result := map[string]bool{}
	for k, _ := range a {
		if !b[k] {
			result[k] = true
		}
	}
	return result
}


//
// k8s Node helper functions
//
func getNameSetForNodes(nodes []*corev1.Node) map[string]bool {
	result := map[string]bool{}
	for _, node := range nodes {
		result[node.Name] = true
	}
	return result
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


//
// k8s Pod helper functions 
//
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

func getPodNameSet(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Name] = true
	}

	return names
}

func getNodeNameSetForPods(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Spec.NodeName] = true
	}
	return names
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

func filterPodsWithAnnotationKey(pods []*corev1.Pod, key string) []*corev1.Pod {
	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
		annos := pod.ObjectMeta.Annotations
		if annos != nil {
			_, ok := annos[key]
			return ok
		}
		return false
	})
}

func filterPodsWithLabel(pods []*corev1.Pod, label, value string) []*corev1.Pod {
	return filterPodsWithFn(pods, func(pod *corev1.Pod) bool {
		labels := pod.Labels
		if labels != nil {
			labelValue, ok := labels[label]
			return ok && labelValue == value
		}
		return false
	})
}

//
// k8s PVC helpers
//
func filterPVCsWithFn(pvcs []*corev1.PersistentVolumeClaim, fn func(*corev1.PersistentVolumeClaim) bool) []*corev1.PersistentVolumeClaim {
	result := []*corev1.PersistentVolumeClaim{}
	for _, pvc := range pvcs {
		if fn(pvc) {
			result = append(result, pvc)
		}
	}
	return result
}


//
// Util
//
func getRackNameSetForPods(pods []*corev1.Pod) map[string]bool {
	names := map[string]bool{}
	for _, pod := range pods {
		names[pod.Labels[api.RackLabel]] = true
	}
	return names
}

func filterPodsByRackName(pods []*corev1.Pod, rackName string) []*corev1.Pod {
	return filterPodsWithLabel(pods, api.RackLabel, rackName)
}


//
// EMMOperations impl
//
type EMMServiceImpl struct {
	EMMSPI
}

func (impl *EMMServiceImpl) cleanupEMMAnnotations() (bool, error) {
	nodes, err := impl.getNodeNameSetForEvacuateAllData()
	if err != nil {
		return false, err
	}
	nodes2, err := impl.getNodeNameSetForPlannedDownTime()
	if err != nil {
		return false, err
	}
	nodes = unionStringSet(nodes, nodes2)

	// Strip EMM failure annotation from pods where node is no longer tainted
	podsFailedEmm := impl.getPodsWithAnnotationKey(EMMFailureAnnotation)
	nodesWithPodsFailedEMM := getNodeNameSetForPods(podsFailedEmm)
	nodesNoLongerEMM := subtractStringSet(
		nodesWithPodsFailedEMM,
		nodes)
	
	podsNoLongerFailed := filterPodsWithNodeInNameSet(podsFailedEmm, nodesNoLongerEMM)
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

func (impl *EMMServiceImpl) getNodeNameSetForPlannedDownTime() (map[string]bool, error) {
	nodes, err := impl.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "planned-downtime", corev1.TaintEffectNoSchedule)
	if err != nil {
		return nil, err
	}
	return getNameSetForNodes(nodes), nil
}

func (impl *EMMServiceImpl) getNodeNameSetForEvacuateAllData() (map[string]bool, error) {
	nodes, err := impl.getNodesWithTaintKeyValueEffect("node.vmware.com/drain", "drain", corev1.TaintEffectNoSchedule)
	if err != nil {
		return nil, err
	}
	return getNameSetForNodes(nodes), nil
}

func (impl *EMMServiceImpl) removeAllNotReadyPodsOnEMMNodes() (bool, error) {
	podsNotReady := impl.GetAllPodsNotReadyInDC()
	plannedDownNameSet, err := impl.getNodeNameSetForPlannedDownTime()
	if err != nil {
		return false, err
	}

	evacuateDataNameSet, err := impl.getNodeNameSetForEvacuateAllData()
	if err != nil {
		return false, err
	}

	taintedNodesNameSet := unionStringSet(plannedDownNameSet, evacuateDataNameSet)
	podsNotReadyOnTaintedNodes := filterPodsWithNodeInNameSet(podsNotReady, taintedNodesNameSet)
	
	if len(podsNotReadyOnTaintedNodes) > 1 {
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
		err := impl.addPodAnnotation(pod, EMMFailureAnnotation, string(failure))
		if err != nil {
			return false, err
		}
	}
	return didUpdate, nil
}

func (impl *EMMServiceImpl) performPodReplaceForEvacuateData() (bool, error) {
	downPods := impl.GetNotReadyPodsJoinedInDC()
	if len(downPods) > 0 {
		evacuateAllDataNameSet, err := impl.getNodeNameSetForEvacuateAllData()
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
			if isPodUnschedulable(pod) {

				// NOTE: There isn't a great machine readable way to know why
				// the pod is unschedulable. The reasons are, unfortunately,
				// buried within human readable explanation text. As a result,
				// a pod might not get scheduled due to no nodes having 
				// sufficent memory, and then we delete a PVC thinking that
				// the PVC was causing scheduling to fail even though it 
				// wasn't.
				
				pvcNode, err := impl.GetSelectedNodeNameForPodPVC(pod)
				if err != nil {
					return false, err
				}
				if pvcNode != "" && pod.Spec.NodeName != pvcNode{
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
	nodeNameSet, err := impl.getNodeNameSetForEvacuateAllData()
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

func (impl *EMMServiceImpl) removeAllPodsFromPlannedDowntimeNode() (bool, error) {
	nodeNameSet, err := impl.getNodeNameSetForPlannedDownTime()
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
func (impl *EMMServiceImpl) getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (map[string]bool, error) {
	pods := impl.GetDCPods()
	result := []*corev1.Pod{}
	for _, pod := range pods {
		pvcs, err := impl.GetPVCsForPod(pod)
		if err != nil {
			return nil, err
		}
		
		pvcs = filterPVCsWithFn(pvcs, func(pvc *corev1.PersistentVolumeClaim) bool {
			return pvc.Annotations != nil && pvc.Annotations[VolumeHealthAnnotation] == string(VolumeHealthInaccessible)
		})

		if len(pvcs) > 0 {
			result = append(result, pod)
		}
	}

	if rackName != "" {
		return getPodNameSet(filterPodsByRackName(result, rackName)), nil
	} else {
		return getPodNameSet(result), nil
	}
}

func (impl *EMMServiceImpl) getRacksWithNotReadyPodsJoined() []string {
	pods := impl.GetNotReadyPodsJoinedInDC()
	rackNameSet := getRackNameSetForPods(pods)
	rackNames := []string{}
	for rackName, _ := range rackNameSet {
		rackNames = append(rackNames, rackName)
	}
	return rackNames
}

func (impl *EMMServiceImpl) getNodeNameSetForRack(rackName string) map[string]bool {
	podsForDownRack := filterPodsWithLabel(impl.GetDCPods(), api.RackLabel, rackName)
	nodeNameSetForRack := getNodeNameSetForPods(podsForDownRack)
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
	return filterNodesWithTaintKeyValueEffect(nodes, taintKey, value, effect), nil
}

func (impl *EMMServiceImpl) getPodsForNodeName(nodeName string) []*corev1.Pod {
	return filterPodsWithNodeInNameSet(impl.GetDCPods(), map[string]bool{nodeName: true})
}

func (impl *EMMServiceImpl) getPodsWithAnnotationKey(key string) []*corev1.Pod {
	pods := impl.GetDCPods()
	return filterPodsWithAnnotationKey(pods, key)
}

func (impl *EMMServiceImpl) addPodAnnotation(pod *corev1.Pod, key, value string) error {
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = map[string]string{}
	}

	pod.Annotations[key] = value
	return impl.UpdatePod(pod)
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
	// Strip EMM failure annotation from pods where node is no longer tainted
	didUpdate, err := provider.cleanupEMMAnnotations()
	if err != nil {
		return result.Error(err)
	}
	if didUpdate {
		return result.RequeueSoon(2)
	}

	// Do not perform EMM operations while the datacenter is initializing
	if !provider.IsInitialized() {
		return result.Continue()
	}

	// Find tainted nodes
	//
	// We may have some tainted nodes where we had previously failed the EMM
	// operation, so be sure to filter those out.
	plannedDownNodeNameSet, err := provider.getNodeNameSetForPlannedDownTime()
	if err != nil {
		return result.Error(err)
	}
	evacuateDataNodeNameSet, err := provider.getNodeNameSetForEvacuateAllData()
	if err != nil {
		return result.Error(err)
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
			didUpdate = didUpdate || podsUpdated
		}

		if didUpdate {
			return result.RequeueSoon(2)
		}
	}

	// NOTE: There might be pods that aren't ready for a variety of reasons,
	// however, with respect to data availability, we really only care if a
	// pod representing a cassandra node that is _currently_ joined to the 
	// cluster is down. We do not care if a pod is down for a cassandra node
    // that was never joined (for example, maybe we are in the middle of scaling
	// up), as such pods are not part of the cluster presently and their being
	// down has no impact on data availability. Also, simply looking at the pod
	// state label is insufficient here. The pod might be brand new, but 
	// represents a cassandra node that is already part of the cluster.
	racksWithDownPods := provider.getRacksWithNotReadyPodsJoined()

	// If we have multipe racks with down pods we will need to fail any
	// EMM operation as cluster availability is already compromised.
	if len(racksWithDownPods) > 1 {
		allTaintedNameSet := unionStringSet(plannedDownNodeNameSet, evacuateDataNodeNameSet)
		didUpdate := false
		for nodeName, _ := range allTaintedNameSet {
			didUpdatePods, err := provider.failEMM(nodeName, TooManyExistingFailures)
			if err != nil {
				return result.Error(err)
			}
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
		nodeNameSetForDownRack := provider.getNodeNameSetForRack(downRack)

		nodeNameSetForNoPodsInRack := subtractStringSet(
			unionStringSet(
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
	// so we just delete all the errored pods.
	//
	// Note that, due to earlier checks, we know the only not-ready pods
	// are those that have not joined the cluster (so it doesn't matter
	// if we delete them) or pods that all belong to the same rack. 
	// Consequently, we can delete all such pods on the tainted node without
	// impacting availability.
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
	// If we have pods down (for cassandra nodes that have potentially joined
	// the cluster) that are not on the tainted nodes, these are likely pods 
	// we previously deleted due to the taints. We try to move these pods 
	// one-at-a-time to spare ourselves unnecessary rebuilds if
	// the EMM operation should fail, so we wait for them to become ready.
	if downRack != "" {
		didUpdate, err = provider.performPodReplaceForEvacuateData()
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
	didUpdate, err = provider.removeAllPodsFromPlannedDowntimeNode()
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
	
	racksWithDownPods := provider.getRacksWithNotReadyPodsJoined()

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
	return checkPVCHealth(service)
}

func CheckEMM(spi EMMSPI) result.ReconcileResult {
	service := &EMMServiceImpl{EMMSPI: spi}
	return checkNodeEMM(service)
}