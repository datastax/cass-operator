package reconciliation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestCalculateReconciliationActions(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	datacenterReconcile, reconcileRacks, reconcileServices := getReconcilers(rc)

	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.NoErrorf(t, err, "Should not have returned an error while calculating reconciliation actions")
	assert.NotNil(t, result, "Result should not be nil")

	// Add a service and check the logic

	fakeClient, _ := fakeClientWithService(rc.DseDatacenter)
	rc.Client = *fakeClient

	result, err = calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.NoErrorf(t, err, "Should not have returned an error while calculating reconciliation actions")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestCalculateReconciliationActions_GetServiceError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil).Times(1)

	datacenterReconcile, reconcileRacks, reconcileServices := getReconcilers(rc)

	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, &ReconcileDseDatacenter{client: rc.Client})
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

	datacenterReconcile, reconcileRacks, reconcileServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, &ReconcileDseDatacenter{client: rc.Client})
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
			arg := args.Get(1).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	k8sMockClientDelete(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil).Times(1)

	now := metav1.Now()
	rc.DseDatacenter.SetDeletionTimestamp(&now)

	datacenterReconcile, reconcileRacks, reconcileServices := getReconcilers(rc)
	result, err := calculateReconciliationActions(rc, datacenterReconcile, reconcileRacks, reconcileServices, &ReconcileDseDatacenter{client: rc.Client})
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
}

func TestReconcile(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name            = "cluster-example-cluster.dc-example-dsedatacenter"
		namespace       = "default"
		size      int32 = 2
	)

	// Instance a dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			ManagementApiAuth: datastaxv1alpha1.ManagementApiAuthConfig{
				Insecure: &datastaxv1alpha1.ManagementApiAuthInsecureConfig{},
			},
			Size: size,
		},
	}

	// Objects to keep track of
	trackObjects := []runtime.Object{
		dseDatacenter,
	}

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	fakeClient := fake.NewFakeClient(trackObjects...)

	r := &ReconcileDseDatacenter{
		client: fakeClient,
		scheme: s,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	result, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconciliation Failure: (%v)", err)
	}

	if result != (reconcile.Result{Requeue: true}) {
		t.Error("Reconcile did not return a correct result.")
	}
}

func TestReconcile_NotFound(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name            = "dsedatacenter-example"
		namespace       = "default"
		size      int32 = 2
	)

	// Instance a dseDatacenter

	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			ManagementApiAuth: datastaxv1alpha1.ManagementApiAuthConfig{
				Insecure: &datastaxv1alpha1.ManagementApiAuthInsecureConfig{},
			},
			Size: size,
		},
	}

	// Objects to keep track of
	trackObjects := []runtime.Object{}

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	fakeClient := fake.NewFakeClient(trackObjects...)

	r := &ReconcileDseDatacenter{
		client: fakeClient,
		scheme: s,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	result, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconciliation Failure: (%v)", err)
	}

	expected := reconcile.Result{}
	if result != expected {
		t.Error("expected to get a zero-value reconcile.Result")
	}
}

func TestReconcile_Error(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name            = "dsedatacenter-example"
		namespace       = "default"
		size      int32 = 2
	)

	// Instance a dseDatacenter

	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			ManagementApiAuth: datastaxv1alpha1.ManagementApiAuthConfig{
				Insecure: &datastaxv1alpha1.ManagementApiAuthInsecureConfig{},
			},
			Size: size,
		},
	}

	// Objects to keep track of

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	mockClient := &mocks.Client{}
	k8sMockClientGet(mockClient, fmt.Errorf(""))

	r := &ReconcileDseDatacenter{
		client: mockClient,
		scheme: s,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	result, err := r.Reconcile(request)
	if err == nil {
		t.Fatalf("Reconciliation should have failed")
	}

	if result != (reconcile.Result{Requeue: true}) {
		t.Error("Reconcile did not return an empty result.")
	}
}

func TestReconcile_DseDatacenterToBeDeleted(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name            = "dsedatacenter-example"
		namespace       = "default"
		size      int32 = 2
	)

	// Instance a dseDatacenter
	now := metav1.Now()
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			DeletionTimestamp: &now,
			Finalizers:        nil,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			ManagementApiAuth: datastaxv1alpha1.ManagementApiAuthConfig{
				Insecure: &datastaxv1alpha1.ManagementApiAuthInsecureConfig{},
			},
			Size: size,
		},
	}

	// Objects to keep track of
	trackObjects := []runtime.Object{
		dseDatacenter,
	}

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	fakeClient := fake.NewFakeClient(trackObjects...)

	r := &ReconcileDseDatacenter{
		client: fakeClient,
		scheme: s,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	result, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconciliation Failure: (%v)", err)
	}

	if result != (reconcile.Result{}) {
		t.Error("Reconcile did not return an empty result.")
	}
}
