// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"sync"
	"time"

	// "github.com/datastax/cass-operator/operator/reconciliation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (rc *ReconciliationContext) DeletePvcIgnoreFinalizers(podNamespace string, podName string) error {
	var wg sync.WaitGroup

	wg.Add(1)

	result := nil

	pvcName := fmt.Sprintf("%s-%s", PvcName, podName)

	pvc := &corev1.PersistentVolumeClaim{}
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: podNamespace, Name: pvcName}, pvc)
	if err != nil {
		rc.ReqLogger.Error(err, "error retrieving PersistentVolumeClaim for deletion")
		return err
	}

	// Delete might hang due to a finalizer such as kubernetes.io/pvc-protection
	// so we run it asynchronously and then remove any finalizers to unblock it.
	go func() {
		defer wg.Done()

		err = rc.Client.Delete(rc.Ctx, pvc)
		if err != nil {
			rc.ReqLogger.Error(err, "error removing PersistentVolumeClaim",
				"name", pvcName)
			result = err
		}
	}()

	// Give the resource a second to get to a terminating state. Note that this
	// may not be reflected in the resource's status... hence the sleep here as
	// opposed to checking the status.
	time.Sleep(5 * time.Second)

	// In the case of PVCs at least, finalizers removed before deletion can be
	// automatically added back. Consequently, we delete the resource first,
	// then remove any finalizers while it is terminating.

	pvc.ObjectMeta.Finalizers = []string{}

	// Ignore errors as this may fail due to the resource already having been
	// deleted (which is what we want).
	_ = rc.Client.Update(rc.Ctx, pvc)

	// Wait for the delete to finish, which should have been unblocked by
	// removing the finalizers.
	wg.Wait()

	return result
}

// Check nodes for vmware draining taints
func (rc *ReconciliationContext) checkNodeTaints() error {
	logger := rc.ReqLogger
	rc.ReqLogger.Info("reconciler::checkNodesTaints")

	// Get the pods

	podList, err := rc.listPods(rc.Datacenter.GetClusterLabels())
	if err != nil {
		logger.Error(err, "error listing all pods in the cluster")
	}

	rc.clusterPods = PodPtrsFromPodList(podList)

	for _, pod := range podList.Items {
		// Check the related node for taints
		node := &corev1.Node{}
		err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: "", Name: pod.Spec.NodeName}, node)
		if err != nil {
			logger.Error(err, "error retrieving node for pod for node taint check")
			return err
		}

		rc.ReqLogger.Info(fmt.Sprintf("node %s has %d taints", node.ObjectMeta.Name, len(node.Spec.Taints)))

		for _, taint := range node.Spec.Taints {
			if taint.Key == "node.vmware.com/drain" && taint.Effect == "NoSchedule" {
				if taint.Value == "planned-downtime" || taint.Value == "drain" {
					// Drain the cassandra node
					rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found.  draining and deleting pod")

					if isMgmtApiRunning(&pod) {
						err = rc.NodeMgmtClient.CallDrainEndpoint(&pod)
						if err != nil {
							rc.ReqLogger.Error(err, "error during cassandra node drain for vmware drain",
								"pod", pod.Name)
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

					rc.DeletePvcIgnoreFinalizers(pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)

					// Remove the pod

					err = rc.Client.Delete(rc.Ctx, &pod)
					if err != nil {
						rc.ReqLogger.Error(err, "error during cassandra node delete for vmware drain",
							"pod", pod.ObjectMeta.Name)
						return err
					}
				}
			}
		}
	}

	return nil
}
