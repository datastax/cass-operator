// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Check nodes for vmware draining taints
func (rc *ReconciliationContext) checkNodeTaints() error {
	logger := rc.ReqLogger
	rc.ReqLogger.Info("reconciler::checkNodesTaints")

	// Get the pods

	podList, err := rc.listPods(rc.Datacenter.GetClusterLabels())
	if err != nil {
		logger.Error(err, "error listing all pods in the cluster")
	}

	rc.ReqLogger.Info("reconciler::checkNodesTaints podlist - listing pods")
	rc.ReqLogger.Info(fmt.Sprintf("there are %d pods", len(podList.Items)))

	rc.clusterPods = PodPtrsFromPodList(podList)

	for _, pod := range podList.Items {
		rc.ReqLogger.Info("reconciler::checkNodesTaints - examine pod")
		// Check the related node for taints
		node := &corev1.Node{}
		err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: "", Name: pod.Spec.NodeName}, node)
		if err != nil {
			logger.Error(err, "error retrieving node for pod for node taint check")
			return err
		}

		rc.ReqLogger.Info(fmt.Sprintf("node %s has %d taints", node.ObjectMeta.Name, len(node.Spec.Taints)))

		for _, taint := range node.Spec.Taints {
			rc.ReqLogger.Info("reconciler::checkNodesTaints examine taint")
			rc.ReqLogger.Info(taint.ToString())

			if taint.Key == "node.vmware.com/drain" && taint.Effect == "NoSchedule" {
				if taint.Value == "planned-downtime" || taint.Value == "drain" {
					// Drain the cassandra node
					rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found - before drain called")

					if isMgmtApiRunning(&pod) {
						err = rc.NodeMgmtClient.CallDrainEndpoint(&pod)
						if err != nil {
							logger.Error(err, "error during cassandra node drain for vmware drain",
								"pod", pod.Name)
						}
					}

					rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found - after drain called")

					err = rc.Client.Delete(rc.Ctx, &pod)
					if err != nil {
						logger.Error(err, "error during cassandra node delete for vmware drain",
							"pod", pod.Name)
						return err
					}

					rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found - after pod deleted")
				}
			}
		}
		rc.ReqLogger.Info("reconciler::checkNodesTaints after taints - items")
	}
	rc.ReqLogger.Info("reconciler::checkNodesTaints after podlist - items")

	return nil
}
