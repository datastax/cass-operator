package reconciliation

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"
)

type ReconcileServices struct {
	ReconcileContext *dsereconciliation.ReconciliationContext
	Service          *corev1.Service
}

type ReconcileSeedServices struct {
	ReconcileContext *dsereconciliation.ReconciliationContext
	Service          *corev1.Service
}

func (r *ReconcileServices) Apply() (reconcile.Result, error) {
	r.ReconcileContext.ReqLogger.Info(
		"Creating a new headless service",
		"serviceNamespace",
		r.Service.Namespace,
		"serviceName",
		r.Service.Name)

	err := r.ReconcileContext.Client.Create(
		r.ReconcileContext.Ctx,
		r.Service)
	if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not create headless service")

		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{Requeue: true}, nil
}

func (r *ReconcileServices) ReconcileHeadlessService() (reconcileriface.Reconciler, error) {
	r.ReconcileContext.ReqLogger.Info("reconcile_services::reconcileHeadlessService")

	desiredService := newServiceForDseDatacenter(r.ReconcileContext.DseDatacenter)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err := setControllerReference(
		r.ReconcileContext.DseDatacenter,
		desiredService,
		r.ReconcileContext.Scheme)
	if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not set controller reference for headless service")
		return nil, err
	}

	currentService := &corev1.Service{}
	err = r.ReconcileContext.Client.Get(
		r.ReconcileContext.Ctx,
		types.NamespacedName{
			Name:      desiredService.Name,
			Namespace: desiredService.Namespace},
		currentService)

	if err != nil && errors.IsNotFound(err) {
		return &ReconcileServices{
			ReconcileContext: r.ReconcileContext,
			Service:          desiredService,
		}, nil
	} else if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not get headless seed service")

		return nil, err
	}

	svcLabels := currentService.GetLabels()
	shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForDatacenterResource(svcLabels, r.ReconcileContext.DseDatacenter)

	if shouldUpdateLabels {
		r.ReconcileContext.ReqLogger.Info("Updating labels",
			"service", currentService,
			"current", svcLabels,
			"desired", updatedLabels)
		currentService.SetLabels(updatedLabels)

		if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, currentService); err != nil {
			r.ReconcileContext.ReqLogger.Info("Unable to update service with labels",
				"service",
				currentService)
		}
	}

	return nil, nil
}

//
// Create a headless service for this datacenter.
//

func (r *ReconcileSeedServices) Apply() (reconcile.Result, error) {
	r.ReconcileContext.ReqLogger.Info(
		"Creating a new headless seed service",
		"serviceNamespace",
		r.Service.Namespace,
		"serviceName",
		r.Service.Name)

	err := r.ReconcileContext.Client.Create(
		r.ReconcileContext.Ctx,
		r.Service)
	if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not create headless service")

		return reconcile.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileSeedServices) ReconcileHeadlessSeedService() (reconcileriface.Reconciler, error) {

	r.ReconcileContext.ReqLogger.Info("reconcile_services::reconcileHeadlessSeedService")

	//
	// Check if there is a headless seed service for the cluster
	//

	desiredService := newSeedServiceForDseDatacenter(r.ReconcileContext.DseDatacenter)

	// Set DseDatacenter dseDatacenter as the owner and controller
	err := setControllerReference(
		r.ReconcileContext.DseDatacenter,
		desiredService,
		r.ReconcileContext.Scheme)
	if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not set controller reference for headless seed service")
		return nil, err
	}

	currentService := &corev1.Service{}

	err = r.ReconcileContext.Client.Get(
		r.ReconcileContext.Ctx,
		types.NamespacedName{
			Name:      desiredService.Name,
			Namespace: desiredService.Namespace},
		currentService)

	if err != nil && errors.IsNotFound(err) {
		return &ReconcileSeedServices{
			ReconcileContext: r.ReconcileContext,
			Service:          desiredService,
		}, nil
	} else if err != nil {
		r.ReconcileContext.ReqLogger.Error(
			err,
			"Could not get headless seed service")

		return nil, err
	}

	svcLabels := currentService.GetLabels()
	shouldUpdateLabels, updatedLabels := shouldUpdateLabelsForDatacenterResource(svcLabels, r.ReconcileContext.DseDatacenter)

	if shouldUpdateLabels {
		r.ReconcileContext.ReqLogger.Info("Updating labels",
			"service", currentService,
			"current", svcLabels,
			"desired", updatedLabels)
		currentService.SetLabels(updatedLabels)

		if err := r.ReconcileContext.Client.Update(r.ReconcileContext.Ctx, currentService); err != nil {
			r.ReconcileContext.ReqLogger.Info("Unable to update service with labels",
				"service",
				currentService)
		}
	}

	return nil, nil
}
