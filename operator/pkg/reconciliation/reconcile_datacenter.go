package reconciliation

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"
)

// ReconcileDatacenter ...
type ReconcileDatacenter struct {
	ReconcileContext *dsereconciliation.ReconciliationContext
}

// Apply ...
func (r *ReconcileDatacenter) Apply() (reconcile.Result, error) {
	// set the label here but no need to remove since we're deleting the DseDatacenter
	if err := addOperatorProgressLabel(r.ReconcileContext, updating); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	if err := r.deletePVCs(); err != nil {
		r.ReconcileContext.ReqLogger.Error(err, "Failed to delete PVCs for DseDatacenter")
		return reconcile.Result{Requeue: true}, err
	}

	// Update finalizer to allow delete of DseDatacenter
	r.ReconcileContext.Datacenter.SetFinalizers(nil)

	// Update DseDatacenter
	if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, r.ReconcileContext.Datacenter); err != nil {
		r.ReconcileContext.ReqLogger.Error(err, "Failed to update DseDatacenter with removed finalizers")
		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

// ProcessDeletion ...
func (r *ReconcileDatacenter) ProcessDeletion() (reconcileriface.Reconciler, error) {
	if r.ReconcileContext.Datacenter.GetDeletionTimestamp() != nil {
		return &ReconcileDatacenter{
			ReconcileContext: r.ReconcileContext,
		}, nil
	}

	return nil, nil
}

func (r *ReconcileDatacenter) deletePVCs() error {
	r.ReconcileContext.ReqLogger.Info("reconciler::deletePVCs")

	persistentVolumeClaimList, err := r.listPVCs()
	if err != nil {
		if errors.IsNotFound(err) {
			r.ReconcileContext.ReqLogger.Info(
				"No PVCs found for DseDatacenter",
				"dseDatacenterNamespace", r.ReconcileContext.Datacenter.Namespace,
				"dseDatacenterName", r.ReconcileContext.Datacenter.Name)
			return nil
		}
		r.ReconcileContext.ReqLogger.Error(err,
			"Failed to list PVCs for dseDatacenter",
			"dseDatacenterNamespace", r.ReconcileContext.Datacenter.Namespace,
			"dseDatacenterName", r.ReconcileContext.Datacenter.Name)
		return err
	}

	r.ReconcileContext.ReqLogger.Info(
		fmt.Sprintf("Found %d PVCs for dseDatacenter", len(persistentVolumeClaimList.Items)),
		"dseDatacenterNamespace", r.ReconcileContext.Datacenter.Namespace,
		"dseDatacenterName", r.ReconcileContext.Datacenter.Name)

	for _, pvc := range persistentVolumeClaimList.Items {
		if err := r.ReconcileContext.Client.Delete(r.ReconcileContext.Ctx, &pvc); err != nil {
			r.ReconcileContext.ReqLogger.Error(err,
				"Failed to delete PVCs for dseDatacenter",
				"dseDatacenterNamespace", r.ReconcileContext.Datacenter.Namespace,
				"dseDatacenterName", r.ReconcileContext.Datacenter.Name)
			return err
		}
		r.ReconcileContext.ReqLogger.Info(
			"Deleted PVC",
			"pvcNamespace", pvc.Namespace,
			"pvcName", pvc.Name)
	}

	return nil
}

func (r *ReconcileDatacenter) listPVCs() (*corev1.PersistentVolumeClaimList, error) {
	r.ReconcileContext.ReqLogger.Info("reconciler::listPVCs")

	selector := map[string]string{
		datastaxv1alpha1.DatacenterLabel: r.ReconcileContext.Datacenter.Name,
	}

	listOptions := &client.ListOptions{
		Namespace:     r.ReconcileContext.Datacenter.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	persistentVolumeClaimList := &corev1.PersistentVolumeClaimList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
	}

	return persistentVolumeClaimList, r.ReconcileContext.Client.List(r.ReconcileContext.Ctx, persistentVolumeClaimList, listOptions)
}
