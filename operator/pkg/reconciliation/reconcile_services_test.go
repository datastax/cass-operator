package reconciliation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestReconcileHeadlessService(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
	}
	rec, err := reconcileServices.ReconcileHeadlessServices()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.NotNil(t, rec, "Reconciler should not be nil")
}

func TestReconcileHeadlessService_UpdateLabels(t *testing.T) {
	rc, service, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*corev1.Service)
			arg.SetLabels(make(map[string]string))
		}).
		Return(nil).
		Times(3)
	k8sMockClientUpdate(mockClient, nil).
		Times(3)

	service.SetLabels(make(map[string]string))

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
	}
	rec, err := reconcileServices.ReconcileHeadlessServices()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Nil(t, rec, "Reconciler should be nil")
}

func TestCreateHeadlessService(t *testing.T) {
	rc, svc, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
		Services:         []*corev1.Service{svc},
	}
	result, err := reconcileServices.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestCreateHeadlessService_ClientReturnsError(t *testing.T) {
	t.Skip()
	rc, svc, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientCreate(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil).Times(1)

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
		Services:         []*corev1.Service{svc},
	}
	result, err := reconcileServices.Apply()
	assert.Errorf(t, err, "Should have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}
