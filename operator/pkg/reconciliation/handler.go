package reconciliation

//
// This file defines handlers for events on the EventBus
//

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
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
			RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
			rc,
			currentService)
	}

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
		"Service.Namespace",
		service.Namespace,
		"Service.Name",
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
		"Service.Namespace",
		seedService.Namespace,
		"Service.Name",
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
				"Could not locate statefulSet for ",
				"Rack: ",
				rackInfo.RackName)
			return err
		}

		if statefulSetFound == false {
			rc.reqLogger.Info(
				"Need to create new StatefulSet for ",
				"Rack: ",
				rackInfo.RackName)

			EventBus.Publish(
				RECONCILE_NEXT_RACK_TOPIC,
				rc,
				statefulSet)

			return nil
		}

		//
		// Has this statefulset been reconciled?
		//

		rc.reqLogger.Info(
			"StatefulSet found: ",
			"ResourceVersion: ",
			statefulSet.ResourceVersion)

		labelSeedPods(rc, statefulSet)

		if statefulSet.Status.ReadyReplicas < int32(rackInfo.NodeCount) {
			// We should do nothing but wait until all replicas are ready

			rc.reqLogger.Info(
				"Not all replicas for StatefulSet are ready.",
				"Desired:",
				rackInfo.NodeCount,
				"Ready:",
				statefulSet.Status.ReadyReplicas)

			return nil
		}

		rc.reqLogger.Info(
			"All replicas are ready for StatefulSet for ",
			"Rack:",
			rackInfo.RackName)
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
				"Pod:",
				podName)
			return
		}

		labels := pod.GetLabels()

		if _, ok := labels[datastaxv1alpha1.SEED_NODE_LABEL]; !ok {
			labels[datastaxv1alpha1.SEED_NODE_LABEL] = "true"
			pod.SetLabels(labels)

			if err := rc.reconciler.client.Update(rc.ctx, pod); err != nil {
				rc.reqLogger.Info("Unable to update pod with seed label",
					"Pod:",
					podName)
			}
		}
	}
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
		"StatefulSet.Namespace:",
		statefulSet.Namespace,
		"StatefulSet.Name:",
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
			"PodDisruptionBudget.Namespace:",
			desiredBudget.Namespace,
			"PodDisruptionBudget.Name:",
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
