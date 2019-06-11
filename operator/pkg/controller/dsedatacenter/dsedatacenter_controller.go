package dsedatacenter

//
// This file creates the DseDatacenter controller and adds it to the Manager.
//

import (
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/reconciliation"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new DseDatacenter Controller and adds it to the Manager.
// The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, reconciliation.NewReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(
		"dsedatacenter-controller",
		mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DseDatacenter
	err = c.Watch(
		&source.Kind{Type: &datastaxv1alpha1.DseDatacenter{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Here we list all the types that we create
	// that are owned by the primary resource.
	//
	// Watch for changes to secondary resources StatefulSets, PodDisruptionBudgets, and Services and requeue the
	// DseDatacenter that owns them.

	err = c.Watch(
		&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &datastaxv1alpha1.DseDatacenter{},
		})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &policyv1beta1.PodDisruptionBudget{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &datastaxv1alpha1.DseDatacenter{},
		})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &datastaxv1alpha1.DseDatacenter{},
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &reconciliation.ReconcileDseDatacenter{}
