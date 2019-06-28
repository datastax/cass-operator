package reconciliation

//
// This file defines handlers for events on the EventBus
//

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/utils"
)

// Use a var so we can mock this function
var setControllerReference = controllerutil.SetControllerReference

//
// Determine which actions need to be performed first.
//
// It either recommends a service be created,
// or that a seed service be reconciled.
//
func calculateReconciliationActions(
	rc *ReconciliationContext) error {

	rc.reqLogger.Info("handler::calculateReconciliationActions")

	// Check if the DseDatacenter was marked to be deleted
	isMarkedToBeDeleted := rc.dseDatacenter.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		EventBus.Publish(
			PROCESS_DELETION_TOPIC,
			rc)

		return nil
	}

	if err := rc.reconciler.addFinalizer(rc); err != nil {
		return err
	}

	//
	// Check if there is a headless service for the cluster
	//

	desiredService := newServiceForDseDatacenter(rc.dseDatacenter)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err := setControllerReference(
		rc.dseDatacenter,
		desiredService,
		rc.reconciler.scheme)
	if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not set controller reference for headless service")
		return err
	}

	currentService := &corev1.Service{}

	err = rc.reconciler.client.Get(
		rc.ctx,
		types.NamespacedName{
			Name:      desiredService.Name,
			Namespace: desiredService.Namespace},
		currentService)

	if err != nil && errors.IsNotFound(err) {
		EventBus.Publish(
			CREATE_HEADLESS_SERVICE_TOPIC,
			rc,
			desiredService)
	} else if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not get headless service")

		return err
	} else {
		EventBus.Publish(
			RECONCILE_HEADLESS_SERVICE_TOPIC,
			rc,
			currentService)
	}

	return nil
}

func processDeletion(rc *ReconciliationContext) error {
	rc.reqLogger.Info("handler::processDeletion")

	if err := deletePVCs(rc); err != nil {
		rc.reqLogger.Error(err, "Failed to delete PVCs for DseDatacenter")
		return err
	}

	// Update finalizer to allow delete of DseDatacenter
	rc.dseDatacenter.SetFinalizers(nil)

	// Update DseDatacenter
	if err := rc.reconciler.client.Update(rc.ctx, rc.dseDatacenter); err != nil {
		rc.reqLogger.Error(err, "Failed to update DseDatacenter with removed finalizers")
		return err
	}

	return nil
}

func reconcileHeadlessService(rc *ReconciliationContext, service *corev1.Service) error {
	rc.reqLogger.Info("handler::reconcileHeadlessService")

	svcLabels := service.GetLabels()
	shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForDatacenterResource(svcLabels, rc.dseDatacenter)

	if shouldUpdateLabels {
		rc.reqLogger.Info("Updating labels",
			"service", service,
			"current", svcLabels,
			"desired", updatedLabels)
		service.SetLabels(updatedLabels)

		if err := rc.reconciler.client.Update(rc.ctx, service); err != nil {
			rc.reqLogger.Info("Unable to update service with labels",
				"service",
				service)
		}
	}

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	return nil
}

//
// Create a headless service for this datacenter.
//

func createHeadlessService(
	rc *ReconciliationContext,
	service *corev1.Service) error {

	rc.reqLogger.Info(
		"Creating a new headless service",
		"ServiceNamespace",
		service.Namespace,
		"ServiceName",
		service.Name)

	err := rc.reconciler.client.Create(
		rc.ctx,
		service)
	if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not create headless service")

		return err
	}

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	return nil
}

func reconcileHeadlessSeedService(
	rc *ReconciliationContext,
	service *corev1.Service) error {

	rc.reqLogger.Info("handler::reconcileHeadlessSeedService")

	//
	// Check if there is a headless seed service for the cluster
	//

	desiredService := newSeedServiceForDseDatacenter(rc.dseDatacenter)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err := setControllerReference(
		rc.dseDatacenter,
		desiredService,
		rc.reconciler.scheme)
	if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not set controller reference for headless seed service")
		return err
	}

	currentService := &corev1.Service{}

	err = rc.reconciler.client.Get(
		rc.ctx,
		types.NamespacedName{
			Name:      desiredService.Name,
			Namespace: desiredService.Namespace},
		currentService)

	if err != nil && errors.IsNotFound(err) {
		EventBus.Publish(
			CREATE_HEADLESS_SEED_SERVICE_TOPIC,
			rc,
			desiredService,
			service)
	} else if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not get headless seed service")

		return err
	} else {
		svcLabels := currentService.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForClusterResource(svcLabels, rc.dseDatacenter)

		if shouldUpdateLabels {
			rc.reqLogger.Info("Updating labels",
				"service", currentService,
				"current", svcLabels,
				"desired", updatedLabels)
			currentService.SetLabels(updatedLabels)

			if err := rc.reconciler.client.Update(rc.ctx, currentService); err != nil {
				rc.reqLogger.Info("Unable to update service with labels",
					"service",
					currentService)
			}
		}

		EventBus.Publish(
			CALCULATE_RACK_INFORMATION_TOPIC,
			rc,
			service)
	}

	return nil
}

func createHeadlessSeedService(
	rc *ReconciliationContext,
	seedService *corev1.Service,
	service *corev1.Service) error {

	rc.reqLogger.Info(
		"Creating a new headless seed service",
		"ServiceNamespace",
		seedService.Namespace,
		"ServiceName",
		seedService.Name)

	err := rc.reconciler.client.Create(
		rc.ctx,
		seedService)
	if err != nil {
		rc.reqLogger.Error(
			err,
			"Could not create headless service")

		return err
	}

	EventBus.Publish(
		CALCULATE_RACK_INFORMATION_TOPIC,
		rc,
		service)

	return nil
}

type RackInformation struct {
	RackName  string
	NodeCount int
}

//
// Determine how many nodes per rack are needed
//
func calculateRackInformation(
	rc *ReconciliationContext,
	service *corev1.Service) error {

	rc.reqLogger.Info("handler::calculateRackInformation")

	//
	// Create RackInformation
	//

	nodeCount := int(rc.dseDatacenter.Spec.Size)
	rackCount := len(rc.dseDatacenter.Spec.Racks)

	var desiredRackInformation []*RackInformation

	// If explicit racks are not specified,
	// then we have only one
	if rackCount == 0 {
		rackCount = 1

		nextRack := &RackInformation{}
		nextRack.RackName = "default"
		nextRack.NodeCount = nodeCount

		desiredRackInformation = append(desiredRackInformation, nextRack)
	} else {
		// nodes_per_rack = total_size / rack_count + 1 if rack_index < remainder

		nodesPerRack, extraNodes := nodeCount/rackCount, nodeCount%rackCount

		for rackIndex, dseRack := range rc.dseDatacenter.Spec.Racks {
			nodesForThisRack := nodesPerRack
			if rackIndex < extraNodes {
				nodesForThisRack++
			}
			nextRack := &RackInformation{}
			nextRack.RackName = dseRack.Name
			nextRack.NodeCount = nodesForThisRack

			desiredRackInformation = append(desiredRackInformation, nextRack)
		}
	}

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		desiredRackInformation)

	return nil
}

//
// Determine if a rack needs to be reconciled.
//
func reconcileRacks(
	rc *ReconciliationContext,
	service *corev1.Service,
	desiredRackInformation []*RackInformation) error {

	rc.reqLogger.Info("handler::reconcileRacks")

	for _, rackInfo := range desiredRackInformation {
		//
		// Does this rack have a statefulset?
		//

		statefulSet, statefulSetFound, err := getStatefulSetForRack(
			rc,
			service,
			rackInfo)
		if err != nil {
			rc.reqLogger.Error(
				err,
				"Could not locate statefulSet for",
				"Rack",
				rackInfo.RackName)
			return err
		}

		if statefulSetFound == false {
			rc.reqLogger.Info(
				"Need to create new StatefulSet for",
				"Rack",
				rackInfo.RackName)

			EventBus.Publish(
				RECONCILE_NEXT_RACK_TOPIC,
				rc,
				statefulSet)

			return nil
		}

		stsLabels := statefulSet.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(stsLabels, rc.dseDatacenter, rackInfo.RackName)

		if shouldUpdateLabels {
			rc.reqLogger.Info("Updating labels",
				"statefulSet", statefulSet,
				"current", stsLabels,
				"desired", updatedLabels)
			statefulSet.SetLabels(updatedLabels)

			if err := rc.reconciler.client.Update(rc.ctx, statefulSet); err != nil {
				rc.reqLogger.Info("Unable to update statefulSet with labels",
					"statefulSet",
					statefulSet)
			}
		}

		desiredNodeCount := int32(rackInfo.NodeCount)

		if *statefulSet.Spec.Replicas < desiredNodeCount {
			// update it
			rc.reqLogger.Info(
				"Need to update the rack's node count",
				"Rack", rackInfo.RackName,
				"currentSize", *statefulSet.Spec.Replicas,
				"desiredSize", desiredNodeCount)

			EventBus.Publish(
				UPDATE_RACK_TOPIC,
				rc,
				statefulSet,
				desiredNodeCount)

			return nil
		}

		//
		// Has this statefulset been reconciled?
		//

		rc.reqLogger.Info(
			"StatefulSet found",
			"ResourceVersion",
			statefulSet.ResourceVersion)

		labelSeedPods(rc, statefulSet)

		readyReplicas := statefulSet.Status.ReadyReplicas

		if readyReplicas < desiredNodeCount {
			// We should do nothing but wait until all replicas are ready

			rc.reqLogger.Info(
				"Not all replicas for StatefulSet are ready.",
				"desiredCount", desiredNodeCount,
				"readyCount", readyReplicas)

			return nil
		}

		if statefulSet.Status.ReadyReplicas > desiredNodeCount {
			// too many ready replicas, how did this happen?
			rc.reqLogger.Info(
				"Too many replicas for StatefulSet are ready",
				"desiredCount", desiredNodeCount,
				"readyCount", readyReplicas)
			return nil
		}

		rc.reqLogger.Info(
			"All replicas are ready for StatefulSet for",
			"Rack",
			rackInfo.RackName)

		EventBus.Publish(
			RECONCILE_PODS_TOPIC,
			rc,
			statefulSet)
	}

	//
	// All statefulSets should be reconciled
	//

	rc.reqLogger.Info("All StatefulSets should now be reconciled.")

	return nil
}

// labelSeedsPods will iterate over all seed node pods for a datacenter and if the pod exists and is not already labeled will
// add the dse-seed=true label to the pod so that its picked up by the headless seed service
func labelSeedPods(rc *ReconciliationContext, statefulSet *appsv1.StatefulSet) {
	seeds := rc.dseDatacenter.GetSeedList()
	for _, seed := range seeds {
		podName := strings.Split(seed, ".")[0]
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: statefulSet.Namespace,
			},
		}
		err := rc.reconciler.client.Get(
			rc.ctx,
			types.NamespacedName{
				Name:      podName,
				Namespace: statefulSet.Namespace},
			pod)
		if err != nil {
			rc.reqLogger.Info("Unable to get seed pod",
				"Pod",
				podName)
			return
		}

		podLabels := pod.GetLabels()

		if _, ok := podLabels[datastaxv1alpha1.SEED_NODE_LABEL]; !ok {
			podLabels[datastaxv1alpha1.SEED_NODE_LABEL] = "true"
			pod.SetLabels(podLabels)

			if err := rc.reconciler.client.Update(rc.ctx, pod); err != nil {
				rc.reqLogger.Info("Unable to update pod with seed label",
					"Pod",
					podName)
			}
		}
	}
}

func reconcilePods(rc *ReconciliationContext, statefulSet *appsv1.StatefulSet) error {
	rc.reqLogger.Info("handler::reconcilePods")

	for i := int32(0); i < statefulSet.Status.Replicas; i++ {
		podName := fmt.Sprintf("%s-%v", statefulSet.Name, i)

		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: statefulSet.Namespace,
			},
		}
		err := rc.reconciler.client.Get(
			rc.ctx,
			types.NamespacedName{
				Name:      podName,
				Namespace: statefulSet.Namespace},
			pod)
		if err != nil {
			rc.reqLogger.Info("Unable to get pod",
				"Pod",
				podName)
			return err
		}

		podLabels := pod.GetLabels()
		shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForRackResource(podLabels, rc.dseDatacenter, statefulSet.Name)
		if shouldUpdateLabels {
			rc.reqLogger.Info("Updating labels",
				"Pod", podName,
				"current", podLabels,
				"desired", updatedLabels)
			pod.SetLabels(updatedLabels)

			if err := rc.reconciler.client.Update(rc.ctx, pod); err != nil {
				rc.reqLogger.Info("Unable to update pod with label",
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
		err = rc.reconciler.client.Get(
			rc.ctx,
			types.NamespacedName{
				Name:      pvcName,
				Namespace: statefulSet.Namespace},
			pvc)
		if err != nil {
			rc.reqLogger.Info("Unable to get pvc",
				"PVC",
				pvcName)
			return err
		}

		pvcLabels := pvc.GetLabels()
		shouldUpdateLabels, updatedLabels = shouldUpdateLabelsForRackResource(pvcLabels, rc.dseDatacenter, statefulSet.Name)
		if shouldUpdateLabels {
			rc.reqLogger.Info("Updating labels",
				"PVC", pvc,
				"current", pvcLabels,
				"desired", updatedLabels)

			pvc.SetLabels(updatedLabels)

			if err := rc.reconciler.client.Update(rc.ctx, pvc); err != nil {
				rc.reqLogger.Info("Unable to update pvc with labels",
					"PVC",
					pvc)
			}
		}
	}

	return nil
}

// Returns the statefulset for the rack
// and whether it currently exists
// and whether an error occured
func getStatefulSetForRack(
	rc *ReconciliationContext,
	service *corev1.Service,
	nextRack *RackInformation) (*appsv1.StatefulSet, bool, error) {

	rc.reqLogger.Info("handler::getStatefulSetForRack")

	desiredStatefulSet := newStatefulSetForDseDatacenter(
		nextRack.RackName,
		rc.dseDatacenter,
		service,
		nextRack.NodeCount)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err := setControllerReference(
		rc.dseDatacenter,
		desiredStatefulSet,
		rc.reconciler.scheme)
	if err != nil {
		return nil, false, err
	}

	// Check if the desiredStatefulSet already exists
	currentStatefulSet := &appsv1.StatefulSet{}
	err = rc.reconciler.client.Get(
		rc.ctx,
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
func reconcileNextRack(rc *ReconciliationContext, statefulSet *appsv1.StatefulSet) error {

	rc.reqLogger.Info("handler::reconcileNextRack")

	// Create the StatefulSet
	rc.reqLogger.Info(
		"Creating a new StatefulSet.",
		"StatefulSetNamespace",
		statefulSet.Namespace,
		"StatefulSetName",
		statefulSet.Name)
	err := rc.reconciler.client.Create(
		rc.ctx,
		statefulSet)
	if err != nil {
		return err
	}

	//
	// Create a PodDisruptionBudget for the StatefulSet
	//

	desiredBudget := newPodDisruptionBudgetForStatefulSet(
		rc.dseDatacenter,
		statefulSet)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err = setControllerReference(
		rc.dseDatacenter,
		desiredBudget,
		rc.reconciler.scheme)
	if err != nil {
		return err
	}

	// Check if the budget already exists
	currentBudget := &policyv1beta1.PodDisruptionBudget{}
	err = rc.reconciler.client.Get(
		rc.ctx,
		types.NamespacedName{
			Name:      desiredBudget.Name,
			Namespace: desiredBudget.Namespace},
		currentBudget)

	if err != nil && errors.IsNotFound(err) {
		// Create the Budget
		rc.reqLogger.Info(
			"Creating a new PodDisruptionBudget.",
			"PodDisruptionBudgetNamespace",
			desiredBudget.Namespace,
			"PodDisruptionBudgetName",
			desiredBudget.Name)
		err = rc.reconciler.client.Create(
			rc.ctx,
			desiredBudget)
		if err != nil {
			return err
		}
	}

	return nil
}

func updateRackNodeCount(rc *ReconciliationContext, statefulSet *appsv1.StatefulSet, newNodeCount int32) error {

	rc.reqLogger.Info("handler::updateRack")

	rc.reqLogger.Info(
		"updating StatefulSet node count",
		"StatefulSetNamespace", statefulSet.Namespace,
		"StatefulSetName", statefulSet.Name,
		"newNodeCount", newNodeCount,
	)

	statefulSet.Spec.Replicas = &newNodeCount

	err := rc.reconciler.client.Update(
		rc.ctx,
		statefulSet)

	return err
}

func deletePVCs(rc *ReconciliationContext) error {
	rc.reqLogger.Info("handler::deletePVCs")

	persistentVolumeClaimList, err := listPVCs(rc)
	if err != nil {
		if errors.IsNotFound(err) {
			rc.reqLogger.Info(
				"No PVCs found for DseDatacenter",
				"DseDatacenter.Namespace",
				rc.dseDatacenter.Namespace,
				"DseDatacenter.Name",
				rc.dseDatacenter.Name)
			return nil
		}
		rc.reqLogger.Error(err,
			"Failed to list PVCs for DseDatacenter",
			"DseDatacenter.Namespace",
			rc.dseDatacenter.Namespace,
			"DseDatacenter.Name",
			rc.dseDatacenter.Name)
		return err
	}

	rc.reqLogger.Info(
		fmt.Sprintf("Found %d PVCs for DseDatacenter", len(persistentVolumeClaimList.Items)),
		"DseDatacenter.Namespace",
		rc.dseDatacenter.Namespace,
		"DseDatacenter.Name",
		rc.dseDatacenter.Name)

	for _, pvc := range persistentVolumeClaimList.Items {
		if err := rc.reconciler.client.Delete(rc.ctx, &pvc); err != nil {
			rc.reqLogger.Error(err,
				"Failed to delete PVCs for DseDatacenter",
				"DseDatacenter.Namespace",
				rc.dseDatacenter.Namespace,
				"DseDatacenter.Name",
				rc.dseDatacenter.Name)
			return err
		} else {
			rc.reqLogger.Info(
				"Deleted PVC",
				"PVC.Namespace",
				pvc.Namespace,
				"PVC.Name",
				pvc.Name)
		}
	}

	return nil
}

func listPVCs(rc *ReconciliationContext) (*v1.PersistentVolumeClaimList, error) {
	rc.reqLogger.Info("handler::listPVCs")

	selector := map[string]string{
		datastaxv1alpha1.DATACENTER_LABEL: rc.dseDatacenter.Name,
	}

	listOptions := &client.ListOptions{
		Namespace:     rc.dseDatacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	persistentVolumeClaimList := &v1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
	}

	return persistentVolumeClaimList, rc.reconciler.client.List(rc.ctx, listOptions, persistentVolumeClaimList)
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
