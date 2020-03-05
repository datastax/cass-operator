package cassandradatacenter

import (
	"context"

	cassandrav1alpha2 "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1alpha2"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_cassandradatacenter")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new CassandraDatacenter Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCassandraDatacenter{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("cassandradatacenter-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CassandraDatacenter
	err = c.Watch(&source.Kind{Type: &cassandrav1alpha2.CassandraDatacenter{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner CassandraDatacenter
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cassandrav1alpha2.CassandraDatacenter{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCassandraDatacenter implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCassandraDatacenter{}

// ReconcileCassandraDatacenter reconciles a CassandraDatacenter object
type ReconcileCassandraDatacenter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a CassandraDatacenter object and makes changes based on the state read
// and what is in the CassandraDatacenter.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCassandraDatacenter) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling CassandraDatacenter")

	// Fetch the CassandraDatacenter instance
	instance := &cassandrav1alpha2.CassandraDatacenter{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// WIP - just create a DseDatacenter

	// Define a new DseDatacenter object
	dseDc := newDseDatacenter(instance)

	// Set CassandraDatacenter instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, dseDc, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this dseDc already exists
	found := &datastaxv1alpha1.DseDatacenter{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: dseDc.Name, Namespace: dseDc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DseDatacenter", "dseDc.Namespace", dseDc.Namespace, "dseDc.Name", dseDc.Name)
		err = r.client.Create(context.TODO(), dseDc)
		if err != nil {
			return reconcile.Result{}, err
		}

		// DseDc created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// dseDc already exists - don't requeue
	reqLogger.Info("Skip reconcile: DseDatacenter already exists", "dseDc.Namespace", found.Namespace, "dseDc.Name", found.Name)

	return reconcile.Result{}, nil
}

func newDseDatacenter(cr *cassandrav1alpha2.CassandraDatacenter) *datastaxv1alpha1.DseDatacenter {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &datastaxv1alpha1.DseDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-dse",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: datastaxv1alpha1.DseDatacenterSpec{
			Size:                        cr.Spec.Size,
			DseVersion:                  cr.Spec.ImageVersion,
			DseImage:                    cr.Spec.ServerImage,
			Config:                      cr.Spec.Config,
			ManagementApiAuth:           cr.Spec.ManagementApiAuth,
			Resources:                   cr.Spec.Resources,
			Racks:                       cr.Spec.Racks,
			StorageClaim:                cr.Spec.StorageClaim,
			DseClusterName:              cr.Spec.ClusterName,
			Parked:                      cr.Spec.Parked,
			ConfigBuilderImage:          cr.Spec.ConfigBuilderImage,
			CanaryUpgrade:               cr.Spec.CanaryUpgrade,
			AllowMultipleNodesPerWorker: cr.Spec.AllowMultipleNodesPerWorker,
			ServiceAccount:              cr.Spec.ServiceAccount,
		},
	}
}
