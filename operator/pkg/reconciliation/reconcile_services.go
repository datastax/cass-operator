package reconciliation

import (
	"reflect"

	api "github.com/riptano/dse-operator/operator/pkg/apis/cassandra/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (rc *ReconciliationContext) CreateHeadlessServices() (reconcile.Result, error) {
	// unpacking
	logger := rc.ReqLogger
	client := rc.Client

	for idx := range rc.Services {
		service := rc.Services[idx]

		logger.Info(
			"Creating a new headless service",
			"serviceNamespace", service.Namespace,
			"serviceName", service.Name)

		if err := setOperatorProgressStatus(rc, api.ProgressUpdating); err != nil {
			return reconcile.Result{Requeue: true}, err
		}

		if err := client.Create(rc.Ctx, service); err != nil {
			logger.Error(
				err, "Could not create headless service")

			return reconcile.Result{Requeue: true}, err
		}

		rc.Recorder.Eventf(rc.Datacenter, "Normal", "CreatedResource", "Created service %s", service.Name)
	}

	return reconcile.Result{Requeue: true}, nil
}

// ReconcileHeadlessService ...
func (rc *ReconciliationContext) CheckHeadlessServices() (bool, error) {
	// unpacking
	logger := rc.ReqLogger
	dc := rc.Datacenter
	client := rc.Client

	logger.Info("reconcile_services::ReconcileHeadlessServices")

	// Check if there is a headless service for the cluster

	cqlService := newServiceForCassandraDatacenter(dc)
	seedService := newSeedServiceForCassandraDatacenter(dc)
	allPodsService := newAllDsePodsServiceForCassandraDatacenter(dc)

	services := []*corev1.Service{cqlService, seedService, allPodsService}

	createNeeded := []*corev1.Service{}

	for idx := range services {
		desiredSvc := services[idx]

		// Set CassandraDatacenter dc as the owner and controller
		err := setControllerReference(dc, desiredSvc, rc.Scheme)
		if err != nil {
			logger.Error(
				err, "Could not set controller reference for headless service")
			return false, err
		}

		// See if the service already exists
		nsName := types.NamespacedName{Name: desiredSvc.Name, Namespace: desiredSvc.Namespace}
		currentService := &corev1.Service{}
		err = client.Get(rc.Ctx, nsName, currentService)

		if err != nil && errors.IsNotFound(err) {
			// if it's not found, put the service in the slice to be created when Apply is called
			createNeeded = append(createNeeded, desiredSvc)

		} else if err != nil {
			// if we hit a k8s error, log it and error out
			logger.Error(
				err, "Could not get headless seed service",
				"name", nsName,
			)
			return false, err

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

				if err := client.Update(rc.Ctx, currentService); err != nil {
					logger.Error(err, "Unable to update service with labels",
						"service", currentService)
					return false, err
				}
			}
		}
	}

	if len(createNeeded) > 0 {
		rc.Services = createNeeded
		return true, nil
	}

	return false, nil
}
