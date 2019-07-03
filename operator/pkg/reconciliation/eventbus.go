package reconciliation

//
// This file contains various definitions and plumbing setup for the EventBus
// used for reconciliation.
//

import (
	"context"
	"fmt"

	evbus "github.com/asaskevich/EventBus"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
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

	reqLogger := log.
		WithValues("Request.Namespace", request.Namespace).
		WithValues("Request.Name", request.Name).
		// loopID is used to tie all events together that are spawned by the same reconciliation loop
		WithValues("loopID", uuid.New().String())

	reqLogger.Info("======== handler::Reconcile has been called")

	rc, err := CreateReconciliationContext(
		&request,
		r,
		reqLogger)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("DseDatacenter resource not found. Ignoring since object must be deleted.")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get DseDatacenter.")
		return reconcile.Result{Requeue: true}, err
	}

	EventBus.Publish(
		RECONCILIATION_REQUEST_TOPIC,
		rc)

	return reconcile.Result{}, nil
}

func (r *ReconcileDseDatacenter) addFinalizer(rc *ReconciliationContext) error {
	if len(rc.dseDatacenter.GetFinalizers()) < 1 && rc.dseDatacenter.GetDeletionTimestamp() == nil {
		rc.reqLogger.Info("Adding Finalizer for the DseDatacenter")
		rc.dseDatacenter.SetFinalizers([]string{"com.datastax.dse.finalizer"})

		// Update CR
		err := r.client.Update(rc.ctx, rc.dseDatacenter)
		if err != nil {
			rc.reqLogger.Error(err, "Failed to update DseDatacenter with finalizer")
			return err
		}
	}
	return nil
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
	// According to golang recommendations the context should not be stored in a struct but given that
	// this is passed around as a parameter we feel that its a fair compromise. For further discussion
	// see: golang/go#22602
	ctx context.Context
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
	// The below subscriptions are intentionally set to transactional=false because if we try to run them in a transactional
	// manner we get into a deadlock situation.
	EventBus.SubscribeAsync(RECONCILIATION_REQUEST_TOPIC, calculateReconciliationActions, false)

	// Operations that need to be performed

	EventBus.SubscribeAsync(CREATE_HEADLESS_SERVICE_TOPIC, createHeadlessService, false)
	EventBus.SubscribeAsync(CREATE_HEADLESS_SEED_SERVICE_TOPIC, createHeadlessSeedService, false)
	EventBus.SubscribeAsync(RECONCILE_HEADLESS_SEED_SERVICE_TOPIC, reconcileHeadlessSeedService, false)
	EventBus.SubscribeAsync(CALCULATE_RACK_INFORMATION_TOPIC, calculateRackInformation, false)
	EventBus.SubscribeAsync(RECONCILE_RACKS_TOPIC, reconcileRacks, false)
	EventBus.SubscribeAsync(RECONCILE_NEXT_RACK_TOPIC, reconcileNextRack, false)
	EventBus.SubscribeAsync(UPDATE_RACK_TOPIC, updateRackNodeCount, false)
	EventBus.SubscribeAsync(PROCESS_DELETION_TOPIC, processDeletion, false)
	EventBus.SubscribeAsync(RECONCILE_HEADLESS_SERVICE_TOPIC, reconcileHeadlessService, false)
	EventBus.SubscribeAsync(RECONCILE_PODS_TOPIC, reconcilePods, false)
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
	rc.ctx = context.Background()

	rc.reqLogger = rc.reqLogger.
		WithValues("namespace", request.Namespace)

	rc.reqLogger.Info("handler::CreateReconciliationContext")

	// Fetch the DseDatacenter dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{}
	if err := retrieveDseDatacenter(rc, request, dseDatacenter); err != nil {
		return nil, err
	}
	rc.dseDatacenter = dseDatacenter

	rc.reqLogger = rc.reqLogger.
		WithValues("dseDatacenterName", dseDatacenter.Name).
		WithValues("dseDatacenterClusterName", dseDatacenter.ClusterName)

	return rc, nil
}

func retrieveDseDatacenter(rc *ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	err := rc.reconciler.client.Get(
		rc.ctx,
		request.NamespacedName,
		dseDatacenter)
	if err != nil {
		if errors.IsNotFound(err) {
			// Chance this was a pod event so get the DseDatacenter via the pod
			if innerErr := retrieveDseDatacenterByPod(rc, request, dseDatacenter); innerErr != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func retrieveDseDatacenterByPod(rc *ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	pod := &corev1.Pod{}
	err := rc.reconciler.client.Get(
		rc.ctx,
		request.NamespacedName,
		pod)
	if err != nil {
		rc.reqLogger.Info("Unable to get pod",
			"podName", request.Name)
		return err
	}

	// Its entirely possible that a pod could be missing OwnerReferences even though it should be owned by a
	// statefulset. The most likely scenario for this would be if a pod label was modified, causing the selector on
	// the statefulset to no longer find the pod. Once the pod has been reconciled and we've fixed the label its OwnerReferences
	// should show back up and everything will be fine.
	if len(pod.OwnerReferences) == 0 {
		rc.reqLogger.Info("OwnerReferences missing for pod",
			"podName",
			pod.Name)
		return fmt.Errorf("pod=%s missing OwnerReferences", pod.Name)
	}

	statefulSet := &appsv1.StatefulSet{}
	err = rc.reconciler.client.Get(
		rc.ctx,
		types.NamespacedName{
			Name:      pod.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		statefulSet)
	if err != nil {
		rc.reqLogger.Info("Unable to get statefulset",
			"statefulsetName", pod.OwnerReferences[0].Name)
		return err
	}

	err = rc.reconciler.client.Get(
		rc.ctx,
		types.NamespacedName{
			Name:      statefulSet.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		dseDatacenter)
	if err != nil {
		rc.reqLogger.Info("Unable to get DseDatacenter",
			"dseDatacenterName", statefulSet.OwnerReferences[0].Name)
		return err
	}
	return nil
}
