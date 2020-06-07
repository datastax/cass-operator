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
	"math"
	"strconv"
)

const (
	ReaperUIPort          = 7080
	ReaperAdminPort       = 7081
	ReaperDefaultImage    = "thelastpickle/cassandra-reaper:2.0.5"
	ReaperContainerName   = "reaper"
	ReaperHealthCheckPath = "/healthcheck"
	ReaperKeyspace        = "reaper_db"
)

func buildReaperContainer(dc *api.CassandraDatacenter) corev1.Container {
	ports := []corev1.ContainerPort{
		{Name: "ui", ContainerPort: ReaperUIPort, Protocol: "TCP"},
		{Name: "admin", ContainerPort: ReaperAdminPort, Protocol: "TCP"},
	}

	container := corev1.Container{
		Name: ReaperContainerName,
		Image: ReaperDefaultImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: ports,
		LivenessProbe: probe(ReaperAdminPort, ReaperHealthCheckPath, 30, 10),
		// TODO Calculate the initial delay based on the initial delay of the Cassandra containers and the size
		//      of the Cassandra cluster. The readiness probe will fail until the Reaper keyspace is created,
		//      and that cannot happen until the C* cluster is ready.
		ReadinessProbe: probe(ReaperAdminPort, ReaperHealthCheckPath, 180, 15),
		Env: []corev1.EnvVar{
			{Name: "REAPER_STORAGE_TYPE", Value: "cassandra"},
			{Name: "REAPER_ENABLE_DYNAMIC_SEED_LIST", Value: "false"},
			{Name: "REAPER_DATACENTER_AVAILABILITY", Value: "SIDECAR"},
			{Name: "REAPER_SERVER_APP_PORT", Value: strconv.Itoa(ReaperUIPort)},
			{Name: "REAPER_SERVER_ADMIN_PORT", Value: strconv.Itoa(ReaperAdminPort)},
			{Name: "REAPER_CASS_CLUSTER_NAME", Value: dc.ClusterName},
			{Name: "REAPER_CASS_CONTACT_POINTS", Value: fmt.Sprintf("[%s]", dc.GetSeedServiceName())},
		},
	}

	return container
}

func (rc *ReconciliationContext) CheckReaperSchemaInitialized() result.ReconcileResult {
	rc.ReqLogger.Info("reconcile_reaper::CheckReaperSchemaInitialized")

	jobName := getReaperSchemaInitJobName(rc.Datacenter)
	schemaJob := &v1batch.Job{}

	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: jobName}, schemaJob)
	if err != nil && errors.IsNotFound(err) {
		// Create the job
		schemaJob := buildInitReaperSchemaJob(rc)
		rc.ReqLogger.Info("creating Reaper schema init job", "ReaperSchemaInitJob", schemaJob.Name)
		if err := setControllerReference(rc.Datacenter, schemaJob, rc.Scheme); err != nil {
			rc.ReqLogger.Error(err, "failed to set owner reference", "ReaperSchemaInitJob", schemaJob.Name)
			return result.Error(err)
		}
		if err := rc.Client.Create(rc.Ctx, schemaJob); err != nil {
			rc.ReqLogger.Error(err, "failed to create job", "ReaperSchemaInitJob", schemaJob.Name)
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

func buildInitReaperSchemaJob(rc *ReconciliationContext) *v1batch.Job {
	return &v1batch.Job{
		TypeMeta: metav1.TypeMeta{
			Kind: "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: rc.Datacenter.Namespace,
			Name: getReaperSchemaInitJobName(rc.Datacenter),
			Labels: rc.Datacenter.GetDatacenterLabels(),
		},
		Spec: v1batch.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:getReaperSchemaInitJobName(rc.Datacenter),
							Image: "jsanda/reaper-init-keyspace:latest",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: "KEYSPACE",
									Value: ReaperKeyspace,
								},
								{
									Name: "CONTACT_POINTS",
									Value: rc.Datacenter.GetSeedServiceName(),
								},
								// TODO Add replication_factor. There is already a function in tlp-stress-operator
								//      that does the serialization. I need to move that function to a shared lib.
								{
									Name: "REPLICATION",
									Value: getReaperReplication(rc.Datacenter),
								},
							},
						},
					},
				},
			},
		},
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