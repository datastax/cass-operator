// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/datastax/cass-operator/operator/pkg/utils"
)

func (rc *ReconciliationContext) CreateHeadlessServices() result.ReconcileResult {
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
			return result.Error(err)
		}

		if err := client.Create(rc.Ctx, service); err != nil {
			logger.Error(err, "Could not create headless service")

			return result.Error(err)
		}

		rc.Recorder.Eventf(rc.Datacenter, "Normal", "CreatedResource", "Created service %s", service.Name)
	}

	// at this point we had previously been saying this reconcile call was over, we're done
	// but that seems wrong, we should just continue on to the next resources
	return result.Continue()
}

// ReconcileHeadlessService ...
func (rc *ReconciliationContext) CheckHeadlessServices() result.ReconcileResult {
	// unpacking
	logger := rc.ReqLogger
	dc := rc.Datacenter
	client := rc.Client

	logger.Info("reconcile_services::ReconcileHeadlessServices")

	// Check if there is a headless service for the cluster

	cqlService := newServiceForCassandraDatacenter(dc)
	seedService := newSeedServiceForCassandraDatacenter(dc)
	allPodsService := newAllPodsServiceForCassandraDatacenter(dc)

	services := []*corev1.Service{cqlService, seedService, allPodsService}

	createNeeded := []*corev1.Service{}

	for idx := range services {
		desiredSvc := services[idx]

		// Set CassandraDatacenter dc as the owner and controller
		err := setControllerReference(dc, desiredSvc, rc.Scheme)
		if err != nil {
			logger.Error(err, "Could not set controller reference for headless service")
			return result.Error(err)
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
			logger.Error(err, "Could not get headless seed service",
				"name", nsName,
			)
			return result.Error(err)

		} else {
			// if we found the service already, check if they need updating
			if !resourcesHaveSameHash(currentService, desiredSvc) {
				// preserve any labels and annotations that were added to the service post-creation
				desiredSvc.Labels = utils.MergeMap(map[string]string{}, currentService.Labels, desiredSvc.Labels)
				desiredSvc.Annotations = utils.MergeMap(map[string]string{}, currentService.Annotations, desiredSvc.Annotations)

				logger.Info("Updating service",
					"service", currentService,
					"desired", desiredSvc)

				desiredSvc.DeepCopyInto(currentService)

				if err := client.Update(rc.Ctx, currentService); err != nil {
					logger.Error(err, "Unable to update service",
						"service", currentService)
					return result.Error(err)
				}
			}
		}
	}

	if len(createNeeded) > 0 {
		rc.Services = createNeeded
		return rc.CreateHeadlessServices()
	}

	return result.Continue()
}
