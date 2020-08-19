// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"math"
	"strconv"
)

const (
	ReaperUIPort             = 7080
	ReaperAdminPort          = 7081
	ReaperDefaultImage     = "thelastpickle/cassandra-reaper:2.0.5"
	ReaperDefaultPullPolicy  = corev1.PullIfNotPresent
	ReaperContainerName      = "reaper"
	ReaperHealthCheckPath    = "/healthcheck"
	ReaperKeyspace           = "reaper_db"
	ReaperSchemaInitJob      = "ReaperSchemaInitJob"
	// This code currently lives at https://github.com/jsanda/create_keyspace.
	ReaperSchemaInitJobImage = "jsanda/reaper-init-keyspace:latest"
)

func buildReaperContainer(dc *api.CassandraDatacenter) corev1.Container {
	ports := []corev1.ContainerPort{
		{Name: "ui", ContainerPort: ReaperUIPort, Protocol: "TCP"},
		{Name: "admin", ContainerPort: ReaperAdminPort, Protocol: "TCP"},
	}

	container := corev1.Container{
		Name: ReaperContainerName,
		Image: getReaperImage(dc),
		ImagePullPolicy: getReaperPullPolicy(dc),
		Ports: ports,
		LivenessProbe: probe(ReaperAdminPort, ReaperHealthCheckPath, int(60 * dc.Spec.Size), 10),
		ReadinessProbe: probe(ReaperAdminPort, ReaperHealthCheckPath, 30, 15),
		Env: []corev1.EnvVar{
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
		},
		Resources: *getResourcesOrDefault(&dc.Spec.Reaper.Resources, &DefaultsReaperContainer),
	}

	return container
}

func getReaperImage(dc *api.CassandraDatacenter) string {
	if len(dc.Spec.Reaper.Image) == 0 {
		return ReaperDefaultImage
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

func getReaperServiceName(dc *api.CassandraDatacenter) string {
	return fmt.Sprintf("%s-reaper-service", dc.Spec.ClusterName)
}

func newReaperService(dc *api.CassandraDatacenter) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
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
					Port: ReaperUIPort,
					Name: "ui",
					Protocol: corev1.ProtocolTCP,
					TargetPort: intstr.IntOrString{
						Type: intstr.String,
						StrVal: "ui",
					},
				},
			},
			Selector: dc.GetDatacenterLabels(),
		},
	}
}