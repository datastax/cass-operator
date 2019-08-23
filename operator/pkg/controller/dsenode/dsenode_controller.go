package dsenode

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/reconciliation"
)

// Add creates a new DseDatacenter Controller and adds it to the Manager.
// The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, reconciliation.NewDseNodeReconciler(mgr, http.DefaultClient))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(
		"dsenode-controller",
		mgr,
		controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
			CreateFunc: func(e event.CreateEvent) bool {
				if _, ok := e.Meta.GetLabels()[datastaxv1alpha1.SeedNodeLabel]; ok {
					return true
				}

				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {

				return shouldReconcilePod(e.MetaNew, e.MetaOld)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				if _, ok := e.Meta.GetLabels()[datastaxv1alpha1.SeedNodeLabel]; ok {
					return true
				}
				return false
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func shouldReconcilePod(metaNew metav1.Object, metaOld metav1.Object) bool {
	if _, ok := metaNew.GetLabels()[datastaxv1alpha1.SeedNodeLabel]; !ok {
		return false
	}

	newPod := metaNew.(*corev1.Pod)
	oldPod := metaOld.(*corev1.Pod)

	return newPod.Status.PodIP != oldPod.Status.PodIP
}

var _ reconcile.Reconciler = &reconciliation.ReconcileDseNode{}
