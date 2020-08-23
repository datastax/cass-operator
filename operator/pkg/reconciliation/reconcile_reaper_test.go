package reconciliation

import (
	"testing"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	v1batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileReaper_buildInitReaperSchemaJob(t *testing.T) {
	dc := newCassandraDatacenter()
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

func TestReconcileReaper_newReaperService(t *testing.T) {
	dc := newCassandraDatacenter()
	service := newReaperService(dc)

	assert.Equal(t, getReaperServiceName(dc), service.Name)
	assert.Equal(t, dc.GetDatacenterLabels(), service.Labels)
	assert.Equal(t, 1, len(service.Spec.Ports))

	port := service.Spec.Ports[0]
	assert.Equal(t, int32(ReaperUIPort), port.Port)
	assert.Equal(t, dc.GetDatacenterLabels(), service.Spec.Selector)
}

func TestReconcileReaper_CheckReaperSchemaInitialized(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	rc.Datacenter.Spec.Reaper = &api.ReaperConfig{Enabled: true}
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

	job := &v1batch.Job{}
	jobName := getReaperSchemaInitJobName(rc.Datacenter)
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: jobName}, job)

	assert.True(t, errors.IsNotFound(err), "did not expect to find job %s", jobName)
}

func TestReconcileReaper_CheckReaperService(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	rc.Datacenter.Spec.Reaper = &api.ReaperConfig{Enabled: true}

	trackObjects := []runtime.Object{rc.Datacenter}

	rc.Client = fake.NewFakeClient(trackObjects...)

	reconcileResult := rc.CheckReaperService()
	assert.False(t, reconcileResult.Completed())

	service := &corev1.Service{}
	serviceName := getReaperServiceName(rc.Datacenter)
	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: serviceName}, service)

	assert.NoErrorf(t, err, "failed to get service %s", serviceName)
}

func TestReconcileReaper_CheckReaperServiceReaperDisabled(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	serviceName := getReaperServiceName(rc.Datacenter)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: rc.Datacenter.Namespace,
			Name:      serviceName,
		},
	}

	trackObjects := []runtime.Object{rc.Datacenter, service}

	rc.Client = fake.NewFakeClient(trackObjects...)

	reconcileResult := rc.CheckReaperService()
	assert.False(t, reconcileResult.Completed())

	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: serviceName}, service)

	assert.True(t, errors.IsNotFound(err), "did not expect to find service %s", serviceName)
}

func newCassandraDatacenter() *api.CassandraDatacenter {
	return &api.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ReaperSchemaJobTest",
			Namespace: "reaper",
			Labels: map[string]string{
				api.DatacenterLabel: "ReaperSchemaJobTest",
				api.ClusterLabel:    "reaper",
			},
		},
		Spec: api.CassandraDatacenterSpec{
			Size:          6,
			ClusterName:   "reaper",
			ServerType:    "Cassandra",
			ServerVersion: "3.11.7",
		},
	}
}
