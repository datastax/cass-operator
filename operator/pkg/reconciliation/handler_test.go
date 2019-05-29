package reconciliation

//
// This file defines tests for the handlers for events on the EventBus
//

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"testing"
)

func TestCalculateReconciliationActions(t *testing.T) {
	// Set up verbose logging
	//logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	cleanupMockScr := MockSetControllerReference()
	defer cleanupMockScr()

	rc := CreateMockReconciliationContext(logger)

	var (
		calledCreate    = false
		calledCalculate = false
	)

	testCreateHeadlessService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCreate = true
		return nil
	}

	testCalculateRackInformation := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCalculate = true
		return nil
	}

	EventBus.SubscribeAsync("ReconciliationRequest", calculateReconciliationActions, true)
	EventBus.SubscribeAsync("CreateHeadlessService", testCreateHeadlessService, true)
	EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)

	EventBus.Publish(
		"ReconciliationRequest",
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	if calledCreate == false {
		t.Error("Did not call the correct handler.")
	}

	if calledCalculate == true {
		t.Error("Called incorrect handler.")
	}

	// Add a service and check the logic

	fakeClient, _ := fakeClientWithService(rc.dseDatacenter)
	rc.reconciler.client = *fakeClient

	calledCreate = false
	calledCalculate = false

	EventBus.Publish(
		"ReconciliationRequest",
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	if calledCreate == true {
		t.Error("Called incorrect handler.")
	}

	if calledCalculate == false {
		t.Error("Did not call the correct handler.")
	}

	EventBus.Unsubscribe("ReconciliationRequest", calculateReconciliationActions)
	EventBus.Unsubscribe("CreateHeadlessService", testCreateHeadlessService)
	EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
}

func TestCreateHeadlessService(t *testing.T) {
	// Set up verbose logging
	//logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	cleanupMockScr := MockSetControllerReference()
	defer cleanupMockScr()

	rc := CreateMockReconciliationContext(logger)

	service := newServiceForDseDatacenter(rc.dseDatacenter)

	var (
		calledCalculate = false
	)

	testCalculateRackInformation := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCalculate = true
		return nil
	}

	EventBus.SubscribeAsync("CreateHeadlessService", createHeadlessService, true)
	EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)

	EventBus.Publish(
		"CreateHeadlessService",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	if calledCalculate == false {
		t.Error("Did not call the correct handler.")
	}

	EventBus.Unsubscribe("CreateHeadlessService", createHeadlessService)
	EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
}

func TestCalculateRackInformation(t *testing.T) {
	// Set up verbose logging
	//logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	cleanupMockScr := MockSetControllerReference()
	defer cleanupMockScr()

	rc := CreateMockReconciliationContext(logger)

	service := newServiceForDseDatacenter(rc.dseDatacenter)

	var (
		calledReconcile                       = false
		rackInfoToValidate []*RackInformation = nil
	)

	testReconcileRacks := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		desiredRackInformation []*RackInformation) error {
		calledReconcile = true

		rackInfoToValidate = desiredRackInformation
		return nil
	}

	EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)

	EventBus.Publish(
		"CalculateRackInformation",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	if calledReconcile == false {
		t.Error("Did not call the correct handler.")
	}

	rackInfo := rackInfoToValidate[0]

	if rackInfo.RackName != "default" {
		t.Error("Rack name not equal to default")
	}

	rc.reqLogger.Info(
		"Node count is ",
		"Node Count: ",
		rackInfo.NodeCount)

	if rackInfo.NodeCount != 2 {
		t.Error("Node count incorrect")
	}

	// TODO add more RackInformation validation

	EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)
}

func TestReconcileRacks(t *testing.T) {
	// Set up verbose logging
	//logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	cleanupMockScr := MockSetControllerReference()
	defer cleanupMockScr()

	rc := CreateMockReconciliationContext(logger)

	service := newServiceForDseDatacenter(rc.dseDatacenter)

	var (
		calledReconcile = false
	)

	testReconcileNextRack := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcile = true
		return nil
	}

	EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	EventBus.SubscribeAsync("ReconcileNextRack", testReconcileNextRack, true)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		"ReconcileRacks",
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	if calledReconcile == false {
		t.Error("Did not call the correct handler.")
	}

	EventBus.Unsubscribe("ReconcileRacks", reconcileRacks)
	EventBus.Unsubscribe("ReconcileNextRack", testReconcileNextRack)
}

// Note: getStatefulSetForRack is currently just a query,
// and there is really no logic to test.
// We can add a unit test later, if needed.

func TestReconcileNextRack(t *testing.T) {
	// Set up verbose logging
	//logf.SetLogger(logf.ZapLogger(true))
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	cleanupMockScr := MockSetControllerReference()
	defer cleanupMockScr()

	rc := CreateMockReconciliationContext(logger)

	service := newServiceForDseDatacenter(rc.dseDatacenter)

	var (
		nextRack = &RackInformation{}
	)

	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	statefulSet, _, _ := getStatefulSetForRack(
		rc,
		service,
		nextRack)

	EventBus.SubscribeAsync("ReconcileNextRack", reconcileNextRack, true)

	EventBus.Publish(
		"ReconcileNextRack",
		rc,
		service,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	// Validation:
	// Currently reconcileNextRack does two things
	// 1. Creates the given StatefulSet in k8s.
	// 2. Creates a PodDisruptionBudget for the StatefulSet.
	//
	// TODO: check if Create() has been called on the fake client

	EventBus.Unsubscribe("ReconcileNextRack", reconcileNextRack)
}
