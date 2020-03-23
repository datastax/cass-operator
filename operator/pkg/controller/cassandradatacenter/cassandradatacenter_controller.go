package cassandradatacenter

import (
	api "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1beta1"

	"github.com/riptano/dse-operator/operator/pkg/oplabels"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"

	"github.com/riptano/dse-operator/operator/pkg/reconciliation"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_cassandradatacenter")

// Add creates a new CassandraDatacenter Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, reconciliation.NewReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(
		"cassandradatacenter-controller",
		mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CassandraDatacenter
	err = c.Watch(
		&source.Kind{Type: &api.CassandraDatacenter{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Here we list all the types that we create that are owned by the primary resource.
	//
	// Watch for changes to secondary resources StatefulSets, PodDisruptionBudgets, and Services and requeue the
	// CassandraDatacenter that owns them.

	managedByCassandraOperatorPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return oplabels.HasManagedByCassandraOperatorLabel(e.Meta.GetLabels())
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return oplabels.HasManagedByCassandraOperatorLabel(e.Meta.GetLabels())
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return oplabels.HasManagedByCassandraOperatorLabel(e.MetaOld.GetLabels()) ||
				oplabels.HasManagedByCassandraOperatorLabel(e.MetaNew.GetLabels())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return oplabels.HasManagedByCassandraOperatorLabel(e.Meta.GetLabels())
		},
	}

	err = c.Watch(
		&source.Kind{Type: &appsv1.StatefulSet{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &api.CassandraDatacenter{},
		},
		managedByCassandraOperatorPredicate,
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &policyv1beta1.PodDisruptionBudget{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &api.CassandraDatacenter{},
		},
		managedByCassandraOperatorPredicate,
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Service{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &api.CassandraDatacenter{},
		},
		managedByCassandraOperatorPredicate,
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCassandraDatacenter implements reconciliation.Reconciler
var _ reconcile.Reconciler = &reconciliation.ReconcileCassandraDatacenter{}
