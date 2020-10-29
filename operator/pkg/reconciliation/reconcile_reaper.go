// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"math"
	"strconv"

	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/images"
	reapergo "github.com/jsanda/reaper-client-go/reaper"
)

const (
	ReaperUIPort            = 7080
	ReaperAdminPort         = 7081
	ReaperDefaultPullPolicy = corev1.PullIfNotPresent
	ReaperContainerName     = "reaper"
	ReaperHealthCheckPath   = "/healthcheck"
	ReaperKeyspace          = "reaper_db"
	ReaperSchemaInitJob     = "ReaperSchemaInitJob"
	// This code currently lives at https://github.com/jsanda/create_keyspace.
	ReaperSchemaInitJobImage = "jsanda/reaper-init-keyspace:latest"
)

// We are passing in container by reference because the user may have already provided a
// reaper container with desired settings in CassandraDatacenter.Spec.PodTemplateSpec.
// Therefore we will simply fill-in any missing defaults inside of buildReaperContainer.
// TODO: provide more unit testing for building reaper containers.
func buildReaperContainer(dc *api.CassandraDatacenter, container *corev1.Container) {

	ports := []corev1.ContainerPort{
		{Name: "ui", ContainerPort: ReaperUIPort, Protocol: "TCP"},
		{Name: "admin", ContainerPort: ReaperAdminPort, Protocol: "TCP"},
	}

	container.Name = ReaperContainerName
	container.Image = getReaperImage(dc)
	container.ImagePullPolicy = getReaperPullPolicy(dc)

	container.Ports = combinePortSlices(ports, container.Ports)

	if container.LivenessProbe == nil {
		container.LivenessProbe = probe(ReaperAdminPort, ReaperHealthCheckPath, int(60*dc.Spec.Size), 10)
	}

	if container.ReadinessProbe == nil {
		container.ReadinessProbe = probe(ReaperAdminPort, ReaperHealthCheckPath, 30, 15)
	}

	envDefaults := []corev1.EnvVar{
		{Name: "REAPER_STORAGE_TYPE", Value: "cassandra"},
		{Name: "REAPER_ENABLE_DYNAMIC_SEED_LIST", Value: "false"},
		{Name: "REAPER_DATACENTER_AVAILABILITY", Value: "SIDECAR"},
		{Name: "REAPER_SERVER_APP_PORT", Value: strconv.Itoa(ReaperUIPort)},
		{Name: "REAPER_SERVER_ADMIN_PORT", Value: strconv.Itoa(ReaperAdminPort)},
		{Name: "REAPER_CASS_CLUSTER_NAME", Value: dc.ClusterName},
		{Name: "REAPER_CASS_CONTACT_POINTS", Value: fmt.Sprintf("[%s]", dc.GetSeedServiceName())},
		{Name: "REAPER_AUTH_ENABLED", Value: "false"},
		{Name: "REAPER_JMX_AUTH_USERNAME", Value: ""},
		{Name: "REAPER_JMX_AUTH_PASSWORD", Value: ""},
	}

	container.Env = combineEnvSlices(envDefaults, container.Env)

	container.Resources = *getResourcesOrDefault(&dc.Spec.Reaper.Resources, &DefaultsReaperContainer)
}

func getReaperImage(dc *api.CassandraDatacenter) string {
	if len(dc.Spec.Reaper.Image) == 0 {
		return images.GetReaperImage()
	}
	return dc.Spec.Reaper.Image
}

func getReaperPullPolicy(dc *api.CassandraDatacenter) corev1.PullPolicy {
	if len(dc.Spec.Reaper.ImagePullPolicy) == 0 {
		return ReaperDefaultPullPolicy
	}
	return dc.Spec.Reaper.ImagePullPolicy
}

func (rc *ReconciliationContext) CheckReaperSchemaInitialized() result.ReconcileResult {
	// Using a job eventually get replaced with calls to the mgmt api once it has support for
	// creating keyspaces and tables.

	rc.ReqLogger.Info("reconcile_reaper::CheckReaperSchemaInitialized")

	if rc.Datacenter.Spec.Reaper == nil || !rc.Datacenter.Spec.Reaper.Enabled {
		return result.Continue()
	}

	jobName := getReaperSchemaInitJobName(rc.Datacenter)
	schemaJob := &v1batch.Job{}

	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: jobName}, schemaJob)
	if err != nil && errors.IsNotFound(err) {
		// Create the job
		schemaJob := buildInitReaperSchemaJob(rc.Datacenter)
		rc.ReqLogger.Info("creating Reaper schema init job", ReaperSchemaInitJob, schemaJob.Name)
		if err := setControllerReference(rc.Datacenter, schemaJob, rc.Scheme); err != nil {
			rc.ReqLogger.Error(err, "failed to set owner reference", ReaperSchemaInitJob, schemaJob.Name)
			return result.Error(err)
		}
		if err := rc.Client.Create(rc.Ctx, schemaJob); err != nil {
			rc.ReqLogger.Error(err, "failed to create job", ReaperSchemaInitJob, schemaJob.Name)
			return result.Error(err)
		} else {
			return result.RequeueSoon(2)
		}
	} else if err != nil {
		return result.Error(err)
	} else if jobFinished(schemaJob) {
		return result.Continue()
	} else {
		return result.RequeueSoon(2)
	}
}

func getReaperSchemaInitJobName(dc *api.CassandraDatacenter) string {
	return fmt.Sprintf("%s-reaper-init-schema", dc.Spec.ClusterName)
}

func getReaperReplication(dc *api.CassandraDatacenter) string {
	replicationFactor := int(math.Min(float64(dc.Spec.Size), 3))
	return fmt.Sprintf("{'class': 'NetworkTopologyStrategy', '%s': %d}", dc.Name, replicationFactor)
}

func jobFinished(job *v1batch.Job) bool {
	for _, c := range job.Status.Conditions {
		if (c.Type == v1batch.JobComplete || c.Type == v1batch.JobFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func (rc *ReconciliationContext) CheckReaperService() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_reaper::CheckReaperService")

	serviceName := getReaperServiceName(rc.Datacenter)
	service := &corev1.Service{}

	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: serviceName}, service)
	if err != nil && errors.IsNotFound(err) {
		if rc.Datacenter.Spec.Reaper != nil && rc.Datacenter.Spec.Reaper.Enabled {
			// Create the service
			service = newReaperService(rc.Datacenter)
			rc.ReqLogger.Info("creating Reaper service")
			if err := setControllerReference(rc.Datacenter, service, rc.Scheme); err != nil {
				rc.ReqLogger.Error(err, "failed to set owner reference", "ReaperService", serviceName)
				return result.Error(err)
			}
			if err := rc.Client.Create(rc.Ctx, service); err != nil {
				rc.ReqLogger.Error(err, "failed to create Reaper service")
				return result.Error(err)
			}
			return result.Continue()
		}
	} else if err != nil {
		return result.Error(err)
	} else if rc.Datacenter.Spec.Reaper == nil || !rc.Datacenter.Spec.Reaper.Enabled {
		if err := rc.Client.Delete(rc.Ctx, service); err != nil {
			rc.ReqLogger.Error(err, "failed to delete Reaper service", "ReaperService", serviceName)
		}
	}
	return result.Continue()
}

// Makes sure that the cluster is registered with Reaper. It is assumed that this cluster
// is also the backend storage for Reaper. Reaper won't be ready and won't be able to serve
// REST api calls until schema initialization in its backend storage is complete (chicken
// meet egg). To avoid a lot of thrashing this function first checks that Reaper is ready.
// It will requeue the reconciliation request until Reaper is ready.
func (rc *ReconciliationContext) CheckRegisteredWithReaper() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_reaper::CheckRegisteredWithReaper")

	if !rc.Datacenter.IsReaperEnabled() {
		return result.Continue()
	}

	// TODO Do not hard code the port
	restClient, err := reapergo.NewReaperClient(fmt.Sprintf("http://%s:8080", rc.Datacenter.Spec.Reaper.Service))
	if err != nil {
		rc.ReqLogger.Error(err, "failed to create Reaper REST client", "ReaperService", rc.Datacenter.Spec.Reaper.Service)
		return result.Error(err)
	}

	if isReady, err := restClient.IsReaperUp(rc.Ctx); err == nil {
		if !isReady {
			rc.ReqLogger.Info("waiting for reaper to become ready")
			return result.RequeueSoon(10)
		}
	} else {
		rc.ReqLogger.Error(err, "reaper readiness check failed")
		// We return result.RequestSoon here instead of result.Error because Reaper does not
		// start serving requests, including for its health check point, until schema
		// initialization has completed.
		return result.RequeueSoon(30)
	}

	if err := restClient.AddCluster(rc.Ctx, rc.Datacenter.Spec.ClusterName,rc.Datacenter.GetDatacenterServiceName()); err == nil {
		return result.Continue()
	} else {
		rc.ReqLogger.Error(err,"failed to register cluster with Reaper")
		return result.Error(err)
	}
}

func getReaperServiceName(dc *api.CassandraDatacenter) string {
	return fmt.Sprintf("%s-reaper-service", dc.Spec.ClusterName)
}

func newReaperService(dc *api.CassandraDatacenter) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getReaperServiceName(dc),
			Namespace: dc.Namespace,
			Labels:    dc.GetDatacenterLabels(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     ReaperUIPort,
					Name:     "ui",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "ui",
					},
				},
			},
			Selector: dc.GetDatacenterLabels(),
		},
	}
}
