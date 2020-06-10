package reconciliation

import (
	"github.com/stretchr/testify/assert"
	v1batch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

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
