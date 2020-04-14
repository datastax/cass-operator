// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"reflect"
	"sort"
	"time"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

var (
	ResultShouldNotRequeue     reconcile.Result = reconcile.Result{Requeue: false}
	ResultShouldRequeueNow     reconcile.Result = reconcile.Result{Requeue: true}
	ResultShouldRequeueSoon    reconcile.Result = reconcile.Result{Requeue: true, RequeueAfter: 2 * time.Second}
	ResultShouldRequeueTenSecs reconcile.Result = reconcile.Result{Requeue: true, RequeueAfter: 10 * time.Second}
)

const (
	stateReadyToStart    = "Ready-to-Start"
	stateStartedNotReady = "Started-not-Ready"
	stateStarted         = "Started"
	stateStarting        = "Starting"
)

// CalculateRackInformation determine how many nodes per rack are needed
func (rc *ReconciliationContext) CalculateRackInformation() error {

	rc.ReqLogger.Info("reconcile_racks::calculateRackInformation")

	// Create RackInformation

	nodeCount := int(rc.Datacenter.Spec.Size)
	racks := rc.Datacenter.Spec.GetRacks()
	rackCount := len(racks)

	// TODO error if nodeCount < rackCount

	if rc.Datacenter.Spec.Stopped {
		nodeCount = 0
	}

	// 3 seeds per datacenter (this could be two, but we would like three seeds per cluster
	// and it's not easy for us to know if we're in a multi DC cluster in this part of the code)
	// OR all of the nodes, if there's less than 3
	// OR one per rack if there are four or more racks
	seedCount := 3
	if nodeCount < 3 {
		seedCount = nodeCount
	} else if rackCount > 3 {
		seedCount = rackCount
	}

	var desiredRackInformation []*RackInformation

	if rackCount < 1 {
		return fmt.Errorf("assertion failed! rackCount should not possibly be zero here")
	}

	// nodes_per_rack = total_size / rack_count + 1 if rack_index < remainder

	nodesPerRack, extraNodes := nodeCount/rackCount, nodeCount%rackCount
	seedsPerRack, extraSeeds := seedCount/rackCount, seedCount%rackCount

	for rackIndex, currentRack := range racks {
		nodesForThisRack := nodesPerRack
		if rackIndex < extraNodes {
			nodesForThisRack++
		}
		seedsForThisRack := seedsPerRack
		if rackIndex < extraSeeds {
			seedsForThisRack++
		}
		nextRack := &RackInformation{}
		nextRack.RackName = currentRack.Name
		nextRack.NodeCount = nodesForThisRack
		nextRack.SeedCount = seedsForThisRack

		desiredRackInformation = append(desiredRackInformation, nextRack)
	}

	statefulSets := make([]*appsv1.StatefulSet, len(desiredRackInformation), len(desiredRackInformation))

	rc.desiredRackInformation = desiredRackInformation
	rc.statefulSets = statefulSets

	return nil
}

func (rc *ReconciliationContext) CheckSuperuserSecretCreation() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckSuperuserSecretCreation")

	_, err := rc.retrieveSuperuserSecretOrCreateDefault()
	if err != nil {
		rc.ReqLogger.Error(err, "error retrieving SuperuserSecret for CassandraDatacenter.")
		return result.Error(err)
	}

	return result.Continue()
}

func (rc *ReconciliationContext) CheckRackCreation() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckRackCreation")
	for idx := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]

		// Does this rack have a statefulset?

		statefulSet, statefulSetFound, err := rc.GetStatefulSetForRack(rackInfo)
		if err != nil {
			rc.ReqLogger.Error(
				err,
				"Could not locate statefulSet for",
				"Rack", rackInfo.RackName)
			return result.Error(err)
		}

		if statefulSetFound == false {
			rc.ReqLogger.Info(
				"Need to create new StatefulSet for",
				"Rack", rackInfo.RackName)
			err := rc.ReconcileNextRack(statefulSet)
			if err != nil {
				rc.ReqLogger.Error(
					err,
					"error creating new StatefulSet",
					"Rack", rackInfo.RackName)
				return result.Error(err)
			}
		}

		rc.statefulSets[idx] = statefulSet
	}

	return result.Continue()
}

func (rc *ReconciliationContext) CheckRackPodTemplate() result.ReconcileResult {
	logger := rc.ReqLogger
	dc := rc.Datacenter
	logger.Info("starting CheckRackPodTemplate()")

	for idx := range rc.desiredRackInformation {
		rackName := rc.desiredRackInformation[idx].RackName
		if dc.Spec.CanaryUpgrade && idx > 0 {
			logger.
				WithValues("rackName", rackName).
				Info("Skipping rack because CanaryUpgrade is turned on")
			return result.Continue()
		}
		statefulSet := rc.statefulSets[idx]

		// have to use zero here, because each statefulset is created with no replicas
		// in GetStatefulSetForRack()
		desiredSts, err := newStatefulSetForCassandraDatacenter(rackName, dc, 0)
		if err != nil {
			logger.Error(err, "error calling newStatefulSetForCassandraDatacenter")
			return result.Error(err)
		}

		needsUpdate := false

		if !resourcesHaveSameHash(statefulSet, desiredSts) {
			logger.
				WithValues("rackName", rackName).
				Info("statefulset needs an update")

			needsUpdate = true

			// "fix" the replica count, and maintain labels and annotations the k8s admin may have set
			desiredSts.Spec.Replicas = statefulSet.Spec.Replicas
			desiredSts.Labels = utils.MergeMap(map[string]string{}, statefulSet.Labels, desiredSts.Labels)
			desiredSts.Annotations = utils.MergeMap(map[string]string{}, statefulSet.Annotations, desiredSts.Annotations)

			desiredSts.DeepCopyInto(statefulSet)
		}

		if needsUpdate {

			rc.Recorder.Eventf(rc.Datacenter, "Normal", "UpdatingRack",
				"Updating rack %s", rackName)

			if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
				return result.Error(err)
			}

			logger.Info("Updating statefulset pod specs",
				"statefulSet", statefulSet,
			)

			err = rc.Client.Update(rc.Ctx, statefulSet)
			if err != nil {
				logger.Error(
					err,
					"Unable to perform update on statefulset for config",
					"statefulSet", statefulSet)
				return result.Error(err)
			}

			// we just updated k8s and pods will be knocked out of ready state, so let k8s
			// call us back when these changes are done and the new pods are back to ready
			// TODO should we requeue for some amount of time in the future instead?
			return result.Done()
		} else {

			// the pod template is right, but if any pods don't match it,
			// or are missing, we should not move onto the next rack,
			// because there's an upgrade in progress

			status := statefulSet.Status
			if status.Replicas != status.ReadyReplicas ||
				status.Replicas != status.CurrentReplicas ||
				status.Replicas != status.UpdatedReplicas {

				logger.Info(
					"waiting for upgrade to finish on statefulset",
					"statefulset", statefulSet.Name,
					"replicas", status.Replicas,
					"readyReplicas", status.ReadyReplicas,
					"currentReplicas", status.CurrentReplicas,
					"updatedReplicas", status.UpdatedReplicas,
				)

				return result.RequeueSoon(10)
			}
		}
	}

	logger.Info("done CheckRackPodTemplate()")
	return result.Continue()
}

func (rc *ReconciliationContext) CheckRackLabels() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckRackLabels")

	for idx, _ := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]
		statefulSet := rc.statefulSets[idx]
		patch := client.MergeFrom(statefulSet.DeepCopy())

		// Has this statefulset been reconciled?

		stsLabels := statefulSet.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(stsLabels, rc.Datacenter, rackInfo.RackName)

		if shouldUpdateLabels {
			rc.ReqLogger.Info("Updating labels",
				"statefulSet", statefulSet,
				"current", stsLabels,
				"desired", updatedLabels)
			statefulSet.SetLabels(updatedLabels)

			if err := rc.Client.Patch(rc.Ctx, statefulSet, patch); err != nil {
				rc.ReqLogger.Info("Unable to update statefulSet with labels",
					"statefulSet", statefulSet)

				// FIXME we had not been passing this error up - why?
				return result.Error(err)
			}

			rc.Recorder.Eventf(rc.Datacenter, "Normal", "UpateRackLabelsOnResource",
				"Update rack labels for StatefulSet %s", statefulSet.Name)
		}
	}

	return result.Continue()
}

func (rc *ReconciliationContext) CheckRackStoppedState() result.ReconcileResult {
	logger := rc.ReqLogger

	emittedStoppingEvent := false
	racksUpdated := false
	for idx := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]
		statefulSet := rc.statefulSets[idx]

		stopped := rc.Datacenter.Spec.Stopped
		currentPodCount := *statefulSet.Spec.Replicas

		if stopped && currentPodCount > 0 {
			logger.Info(
				"CassandraDatacenter is stopped, setting rack to zero replicas",
				"rack", rackInfo.RackName,
				"currentSize", currentPodCount,
			)

			if !emittedStoppingEvent {
				rc.Recorder.Eventf(rc.Datacenter, "Normal", "StoppingDatacenter",
					"Stopping datacenter")
				emittedStoppingEvent = true
			}

			rackPods := FilterPodListByLabels(rc.dcPods, rc.Datacenter.GetRackLabels(rackInfo.RackName))

			nodesDrained := 0
			nodeDrainErrors := 0

			for _, pod := range rackPods {
				if isMgmtApiRunning(pod) {
					nodesDrained++
					err := rc.NodeMgmtClient.CallDrainEndpoint(pod)
					// if we got an error during drain, just log it and count it
					// and then keep going, because we don't want to try restarting
					// the server just to bring it down
					if err != nil {
						logger.Error(err, "error during node drain",
							"pod", pod.Name)
						nodeDrainErrors++
					}
				}
			}

			logger.Info("rack drains done",
				"rack", rackInfo.RackName,
				"nodesDrained", nodesDrained,
				"nodeDrainErrors", nodeDrainErrors,
			)

			err := rc.UpdateRackNodeCount(statefulSet, 0)
			if err != nil {
				return result.Error(err)
			}
			racksUpdated = true
		}
	}

	if racksUpdated {
		return result.Done()
	}
	return result.Continue()
}

// checkSeedLabels loops over all racks and makes sure that the proper pods are labelled as seeds.
func (rc *ReconciliationContext) checkSeedLabels() (int, error) {
	rc.ReqLogger.Info("reconcile_racks::CheckSeedLabels")
	seedCount := 0
	for idx := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]
		n, err := rc.labelSeedPods(rackInfo)
		seedCount += n
		if err != nil {
			return 0, err
		}
	}
	return seedCount, nil
}

// CheckPodsReady loops over all the server pods and starts them
func (rc *ReconciliationContext) CheckPodsReady(endpointData httphelper.CassMetadataEndpoints) result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckPodsReady")

	if rc.Datacenter.Spec.Stopped {
		return result.Continue()
	}

	// all errors in this function we're going to treat as likely ephemeral problems that would resolve
	// so we use ResultShouldRequeueSoon to check again soon

	// successes where we want to end this reconcile loop, we generally also want to wait a bit
	// because stuff is happening concurrently in k8s (getting pods from pending to running)
	// or Cassandra (getting a node bootstrapped and ready), so we use ResultShouldRequeueSoon to try again soon

	// step 0 - see if any nodes lost their readiness
	// or gained it back
	nodeStartedNotReady, err := rc.findStartedNotReadyNodes()
	if err != nil {
		return result.Error(err)
	}
	if nodeStartedNotReady {
		return result.RequeueSoon(2)
	}

	// delete stuck nodes

	deletedNode, err := rc.deleteStuckNodes()
	if err != nil {
		return result.Error(err)
	}
	if deletedNode {
		return result.Done()
	}

	// get the nodes labelled as seeds before we start any nodes

	seedCount, err := rc.checkSeedLabels()
	if err != nil {
		return result.Error(err)
	}
	err = rc.refreshSeeds()
	if err != nil {
		return result.Error(err)
	}

	// step 1 - see if any nodes are already coming up

	nodeIsStarting, err := rc.findStartingNodes()

	if err != nil {
		return result.Error(err)
	}
	if nodeIsStarting {
		return result.RequeueSoon(2)
	}

	// step 2 - get one node up per rack

	rackWaitingForANode, err := rc.startOneNodePerRack(endpointData, seedCount)

	if err != nil {
		return result.Error(err)
	}
	if rackWaitingForANode != "" {
		return result.RequeueSoon(2)
	}

	// step 3 - get all nodes up
	// if the cluster isn't healthy, that's ok, but go back to step 1
	if !rc.isClusterHealthy() {
		rc.ReqLogger.Info(
			"cluster isn't healthy",
		)
		// FIXME this is one spot I've seen get spammy, should we raise this number?
		return result.RequeueSoon(2)
	}

	needsMoreNodes, err := rc.startAllNodes(endpointData)
	if err != nil {
		return result.Error(err)
	}
	if needsMoreNodes {
		return result.RequeueSoon(2)
	}

	// step 5 sanity check that all pods are labelled as started and are ready

	readyPodCount, startedLabelCount := rc.countReadyAndStarted()
	desiredSize := int(rc.Datacenter.Spec.Size)

	if desiredSize == readyPodCount && desiredSize == startedLabelCount {
		return result.Continue()
	} else {
		err := fmt.Errorf("checks failed desired:%d, ready:%d, started:%d", desiredSize, readyPodCount, startedLabelCount)
		return result.Error(err)
	}
}

// CheckRackScale loops over each statefulset and makes sure that it has the right
// amount of desired replicas. At this time we can only increase the amount of replicas.
func (rc *ReconciliationContext) CheckRackScale() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckRackScale")

	for idx := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]
		statefulSet := rc.statefulSets[idx]

		// By the time we get here we know all the racks are ready for that particular size

		desiredNodeCount := int32(rackInfo.NodeCount)
		maxReplicas := *statefulSet.Spec.Replicas

		if maxReplicas < desiredNodeCount {

			// update it
			rc.ReqLogger.Info(
				"Need to update the rack's node count",
				"Rack", rackInfo.RackName,
				"maxReplicas", maxReplicas,
				"desiredSize", desiredNodeCount,
			)

			rc.Recorder.Eventf(rc.Datacenter, "ScaleUpRack",
				"Scaling up rack %s", rackInfo.RackName)

			err := rc.UpdateRackNodeCount(statefulSet, desiredNodeCount)
			if err != nil {
				return result.Error(err)
			}
		}

		currentReplicas := statefulSet.Status.CurrentReplicas
		if currentReplicas > desiredNodeCount {
			// too many ready replicas, how did this happen?
			rc.ReqLogger.Info(
				"Too many replicas for StatefulSet",
				"desiredCount", desiredNodeCount,
				"currentCount", currentReplicas)
			err := fmt.Errorf("too many replicas")
			return result.Error(err)
		}
	}

	return result.Continue()
}

// CheckRackPodLabels checks each pod and its volume(s) and makes sure they have the
// proper labels
func (rc *ReconciliationContext) CheckRackPodLabels() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_racks::CheckRackPodLabels")

	for idx, _ := range rc.desiredRackInformation {
		statefulSet := rc.statefulSets[idx]

		if err := rc.ReconcilePods(statefulSet); err != nil {
			return result.Error(err)
		}
	}

	return result.Continue()
}

func shouldUpsertSuperUser(dc api.CassandraDatacenter) bool {
	lastCreated := dc.Status.SuperUserUpserted
	return time.Now().After(lastCreated.Add(time.Minute * 4))
}

func (rc *ReconciliationContext) CreateSuperuser() result.ReconcileResult {
	if rc.Datacenter.Spec.Stopped {
		rc.ReqLogger.Info("cluster is stopped, skipping CreateSuperuser")
		return result.Continue()
	}

	//Skip upsert if already did so recently
	if !shouldUpsertSuperUser(*rc.Datacenter) {
		rc.ReqLogger.Info(fmt.Sprintf("The CQL superuser was last upserted at %v, skipping upsert", rc.Datacenter.Status.SuperUserUpserted))
		return result.Continue()
	}

	rc.ReqLogger.Info("reconcile_racks::CreateSuperuser")

	rc.Recorder.Eventf(rc.Datacenter, "Normal", "CreateSuperuser",
		"Creating superuser")

	// Get the secret
	secret, err := rc.retrieveSuperuserSecret()
	if err != nil {
		rc.ReqLogger.Error(err, "error retrieving SuperuserSecret for CassandraDatacenter.")
		return result.Error(err)
	}

	// We will call mgmt API on the first pod

	pod := rc.dcPods[0]

	err = rc.NodeMgmtClient.CallCreateRoleEndpoint(
		pod,
		string(secret.Data["username"]),
		string(secret.Data["password"]))
	if err != nil {
		rc.ReqLogger.Error(err, "error creating superuser")
		return result.Error(err)
	}

	patch := client.MergeFrom(rc.Datacenter.DeepCopy())
	rc.Datacenter.Status.SuperUserUpserted = metav1.Now()
	if err = rc.Client.Status().Patch(rc.Ctx, rc.Datacenter, patch); err != nil {
		rc.ReqLogger.Error(err, "error updating the CQL superuser upsert timestamp")
		return result.Error(err)
	}

	return result.Continue()
}

func findHostIdForIpFromEndpointsData(endpointsData []httphelper.EndpointState, ip string) string {
	for _, data := range endpointsData {
		if data.GetRpcAddress() == ip {
			return data.HostID
		}
	}
	return ""
}

func findValueIndexFromStringArray(a []string, v string) int {
	foundIdx := -1
	for idx, item := range a {
		if item == v {
			foundIdx = idx
			break
		}
	}

	return foundIdx
}

func removeValueFromStringArray(a []string, v string) []string {
	foundIdx := findValueIndexFromStringArray(a, v)

	if foundIdx > -1 {
		copy(a[foundIdx:], a[foundIdx+1:])
		a[len(a)-1] = ""
		a = a[:len(a)-1]
	}
	return a
}

func appendValuesToStringArrayIfNotPresent(a []string, values ...string) []string {
	for _, v := range values {
		idx := findValueIndexFromStringArray(a, v)
		if idx < 0 {
			// array does not contain this value, so add it
			a = append(a, v)
		}
	}
	return a
}

func (rc *ReconciliationContext) UpdateCassandraNodeStatus() error {
	dc := rc.Datacenter
	status := &dc.Status
	logger := rc.ReqLogger

	pods := ListAllStartedPods(rc.dcPods)

	if status.NodeStatuses == nil {
		status.NodeStatuses = map[string]api.CassandraNodeStatus{}
	}

	for _, pod := range pods {
		nodeStatus, ok := status.NodeStatuses[pod.Name]
		if !ok {
			nodeStatus = api.CassandraNodeStatus{}
		}

		if pod.Status.PodIP != "" && isMgmtApiRunning(pod) {
			// Getting the HostID requires a call to the node management API which is
			// moderately expensive, so if we already have a HostID, don't bother. This
			// would only change if something has gone horribly horribly wrong.
			if nodeStatus.HostID == "" {
				endpointsResponse, err := rc.NodeMgmtClient.CallMetadataEndpointsEndpoint(pod)
				if err == nil {
					nodeStatus.HostID = findHostIdForIpFromEndpointsData(
						endpointsResponse.Entity, pod.Status.PodIP)
					if nodeStatus.HostID == "" {
						logger.Info("Failed to find host ID", pod, pod.Name)
					}
				} else {
					rc.ReqLogger.Error(err, "Could not get enpoints data")
				}
			}

			if nodeStatus.HostID != "" {
				nodeStatus.NodeIP = pod.Status.PodIP
			}
		}

		status.NodeStatuses[pod.Name] = nodeStatus
	}

	return nil
}

func (rc *ReconciliationContext) updateCurrentReplacePodsProgress() error {
	dc := rc.Datacenter
	logger := rc.ReqLogger
	startedPods := ListAllStartedPods(rc.dcPods)

	// Update current progress of replacing pods
	if len(dc.Status.NodeReplacements) > 0 {
		for _, pod := range startedPods {
			// Since pod is labeled as started, it should be done being replaced
			if findValueIndexFromStringArray(dc.Status.NodeReplacements, pod.Name) > -1 {
				logger.Info("Finished replacing pod", "pod", pod.Name)

				rc.Recorder.Eventf(rc.Datacenter, "Normal", "FinishReplaceNode",
					"Finished replacing pod %s", pod.Name)

				dc.Status.NodeReplacements = removeValueFromStringArray(dc.Status.NodeReplacements, pod.Name)
			}
		}
	}

	return nil
}

func (rc *ReconciliationContext) startReplacePodsIfReplacePodsSpecified() error {
	dc := rc.Datacenter
	if len(dc.Spec.ReplaceNodes) > 0 {
		rc.ReqLogger.Info("Replacing pods", "pods", dc.Spec.ReplaceNodes)

		podNamesString := strings.Join(dc.Spec.ReplaceNodes, ", ")
		rc.Recorder.Eventf(rc.Datacenter, "Normal", "StartReplaceNode",
			"Replacing Cassandra nodes for pods %s", podNamesString)

		dc.Status.NodeReplacements = appendValuesToStringArrayIfNotPresent(
			dc.Status.NodeReplacements,
			dc.Spec.ReplaceNodes...)

		// Now that we've recorded these nodes in the status, we can blank
		// out this field on the spec
		dc.Spec.ReplaceNodes = []string{}
	}

	return nil
}

func (rc *ReconciliationContext) UpdateStatusForUserActions() error {
	var err error

	err = rc.updateCurrentReplacePodsProgress()
	if err != nil {
		return err
	}

	err = rc.startReplacePodsIfReplacePodsSpecified()
	if err != nil {
		return err
	}

	return nil
}

func (rc *ReconciliationContext) UpdateStatus() result.ReconcileResult {
	dc := rc.Datacenter
	oldDc := rc.Datacenter.DeepCopy()

	err := rc.UpdateCassandraNodeStatus()
	if err != nil {
		return result.Error(err)
	}

	err = rc.UpdateStatusForUserActions()
	if err != nil {
		return result.Error(err)
	}

	// Update the status if it changed
	patch := client.MergeFrom(oldDc)
	if !reflect.DeepEqual(dc, oldDc) {
		// If we update the status to account for some user action, for example a
		// pod replace, then we may have also updated the datacenter spec, so patch
		// it as well.
		if err := rc.Client.Patch(rc.Ctx, dc, patch); err != nil {
			return result.Error(err)
		}

		if err := rc.Client.Status().Patch(rc.Ctx, dc, patch); err != nil {
			return result.Error(err)
		}
	}

	return result.Continue()
}

func hasBeenXMinutes(x int, sinceTime time.Time) bool {
	xMinutesAgo := time.Now().Add(time.Minute * time.Duration(-x))
	return sinceTime.Before(xMinutesAgo)
}

func hasBeenXMinutesSinceReady(x int, pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == "Ready" && c.Status == "False" {
			return hasBeenXMinutes(x, c.LastTransitionTime.Time)
		}
	}
	return false
}

func hasBeenXMinutesSinceStarted(x int, pod *corev1.Pod) bool {
	if status := getCassContainerStatus(pod); status != nil {
		running := status.State.Running
		if running != nil {
			return hasBeenXMinutes(x, running.StartedAt.Time)
		}
	}
	return false
}

func hasBeenXMinutesSinceTerminated(x int, pod *corev1.Pod) bool {
	if status := getCassContainerStatus(pod); status != nil {
		lastState := status.LastTerminationState
		if lastState.Terminated != nil {
			return hasBeenXMinutes(x, lastState.Terminated.FinishedAt.Time)
		}
	}
	return false
}

func getCassContainerStatus(pod *corev1.Pod) *corev1.ContainerStatus {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != "cassandra" {
			continue
		}
		return &status
	}
	return nil
}

func isNodeStuckAfterTerminating(pod *corev1.Pod) bool {
	if isServerReady(pod) || isServerReadyToStart(pod) {
		return false
	}

	return hasBeenXMinutesSinceTerminated(10, pod)
}

func isNodeStuckAfterLosingReadiness(pod *corev1.Pod) bool {
	if !isServerStartedNotReady(pod) || isServerReadyToStart(pod) {
		return false
	}
	return hasBeenXMinutesSinceReady(10, pod)
}

func (rc *ReconciliationContext) getCassMetadataEndpoints() httphelper.CassMetadataEndpoints {
	var metadata httphelper.CassMetadataEndpoints
	for _, pod := range rc.clusterPods {
		// Try to query the first ready pod we find.
		// We won't get any endpoints back if no pods are ready yet.
		if !isServerReady(pod) {
			continue
		}

		metadata, _ = rc.NodeMgmtClient.CallMetadataEndpointsEndpoint(pod)

		if len(metadata.Entity) == 0 {
			continue
		}
		break
	}

	return metadata
}

func (rc *ReconciliationContext) deleteStuckNodes() (bool, error) {
	rc.ReqLogger.Info("reconcile_racks::deleteStuckNodes")
	for _, pod := range rc.dcPods {
		shouldDelete := false
		reason := ""
		if isNodeStuckAfterTerminating(pod) {
			reason = "Pod got stuck after Cassandra container terminated"
			shouldDelete = true
		} else if isNodeStuckAfterLosingReadiness(pod) {
			reason = "Pod got stuck after losing readiness"
			shouldDelete = true
		}

		if shouldDelete {
			rc.ReqLogger.Info(fmt.Sprintf("Deleting stuck pod: %s. Reason: %s", pod.Name, reason))
			rc.Recorder.Eventf(rc.Datacenter, "Normal", "DeletingStuckPod",
				reason)
			return true, rc.Client.Delete(rc.Ctx, pod)
		}
	}

	return false, nil
}

func (rc *ReconciliationContext) isClusterHealthy() bool {
	pods := FilterPodListByCassNodeState(rc.clusterPods, stateStarted)

	numRacks := len(rc.Datacenter.Spec.GetRacks())
	for _, pod := range pods {
		err := rc.NodeMgmtClient.CallProbeClusterEndpoint(pod, "LOCAL_QUORUM", numRacks)
		if err != nil {
			return false
		}
	}

	return true
}

// labelSeedPods iterates over all pods for a statefulset and makes sure the right number of
// ready pods are labelled as seeds, so that they are picked up by the headless seed service
// Returns the number of ready seeds.
func (rc *ReconciliationContext) labelSeedPods(rackInfo *RackInformation) (int, error) {
	logger := rc.ReqLogger.WithName("labelSeedPods")

	rackLabels := rc.Datacenter.GetRackLabels(rackInfo.RackName)
	rackPods := FilterPodListByLabels(rc.dcPods, rackLabels)
	sort.SliceStable(rackPods, func(i, j int) bool {
		return rackPods[i].Name < rackPods[j].Name
	})
	count := 0
	for _, pod := range rackPods {
		patch := client.MergeFrom(pod.DeepCopy())

		newLabels := make(map[string]string)
		utils.MergeMap(newLabels, pod.GetLabels())

		ready := isServerReady(pod)
		starting := isServerStarting(pod)

		isSeed := ready && count < rackInfo.SeedCount
		currentVal := pod.GetLabels()[api.SeedNodeLabel]
		if isSeed {
			count++
		}

		// this is the main place we label pods as seeds / not-seeds
		// the one exception to this is the very first node we bring up
		// in an empty cluster, and we set that node as a seed
		// in startOneNodePerRack()

		shouldUpdate := false
		if isSeed && currentVal != "true" {
			rc.Recorder.Eventf(rc.Datacenter, "Normal", "SetPodAsSeed",
				"Making pod %s a seed node", pod.Name)

			newLabels[api.SeedNodeLabel] = "true"
			shouldUpdate = true
		}
		// if this pod is starting, we should leave the seed label alone
		if !isSeed && currentVal == "true" && !starting {
			rc.Recorder.Eventf(rc.Datacenter, "Normal", "UnsetPodAsSeed",
				"Making pod %s a non-seed node", pod.Name)

			delete(newLabels, api.SeedNodeLabel)
			shouldUpdate = true
		}

		if shouldUpdate {
			pod.SetLabels(newLabels)
			if err := rc.Client.Patch(rc.Ctx, pod, patch); err != nil {
				logger.Error(
					err, "Unable to update pod with seed label",
					"pod", pod.Name)
				return 0, err
			}
		}
	}
	return count, nil
}

// GetStatefulSetForRack returns the statefulset for the rack
// and whether it currently exists and whether an error occured
func (rc *ReconciliationContext) GetStatefulSetForRack(
	nextRack *RackInformation) (*appsv1.StatefulSet, bool, error) {

	rc.ReqLogger.Info("reconcile_racks::getStatefulSetForRack")

	// Check if the desiredStatefulSet already exists
	currentStatefulSet := &appsv1.StatefulSet{}
	err := rc.Client.Get(
		rc.Ctx,
		newNamespacedNameForStatefulSet(rc.Datacenter, nextRack.RackName),
		currentStatefulSet)

	if err == nil {
		return currentStatefulSet, true, nil
	}

	if !errors.IsNotFound(err) {
		return nil, false, err
	}

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		nextRack.RackName,
		rc.Datacenter,
		0)
	if err != nil {
		return nil, false, err
	}

	// Set the CassandraDatacenter as the owner and controller
	err = setControllerReference(
		rc.Datacenter,
		desiredStatefulSet,
		rc.Scheme)
	if err != nil {
		return nil, false, err
	}

	return desiredStatefulSet, false, nil
}

// ReconcileNextRack ensures that the resources for a rack have been properly created
func (rc *ReconciliationContext) ReconcileNextRack(statefulSet *appsv1.StatefulSet) error {

	rc.ReqLogger.Info("reconcile_racks::reconcileNextRack")

	if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
		return err
	}

	// Create the StatefulSet

	rc.ReqLogger.Info(
		"Creating a new StatefulSet.",
		"statefulSetNamespace", statefulSet.Namespace,
		"statefulSetName", statefulSet.Name)
	if err := rc.Client.Create(rc.Ctx, statefulSet); err != nil {
		return err
	}

	rc.Recorder.Eventf(rc.Datacenter, "Normal", "CreatedResource",
		"Created statefulset %s", statefulSet.Name)

	return nil
}

func (rc *ReconciliationContext) CheckDcPodDisruptionBudget() result.ReconcileResult {
	// Create a PodDisruptionBudget for the CassandraDatacenter
	dc := rc.Datacenter
	ctx := rc.Ctx
	desiredBudget := newPodDisruptionBudgetForDatacenter(dc)

	// Set CassandraDatacenter as the owner and controller
	if err := setControllerReference(dc, desiredBudget, rc.Scheme); err != nil {
		return result.Error(err)
	}

	// Check if the budget already exists
	currentBudget := &policyv1beta1.PodDisruptionBudget{}
	err := rc.Client.Get(
		ctx,
		types.NamespacedName{
			Name:      desiredBudget.Name,
			Namespace: desiredBudget.Namespace},
		currentBudget)

	if err != nil && !errors.IsNotFound(err) {
		return result.Error(err)
	}

	found := err == nil

	if found && resourcesHaveSameHash(currentBudget, desiredBudget) {
		return result.Continue()
	}

	// it's not possible to update a PodDisruptionBudget, so we need to delete this one and remake it
	if found {
		rc.ReqLogger.Info(
			"Deleting and re-creating a PodDisruptionBudget",
			"pdbNamespace", desiredBudget.Namespace,
			"pdbName", desiredBudget.Name,
			"oldMinAvailable", currentBudget.Spec.MinAvailable,
			"desiredMinAvailable", desiredBudget.Spec.MinAvailable,
		)
		err = rc.Client.Delete(ctx, currentBudget)
		if err != nil {
			return result.Error(err)
		}
	}

	// Create the Budget
	rc.ReqLogger.Info(
		"Creating a new PodDisruptionBudget.",
		"pdbNamespace", desiredBudget.Namespace,
		"pdbName", desiredBudget.Name)

	err = rc.Client.Create(ctx, desiredBudget)
	if err != nil {
		return result.Error(err)
	}

	rc.Recorder.Eventf(rc.Datacenter, "Normal", "CreatedResource",
		"Created PodDisruptionBudget %s", desiredBudget.Name)

	return result.Continue()
}

// UpdateRackNodeCount ...
func (rc *ReconciliationContext) UpdateRackNodeCount(statefulSet *appsv1.StatefulSet, newNodeCount int32) error {

	rc.ReqLogger.Info("reconcile_racks::updateRack")

	rc.ReqLogger.Info(
		"updating StatefulSet node count",
		"statefulSetNamespace", statefulSet.Namespace,
		"statefulSetName", statefulSet.Name,
		"newNodeCount", newNodeCount,
	)

	if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
		return err
	}

	patch := client.MergeFrom(statefulSet.DeepCopy())
	statefulSet.Spec.Replicas = &newNodeCount

	err := rc.Client.Patch(rc.Ctx, statefulSet, patch)

	return err
}

// ReconcilePods ...
func (rc *ReconciliationContext) ReconcilePods(statefulSet *appsv1.StatefulSet) error {
	rc.ReqLogger.Info("reconcile_racks::ReconcilePods")

	for i := int32(0); i < statefulSet.Status.Replicas; i++ {
		podName := fmt.Sprintf("%s-%v", statefulSet.Name, i)

		pod := &corev1.Pod{}
		err := rc.Client.Get(
			rc.Ctx,
			types.NamespacedName{
				Name:      podName,
				Namespace: statefulSet.Namespace},
			pod)
		if err != nil {
			rc.ReqLogger.Error(
				err,
				"Unable to get pod",
				"Pod", podName,
			)
			return err
		}

		podPatch := client.MergeFrom(pod.DeepCopy())

		podLabels := pod.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(podLabels,
			rc.Datacenter, statefulSet.GetLabels()[api.RackLabel])
		if shouldUpdateLabels {
			rc.ReqLogger.Info(
				"Updating labels",
				"Pod", podName,
				"current", podLabels,
				"desired", updatedLabels)

			pod.SetLabels(updatedLabels)

			if err := rc.Client.Patch(rc.Ctx, pod, podPatch); err != nil {
				rc.ReqLogger.Error(
					err,
					"Unable to update pod with label",
					"Pod", podName,
				)
			}

			rc.Recorder.Eventf(rc.Datacenter, "Normal", "UpateRackLabelsOnResource",
				"Update rack labels for Pod %s", podName)
		}

		if pod.Spec.Volumes == nil || len(pod.Spec.Volumes) == 0 || pod.Spec.Volumes[0].PersistentVolumeClaim == nil {
			continue
		}

		pvcName := pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
		pvc := &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: statefulSet.Namespace,
			},
		}
		err = rc.Client.Get(
			rc.Ctx,
			types.NamespacedName{
				Name:      pvcName,
				Namespace: statefulSet.Namespace},
			pvc)
		if err != nil {
			rc.ReqLogger.Error(
				err,
				"Unable to get pvc",
				"PVC", pvcName,
			)
			return err
		}

		pvcPatch := client.MergeFrom(pvc.DeepCopy())

		pvcLabels := pvc.GetLabels()
		shouldUpdateLabels, updatedLabels = shouldUpdateLabelsForRackResource(pvcLabels,
			rc.Datacenter, statefulSet.GetLabels()[api.RackLabel])
		if shouldUpdateLabels {
			rc.ReqLogger.Info("Updating labels",
				"PVC", pvc,
				"current", pvcLabels,
				"desired", updatedLabels)

			pvc.SetLabels(updatedLabels)

			if err := rc.Client.Patch(rc.Ctx, pvc, pvcPatch); err != nil {
				rc.ReqLogger.Error(
					err,
					"Unable to update pvc with labels",
					"PVC", pvc,
				)
			}

			rc.Recorder.Eventf(rc.Datacenter, "Normal", "UpateRackLabelsOnResource",
				"Update rack labels for PersistentVolumeClaim %s", pvc.Name)
		}
	}

	return nil
}

func mergeInLabelsIfDifferent(existingLabels, newLabels map[string]string) (bool, map[string]string) {
	updatedLabels := utils.MergeMap(map[string]string{}, existingLabels, newLabels)
	if reflect.DeepEqual(existingLabels, updatedLabels) {
		return false, existingLabels
	} else {
		return true, updatedLabels
	}
}

// shouldUpdateLabelsForClusterResource will compare the labels passed in with what the labels should be for a cluster level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForClusterResource(resourceLabels map[string]string, dc *api.CassandraDatacenter) (bool, map[string]string) {
	desired := dc.GetClusterLabels()
	oplabels.AddManagedByLabel(desired)
	return mergeInLabelsIfDifferent(resourceLabels, desired)
}

// shouldUpdateLabelsForRackResource will compare the labels passed in with what the labels should be for a rack level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForRackResource(resourceLabels map[string]string, dc *api.CassandraDatacenter, rackName string) (bool, map[string]string) {
	desired := dc.GetRackLabels(rackName)
	oplabels.AddManagedByLabel(desired)
	return mergeInLabelsIfDifferent(resourceLabels, desired)
}

// shouldUpdateLabelsForDatacenterResource will compare the labels passed in with what the labels should be for a datacenter level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForDatacenterResource(resourceLabels map[string]string, dc *api.CassandraDatacenter) (bool, map[string]string) {
	desired := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(desired)
	return mergeInLabelsIfDifferent(resourceLabels, desired)
}

func (rc *ReconciliationContext) labelServerPodStarting(pod *corev1.Pod) error {
	ctx := rc.Ctx
	dc := rc.Datacenter
	podPatch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[api.CassNodeState] = stateStarting
	err := rc.Client.Patch(ctx, pod, podPatch)
	if err != nil {
		return err
	}
	
	statusPatch := client.MergeFrom(dc.DeepCopy())
	dc.Status.LastServerNodeStarted = metav1.Now()
	err = rc.Client.Status().Patch(ctx, dc, statusPatch)
	return err
}

func (rc *ReconciliationContext) labelServerPodStarted(pod *corev1.Pod) error {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[api.CassNodeState] = stateStarted
	err := rc.Client.Patch(rc.Ctx, pod, patch)
	return err
}

func (rc *ReconciliationContext) labelServerPodStartedNotReady(pod *corev1.Pod) error {
	patch := client.MergeFrom(pod.DeepCopy())
	pod.Labels[api.CassNodeState] = stateStartedNotReady
	err := rc.Client.Patch(rc.Ctx, pod, patch)
	return err
}

func (rc *ReconciliationContext) callNodeManagementStart(pod *corev1.Pod) error {
	mgmtClient := rc.NodeMgmtClient
	err := mgmtClient.CallLifecycleStartEndpoint(pod)

	return err
}

func (rc *ReconciliationContext) findStartingNodes() (bool, error) {
	rc.ReqLogger.Info("reconcile_racks::findStartingNodes")

	for _, pod := range rc.clusterPods {
		if pod.Labels[api.CassNodeState] == stateStarting {
			if isServerReady(pod) {
				rc.Recorder.Eventf(rc.Datacenter, "Normal", "StartedCassandra",
					"Started Cassandra for pod %s", pod.Name)
				if err := rc.labelServerPodStarted(pod); err != nil {
					return false, err
				}
			} else {
				// TODO Calling start again on the pod seemed like a good defensive practice
				// TODO but was making problems w/ overloading management API
				// TODO Use a label to hold state and request starting no more than once per minute?

				// if err := rc.callNodeManagementStart(pod); err != nil {
				// 	return false, err
				// }
				return true, nil
			}
		}
	}
	return false, nil
}

func (rc *ReconciliationContext) findStartedNotReadyNodes() (bool, error) {
	rc.ReqLogger.Info("reconcile_racks::findStartedNotReadyNodes")

	for _, pod := range rc.dcPods {
		if didServerLoseReadiness(pod) {
			if err := rc.labelServerPodStartedNotReady(pod); err != nil {
				return false, err
			}
			return true, nil
		}

		if isServerStartedNotReady(pod) {
			if isServerReady(pod) {
				if err := rc.labelServerPodStarted(pod); err != nil {
					return false, err
				}
				return false, nil
			}
		}
	}
	return false, nil
}

func (rc *ReconciliationContext) startCassandra(endpointData httphelper.CassMetadataEndpoints, pod *corev1.Pod) error {
	dc := rc.Datacenter
	mgmtClient := rc.NodeMgmtClient

	// Are we replacing this node?
	shouldReplacePod := findValueIndexFromStringArray(dc.Status.NodeReplacements, pod.Name) > -1

	replaceAddress := ""

	if shouldReplacePod {
		// Get the HostID for pod if it has one
		nodeStatus, ok := dc.Status.NodeStatuses[pod.Name]
		hostId := ""
		if ok {
			hostId = nodeStatus.HostID
		}

		// Get the replace address
		var err error
		if hostId != "" {
			replaceAddress, err = FindIpForHostId(endpointData, hostId)
			if err != nil {
				return fmt.Errorf("Failed to start replace of cassandra node %s for pod %s due to error: %w", hostId, pod.Name, err)
			}
		}
	}

	var err error
	if shouldReplacePod && replaceAddress != "" {
		// If we have a replace address that means the cassandra node did
		// join the ring previously and is marked for replacement, so we
		// start it accordingly
		rc.Recorder.Eventf(rc.Datacenter, "Normal", "StartCassandraAndReplaceNode",
			"Starting Cassandra for pod %s to replace Cassandra node with address %s", pod.Name, replaceAddress)
		err = mgmtClient.CallLifecycleStartEndpointWithReplaceIp(pod, replaceAddress)
	} else {
		// Either we are not replacing this pod or the relevant cassandra node
		// never joined the ring in the first place and can be started normally
		rc.Recorder.Eventf(rc.Datacenter, "Normal", "StartCassandra",
			"Starting Cassandra for pod %s", pod.Name)
		err = mgmtClient.CallLifecycleStartEndpoint(pod)
	}

	if err != nil {
		return err
	}

	return rc.labelServerPodStarting(pod)
}

// returns the name of one rack without any ready node
func (rc *ReconciliationContext) startOneNodePerRack(endpointData httphelper.CassMetadataEndpoints, readySeeds int) (string, error) {

	rc.ReqLogger.Info("reconcile_racks::startOneNodePerRack")

	rackReadyCount := map[string]int{}
	for _, rackInfo := range rc.desiredRackInformation {
		rackReadyCount[rackInfo.RackName] = 0
	}

	for _, pod := range rc.dcPods {
		rackName := pod.Labels[api.RackLabel]
		if isServerReady(pod) {
			rackReadyCount[rackName]++
		}
	}

	// if the DC has no ready seeds, label a pod as a seed before we start Cassandra on it
	labelSeedBeforeStart := readySeeds == 0

	rackThatNeedsNode := ""
	for rackName, readyCount := range rackReadyCount {
		if readyCount > 0 {
			continue
		}
		rackThatNeedsNode = rackName
		for _, pod := range rc.dcPods {
			mgmtApiUp := isMgmtApiRunning(pod)
			if !isServerReadyToStart(pod) || !mgmtApiUp {
				continue
			}
			podRack := pod.Labels[api.RackLabel]
			if podRack == rackName {
				// this is the one exception to all seed labelling happening in labelSeedPods()
				if labelSeedBeforeStart {
					patch := client.MergeFrom(pod.DeepCopy())
					pod.Labels[api.SeedNodeLabel] = "true"
					if err := rc.Client.Patch(rc.Ctx, pod, patch); err != nil {
						return "", err
					}

					rc.Recorder.Eventf(rc.Datacenter, "Normal", "SetPodAsSeed",
						"Making pod %s a seed node", pod.Name)

					// sleeping five seconds for DNS paranoia
					time.Sleep(5 * time.Second)
				}
				if err := rc.startCassandra(endpointData, pod); err != nil {
					return "", err
				}
				return rackName, nil
			}
		}
	}

	return rackThatNeedsNode, nil
}

// returns whether one or more server nodes is not running or ready
func (rc *ReconciliationContext) startAllNodes(endpointData httphelper.CassMetadataEndpoints) (bool, error) {
	rc.ReqLogger.Info("reconcile_racks::startAllNodes")

	for _, pod := range rc.dcPods {
		if isMgmtApiRunning(pod) && !isServerReady(pod) && !isServerStarted(pod) {
			if err := rc.startCassandra(endpointData, pod); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// this extra pass only does anything when we have a combination of
	// ready server pods and pods that are not running - possibly stuck pending
	for _, pod := range rc.dcPods {
		if !isMgmtApiRunning(pod) {
			rc.ReqLogger.Info(
				"management api is not running on pod",
				"pod", pod.Name,
			)
			return true, nil
		}
	}

	return false, nil
}

func (rc *ReconciliationContext) countReadyAndStarted() (int, int) {
	ready := 0
	started := 0
	for _, pod := range rc.dcPods {
		if isServerReady(pod) {
			ready++
			rc.ReqLogger.Info(
				"found a ready pod",
				"podName", pod.Name,
				"runningCountReady", ready,
			)
		}

		if isServerStarted(pod) {
			started++
			rc.ReqLogger.Info(
				"found a pod we labeled Started",
				"podName", pod.Name,
				"runningCountStarted", started,
			)
		}
	}
	return ready, started
}

func isMgmtApiRunning(pod *corev1.Pod) bool {
	podStatus := pod.Status
	statuses := podStatus.ContainerStatuses
	for _, status := range statuses {
		if status.Name != "cassandra" {
			continue
		}
		state := status.State
		runInfo := state.Running
		if runInfo != nil {
			// give management API ten seconds to come up
			tenSecondsAgo := time.Now().Add(time.Second * -10)
			return runInfo.StartedAt.Time.Before(tenSecondsAgo)
		}
	}
	return false
}

func isServerStarting(pod *corev1.Pod) bool {
	return pod.Labels[api.CassNodeState] == stateStarting
}

func isServerStarted(pod *corev1.Pod) bool {
	return pod.Labels[api.CassNodeState] == stateStarted ||
		pod.Labels[api.CassNodeState] == stateStartedNotReady
}

func isServerStartedNotReady(pod *corev1.Pod) bool {
	return pod.Labels[api.CassNodeState] == stateStartedNotReady
}

func isServerReadyToStart(pod *corev1.Pod) bool {
	return pod.Labels[api.CassNodeState] == stateReadyToStart
}

func didServerLoseReadiness(pod *corev1.Pod) bool {
	if pod.Labels[api.CassNodeState] == stateStarted {
		return !isServerReady(pod)
	}
	return false
}

func isServerReady(pod *corev1.Pod) bool {
	status := pod.Status
	statuses := status.ContainerStatuses
	for _, status := range statuses {
		if status.Name != "cassandra" {
			continue
		}
		return status.Ready
	}
	return false
}

func (rc *ReconciliationContext) refreshSeeds() error {
	rc.ReqLogger.Info("reconcile_racks::refreshSeeds")
	if rc.Datacenter.Spec.Stopped {
		rc.ReqLogger.Info("cluster is stopped, skipping refreshSeeds")
		return nil
	}

	startedPods := FilterPodListByCassNodeState(rc.clusterPods, stateStarted)

	for _, pod := range startedPods {
		if err := rc.NodeMgmtClient.CallReloadSeedsEndpoint(pod); err != nil {
			return err
		}
	}

	return nil
}

func (rc *ReconciliationContext) listPods(selector map[string]string) (*corev1.PodList, error) {
	rc.ReqLogger.Info("reconcile_racks::listPods")

	listOptions := &client.ListOptions{
		Namespace:     rc.Datacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	podList := &corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
	}

	return podList, rc.Client.List(rc.Ctx, podList, listOptions)
}

func (rc *ReconciliationContext) CheckRollingRestart() result.ReconcileResult {
	dc := rc.Datacenter
	logger := rc.ReqLogger

	if dc.Spec.RollingRestartRequested {
		dcPatch := client.MergeFrom(dc.DeepCopy())
		dc.Status.LastRollingRestart = metav1.Now()
		err := rc.Client.Status().Patch(rc.Ctx, dc, dcPatch)
		if err != nil {
			logger.Error(err, "error patching datacenter status for rolling restart")
			return result.Error(err)
		}

		dcPatch = client.MergeFrom(dc.DeepCopy())
		dc.Spec.RollingRestartRequested = false
		err = rc.Client.Patch(rc.Ctx, dc, dcPatch)
		if err != nil {
			logger.Error(err, "error patching datacenter for rolling restart")
			return result.Error(err)
		}
	}

	cutoff := &dc.Status.LastRollingRestart
	for _, pod := range rc.dcPods {
		podStartTime := pod.GetCreationTimestamp()
		if podStartTime.Before(cutoff) {
			rc.Recorder.Eventf(rc.Datacenter, "Normal", "RestartingCassandra",
				"Restarting Cassandra for pod %s", pod.Name)

			// drain the node
			err := rc.NodeMgmtClient.CallDrainEndpoint(pod)
			if err != nil {
				logger.Error(err, "error during drain during rolling restart",
					"pod", pod.Name)
			}
			// get a fresh pod
			// TODO should we keep the pod and cycle the DB with mgmt api?
			err = rc.Client.Delete(rc.Ctx, pod)
			if err != nil {
				return result.Error(err)
			}
			return result.Done()
		}
	}

	return result.Continue()
}

// ReconcileAllRacks determines if a rack needs to be reconciled.
func (rc *ReconciliationContext) ReconcileAllRacks() (reconcile.Result, error) {
	logger := rc.ReqLogger
	logger.Info("reconcile_racks::Apply")

	podList, err := rc.listPods(rc.Datacenter.GetClusterLabels())
	if err != nil {
		logger.Error(err, "error listing all pods in the cluster")
	}

	rc.clusterPods = PodPtrsFromPodList(podList)

	dcSelector := rc.Datacenter.GetDatacenterLabels()
	rc.dcPods = FilterPodListByLabels(rc.clusterPods, dcSelector)

	endpointData := rc.getCassMetadataEndpoints()

	if recResult := rc.UpdateStatus(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckSuperuserSecretCreation(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackCreation(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackLabels(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackStoppedState(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackScale(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckPodsReady(endpointData); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRollingRestart(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckDcPodDisruptionBudget(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackPodTemplate(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CheckRackPodLabels(); recResult.Completed() {
		return recResult.Output()
	}

	if recResult := rc.CreateSuperuser(); recResult.Completed() {
		return recResult.Output()
	}

	if err := setOperatorProgressStatus(rc, api.ProgressReady); err != nil {
		return result.Error(err).Output()
	}

	rc.ReqLogger.Info("All StatefulSets should now be reconciled.")

	return result.Done().Output()
}
