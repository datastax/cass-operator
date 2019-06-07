package reconciliation

//
// This file defines tests for the handlers for events on the EventBus
//

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestCalculateReconciliationActions(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

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

	err := EventBus.SubscribeAsync("ReconciliationRequest", calculateReconciliationActions, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CreateHeadlessService", testCreateHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"ReconciliationRequest",
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")
	assert.True(t, calledCreate, "Should call correct handler.")

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

	assert.False(t, calledCreate, "Should call correct handler.")
	assert.True(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconciliationRequest", calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CreateHeadlessService", testCreateHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCalculateReconciliationActions_GetServiceError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

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

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Get",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(key client.ObjectKey) bool {
				return key != client.ObjectKey{}
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).Return(fmt.Errorf("")).Once()

	err := EventBus.SubscribeAsync("ReconciliationRequest", calculateReconciliationActions, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CreateHeadlessService", testCreateHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"ReconciliationRequest",
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")
	assert.False(t, calledCreate, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconciliationRequest", calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CreateHeadlessService", testCreateHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestCreateHeadlessService(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCalculate = false
	)

	testCalculateRackInformation := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCalculate = true
		return nil
	}

	err := EventBus.SubscribeAsync("CreateHeadlessService", createHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"CreateHeadlessService",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe("CreateHeadlessService", createHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCreateHeadlessService_ClientReturnsError(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCalculate = false
	)

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Create",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).Return(fmt.Errorf("")).Once()

	testCalculateRackInformation := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCalculate = true
		return nil
	}

	err := EventBus.SubscribeAsync("CreateHeadlessService", createHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("CalculateRackInformation", testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"CreateHeadlessService",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe("CreateHeadlessService", createHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("CalculateRackInformation", testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestCalculateRackInformation(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

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

	err := EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"CalculateRackInformation",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcile, "Should call correct handler.")

	rackInfo := rackInfoToValidate[0]

	assert.Equal(t, "default", rackInfo.RackName, "Should have correct rack name")

	rc.reqLogger.Info(
		"Node count is ",
		"Node Count: ",
		rackInfo.NodeCount)

	assert.Equal(t, 2, rackInfo.NodeCount, "Should have correct node count")

	// TODO add more RackInformation validation

	err = EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")
}

func TestCalculateRackInformation_MultiRack(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	rc.dseDatacenter.Spec.Racks = []v1alpha1.DseRack{{
		Name: "rack0",
	}, {
		Name: "rack1",
	}, {
		Name: "rack2",
	}}

	rc.dseDatacenter.Spec.Size = 3

	var (
		calledReconcile    = false
		rackInfoToValidate = []*RackInformation{{
			RackName:  "rack0",
			NodeCount: 1,
		}, {
			RackName:  "rack1",
			NodeCount: 1,
		}, {
			RackName:  "rack2",
			NodeCount: 1,
		}}
	)

	testReconcileRacks := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		desiredRackInformation []*RackInformation) error {
		calledReconcile = true

		rackInfoToValidate = desiredRackInformation
		return nil
	}

	err := EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"CalculateRackInformation",
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcile, "Should call correct handler.")

	rackInfo := rackInfoToValidate[0]

	assert.Equal(t, "rack0", rackInfo.RackName, "Should have correct rack name")

	rc.reqLogger.Info(
		"Node count is ",
		"Node Count: ",
		rackInfo.NodeCount)

	assert.Equal(t, 1, rackInfo.NodeCount, "Should have correct node count")

	// TODO add more RackInformation validation

	err = EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileRacks", testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")
}

func TestReconcileRacks(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileNextRack = false
	)

	testReconcileNextRack := func(
		rc *ReconciliationContext,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcileNextRack = true
		return nil
	}

	err := EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileNextRack", testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

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

	assert.True(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconcileRacks", reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("ReconcileNextRack", testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_GetStatefulsetError(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileNextRack = false
	)

	testReconcileNextRack := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcileNextRack = true
		return nil
	}

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Get",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(key client.ObjectKey) bool {
				return key != client.ObjectKey{}
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).Return(fmt.Errorf("")).Once()

	err := EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileNextRack", testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

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

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconcileRacks", reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("ReconcileNextRack", testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestReconcileRacks_WaitingForReplicas(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	trackObjects := []runtime.Object{
		desiredStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	var (
		calledReconcileNextRack = false
	)

	testReconcileNextRack := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcileNextRack = true
		return nil
	}

	err := EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileNextRack", testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

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

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconcileRacks", reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("ReconcileNextRack", testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_AlreadyReconciled(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	desiredStatefulSet.Status.ReadyReplicas = 2

	trackObjects := []runtime.Object{
		desiredStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	var (
		calledReconcileNextRack = false
	)

	testReconcileNextRack := func(
		rc *ReconciliationContext,
		service *corev1.Service,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcileNextRack = true
		return nil
	}

	err := EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync("ReconcileNextRack", testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

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

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe("ReconcileRacks", reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe("ReconcileNextRack", testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

// Note: getStatefulSetForRack is currently just a query,
// and there is really no logic to test.
// We can add a unit test later, if needed.

func TestReconcileNextRack(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		nextRack = &RackInformation{}
	)

	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	statefulSet, _, _ := getStatefulSetForRack(
		rc,
		service,
		nextRack)

	err := EventBus.SubscribeAsync("ReconcileNextRack", reconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"ReconcileNextRack",
		rc,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	// Validation:
	// Currently reconcileNextRack does two things
	// 1. Creates the given StatefulSet in k8s.
	// 2. Creates a PodDisruptionBudget for the StatefulSet.
	//
	// TODO: check if Create() has been called on the fake client

	err = EventBus.Unsubscribe("ReconcileNextRack", reconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileNextRack_CreateError(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		nextRack = &RackInformation{}
	)

	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	statefulSet, _, _ := getStatefulSetForRack(
		rc,
		service,
		nextRack)

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Create",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).Return(fmt.Errorf("")).Once()

	err := EventBus.SubscribeAsync("ReconcileNextRack", reconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		"ReconcileNextRack",
		rc,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe("ReconcileNextRack", reconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func setupTest() (*ReconciliationContext, *corev1.Service, func()) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)
	cleanupMockScr := MockSetControllerReference()

	rc := CreateMockReconciliationContext(logger)
	service := newServiceForDseDatacenter(rc.dseDatacenter)

	return rc, service, cleanupMockScr
}
