package reconciliation

//
// This file defines tests for the eventbus functionality.
//

import (
	//"context"

	//"math/rand"
	//"reflect"
	//"strconv"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	//appsv1 "k8s.io/api/apps/v1"
	//corev1 "k8s.io/api/core/v1"
	//policyv1beta1 "k8s.io/api/policy/v1beta1"
	//"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	datastaxv1alpha1 "github.com/riptano/dse-operator/pkg/apis/datastax/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcile(t *testing.T) {
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

	trackObjects := []runtime.Object{
		dseDatacenter,
	}

	s := scheme.Scheme
	s.AddKnownTypes(datastaxv1alpha1.SchemeGroupVersion, dseDatacenter)

	client := fake.NewFakeClient(trackObjects...)

	r := &ReconcileDseDatacenter{
		client: client,
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

	EventBus.SubscribeAsync("ReconciliationRequest", testHandleReconciliationRequest, true)

	result, err := r.Reconcile(request)
	if err != nil {
		t.Fatalf("Reconciliation Failure: (%v)", err)
	}

	if result != (reconcile.Result{}) {
		t.Error("Reconcile did not return an empty result.")
	}

	// wait for events to be handled
	EventBus.WaitAsync()

	if handlerCalled == false {
		t.Error("Reconcile did not call the handler.")
	}
}
