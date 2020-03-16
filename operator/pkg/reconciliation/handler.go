package reconciliation

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	api "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1alpha2"
	"github.com/riptano/dse-operator/operator/pkg/httphelper"
)

// Use a var so we can mock this function
var setControllerReference = controllerutil.SetControllerReference

// calculateReconciliationActions will iterate over an ordered list of reconcilers which will determine if any action needs to
// be taken on the CassandraDatacenter. If a change is needed then the apply function will be called on that reconciler and the
// request will be requeued for the next reconciler to handle in the subsequent reconcile loop, otherwise the next reconciler
// will be called.
func (rc *ReconciliationContext) calculateReconciliationActions() (reconcile.Result, error) {

	rc.ReqLogger.Info("handler::calculateReconciliationActions")

	// Check if the CassandraDatacenter was marked to be deleted
	if rc.ProcessDeletion() {
		return rc.RemoveDatacenterResources()
	}

	if err := rc.addFinalizer(); err != nil {
		return reconcile.Result{}, err
	}

	if needCreate, err := rc.CheckHeadlessServices(); err != nil {
		return reconcile.Result{}, err
	} else if needCreate {
		return rc.CreateHeadlessServices()
	}

	if err := rc.CalculateRackInformation(); err != nil {
		return reconcile.Result{}, err
	}

	return rc.ReconcileAllRacks()
}

// This file contains various definitions and plumbing setup used for reconciliation.

// For information on log usage, see:
// https://godoc.org/github.com/go-logr/logr

var log = logf.Log.WithName("reconciliation_handler")

// Reconciliation related data structures

// ReconcileCassandraDatacenter reconciles a cassandraDatacenter object
// This is placed here to avoid a circular dependency
type ReconcileCassandraDatacenter struct {
	// This client, initialized using mgr.client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a Datacenter object
// and makes changes based on the state read
// and what is in the cassandraDatacenter.Spec
// Note:
// The Controller will requeue the Request to be processed again
// if the returned error is non-nil or Result.Requeue is true,
// otherwise upon completion it will remove the work from the queue.
// See: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Result
func (r *ReconcileCassandraDatacenter) Reconcile(request reconcile.Request) (reconcile.Result, error) {

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

	rc, err := CreateReconciliationContext(
		&request,
		r.client,
		r.scheme,
		r.recorder,
		logger)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("CassandraDatacenter resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get CassandraDatacenter.")
		return reconcile.Result{}, err
	}

	if err := rc.isValid(rc.Datacenter); err != nil {
		logger.Error(err, "CassandraDatacenter resource is invalid")
		rc.Recorder.Eventf(rc.Datacenter, "Warning", "ValidationFailed", err.Error())

		// No reason to requeue if the resource is invalid as the user will need
		// to fix it before we can do anything further.
		return reconcile.Result{Requeue: false}, err
	}

	twentySecsAgo := metav1.Now().Add(time.Second * -20)
	lastNodeStart := rc.Datacenter.Status.LastServerNodeStarted
	serverRecentlyStarted := lastNodeStart.After(twentySecsAgo)

	if serverRecentlyStarted {
		logger.Info("Ending reconciliation early because a server node was recently started")
		return reconcile.Result{}, nil
	}

	res, err := rc.calculateReconciliationActions()
	if err != nil {
		logger.Error(err, "calculateReconciliationActions returned an error")
		rc.Recorder.Eventf(rc.Datacenter, "Warning", "ReconcileFailed", err.Error())
	}
	return res, err
}

func (rc *ReconciliationContext) addFinalizer() error {
	if len(rc.Datacenter.GetFinalizers()) < 1 && rc.Datacenter.GetDeletionTimestamp() == nil {
		rc.ReqLogger.Info("Adding Finalizer for the CassandraDatacenter")
		rc.Datacenter.SetFinalizers([]string{"finalizer.cassandra.datastax.com"})

		// Update CR
		err := rc.Client.Update(rc.Ctx, rc.Datacenter)
		if err != nil {
			rc.ReqLogger.Error(err, "Failed to update CassandraDatacenter with finalizer")
			return err
		}
	}
	return nil
}

func (rc *ReconciliationContext) isValid(dc *api.CassandraDatacenter) error {
	var errs []error = []error{}

	// Basic validation up here

	errs = append(errs, rc.validateSuperuserSecret()...)

	// Validate Management API config
	errs = append(errs, httphelper.ValidateManagementApiConfig(dc, rc.Client, rc.Ctx)...)
	if len(errs) > 0 {
		return errs[0]
	}

	claim := dc.Spec.StorageConfig.CassandraDataVolumeClaimSpec
	if claim == nil {
		err := fmt.Errorf("storageConfig.cassandraDataVolumeClaimSpec is required")
		return err
	}

	if claim.StorageClassName == nil || *claim.StorageClassName == "" {
		err := fmt.Errorf("storageConfig.cassandraDataVolumeClaimSpec.storageClassName is required")
		return err
	}

	if len(claim.AccessModes) == 0 {
		err := fmt.Errorf("storageConfig.cassandraDataVolumeClaimSpec.accessModes is required")
		return err
	}

	return nil
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCassandraDatacenter{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor("cassandra-operator"),
	}
}
