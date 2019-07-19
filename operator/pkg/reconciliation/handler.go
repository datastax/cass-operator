package reconciliation

//
// This file defines handlers for events on the EventBus
//

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"
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
	reconcileSeedServices ReconcileSeedServices,
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

	if err := addOperatorProgressLabel(rc, updating); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	if err := reconciler.addFinalizer(rc); err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	// order of this list matters!
	reconcilers := []reconcileFun{
		reconcileServices.ReconcileHeadlessService,
		reconcileSeedServices.ReconcileHeadlessSeedService,
		reconcileRacks.CalculateRackInformation}

	for _, fun := range reconcilers {
		if rec, err := fun(); err != nil || rec != nil {
			if err != nil {
				return reconcile.Result{Requeue: true}, err
			}
			recResult, recErr := rec.Apply()

			return recResult, recErr
		}
	}

	// no more changes to make!

	if err := addOperatorProgressLabel(rc, ready); err != nil {
		// this error is especially sad because we were just about to be done reconciling
		return reconcile.Result{Requeue: true}, err
	}

	// nothing happened so return and don't requeue
	return reconcile.Result{}, nil
}
