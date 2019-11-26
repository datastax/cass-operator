package reconciliation

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation/reconcileriface"
)

// ReconcileServices ...
type ReconcileServices struct {
	ReconcileContext *dsereconciliation.ReconciliationContext
	Services         []*corev1.Service
}

// Apply ...
func (r *ReconcileServices) Apply() (reconcile.Result, error) {
	// unpacking
	recCtx := r.ReconcileContext
	logger := recCtx.ReqLogger
	client := recCtx.Client

	for idx := range r.Services {
		service := r.Services[idx]

		logger.Info(
			"Creating a new headless service",
			"serviceNamespace", service.Namespace,
			"serviceName", service.Name)

		if err := addOperatorProgressLabel(recCtx, updating); err != nil {
			return reconcile.Result{Requeue: true}, err
		}

		if err := client.Create(recCtx.Ctx, service); err != nil {
			logger.Error(
				err, "Could not create headless service")

			return reconcile.Result{Requeue: true}, err
		}
	}

	return reconcile.Result{Requeue: true}, nil
}

// ReconcileHeadlessService ...
func (r *ReconcileServices) ReconcileHeadlessServices() (reconcileriface.Reconciler, error) {
	// unpacking
	recCtx := r.ReconcileContext
	logger := recCtx.ReqLogger
	dseDatacenter := recCtx.DseDatacenter
	client := recCtx.Client

	logger.Info("reconcile_services::ReconcileHeadlessServices")

	// Check if there is a headless service for the cluster

	cqlService := newServiceForDseDatacenter(dseDatacenter)
	seedService := newSeedServiceForDseDatacenter(dseDatacenter)
	allPodsService := newAllDsePodsServiceForDseDatacenter(dseDatacenter)

	services := []*corev1.Service{cqlService, seedService, allPodsService}

	var reconciler ReconcileServices
	reconciler.ReconcileContext = recCtx

	createNeeded := []*corev1.Service{}

	for idx := range services {
		desiredSvc := services[idx]

		// Set DseDatacenter dseDatacenter as the owner and controller
		err := setControllerReference(dseDatacenter, desiredSvc, recCtx.Scheme)
		if err != nil {
			logger.Error(
				err, "Could not set controller reference for headless service")
			return nil, err
		}

		// See if the service already exists
		nsName := types.NamespacedName{Name: desiredSvc.Name, Namespace: desiredSvc.Namespace}
		currentService := &corev1.Service{}
		err = client.Get(recCtx.Ctx, nsName, currentService)

		if err != nil && errors.IsNotFound(err) {
			// if it's not found, put the service in the slice to be created when Apply is called
			createNeeded = append(createNeeded, desiredSvc)

		} else if err != nil {
			// if we hit a k8s error, log it and error out
			logger.Error(
				err, "Could not get headless seed service",
				"name", nsName,
			)
			return nil, err

		} else {
			// if we found the service already, check if the labels are right
			currentLabels := currentService.GetLabels()
			desiredLabels := desiredSvc.GetLabels()
			shouldUpdateLabels := !reflect.DeepEqual(currentLabels, desiredLabels)
			if shouldUpdateLabels {
				logger.Info("Updating labels",
					"service", currentService,
					"current", currentLabels,
					"desired", desiredLabels)
				currentService.SetLabels(desiredLabels)

				if err := client.Update(recCtx.Ctx, currentService); err != nil {
					logger.Error(err, "Unable to update service with labels",
						"service", currentService)
					return nil, err
				}
			}
		}
	}

	if len(createNeeded) > 0 {
		reconciler.Services = createNeeded
		return &reconciler, nil
	}

	//
	return nil, nil
}
