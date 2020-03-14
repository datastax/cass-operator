package reconciliation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1alpha2"
)

func (rc *ReconciliationContext) RemoveDatacenterResources() (reconcile.Result, error) {
	// set the label here but no need to remove since we're deleting the CassandraDatacenter
	if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	if err := rc.deletePVCs(); err != nil {
		rc.ReqLogger.Error(err, "Failed to delete PVCs for CassandraDatacenter")
		return reconcile.Result{Requeue: true}, err
	}

	// Update finalizer to allow delete of CassandraDatacenter
	rc.Datacenter.SetFinalizers(nil)

	// Update CassandraDatacenter
	if err := rc.Client.Update(rc.Ctx, rc.Datacenter); err != nil {
		rc.ReqLogger.Error(err, "Failed to update CassandraDatacenter with removed finalizers")
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

// ProcessDeletion ...
func (rc *ReconciliationContext) ProcessDeletion() bool {
	if rc.Datacenter.GetDeletionTimestamp() != nil {
		return true
	}
	return false
}

func (rc *ReconciliationContext) deletePVCs() error {
	rc.ReqLogger.Info("reconciler::deletePVCs")
	logger := rc.ReqLogger.WithValues(
		"cassandraDatacenterNamespace", rc.Datacenter.Namespace,
		"cassandraDatacenterName", rc.Datacenter.Name,
	)

	persistentVolumeClaimList, err := rc.listPVCs()
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("No PVCs found for CassandraDatacenter")
			return nil
		}
		logger.Error(err, "Failed to list PVCs for cassandraDatacenter")
		return err
	}

	logger.Info(
		"Found PVCs for cassandraDatacenter",
		"numPVCs", len(persistentVolumeClaimList.Items))

	for _, pvc := range persistentVolumeClaimList.Items {
		if err := rc.Client.Delete(rc.Ctx, &pvc); err != nil {
			logger.Error(err, "Failed to delete PVCs for cassandraDatacenter")
			return err
		}
		logger.Info(
			"Deleted PVC",
			"pvcNamespace", pvc.Namespace,
			"pvcName", pvc.Name)
	}

	return nil
}

func (rc *ReconciliationContext) listPVCs() (*corev1.PersistentVolumeClaimList, error) {
	rc.ReqLogger.Info("reconciler::listPVCs")

	selector := map[string]string{
		api.DatacenterLabel: rc.Datacenter.Name,
	}

	listOptions := &client.ListOptions{
		Namespace:     rc.Datacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	persistentVolumeClaimList := &corev1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
	}

	return persistentVolumeClaimList, rc.Client.List(rc.Ctx, persistentVolumeClaimList, listOptions)
}
