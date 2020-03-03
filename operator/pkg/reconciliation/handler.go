package reconciliation

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"

	"context"
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/httphelper"
)

// Use a var so we can mock this function
var setControllerReference = controllerutil.SetControllerReference

type reconcileFun func() (reconcileriface.Reconciler, error)

// calculateReconciliationActions will iterate over an ordered list of reconcilers which will determine if any action needs to
// be taken on the DseDatacenter. If a change is needed then the apply function will be called on that reconciler and the
// request will be requeued for the next reconciler to handle in the subsequent reconcile loop, otherwise the next reconciler
// will be called.
func calculateReconciliationActions(
	rc *dsereconciliation.ReconciliationContext,
	reconcileDatacenter ReconcileDatacenter,
	reconcileRacks ReconcileRacks,
	reconcileServices ReconcileServices,
	reconciler *ReconcileDseDatacenter) (reconcile.Result, error) {

	rc.ReqLogger.Info("handler::calculateReconciliationActions")

	// Check if the DseDatacenter was marked to be deleted
	if rec, err := reconcileDatacenter.ProcessDeletion(); err != nil || rec != nil {
		// had to modify the headless service so requeue in order to reconcile the seed service on the next loop

		if err != nil {
			return reconcile.Result{Requeue: true}, err
		}

		return rec.Apply()
	}

	if err := reconciler.addFinalizer(rc); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	// order of this list matters!
	reconcilers := []reconcileFun{
		reconcileServices.ReconcileHeadlessServices,
		reconcileRacks.CalculateRackInformation}

	for _, fun := range reconcilers {
		if rec, err := fun(); err != nil || rec != nil {
			if err != nil {
				return reconcile.Result{Requeue: true}, err
			}

			return rec.Apply()
		}
	}

	// no more changes to make!

	// nothing happened so return and don't requeue
	return reconcile.Result{}, nil
}

// This file contains various definitions and plumbing setup used for reconciliation.

// For information on log usage, see:
// https://godoc.org/github.com/go-logr/logr

var log = logf.Log.WithName("controller_dsedatacenter")

// Reconciliation related data structures

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

	logger := log.
		WithValues("requestNamespace", request.Namespace).
		WithValues("requestName", request.Name).
		// loopID is used to tie all events together that are spawned by the same reconciliation loop
		WithValues("loopID", uuid.New().String())

	defer func() {
		reconcileDuration := time.Since(startReconcile).Seconds()
		logger.Info("Reconcile loop completed",
			"duration", reconcileDuration)
	}()

	logger.Info("======== handler::Reconcile has been called")

	rc, err := dsereconciliation.CreateReconciliationContext(
		&request,
		r.client,
		r.scheme,
		logger)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("DseDatacenter resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get DseDatacenter.")
		return reconcile.Result{Requeue: true}, err
	}

	if ok, err := r.isValid(rc.Datacenter); !ok {
		logger.Error(err, "DseDatacenter resource is invalid.")
		// No reason to requeue if the resource is invalid as the user will need
		// to fix it before we can do anything further.
		return reconcile.Result{Requeue: false}, err
	}

	twentySecsAgo := metav1.Now().Add(time.Second * -20)
	lastNodeStart := rc.Datacenter.Status.LastDseNodeStarted
	dseRecentlyStarted := lastNodeStart.After(twentySecsAgo)

	if dseRecentlyStarted {
		logger.Info("Ending reconciliation early because a DSE node was recently started")
		return reconcile.Result{}, nil
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
	if len(rc.Datacenter.GetFinalizers()) < 1 && rc.Datacenter.GetDeletionTimestamp() == nil {
		rc.ReqLogger.Info("Adding Finalizer for the DseDatacenter")
		rc.Datacenter.SetFinalizers([]string{"com.datastax.dse.finalizer"})

		// Update CR
		err := r.client.Update(rc.Ctx, rc.Datacenter)
		if err != nil {
			rc.ReqLogger.Error(err, "Failed to update DseDatacenter with finalizer")
			return err
		}
	}
	return nil
}

func (r *ReconcileDseDatacenter) isValid(dseDatacenter *datastaxv1alpha1.DseDatacenter) (bool, error) {
	ctx := context.Background()

	// Basic validation up here

	// Validate Management API config
	errs := httphelper.ValidateManagementApiConfig(dseDatacenter, r.client, ctx)
	if errs != nil && len(errs) > 0 {
		return false, errs[0]
	}

	return true, nil
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDseDatacenter{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme()}
}
