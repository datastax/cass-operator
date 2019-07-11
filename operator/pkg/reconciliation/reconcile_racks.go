package reconciliation

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"
	"github.com/riptano/dse-operator/operator/pkg/utils"
)

type ReconcileRacks struct {
	ReconcileContext       *dsereconciliation.ReconciliationContext
	desiredRackInformation []*dsereconciliation.RackInformation
}

//
// Determine how many nodes per rack are needed
//
func (r *ReconcileRacks) CalculateRackInformation() (reconcileriface.Reconciler, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_racks::calculateRackInformation")

	//
	// Create RackInformation
	//

	nodeCount := int(r.ReconcileContext.DseDatacenter.Spec.GetDesiredNodeCount())
	rackCount := len(r.ReconcileContext.DseDatacenter.Spec.Racks)

	var desiredRackInformation []*dsereconciliation.RackInformation

	// If explicit racks are not specified,
	// then we have only one
	if rackCount == 0 {
		rackCount = 1

		nextRack := &dsereconciliation.RackInformation{}
		nextRack.RackName = "default"
		nextRack.NodeCount = nodeCount

		desiredRackInformation = append(desiredRackInformation, nextRack)
	} else {
		// nodes_per_rack = total_size / rack_count + 1 if rack_index < remainder

		nodesPerRack, extraNodes := nodeCount/rackCount, nodeCount%rackCount

		for rackIndex, dseRack := range r.ReconcileContext.DseDatacenter.Spec.Racks {
			nodesForThisRack := nodesPerRack
			if rackIndex < extraNodes {
				nodesForThisRack++
			}
			nextRack := &dsereconciliation.RackInformation{}
			nextRack.RackName = dseRack.Name
			nextRack.NodeCount = nodesForThisRack

			desiredRackInformation = append(desiredRackInformation, nextRack)
		}
	}

	return &ReconcileRacks{
		ReconcileContext:       r.ReconcileContext,
		desiredRackInformation: desiredRackInformation,
	}, nil
}

// reconcileRacks determines if a rack needs to be reconciled.
func (r *ReconcileRacks) Apply() (reconcile.Result, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_racks::reconcileRacks")

	for _, rackInfo := range r.desiredRackInformation {
		// Does this rack have a statefulset?

		statefulSet, statefulSetFound, err := r.GetStatefulSetForRack(rackInfo)
		if err != nil {
			r.ReconcileContext.ReqLogger.Error(
				err,
				"Could not locate statefulSet for",
				"Rack",
				rackInfo.RackName)
			return reconcile.Result{Requeue: true}, err
		}

		if statefulSetFound == false {
			r.ReconcileContext.ReqLogger.Info(
				"Need to create new StatefulSet for",
				"Rack",
				rackInfo.RackName)

			return r.ReconcileNextRack(statefulSet)
		}

		// Has this statefulset been reconciled?

		stsLabels := statefulSet.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(stsLabels, r.ReconcileContext.DseDatacenter, rackInfo.RackName)

		if shouldUpdateLabels {
			r.ReconcileContext.ReqLogger.Info("Updating labels",
				"statefulSet", statefulSet,
				"current", stsLabels,
				"desired", updatedLabels)
			statefulSet.SetLabels(updatedLabels)

			if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, statefulSet); err != nil {
				r.ReconcileContext.ReqLogger.Info("Unable to update statefulSet with labels",
					"statefulSet",
					statefulSet)
			}
		}

		desiredNodeCount := int32(rackInfo.NodeCount)
		currentPodCount := *statefulSet.Spec.Replicas

		if currentPodCount < desiredNodeCount {
			// update it
			r.ReconcileContext.ReqLogger.Info(
				"Need to update the rack's node count",
				"Rack", rackInfo.RackName,
				"currentSize", currentPodCount,
				"desiredSize", desiredNodeCount,
			)

			return r.UpdateRackNodeCount(statefulSet, desiredNodeCount)
		}

		parked := r.ReconcileContext.DseDatacenter.Spec.Parked

		if parked && currentPodCount > 0 {
			r.ReconcileContext.ReqLogger.Info(
				"DseDatacenter is parked, setting rack to zero replicas",
				"Rack", rackInfo.RackName,
				"currentSize", currentPodCount,
				"desiredSize", desiredNodeCount,
			)

			return r.UpdateRackNodeCount(statefulSet, desiredNodeCount)
		}

		r.ReconcileContext.ReqLogger.Info(
			"StatefulSet found",
			"ResourceVersion",
			statefulSet.ResourceVersion)

		r.LabelSeedPods(statefulSet)

		readyReplicas := statefulSet.Status.ReadyReplicas

		if readyReplicas < desiredNodeCount {
			// We should do nothing but wait until all replicas are ready
			r.ReconcileContext.ReqLogger.Info(
				"Not all replicas for StatefulSet are ready.",
				"desiredCount", desiredNodeCount,
				"readyCount", readyReplicas)

			return reconcile.Result{Requeue: true}, nil
		}

		if readyReplicas > desiredNodeCount {
			// too many ready replicas, how did this happen?
			r.ReconcileContext.ReqLogger.Info(
				"Too many replicas for StatefulSet are ready",
				"desiredCount", desiredNodeCount,
				"readyCount", readyReplicas)
			return reconcile.Result{Requeue: true}, nil
		}

		r.ReconcileContext.ReqLogger.Info(
			"All replicas are ready for StatefulSet for",
			"Rack",
			rackInfo.RackName)

		if err := r.ReconcilePods(statefulSet); err != nil {
			return reconcile.Result{Requeue: true}, err
		}
	}

	r.ReconcileContext.ReqLogger.Info("All StatefulSets should now be reconciled.")

	return reconcile.Result{}, nil
}

// labelSeedsPods will iterate over all seed node pods for a datacenter and if the pod exists and is not already labeled will
// add the dse-seed=true label to the pod so that its picked up by the headless seed service
func (r *ReconcileRacks) LabelSeedPods(statefulSet *appsv1.StatefulSet) {
	seeds := r.ReconcileContext.DseDatacenter.GetSeedList()
	for _, seed := range seeds {
		podName := strings.Split(seed, ".")[0]
		pod := &corev1.Pod{}
		err := r.ReconcileContext.Client.Get(
			r.ReconcileContext.Ctx,
			types.NamespacedName{
				Name:      podName,
				Namespace: statefulSet.Namespace},
			pod)
		if err != nil {
			r.ReconcileContext.ReqLogger.Info("Unable to get seed pod",
				"pod",
				podName)
			return
		}

		podLabels := pod.GetLabels()

		if _, ok := podLabels[datastaxv1alpha1.SEED_NODE_LABEL]; !ok {
			podLabels[datastaxv1alpha1.SEED_NODE_LABEL] = "true"
			pod.SetLabels(podLabels)

			if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, pod); err != nil {
				r.ReconcileContext.ReqLogger.Info("Unable to update pod with seed label",
					"pod",
					podName)
			}
		}
	}
}

// Returns the statefulset for the rack
// and whether it currently exists
// and whether an error occured
func (r *ReconcileRacks) GetStatefulSetForRack(
	nextRack *dsereconciliation.RackInformation) (*appsv1.StatefulSet, bool, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_racks::getStatefulSetForRack")

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		nextRack.RackName,
		r.ReconcileContext.DseDatacenter,
		nextRack.NodeCount)
	if err != nil {
		return nil, false, err
	}

	// Set dseDatacenter dseDatacenter as the owner and controller
	err = setControllerReference(
		r.ReconcileContext.DseDatacenter,
		desiredStatefulSet,
		r.ReconcileContext.Scheme)
	if err != nil {
		return nil, false, err
	}

	// Check if the desiredStatefulSet already exists
	currentStatefulSet := &appsv1.StatefulSet{}
	err = r.ReconcileContext.Client.Get(
		r.ReconcileContext.Ctx,
		types.NamespacedName{
			Name:      desiredStatefulSet.Name,
			Namespace: desiredStatefulSet.Namespace},
		currentStatefulSet)
	if err != nil && errors.IsNotFound(err) {
		return desiredStatefulSet, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return currentStatefulSet, true, nil
}

// Ensure that the resources for a dse rack have been properly created
//
// Note that each statefulset is using OrderedReadyPodManagement,
// so it will bring up one node at a time.
func (r *ReconcileRacks) ReconcileNextRack(statefulSet *appsv1.StatefulSet) (reconcile.Result, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_racks::reconcileNextRack")

	// Create the StatefulSet
	r.ReconcileContext.ReqLogger.Info(
		"Creating a new StatefulSet.",
		"statefulSetNamespace",
		statefulSet.Namespace,
		"statefulSetName",
		statefulSet.Name)
	err := r.ReconcileContext.Client.Create(
		r.ReconcileContext.Ctx,
		statefulSet)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	//
	// Create a PodDisruptionBudget for the StatefulSet
	//

	desiredBudget := newPodDisruptionBudgetForStatefulSet(
		r.ReconcileContext.DseDatacenter,
		statefulSet)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err = setControllerReference(
		r.ReconcileContext.DseDatacenter,
		desiredBudget,
		r.ReconcileContext.Scheme)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	// Check if the budget already exists
	currentBudget := &policyv1beta1.PodDisruptionBudget{}
	err = r.ReconcileContext.Client.Get(
		r.ReconcileContext.Ctx,
		types.NamespacedName{
			Name:      desiredBudget.Name,
			Namespace: desiredBudget.Namespace},
		currentBudget)

	if err != nil && errors.IsNotFound(err) {
		// Create the Budget
		r.ReconcileContext.ReqLogger.Info(
			"Creating a new PodDisruptionBudget.",
			"podDisruptionBudgetNamespace:",
			desiredBudget.Namespace,
			"podDisruptionBudgetName:",
			desiredBudget.Name)
		err = r.ReconcileContext.Client.Create(
			r.ReconcileContext.Ctx,
			desiredBudget)
		if err != nil {
			return reconcile.Result{Requeue: true}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileRacks) UpdateRackNodeCount(statefulSet *appsv1.StatefulSet, newNodeCount int32) (reconcile.Result, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_racks::updateRack")

	r.ReconcileContext.ReqLogger.Info(
		"updating StatefulSet node count",
		"statefulSetNamespace", statefulSet.Namespace,
		"statefulSetName", statefulSet.Name,
		"newNodeCount", newNodeCount,
	)

	statefulSet.Spec.Replicas = &newNodeCount

	err := r.ReconcileContext.Client.Update(
		r.ReconcileContext.Ctx,
		statefulSet)

	return reconcile.Result{Requeue: true}, err
}

func (r *ReconcileRacks) ReconcilePods(statefulSet *appsv1.StatefulSet) error {
	r.ReconcileContext.ReqLogger.Info("reconcile_racks::ReconcilePods")

	for i := int32(0); i < statefulSet.Status.Replicas; i++ {
		podName := fmt.Sprintf("%s-%v", statefulSet.Name, i)

		pod := &corev1.Pod{}
		err := r.ReconcileContext.Client.Get(
			r.ReconcileContext.Ctx,
			types.NamespacedName{
				Name:      podName,
				Namespace: statefulSet.Namespace},
			pod)
		if err != nil {
			r.ReconcileContext.ReqLogger.Info("Unable to get pod",
				"Pod",
				podName)
			return err
		}

		podLabels := pod.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(podLabels, r.ReconcileContext.DseDatacenter, statefulSet.GetLabels()[datastaxv1alpha1.RACK_LABEL])
		if shouldUpdateLabels {
			r.ReconcileContext.ReqLogger.Info("Updating labels",
				"Pod", podName,
				"current", podLabels,
				"desired", updatedLabels)
			pod.SetLabels(updatedLabels)

			if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, pod); err != nil {
				r.ReconcileContext.ReqLogger.Info("Unable to update pod with label",
					"Pod",
					podName)
			}
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
		err = r.ReconcileContext.Client.Get(
			r.ReconcileContext.Ctx,
			types.NamespacedName{
				Name:      pvcName,
				Namespace: statefulSet.Namespace},
			pvc)
		if err != nil {
			r.ReconcileContext.ReqLogger.Info("Unable to get pvc",
				"PVC",
				pvcName)
			return err
		}

		pvcLabels := pvc.GetLabels()
		shouldUpdateLabels, updatedLabels = shouldUpdateLabelsForRackResource(pvcLabels, r.ReconcileContext.DseDatacenter, statefulSet.GetLabels()[datastaxv1alpha1.RACK_LABEL])
		if shouldUpdateLabels {
			r.ReconcileContext.ReqLogger.Info("Updating labels",
				"PVC", pvc,
				"current", pvcLabels,
				"desired", updatedLabels)

			pvc.SetLabels(updatedLabels)

			if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, pvc); err != nil {
				r.ReconcileContext.ReqLogger.Info("Unable to update pvc with labels",
					"PVC",
					pvc)
			}
		}
	}

	return nil
}

// shouldUpdateLabelsForClusterResource will compare the labels passed in with what the labels should be for a cluster level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForClusterResource(resourceLabels map[string]string, dseDatacenter *datastaxv1alpha1.DseDatacenter) (bool, map[string]string) {
	labelsUpdated := false

	if resourceLabels == nil {
		resourceLabels = make(map[string]string)
	}

	if _, ok := resourceLabels[datastaxv1alpha1.CLUSTER_LABEL]; !ok {
		labelsUpdated = true
	} else if resourceLabels[datastaxv1alpha1.CLUSTER_LABEL] != dseDatacenter.Spec.ClusterName {
		labelsUpdated = true
	}

	if labelsUpdated {
		utils.MergeMap(&resourceLabels, dseDatacenter.GetClusterLabels())
	}

	return labelsUpdated, resourceLabels
}

// shouldUpdateLabelsForRackResource will compare the labels passed in with what the labels should be for a rack level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForRackResource(resourceLabels map[string]string, dseDatacenter *datastaxv1alpha1.DseDatacenter, rackName string) (bool, map[string]string) {
	labelsUpdated, resourceLabels := shouldUpdateLabelsForDatacenterResource(resourceLabels, dseDatacenter)

	if _, ok := resourceLabels[datastaxv1alpha1.RACK_LABEL]; !ok {
		labelsUpdated = true
	} else if resourceLabels[datastaxv1alpha1.RACK_LABEL] != rackName {
		labelsUpdated = true
	}

	if labelsUpdated {
		utils.MergeMap(&resourceLabels, dseDatacenter.GetRackLabels(rackName))
	}

	return labelsUpdated, resourceLabels
}

// shouldUpdateLabelsForDatacenterResource will compare the labels passed in with what the labels should be for a datacenter level
// resource. It will return the updated map and a boolean denoting whether the resource needs to be updated with the new labels.
func shouldUpdateLabelsForDatacenterResource(resourceLabels map[string]string, dseDatacenter *datastaxv1alpha1.DseDatacenter) (bool, map[string]string) {
	labelsUpdated, resourceLabels := shouldUpdateLabelsForClusterResource(resourceLabels, dseDatacenter)

	if _, ok := resourceLabels[datastaxv1alpha1.DATACENTER_LABEL]; !ok {
		labelsUpdated = true
	} else if resourceLabels[datastaxv1alpha1.DATACENTER_LABEL] != dseDatacenter.Name {
		labelsUpdated = true
	}

	if labelsUpdated {
		utils.MergeMap(&resourceLabels, dseDatacenter.GetDatacenterLabels())
	}

	return labelsUpdated, resourceLabels
}
