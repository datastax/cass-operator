package reconciliation

//
// This file defines helpers for unit testing.
//

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
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
	reqLogger logr.Logger) *ReconciliationContext {

	// These defaults may need to be settable via arguments

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

	reconciler := &ReconcileDseDatacenter{
		client: fakeClient,
		scheme: s,
	}

	request := &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	rc := &ReconciliationContext{}
	rc.request = request
	rc.reconciler = reconciler
	rc.reqLogger = reqLogger
	rc.dseDatacenter = dseDatacenter
	rc.ctx = context.Background()

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

func fakeClientWithSeedService(
	dseDatacenter *datastaxv1alpha1.DseDatacenter) (*client.Client, *corev1.Service) {

	service := newSeedServiceForDseDatacenter(dseDatacenter)

	// Objects to keep track of

	trackObjects := []runtime.Object{
		dseDatacenter,
		service,
	}

	fakeClient := fake.NewFakeClient(trackObjects...)

	return &fakeClient, service
}
