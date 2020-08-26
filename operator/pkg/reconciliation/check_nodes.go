// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

// Check nodes for vmware PSP draining taints
// and check PVCs for vmware PSP failure annotations
func (rc *ReconciliationContext) checkNodeAndPvcTaints() error {
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
					rc.ReqLogger.Info("reconciler::checkNodesTaints vmware taint found.  draining and deleting pod",
						"pod", pod.Name)

					err = rc.removePvcAndPod(pod)
					if err != nil {
						rc.ReqLogger.Error(err, "error during cassandra node drain",
							"pod", pod.Name)
						return err
					}
				}
			}
		}

		// Get the name of the pvc

		pvcName := ""
		for _, vol := range pod.Spec.Volumes {
			if vol.Name == PvcName {
				pvcName = vol.PersistentVolumeClaim.ClaimName
			}
		}

		// Check the related PVCs for annotation

		pvc := &corev1.PersistentVolumeClaim{}
		err = rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: pod.ObjectMeta.Namespace, Name: pvcName}, pvc)
		if err != nil {
			logger.Error(err, "error retrieving PersistentVolumeClaim for pod for pvc annotation check")
			return err
		}

		rc.ReqLogger.Info(fmt.Sprintf("pvc %s has %d annotations", pvc.ObjectMeta.Name, len(pvc.ObjectMeta.Annotations)))

		// “volumehealth.storage.kubernetes.io/health”: “inaccessible”

		for k, v := range pvc.ObjectMeta.Annotations {
			if k == "volumehealth.storage.kubernetes.io/health" && v == "inaccessible" {
				rc.ReqLogger.Info("vmware pvc inaccessible annotation found.  draining and deleting pod",
					"pod", pod.Name)

				err = rc.removePvcAndPod(pod)
				if err != nil {
					rc.ReqLogger.Error(err, "error during cassandra node drain",
						"pod", pod.Name)
					return err
				}
			}
		}
	}

	return nil
}
