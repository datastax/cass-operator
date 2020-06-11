package reconciliation

import (
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestReconcileReaper_buildInitReaperSchemaJob(t *testing.T) {
	dc := &api.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ReaperSchemaJobTest",
			Namespace: "reaper",
			Labels: map[string]string{
				api.DatacenterLabel: "ReaperSchemaJobTest",
				api.ClusterLabel: "reaper",
			},
		},
		Spec: api.CassandraDatacenterSpec{
			Size:          6,
			ClusterName:   "reaper",
			ServerType:    "Cassandra",
			ServerVersion: "3.11.6",
		},
	}

	job := buildInitReaperSchemaJob(dc)

	assert.Equal(t, getReaperSchemaInitJobName(dc), job.Name)
	assert.Equal(t, dc.GetDatacenterLabels(), job.Labels)

	assert.Equal(t, 1, len(job.Spec.Template.Spec.Containers))
	container := job.Spec.Template.Spec.Containers[0]

	assert.Equal(t, ReaperSchemaInitJobImage, container.Image)

	expectedEnvVars := []corev1.EnvVar{
		{Name: "KEYSPACE", Value: ReaperKeyspace},
		{Name: "CONTACT_POINTS", Value: dc.GetSeedServiceName()},
		{Name: "REPLICATION", Value: "{'class': 'NetworkTopologyStrategy', 'ReaperSchemaJobTest': 3}"},
	}
	assert.ElementsMatch(t, expectedEnvVars, container.Env)
}

func TestReconcileReaper_CheckReaperSchemaInitialized(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	rc.Datacenter.Spec.Reaper.Enabled = true
	defer cleanupMockScr()

	trackObjects := []runtime.Object{rc.Datacenter}

	rc.Client = fake.NewFakeClient(trackObjects...)

	reconcileResult := rc.CheckReaperSchemaInitialized()
	assert.True(t, reconcileResult.Completed())

	result, err := reconcileResult.Output()

	assert.NoError(t, err)
	assert.True(t, result.Requeue, "should requeue request")

	job := &v1batch.Job{}
	jobName := getReaperSchemaInitJobName(rc.Datacenter)
	err = rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: jobName}, job)

	assert.NoErrorf(t, err, "failed to get job %s", jobName)
}

func TestReconcileReaper_CheckReaperSchemaNotInitialized(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	trackObjects := []runtime.Object{rc.Datacenter}

	rc.Client = fake.NewFakeClient(trackObjects...)

	reconcileResult := rc.CheckReaperSchemaInitialized()
	assert.False(t, reconcileResult.Completed())

	result, err := reconcileResult.Output()

	assert.NoError(t, err)
	assert.True(t, result.Requeue, "should requeue request")

	job := &v1batch.Job{}
	jobName := getReaperSchemaInitJobName(rc.Datacenter)
	err = rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: jobName}, job)

	assert.True(t, errors.IsNotFound(err), "did not expect to find job %s", jobName)
}
