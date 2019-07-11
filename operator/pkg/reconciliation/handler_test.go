package reconciliation

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestCalculateReconciliationActions(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)

	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.NoErrorf(t, err, "Should not have returned an error while calculating reconciliation actions")
	assert.NotNil(t, result, "Result should not be nil")

	// Add a service and check the logic

	fakeClient, _ := fakeClientWithService(rc.DseDatacenter)
	rc.Client = *fakeClient

	result, err = calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.NoErrorf(t, err, "Should not have returned an error while calculating reconciliation actions")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestCalculateReconciliationActions_GetServiceError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

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

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)

	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}

func TestCalculateReconciliationActions_FailedUpdate(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

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

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}

func TestProcessDeletion_FailedDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

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

	now := metav1.Now()
	rc.DseDatacenter.SetDeletionTimestamp(&now)

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}
