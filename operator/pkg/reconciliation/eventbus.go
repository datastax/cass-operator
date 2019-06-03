package reconciliation

//
// This file contains various definitions and plumbing setup for the EventBus
// used for reconciliation.
//

import (
	"context"
	"fmt"

	logr "github.com/go-logr/logr"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"

	evbus "github.com/asaskevich/EventBus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

//
// For information on log usage, see:
// https://godoc.org/github.com/go-logr/logr
//

var log = logf.Log.WithName("controller_dsedatacenter")

//
// Reconciliation related data structures
//

// ReconcileDseDatacenter reconciles a DseDatacenter object
// This is placed here to avoid a circular dependency
type ReconcileDseDatacenter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// We must define this method here.
// Reconcile reads that state of the cluster for a DseDatacenter object
// and makes changes based on the state read
// and what is in the DseDatacenter.Spec
// Note:
// The Controller will requeue the Request to be processed again
// if the returned error is non-nil or Result.Requeue is true,
// otherwise upon completion it will remove the work from the queue.
// See: https://godoc.org/sigs.k8s.io/controller-runtime/pkg/reconcile#Result
func (r *ReconcileDseDatacenter) Reconcile(
	request reconcile.Request) (reconcile.Result, error) {

	reqLogger := log.WithValues(
		"Request.Namespace",
		request.Namespace,
		"Request.Name",
		request.Name)

	reqLogger.Info("======== handler::Reconcile has been called")

	rc, err := CreateReconciliationContext(
		&request,
		r,
		reqLogger)

	if err != nil {
		return reconcile.Result{}, err
	}

	EventBus.Publish(
		"ReconciliationRequest",
		rc)

	return reconcile.Result{}, nil
}

// newReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDseDatacenter{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme()}
}

// All of the input necessary to calculate a list of ReconciliationActions
type ReconciliationContext struct {
	request       *reconcile.Request
	reconciler    *ReconcileDseDatacenter
	dseDatacenter *datastaxv1alpha1.DseDatacenter
	// Note that logr.Logger is an interface,
	// so we do not want to store a pointer to it
	// see: https://stackoverflow.com/a/44372954
	reqLogger logr.Logger
}

//
// Instance the event bus for the controller.
//

// FIXME wrap all direct access to this variable so no one external uses it.
// consider making it private eventBus, and also creating eventbus.Publish()
// for external clients.
var EventBus = evbus.New()

//
// Attach the event handlers
//
func SubscribeToEventBus() {
	EventBus.SubscribeAsync("ReconciliationRequest", calculateReconciliationActions, true)

	// Operations that need to be performed

	EventBus.SubscribeAsync("CreateHeadlessService", createHeadlessService, true)
	EventBus.SubscribeAsync("CalculateRackInformation", calculateRackInformation, true)
	EventBus.SubscribeAsync("ReconcileRacks", reconcileRacks, true)
	EventBus.SubscribeAsync("ReconcileNextRack", reconcileNextRack, true)
}

//
// Gather all information needed for computeReconciliationActions into a struct.
//
func CreateReconciliationContext(
	request *reconcile.Request,
	reconciler *ReconcileDseDatacenter,
	reqLogger logr.Logger) (*ReconciliationContext, error) {

	rc := &ReconciliationContext{}
	rc.request = request
	rc.reconciler = reconciler
	rc.reqLogger = reqLogger

	rc.reqLogger.Info("handler::CreateReconciliationContext")

	// Fetch the DseDatacenter dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{}
	err := rc.reconciler.client.Get(
		context.TODO(),
		request.NamespacedName,
		dseDatacenter)
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO this situation might be ok
			// is there any well to tell?

			// Request object not found,
			// could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			// Return and don't requeue
			// TODO LOG THIS - and update error
			return nil, fmt.Errorf("DseDatacenter object not found")
		}
		// Error reading the object - requeue the request.
		return nil, fmt.Errorf("Error reading DseDatacenter object")
	}

	rc.dseDatacenter = dseDatacenter

	return rc, nil
}
