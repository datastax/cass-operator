package reconciliation

//
// This file defines helpers for unit testing.
//

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	log2 "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

// MockSetControllerReference returns a method that will automatically reverse the mock
func MockSetControllerReference() func() {
	oldSetControllerReference := setControllerReference
	setControllerReference = func(
		owner,
		object metav1.Object,
		scheme *runtime.Scheme) error {
		return nil
	}

	return func() {
		setControllerReference = oldSetControllerReference
	}
}

func CreateMockReconciliationContext(
	reqLogger logr.Logger) *dsereconciliation.ReconciliationContext {

	// These defaults may need to be settable via arguments

	var (
		name              = "dsedatacenter-example"
		clusterName       = "dsedatacenter-example-cluster"
		namespace         = "default"
		size        int32 = 2
	)

	// Instance a dseDatacenter

	dseDatacenter := &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			Size:        size,
			ClusterName: clusterName,
		},
	}

	// Objects to keep track of

	trackObjects := []runtime.Object{
		dseDatacenter,
	}

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	fakeClient := fake.NewFakeClient(trackObjects...)

	request := &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	rc := &dsereconciliation.ReconciliationContext{}
	rc.Request = request
	rc.Client = fakeClient
	rc.Scheme = s
	rc.ReqLogger = reqLogger
	rc.DseDatacenter = dseDatacenter
	rc.Ctx = context.Background()

	return rc
}

// Create a fake client that is tracking a service
func fakeClientWithService(
	dseDatacenter *datastaxv1alpha1.DseDatacenter) (*client.Client, *corev1.Service) {

	service := newServiceForDseDatacenter(dseDatacenter)

	// Objects to keep track of

	trackObjects := []runtime.Object{
		dseDatacenter,
		service,
	}

	fakeClient := fake.NewFakeClient(trackObjects...)

	return &fakeClient, service
}

func setupTest() (*dsereconciliation.ReconciliationContext, *corev1.Service, func()) {
	// Set up verbose logging
	logger := log2.ZapLogger(true)
	log2.SetLogger(logger)
	cleanupMockScr := MockSetControllerReference()

	rc := CreateMockReconciliationContext(logger)
	service := newServiceForDseDatacenter(rc.DseDatacenter)

	return rc, service, cleanupMockScr
}

func getReconcilers(rc *dsereconciliation.ReconciliationContext) (ReconcileDatacenter, ReconcileRacks, ReconcileServices, ReconcileSeedServices) {
	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
	}

	reconcileSeedServices := ReconcileSeedServices{
		ReconcileContext: rc,
	}

	return reconcileDatacenter, reconcileRacks, reconcileServices, reconcileSeedServices
}

func k8sMockClientGet(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Get",
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
		Return(returnArg).
		Once()
}

func k8sMockClientUpdate(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientCreate(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Create",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientDelete(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientList(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("List",
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
		Return(returnArg).
		Once()
}
