package reconciliation

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/events"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"k8s.io/apimachinery/pkg/types"
)

func (rc *ReconciliationContext) DecommissionNodes() result.ReconcileResult {
	logger := rc.ReqLogger
	logger.Info("reconcile_racks::DecommissionNodes")
	dc := rc.Datacenter

	for idx := range rc.desiredRackInformation {
		rackInfo := rc.desiredRackInformation[idx]
		statefulSet := rc.statefulSets[idx]
		desiredNodeCount := int32(rackInfo.NodeCount)
		maxReplicas := *statefulSet.Spec.Replicas
		lastPodSuffix := stsLastPodSuffix(maxReplicas)

		if maxReplicas > desiredNodeCount {
			dcPatch := client.MergeFrom(dc.DeepCopy())
			updated := false

			updated = rc.setCondition(
				api.NewDatacenterCondition(
					api.DatacenterScalingDown, corev1.ConditionTrue)) || updated

			if updated {
				err := rc.Client.Status().Patch(rc.Ctx, dc, dcPatch)
				if err != nil {
					logger.Error(err, "error patching datacenter status for scaling down rack started")
					return result.Error(err)
				}
			}

			rc.ReqLogger.Info(
				"Need to update the rack's node count",
				"Rack", rackInfo.RackName,
				"maxReplicas", maxReplicas,
				"desiredSize", desiredNodeCount,
			)

			rc.Recorder.Eventf(rc.Datacenter, corev1.EventTypeNormal, events.ScalingDownRack,
				"Scaling down rack %s", rackInfo.RackName)

			if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
				return result.Error(err)
			}

			err := rc.DecommissionNodeOnRack(rackInfo.RackName, lastPodSuffix)
			if err != nil {
				return result.Error(err)
			}

			return result.RequeueSoon(10)
		}
	}

	return result.Continue()
}

func (rc *ReconciliationContext) DecommissionNodeOnRack(rackName string, lastPodSuffix string) error {
	for _, pod := range rc.dcPods {
		podRack := pod.Labels[api.RackLabel]
		if podRack == rackName && strings.HasSuffix(pod.Name, lastPodSuffix) {
			mgmtApiUp := isMgmtApiRunning(pod)
			if !mgmtApiUp {
				return fmt.Errorf("Management API is not up on node that we are trying to decommission")
			}

			if err := rc.NodeMgmtClient.CallDecommissionNodeEndpoint(pod); err != nil {
				// TODO this returns a 500 when it works
				// We are waiting for a new version of dse with a fix for this
				// return false, err
			}

			rc.ReqLogger.Info("Marking node as decommissioning")
			patch := client.MergeFrom(pod.DeepCopy())
			pod.Labels[api.CassNodeState] = stateDecommissioning
			if err := rc.Client.Patch(rc.Ctx, pod, patch); err != nil {
				return err
			}

			rc.Recorder.Eventf(rc.Datacenter, corev1.EventTypeNormal, events.LabeledPodAsDecommissioning,
				"Labeled node as decommissioning %s", pod.Name)

			return nil
		}
	}

	// this shouldn't happen
	return fmt.Errorf("Could not find pod to decommission on rack %s", rackName)
}

// Wait for decommissioning nodes to finish before continuing to reconcile
func (rc *ReconciliationContext) CheckDecommissioningNodes(epData httphelper.CassMetadataEndpoints) result.ReconcileResult {
	for _, pod := range rc.dcPods {
		if pod.Labels[api.CassNodeState] == stateDecommissioning {
			if !IsDoneDecommissioning(pod, epData) {
				rc.ReqLogger.Info("Node decommissioning, reconciling again soon")
			} else {
				rc.ReqLogger.Info("Node finished decommissioning")
				rc.ReqLogger.Info("Deleting pod PVCs")

				err := rc.DeletePodPvcs(pod)
				if err != nil {
					return result.Error(err)
				}

				rc.ReqLogger.Info("Scaling down statefulset")
				err = rc.RemoveDecommissionedPodFromSts(pod)
				if err != nil {
					return result.Error(err)
				}

			}

			return result.RequeueSoon(5)
		}
	}

	dcPatch := client.MergeFrom(rc.Datacenter.DeepCopy())
	updated := false

	updated = rc.setCondition(
		api.NewDatacenterCondition(
			api.DatacenterScalingDown, corev1.ConditionFalse)) || updated

	if updated {
		err := rc.Client.Status().Patch(rc.Ctx, rc.Datacenter, dcPatch)
		if err != nil {
			rc.ReqLogger.Error(err, "error patching datacenter status for scaling down finished")
			return result.Error(err)
		}
	}

	return result.Continue()
}

func IsDoneDecommissioning(pod *v1.Pod, epData httphelper.CassMetadataEndpoints) bool {
	for idx := range epData.Entity {
		ep := &epData.Entity[idx]
		if ep.GetRpcAddress() == pod.Status.PodIP {
			return strings.HasPrefix(ep.Status, "LEFT")
		}
	}

	// If we got here, we could not find endpoint metadata on the node.
	// This should mean that it no longer exists... but typically
	// the endpoint data lingers for a while after it has been decommissioned
	// so this scenario should be unlikely
	return true
}

func (rc *ReconciliationContext) DeletePodPvcs(pod *v1.Pod) error {
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}

		pvcName := v.PersistentVolumeClaim.ClaimName
		name := types.NamespacedName{
			Name:      v.PersistentVolumeClaim.ClaimName,
			Namespace: rc.Datacenter.Namespace,
		}

		podPvc := &corev1.PersistentVolumeClaim{}
		err := rc.Client.Get(rc.Ctx, name, podPvc)
		if err != nil {
			rc.ReqLogger.Info("Failed to get pod PVC with name: %s", pvcName)
			return err
		}

		err = rc.Client.Delete(rc.Ctx, podPvc)
		if err != nil {
			rc.ReqLogger.Info("Failed to delete pod PVC with name: %s", pvcName)
			return err
		}

		rc.Recorder.Eventf(rc.Datacenter, corev1.EventTypeNormal, events.DeletedPvc,
			"Deleted PVC %s", pvcName)

	}

	return nil
}

func (rc *ReconciliationContext) RemoveDecommissionedPodFromSts(pod *v1.Pod) error {
	podRack := pod.Labels[api.RackLabel]
	var sts *appsv1.StatefulSet
	for _, s := range rc.statefulSets {
		if s.Labels[api.RackLabel] == podRack {
			sts = s
			break
		}
	}

	if sts == nil {
		// Failed to find the statefulset for this pod
		return fmt.Errorf("Failed to find matching statefulSet for pod rack: %s", podRack)
	}

	maxReplicas := *sts.Spec.Replicas
	lastPodSuffix := stsLastPodSuffix(maxReplicas)
	if strings.HasSuffix(pod.Name, lastPodSuffix) {
		return rc.UpdateRackNodeCount(sts, *sts.Spec.Replicas-1)
	} else {
		// Pod does not match the last pod in statefulSet
		// This scenario should only happen if the pod
		// has already been terminated
		return nil
	}
}

func stsLastPodSuffix(maxReplicas int32) string {
	return fmt.Sprintf("sts-%v", maxReplicas-1)
}
