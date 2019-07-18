package reconciliation

//
// This file contains various definitions and plumbing setup for the EventBus
// used for reconciliation.
//

import (
	"context"
	"fmt"
	"time"

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

	rc, err := CreateReconciliationContext(
		&request,
		r,
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

	reconcileSeedServices := ReconcileSeedServices{
		ReconcileContext: rc,
	}

	return calculateReconciliationActions(rc, reconcileDatacenter, reconcileRacks, reconcileServices, reconcileSeedServices, r)
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

// CreateReconciliationContext gathers all information needed for computeReconciliationActions into a struct
func CreateReconciliationContext(
	request *reconcile.Request,
	reconciler *ReconcileDseDatacenter,
	ReqLogger logr.Logger) (*dsereconciliation.ReconciliationContext, error) {

	rc := &dsereconciliation.ReconciliationContext{}
	rc.Request = request
	rc.Client = reconciler.client
	rc.Scheme = reconciler.scheme
	rc.ReqLogger = ReqLogger
	rc.Ctx = context.Background()

	rc.ReqLogger.Info("handler::CreateReconciliationContext")

	// Fetch the DseDatacenter dseDatacenter
	dseDatacenter := &datastaxv1alpha1.DseDatacenter{}
	if err := retrieveDseDatacenter(rc, request, dseDatacenter); err != nil {
		return nil, err
	}
	rc.DseDatacenter = dseDatacenter

	rc.ReqLogger = rc.ReqLogger.
		WithValues("dseDatacenterName", dseDatacenter.Name).
		WithValues("dseDatacenterClusterName", dseDatacenter.ClusterName)

	return rc, nil
}

func retrieveDseDatacenter(rc *dsereconciliation.ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	err := rc.Client.Get(
		rc.Ctx,
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

func retrieveDseDatacenterByPod(rc *dsereconciliation.ReconciliationContext, request *reconcile.Request, dseDatacenter *datastaxv1alpha1.DseDatacenter) error {
	pod := &corev1.Pod{}
	err := rc.Client.Get(
		rc.Ctx,
		request.NamespacedName,
		pod)
	if err != nil {
		rc.ReqLogger.Info("Unable to get pod",
			"podName", request.Name)
		return err
	}

	// Its entirely possible that a pod could be missing OwnerReferences even though it should be owned by a
	// statefulset. The most likely scenario for this would be if a pod label was modified, causing the selector on
	// the statefulset to no longer find the pod. Once the pod has been reconciled and we've fixed the label its OwnerReferences
	// should show back up and everything will be fine.
	if len(pod.OwnerReferences) == 0 {
		rc.ReqLogger.Info("OwnerReferences missing for pod",
			"podName",
			pod.Name)
		return fmt.Errorf("pod=%s missing OwnerReferences", pod.Name)
	}

	statefulSet := &appsv1.StatefulSet{}
	err = rc.Client.Get(
		rc.Ctx,
		types.NamespacedName{
			Name:      pod.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		statefulSet)
	if err != nil {
		rc.ReqLogger.Info("Unable to get statefulset",
			"statefulsetName", pod.OwnerReferences[0].Name)
		return err
	}

	err = rc.Client.Get(
		rc.Ctx,
		types.NamespacedName{
			Name:      statefulSet.OwnerReferences[0].Name,
			Namespace: pod.Namespace},
		dseDatacenter)
	if err != nil {
		rc.ReqLogger.Info("Unable to get DseDatacenter",
			"dseDatacenterName", statefulSet.OwnerReferences[0].Name)
		return err
	}
	return nil
}
