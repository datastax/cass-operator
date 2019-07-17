package reconciliation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil)

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)

	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}

func TestCalculateReconciliationActions_FailedUpdate(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientUpdate(mockClient, fmt.Errorf("failed to update DseDatacenter with removed finalizers"))

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}

func TestProcessDeletion_FailedDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientList(mockClient, nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	k8sMockClientDelete(mockClient, fmt.Errorf(""))

	now := metav1.Now()
	rc.DseDatacenter.SetDeletionTimestamp(&now)

	datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, reconcileSeedServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}
