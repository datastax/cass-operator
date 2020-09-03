// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/dynamicwatch"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

// Use a var so we can mock this function
var setControllerReference = controllerutil.SetControllerReference

// key: Node.Name, value: CassandraDatacenter.Name
var nodeToDc = make(map[string][]types.NamespacedName)
var nodeToDcLock = sync.RWMutex{}

// Get the dcNames and dcNamespaces for a node
func DatacentersForNode(nodeName string) []types.NamespacedName {
	nodeToDcLock.RLock()
	defer nodeToDcLock.RUnlock()

	dcs, ok := nodeToDc[nodeName]
	if ok {
		return dcs
	}
	return []types.NamespacedName{}
}

func (rc *ReconciliationContext) RemoveDcFromNodeToDcMap(dcToRemove types.NamespacedName) {
	nodeToDcLock.Lock()
	defer nodeToDcLock.Unlock()

	for nodeName, dcs := range nodeToDc {
		var newDcs = []types.NamespacedName{}
		for _, dc := range dcs {
			if dc != dcToRemove {
				newDcs = append(newDcs, dc)
			}
		}
		nodeToDc[nodeName] = newDcs
	}
}

// We will only update the map for the current CassandraDatacenter
// Every CassandraDatacenter with pods will have produced at least
// one call to the reconcile loop.  Therefore this map will be
// populated with the information for all current CassandraDatacenters.
func (rc *ReconciliationContext) updateDcMaps() error {

	dcName := rc.Datacenter.ObjectMeta.Name

	// List all pods managed by the cass-operator for this dc

	labelSelector := labels.SelectorFromSet(
		labels.Set{
			oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
			api.DatacenterLabel:     dcName,
		})

	listOptions := &client.ListOptions{
		LabelSelector: labelSelector,
	}

	podList := &corev1.PodList{}

	err := rc.Client.List(rc.Ctx, podList, listOptions)
	if err != nil {
		rc.ReqLogger.Error(err, "error listing managed pods for namespace",
			"namespace", rc.Request.Namespace)
		return err
	}

	nodeToDcLock.Lock()
	defer nodeToDcLock.Unlock()

	for _, pod := range podList.Items {
		dcToAdd := types.NamespacedName{
			Namespace: pod.ObjectMeta.Namespace,
			Name:      dcName,
		}

		// Update node map

		nodeName := pod.Spec.NodeName

		needToAdd := true
		for _, dc := range nodeToDc[nodeName] {
			if dc == dcToAdd {
				needToAdd = false
			}
		}

		if needToAdd {
			nodeToDc[nodeName] = append(nodeToDc[nodeName], dcToAdd)
		}
	}

	return nil
}

// calculateReconciliationActions will iterate over an ordered list of reconcilers which will determine if any action needs to
// be taken on the CassandraDatacenter. If a change is needed then the apply function will be called on that reconciler and the
// request will be requeued for the next reconciler to handle in the subsequent reconcile loop, otherwise the next reconciler
// will be called.
func (rc *ReconciliationContext) calculateReconciliationActions() (reconcile.Result, error) {

	rc.ReqLogger.Info("handler::calculateReconciliationActions")
	if utils.IsPSPEnabled() {
		if err := rc.updateDcMaps(); err != nil {
			// We will not skip reconciliation if the map update failed
			// return result.Error(err).Output()
		}
	}

	// Check if the CassandraDatacenter was marked to be deleted
	if result := rc.ProcessDeletion(); result.Completed() {
		return result.Output()
	}

	if err := rc.addFinalizer(); err != nil {
		return result.Error(err).Output()
	}

	if result := rc.CheckHeadlessServices(); result.Completed() {
		return result.Output()
	}

	if result := rc.CheckAdditionalSeedEndpoints(); result.Completed() {
		return result.Output()
	}

	if err := rc.CalculateRackInformation(); err != nil {
		return result.Error(err).Output()
	}

	result, err := rc.ReconcileAllRacks()

	if err == nil {
		// Update PSP status
		// Always sync the datacenter status with the PSP health status
		if err := rc.PSPHealthUpdater.Update(*rc.Datacenter); err != nil {
			return reconcile.Result{}, err
		}
	}

	return result, err
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

	// SecretWatches is used in the controller when setting up the watches and
	// during reconciliation where we update the mappings for the watches.
	// Putting it here allows us to get it to both places.
	SecretWatches dynamicwatch.DynamicWatches
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

	rc, err := CreateReconciliationContext(&request, r.client, r.scheme, r.recorder, r.SecretWatches, logger)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// Return and don't requeue
			logger.Info("CassandraDatacenter resource not found. Ignoring since object must be deleted.")
			return result.Done().Output()
		}

		// Error reading the object
		logger.Error(err, "Failed to get CassandraDatacenter.")
		return result.Error(err).Output()
	}

	if err := rc.isValid(rc.Datacenter); err != nil {
		logger.Error(err, "CassandraDatacenter resource is invalid")
		rc.Recorder.Eventf(rc.Datacenter, "Warning", "ValidationFailed", err.Error())
		return result.Error(err).Output()
	}

	// TODO fold this into the quiet period
	twentySecs := time.Second * 20
	lastNodeStart := rc.Datacenter.Status.LastServerNodeStarted
	cooldownTime := time.Until(lastNodeStart.Add(twentySecs))

	if cooldownTime > 0 {
		logger.Info("Ending reconciliation early because a server node was recently started")
		secs := 1 + int(cooldownTime.Seconds())
		return result.RequeueSoon(secs).Output()
	}

	if rc.Datacenter.Status.QuietPeriod.After(time.Now()) {
		logger.Info("Ending reconciliation early because the datacenter is in a quiet period")
		cooldownTime = rc.Datacenter.Status.QuietPeriod.Sub(time.Now())
		secs := 1 + int(cooldownTime.Seconds())
		return result.RequeueSoon(secs).Output()
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

	// validate the required superuser
	errs = append(errs, rc.validateSuperuserSecret()...)

	// validate any other defined users
	errs = append(errs, rc.validateCassandraUserSecrets()...)

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
	client := mgr.GetClient()
	dynamicWatches := dynamicwatch.NewDynamicSecretWatches(client)
	return &ReconcileCassandraDatacenter{
		client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		recorder:      mgr.GetEventRecorderFor("cass-operator"),
		SecretWatches: dynamicWatches,
	}
}
