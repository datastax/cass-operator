package reconciliation

//
// This file defines tests for the eventbus functionality.
//

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		name                = "cluster-example-cluster.dc-example-dsedatacenter"
		namespace           = "default"
		size          int32 = 2
		handlerCalled       = false
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

	testHandleReconciliationRequest := func(rc *ReconciliationContext) error {
		handlerCalled = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest, true)
	if err != nil {
		t.Errorf("error occurred subscribing to eventbus: %v", err)
	}

	result, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconciliation Failure: (%v)", err)
	}

	if result != (reconcile.Result{}) {
		t.Error("Reconcile did not return an empty result.")
	}

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	assert.True(t, handlerCalled, "Reconcile should have called the handler.")
}

func TestReconcile_NotFound(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name                = "dsedatacenter-example"
		namespace           = "default"
		size          int32 = 2
		handlerCalled       = false
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

	testHandleReconciliationRequest := func(rc *ReconciliationContext) error {
		handlerCalled = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest, true)
	if err != nil {
		t.Errorf("error occurred subscribing to eventbus: %v", err)
	}

	result, err := r.Reconcile(request)
	if err == nil {
		t.Fatalf("Reconciliation should have failed")
	}

	assert.EqualError(t, err, "DseDatacenter object not found")

	if result != (reconcile.Result{}) {
		t.Error("Reconcile did not return an empty result.")
	}

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	assert.False(t, handlerCalled, "Reconcile should not have called the handler.")
}

func TestReconcile_Error(t *testing.T) {
	// Set up verbose logging
	logger := logf.ZapLogger(true)
	logf.SetLogger(logger)

	var (
		name                = "dsedatacenter-example"
		namespace           = "default"
		size          int32 = 2
		handlerCalled       = false
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

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	mockClient := mocks.Client{}
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

	r := &ReconcileDseDatacenter{
		client: &mockClient,
		scheme: s,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	testHandleReconciliationRequest := func(rc *ReconciliationContext) error {
		handlerCalled = true
		return nil
	}

	err := EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest, true)
	if err != nil {
		t.Errorf("error occurred subscribing to eventbus: %v", err)
	}

	result, err := r.Reconcile(request)
	if err == nil {
		t.Fatalf("Reconciliation should have failed")
	}

	assert.EqualError(t, err, "error reading DseDatacenter object")

	if result != (reconcile.Result{}) {
		t.Error("Reconcile did not return an empty result.")
	}

	// wait for events to be handled
	EventBus.WaitAsync()

	err = EventBus.Unsubscribe(RECONCILIATION_REQUEST_TOPIC, testHandleReconciliationRequest)
	assert.NoErrorf(t, err, "error occurred unsubscribing to eventbus")

	assert.False(t, handlerCalled, "Reconcile should not have called the handler.")
}
