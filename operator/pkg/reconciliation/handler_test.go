package reconciliation

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestCalculateReconciliationActions(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate           = false
		calledReconcileService = false
	)

	testCreateHeadlessService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCreate = true
		return nil
	}

	testReconcileHeadlessService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileService = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SERVICE_TOPIC, testReconcileHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILIATION_REQUEST_TOPIC,
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileService, "Should call correct handler.")
	assert.True(t, calledCreate, "Should call correct handler.")

	// Add a service and check the logic

	fakeClient, _ := fakeClientWithService(rc.dseDatacenter)
	rc.reconciler.client = *fakeClient

	calledCreate = false
	calledReconcileService = false

	EventBus.Publish(
		RECONCILIATION_REQUEST_TOPIC,
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCreate, "Should call correct handler.")
	assert.True(t, calledReconcileService, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SERVICE_TOPIC, testReconcileHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCalculateReconciliationActions_GetServiceError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate               = false
		calledReconcileSeedService = false
	)

	testCreateHeadlessService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCreate = true
		return nil
	}

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileSeedService = true
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

	mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILIATION_REQUEST_TOPIC,
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileSeedService, "Should call correct handler.")
	assert.False(t, calledCreate, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestCalculateReconciliationActions_FailedUpdate(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate               = false
		calledReconcileSeedService = false
	)

	testCreateHeadlessService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledCreate = true
		return nil
	}

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileSeedService = true
		return nil
	}

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(fmt.Errorf("failed to update DseDatacenter with removed finalizers")).
		Once()

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILIATION_REQUEST_TOPIC,
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileSeedService, "Should call correct handler.")
	assert.False(t, calledCreate, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SERVICE_TOPIC, testCreateHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestProcessDeletion_FailedDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(nil).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(fmt.Errorf("")).
		Once()

	err := EventBus.SubscribeAsync(PROCESS_DELETION_TOPIC, processDeletion, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		PROCESS_DELETION_TOPIC,
		rc)

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(PROCESS_DELETION_TOPIC, processDeletion)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestReconcileHeadlessService(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileService = false
	)

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileService = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILE_HEADLESS_SERVICE_TOPIC, reconcileHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_HEADLESS_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcileService, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SERVICE_TOPIC, calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileHeadlessService_UpdateLabels(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileService = false
	)

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileService = true
		return nil
	}

	service.SetLabels(make(map[string]string))

	err := EventBus.SubscribeAsync(RECONCILE_HEADLESS_SERVICE_TOPIC, reconcileHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_HEADLESS_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcileService, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SERVICE_TOPIC, calculateReconciliationActions)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCreateHeadlessService(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileSeedService = false
	)

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileSeedService = true
		return nil
	}

	err := EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, createHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CREATE_HEADLESS_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcileSeedService, "Should call correct handler.")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SERVICE_TOPIC, createHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCreateHeadlessService_ClientReturnsError(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcileSeedService = false
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

	testReconcileHeadlessSeedService := func(
		rc *ReconciliationContext,
		service *corev1.Service) error {
		calledReconcileSeedService = true
		return nil
	}

	err := EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, createHeadlessService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CREATE_HEADLESS_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileSeedService, "Should call correct handler.")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SERVICE_TOPIC, createHeadlessService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, testReconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestReconcileHeadlessSeedService(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate    = false
		calledCalculate = false
	)

	testCreateHeadlessSeedService := func(
		rc *ReconciliationContext,
		seedService *corev1.Service,
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

	err := EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")
	assert.True(t, calledCreate, "Should call correct handler.")

	// Add a service and check the logic

	fakeClient, _ := fakeClientWithSeedService(rc.dseDatacenter)
	rc.reconciler.client = *fakeClient

	calledCreate = false
	calledCalculate = false

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCreate, "Should call correct handler.")
	assert.True(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileHeadlessSeedService_GetServiceError(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate    = false
		calledCalculate = false
	)

	testCreateHeadlessSeedService := func(
		rc *ReconciliationContext,
		seedService *corev1.Service,
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

	err := EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")
	assert.False(t, calledCreate, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestReconcileHeadlessSeedService_UpdateLabels(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledCreate    = false
		calledCalculate = false
	)

	testCreateHeadlessSeedService := func(
		rc *ReconciliationContext,
		seedService *corev1.Service,
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
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*corev1.Service)
			arg.SetLabels(make(map[string]string))
		}).Return(nil).Once()

	mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).Return(nil).Once()

	err := EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledCalculate, "Should call correct handler.")
	assert.False(t, calledCreate, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SEED_SERVICE_TOPIC, testCreateHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestCreateHeadlessSeedService(t *testing.T) {
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

	err := EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, createHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CREATE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service,
		newServiceForDseDatacenter(rc.dseDatacenter))

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SEED_SERVICE_TOPIC, createHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestCreateHeadlessSeedService_ClientReturnsError(t *testing.T) {
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

	err := EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, createHeadlessSeedService, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CREATE_HEADLESS_SEED_SERVICE_TOPIC,
		rc,
		service,
		newServiceForDseDatacenter(rc.dseDatacenter))

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledCalculate, "Should call correct handler.")

	err = EventBus.Unsubscribe(CREATE_HEADLESS_SEED_SERVICE_TOPIC, createHeadlessSeedService)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(CALCULATE_RACK_INFORMATION_TOPIC, testCalculateRackInformation)
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

	err := EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CALCULATE_RACK_INFORMATION_TOPIC,
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

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, testReconcileRacks, true)
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

	err := EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, testReconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		CALCULATE_RACK_INFORMATION_TOPIC,
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

	err = EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, calculateRackInformation, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, testReconcileRacks, true)
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

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack)
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

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack)
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

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_NeedMoreReplicas(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	var (
		calledUpdateRackNodeCount = false
	)

	testUpdateRackNodeCount := func(
		rc *ReconciliationContext,
		statefulSet *appsv1.StatefulSet,
		newNodeCount int32) error {
		calledUpdateRackNodeCount = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(UPDATE_RACK_TOPIC, testUpdateRackNodeCount, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 3

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledUpdateRackNodeCount, "Should add more replicas to the statefulset")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(UPDATE_RACK_TOPIC, testUpdateRackNodeCount)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_DoesntScaleDown(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	var (
		calledUpdateRackNodeCount = false
	)

	testUpdateRackNodeCount := func(
		rc *ReconciliationContext,
		statefulSet *appsv1.StatefulSet,
		newNodeCount int32) error {
		calledUpdateRackNodeCount = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(UPDATE_RACK_TOPIC, testUpdateRackNodeCount, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledUpdateRackNodeCount, "Should not scale down the node count, outside of parking")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(UPDATE_RACK_TOPIC, testUpdateRackNodeCount)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_NeedToPark(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		3)

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	var (
		calledUpdateRackNodeCount = false
	)

	testUpdateRackNodeCount := func(
		rc *ReconciliationContext,
		statefulSet *appsv1.StatefulSet,
		newNodeCount int32) error {
		calledUpdateRackNodeCount = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(UPDATE_RACK_TOPIC, testUpdateRackNodeCount, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 0

	rc.dseDatacenter.Spec.Parked = true

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledUpdateRackNodeCount, "Should set statefulset replica count to zero")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(UPDATE_RACK_TOPIC, testUpdateRackNodeCount)
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

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_UpdateLabels(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	desiredStatefulSet.Status.ReadyReplicas = 2
	desiredStatefulSet.SetLabels(make(map[string]string))

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

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.False(t, calledReconcileNextRack, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, testReconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcileRacks_ReconcilePods(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		calledReconcilePods = false
		one                 = int32(1)
	)

	desiredStatefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)

	desiredStatefulSet.Spec.Replicas = &one
	desiredStatefulSet.Status.ReadyReplicas = one

	trackObjects := []runtime.Object{
		desiredStatefulSet,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	testReconcilePods := func(
		rc *ReconciliationContext,
		statefulSet *appsv1.StatefulSet) error {
		calledReconcilePods = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	err = EventBus.SubscribeAsync(RECONCILE_PODS_TOPIC, testReconcilePods, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	EventBus.Publish(
		RECONCILE_RACKS_TOPIC,
		rc,
		service,
		rackInfo)

	// wait for events to be handled
	EventBus.WaitAsync()

	assert.True(t, calledReconcilePods, "Should call correct handler.")

	err = EventBus.Unsubscribe(RECONCILE_RACKS_TOPIC, reconcileRacks)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	err = EventBus.Unsubscribe(RECONCILE_PODS_TOPIC, testReconcilePods)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcilePods(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)
	statefulSet.Status.ReadyReplicas = int32(1)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dsedatacenter-example-cluster-dsedatacenter-example-default-sts-0",
			Namespace: statefulSet.Namespace,
		},
	}

	trackObjects := []runtime.Object{
		pod,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	err := EventBus.SubscribeAsync(RECONCILE_PODS_TOPIC, reconcilePods, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_PODS_TOPIC,
		rc,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILE_PODS_TOPIC, reconcilePods)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")
}

func TestReconcilePods_WithVolumes(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet := newStatefulSetForDseDatacenter(
		"default",
		rc.dseDatacenter,
		service,
		2)
	statefulSet.Status.ReadyReplicas = int32(1)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dsedatacenter-example-cluster-dsedatacenter-example-default-sts-0",
			Namespace: statefulSet.Namespace,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{{
				Name: "dse-data",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "dse-data-example-cluster1-example-dsedatacenter1-rack0-sts-0",
					},
				},
			}},
		},
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dse-data",
			Namespace: statefulSet.Namespace,
		},
	}

	trackObjects := []runtime.Object{
		pod,
		pvc,
	}

	rc.reconciler.client = fake.NewFakeClient(trackObjects...)

	err := EventBus.SubscribeAsync(RECONCILE_PODS_TOPIC, reconcilePods, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_PODS_TOPIC,
		rc,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILE_PODS_TOPIC, reconcilePods)
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

	err := EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, reconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_NEXT_RACK_TOPIC,
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

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, reconcileNextRack)
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

	err := EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, reconcileNextRack, true)
	assert.NoErrorf(t, err, "error occurred subscribing to eventbus")

	EventBus.Publish(
		RECONCILE_NEXT_RACK_TOPIC,
		rc,
		statefulSet)

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILE_NEXT_RACK_TOPIC, reconcileNextRack)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	mockClient.AssertExpectations(t)
}

func TestDeletePVCs(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(nil).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	if err := deletePVCs(rc); err != nil {
		t.Errorf("error occurred deleting PVC: %v", err)
	}
}

func TestDeletePVCs_FailedToList(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(fmt.Errorf("failed to list PVCs for DseDatacenter")).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	err := deletePVCs(rc)
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

	assert.EqualError(t, err, "failed to list PVCs for DseDatacenter")
}

func TestDeletePVCs_PVCsNotFound(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(errors.NewNotFound(schema.GroupResource{}, "name")).
		Once()

	err := deletePVCs(rc)
	if err != nil {
		t.Fatalf("deletePVCs should not have failed")
	}
}

func TestDeletePVCs_FailedToDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.reconciler.client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(nil).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(fmt.Errorf("failed to delete")).
		Once()

	err := deletePVCs(rc)
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

	assert.EqualError(t, err, "failed to delete")
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

func Test_updateRackNodeCount(t *testing.T) {
	type args struct {
		rc           *ReconciliationContext
		statefulSet  *appsv1.StatefulSet
		newNodeCount int32
	}

	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		nextRack = &RackInformation{}
	)

	nextRack.RackName = "default"
	nextRack.NodeCount = 2

	statefulSet, _, _ := getStatefulSetForRack(
		rc,
		service,
		nextRack)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "check that replicas get increased",
			args: args{
				rc:           rc,
				statefulSet:  statefulSet,
				newNodeCount: 3,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackObjects := []runtime.Object{
				tt.args.statefulSet,
			}

			rc.reconciler.client = fake.NewFakeClient(trackObjects...)
			if err := updateRackNodeCount(tt.args.rc, tt.args.statefulSet, tt.args.newNodeCount); (err != nil) != tt.wantErr {
				t.Errorf("updateRackNodeCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.args.newNodeCount != *tt.args.statefulSet.Spec.Replicas {
				t.Errorf("StatefulSet spec should have different replica count, has = %v, want %v", *tt.args.statefulSet.Spec.Replicas, tt.args.newNodeCount)
			}
		})
	}
}

func Test_validateLabelsForCluster(t *testing.T) {
	type args struct {
		resourceLabels map[string]string
		rc             *ReconciliationContext
	}
	tests := []struct {
		name       string
		args       args
		want       bool
		wantLabels map[string]string
	}{
		{
			name: "No labels",
			args: args{
				resourceLabels: make(map[string]string),
				rc: &ReconciliationContext{
					dseDatacenter: &v1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: v1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
			},
		}, {
			name: "Nil labels",
			args: args{
				resourceLabels: nil,
				rc: &ReconciliationContext{
					dseDatacenter: &v1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: v1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
			},
		},
		{
			name: "Has Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
				},
				rc: &ReconciliationContext{
					dseDatacenter: &v1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: v1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: false,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
			},
		}, {
			name: "DC Label, No Cluster Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.DATACENTER_LABEL: "dseDC",
				},
				rc: &ReconciliationContext{
					dseDatacenter: &v1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: v1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.DATACENTER_LABEL: "dseDC",
				datastaxv1alpha1.CLUSTER_LABEL:    "dseCluster",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := shouldUpdateLabelsForClusterResource(tt.args.resourceLabels, tt.args.rc.dseDatacenter)
			if got != tt.want {
				t.Errorf("shouldUpdateLabelsForClusterResource() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.wantLabels) {
				t.Errorf("shouldUpdateLabelsForClusterResource() got1 = %v, want %v", got1, tt.wantLabels)
			}
		})
	}
}
