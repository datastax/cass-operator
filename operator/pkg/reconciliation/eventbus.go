package reconciliation

//
// This file contains various definitions and plumbing setup for the EventBus
// used for reconciliation.
//

import (
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
)

//
// For information on log usage, see:
// https://godoc.org/github.com/go-logr/logr
//

var log = logf.Log.WithName("controller_dsedatacenter")

//
// Reconciliation related data structures
//

// ReconcileDseDatacenter reconciles a dseDatacenter object
// This is placed here to avoid a circular dependency
type ReconcileDseDatacenter struct {
	// This client, initialized using mgr.client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a dseDatacenter object
// and makes changes based on the state read
// and what is in the dseDatacenter.Spec
// Note:
// The Controller will requeue the Request to be processed again
// if the returned error is non-nil or Result.Requeue is true,
// otherwise upon completion it will remove the work from the queue.
// See: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Result
func (r *ReconcileDseDatacenter) Reconcile(
	request reconcile.Request) (reconcile.Result, error) {

	startReconcile := time.Now()

	ReqLogger := log.
		WithValues("requestNamespace", request.Namespace).
		WithValues("requestName", request.Name).
		// loopID is used to tie all events together that are spawned by the same reconciliation loop
		WithValues("loopID", uuid.New().String())

	defer func() {
		reconcileDuration := time.Since(startReconcile).Seconds()
		ReqLogger.Info("Reconcile loop completed",
			"duration", reconcileDuration)
	}()

	ReqLogger.Info("======== handler::Reconcile has been called")

	rc, err := dsereconciliation.CreateReconciliationContext(
		&request,
		r.client,
		r.scheme,
		ReqLogger)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ReqLogger.Info("DseDatacenter resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ReqLogger.Error(err, "Failed to get DseDatacenter.")
		return reconcile.Result{Requeue: true}, err
	}

	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	reconcileServices := ReconcileServices{
		ReconcileContext: rc,
	}

	return calculateReconciliationActions(rc, reconcileDatacenter, reconcileRacks, reconcileServices, r)
}

func (r *ReconcileDseDatacenter) addFinalizer(rc *dsereconciliation.ReconciliationContext) error {
	if len(rc.DseDatacenter.GetFinalizers()) < 1 && rc.DseDatacenter.GetDeletionTimestamp() == nil {
		rc.ReqLogger.Info("Adding Finalizer for the DseDatacenter")
		rc.DseDatacenter.SetFinalizers([]string{"com.datastax.dse.finalizer"})

		// Update CR
		err := r.client.Update(rc.Ctx, rc.DseDatacenter)
		if err != nil {
			rc.ReqLogger.Error(err, "Failed to update DseDatacenter with finalizer")
			return err
		}
	}
	return nil
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDseDatacenter{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme()}
}
