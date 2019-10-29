package reconciliation

//
// This file defines tests for the eventbus functionality.
//

import (
	"fmt"
	"testing"

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

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	fakeClient := fake.NewFakeClient()

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
