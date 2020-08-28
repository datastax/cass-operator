// Copyright DataStax, Inc.
// Please see the included license file for details.

package cassandradatacenter

import (
	"fmt"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"

	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/datastax/cass-operator/operator/pkg/reconciliation"
	corev1 "k8s.io/api/core/v1"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("cassandradatacenter_controller")

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
		&handler.EnqueueRequestForObject{},
		// This allows us to update the status on every reconcile call without
		// triggering an infinite loop.
		predicate.GenerationChangedPredicate{})
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

	// NOTE: We do not currently watch PVC resources, but if we did, we'd have to
	// account for the fact that they might use the old managed-by label value
	// (oplabels.ManagedByLabelDefunctValue) for CassandraDatacenters originally
	// created in version 1.1.0 or earlier.

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

	// Setup watches for Nodes to check for taints being added

	nodeMapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			log.Info("Node Watch called")
			requests := []reconcile.Request{}

			nodeName := a.Object.(*corev1.Node).Name
			dcs := reconciliation.DatacentersForNode(nodeName)

			for _, dc := range dcs {
				log.Info("node watch adding reconcilation request",
					"cassandraDatacenter", dc.Name,
					"namespace", dc.Namespace)

				// Create reconcilerequests for the related cassandraDatacenter
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dc.Name,
						Namespace: dc.Namespace,
					}},
				)
			}
			return requests
		})

	if utils.IsPSPEnabled() {
		err = c.Watch(
			&source.Kind{Type: &corev1.Node{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: nodeMapFn,
			},
		)
		if err != nil {
			return err
		}
	}

	// Setup watches for pvc to check for taints being added

	pvcMapFn := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			log.Info("PersistentVolumeClaim Watch called")
			requests := []reconcile.Request{}

			pvc := a.Object.(*corev1.PersistentVolumeClaim)
			pvcLabels := pvc.ObjectMeta.Labels
			pvcNamespace := pvc.ObjectMeta.Namespace

			managedByValue, ok := pvcLabels[oplabels.ManagedByLabel]
			if !ok {
				return requests
			}

			if (managedByValue == oplabels.ManagedByLabelValue) || (managedByValue == oplabels.ManagedByLabelDefunctValue) {

				dcName := pvcLabels[api.DatacenterLabel]

				log.Info("PersistentVolumeClaim watch adding reconcilation request",
					"cassandraDatacenter", dcName,
					"namespace", pvcNamespace)

				// Create reconcilerequests for the related cassandraDatacenter
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dcName,
						Namespace: pvcNamespace,
					}},
				)
			}
			return requests
		})

	if utils.IsPSPEnabled() {
		err = c.Watch(
			&source.Kind{Type: &corev1.PersistentVolumeClaim{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: pvcMapFn,
			},
		)
		if err != nil {
			return err
		}
	}

	// Setup watches for Secrets. These secrets are often not owned by or created by
	// the operator, so we must create a mapping back to the appropriate datacenters.

	rd, ok := r.(*reconciliation.ReconcileCassandraDatacenter)
	if !ok {
		// This should never happen. - John 06/10/2020
		return fmt.Errorf("%v was not of type ReconcileCassandraDatacenter", r)
	}
	dynamicSecretWatches := rd.SecretWatches

	toRequests := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		watchers := dynamicSecretWatches.FindWatchers(a.Meta, a.Object)
		requests := []reconcile.Request{}
		for _, watcher := range watchers {
			requests = append(requests, reconcile.Request{NamespacedName: watcher})
		}
		return requests
	})

	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: toRequests},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCassandraDatacenter implements reconciliation.Reconciler
var _ reconcile.Reconciler = &reconciliation.ReconcileCassandraDatacenter{}
