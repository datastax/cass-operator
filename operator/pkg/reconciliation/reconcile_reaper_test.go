package reconciliation

import (
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//func TestReconcileReaper_CheckReaperServiceReaperDisabled(t *testing.T) {
//	rc, _, cleanupMockScr := setupTest()
//	defer cleanupMockScr()
//
//	serviceName := getReaperServiceName(rc.Datacenter)
//	service := &corev1.Service{
//		ObjectMeta: metav1.ObjectMeta{
//			Namespace: rc.Datacenter.Namespace,
//			Name:      serviceName,
//		},
//	}
//
//	trackObjects := []runtime.Object{rc.Datacenter, service}
//
//	rc.Client = fake.NewFakeClient(trackObjects...)
//
//	reconcileResult := rc.CheckReaperService()
//	assert.False(t, reconcileResult.Completed())
//
//	err := rc.Client.Get(rc.Ctx, types.NamespacedName{Namespace: rc.Datacenter.Namespace, Name: serviceName}, service)
//
//	assert.True(t, errors.IsNotFound(err), "did not expect to find service %s", serviceName)
//}

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
