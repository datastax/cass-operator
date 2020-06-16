// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/mocks"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test_validateLabelsForCluster(t *testing.T) {
	type args struct {
		resourceLabels map[string]string
		rc             *ReconciliationContext
	}
	tests := []struct {
		name       string
		args       args
		want       bool
		wantLabels map[string]string
	}{
		{
			name: "No labels",
			args: args{
				resourceLabels: make(map[string]string),
				rc: &ReconciliationContext{
					Datacenter: &api.CassandraDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleDC",
						},
						Spec: api.CassandraDatacenterSpec{
							ClusterName: "exampleCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				api.ClusterLabel:        "exampleCluster",
				oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
			},
		}, {
			name: "Nil labels",
			args: args{
				resourceLabels: nil,
				rc: &ReconciliationContext{
					Datacenter: &api.CassandraDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleDC",
						},
						Spec: api.CassandraDatacenterSpec{
							ClusterName: "exampleCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				api.ClusterLabel:        "exampleCluster",
				oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
			},
		},
		{
			name: "Has Label",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:        "exampleCluster",
					oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
				},
				rc: &ReconciliationContext{
					Datacenter: &api.CassandraDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleDC",
						},
						Spec: api.CassandraDatacenterSpec{
							ClusterName: "exampleCluster",
						},
					},
				},
			},
			want: false,
			wantLabels: map[string]string{
				api.ClusterLabel:        "exampleCluster",
				oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
			},
		}, {
			name: "DC Label, No Cluster Label",
			args: args{
				resourceLabels: map[string]string{
					api.DatacenterLabel: "exampleDC",
				},
				rc: &ReconciliationContext{
					Datacenter: &api.CassandraDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "exampleDC",
						},
						Spec: api.CassandraDatacenterSpec{
							ClusterName: "exampleCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				api.DatacenterLabel:     "exampleDC",
				api.ClusterLabel:        "exampleCluster",
				oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := shouldUpdateLabelsForClusterResource(tt.args.resourceLabels, tt.args.rc.Datacenter)
			if got != tt.want {
				t.Errorf("shouldUpdateLabelsForClusterResource() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.wantLabels) {
				t.Errorf("shouldUpdateLabelsForClusterResource() got1 = %v, want %v", got1, tt.wantLabels)
			}
		})
	}
}

func TestReconcileRacks_ReconcilePods(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		one = int32(1)
	)

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Spec.Replicas = &one
	desiredStatefulSet.Status.ReadyReplicas = one

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.Datacenter,
	}

	mockPods := mockReadyPodsForStatefulSet(desiredStatefulSet, rc.Datacenter.Spec.ClusterName, rc.Datacenter.Name)
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo := []*RackInformation{nextRack}

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestCheckRackPodTemplate_SetControllerRefOnStatefulSet(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	rc.Datacenter.Spec.Racks = []api.Rack{
		{Name: "rack1", Zone: "zone-1"},
	}

	if err := rc.CalculateRackInformation(); err != nil {
		t.Fatalf("failed to calculate rack information: %s", err)
	}

	result := rc.CheckRackCreation()
	assert.False(t, result.Completed(), "CheckRackCreation did not complete as expected")

	if err := rc.Client.Update(rc.Ctx, rc.Datacenter); err != nil {
		t.Fatalf("failed to add rack to cassandradatacenter: %s", err)
	}

	var actualOwner, actualObject metav1.Object
	invocations := 0
	setControllerReference = func(owner, object metav1.Object, scheme *runtime.Scheme) error {
		actualOwner = owner
		actualObject = object
		invocations++
		return nil
	}

	terminationGracePeriod := int64(35)
	podTemplateSpec := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &terminationGracePeriod,
		},
	}
	rc.Datacenter.Spec.PodTemplateSpec = podTemplateSpec

	result = rc.CheckRackPodTemplate()
	assert.True(t, result.Completed())

	assert.Equal(t, 1, invocations)
	assert.Equal(t, rc.Datacenter, actualOwner)
	assert.Equal(t, rc.statefulSets[0].Name, actualObject.GetName())
}

func TestReconcilePods(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, nil)

	// this mock will only pass if the pod is updated with the correct labels
	mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj *corev1.Pod) bool {
				dc := api.CassandraDatacenter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cassandradatacenter-example",
						Namespace: "default",
					},
					Spec: api.CassandraDatacenterSpec{
						ClusterName: "cassandradatacenter-example-cluster",
					},
				}
				expected := dc.GetRackLabels("default")
				expected[oplabels.ManagedByLabel] = oplabels.ManagedByLabelValue

				return reflect.DeepEqual(obj.GetLabels(), expected)
			})).
		Return(nil).
		Once()

	statefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	statefulSet.Status.Replicas = int32(1)

	err = rc.ReconcilePods(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")

	mockClient.AssertExpectations(t)
}

func TestReconcilePods_WithVolumes(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	statefulSet.Status.Replicas = int32(1)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cassandradatacenter-example-cluster-cassandradatacenter-example-default-sts-0",
			Namespace: statefulSet.Namespace,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{{
				Name: "server-data",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "cassandra-data-example-cluster1-example-cassandradatacenter1-rack0-sts-0",
					},
				},
			}},
		},
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName,
			Namespace: statefulSet.Namespace,
		},
	}

	trackObjects := []runtime.Object{
		pod,
		pvc,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)
	err = rc.ReconcilePods(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")
}

// Note: getStatefulSetForRack is currently just a query,
// and there is really no logic to test.
// We can add a unit test later, if needed.

func TestReconcileNextRack(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	err = rc.ReconcileNextRack(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")

	// Validation:
	// Currently reconcileNextRack does two things
	// 1. Creates the given StatefulSet in k8s.
	// 2. Creates a PodDisruptionBudget for the StatefulSet.
	//
	// TODO: check if Create() has been called on the fake client

}

func TestReconcileNextRack_CreateError(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientCreate(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil).Times(1)

	err = rc.ReconcileNextRack(statefulSet)

	mockClient.AssertExpectations(t)

	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
}

func TestCalculateRackInformation(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	err := rc.CalculateRackInformation()
	assert.NoErrorf(t, err, "Should not have returned an error")

	rackInfo := rc.desiredRackInformation[0]

	assert.Equal(t, "default", rackInfo.RackName, "Should have correct rack name")

	rc.ReqLogger.Info(
		"Node count is ",
		"Node Count: ",
		rackInfo.NodeCount)

	assert.Equal(t, 2, rackInfo.NodeCount, "Should have correct node count")

	// TODO add more RackInformation validation

}

func TestCalculateRackInformation_MultiRack(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	rc.Datacenter.Spec.Racks = []api.Rack{{
		Name: "rack0",
	}, {
		Name: "rack1",
	}, {
		Name: "rack2",
	}}

	rc.Datacenter.Spec.Size = 3

	err := rc.CalculateRackInformation()
	assert.NoErrorf(t, err, "Should not have returned an error")

	rackInfo := rc.desiredRackInformation[0]

	assert.Equal(t, "rack0", rackInfo.RackName, "Should have correct rack name")

	rc.ReqLogger.Info(
		"Node count is ",
		"Node Count: ",
		rackInfo.NodeCount)

	assert.Equal(t, 1, rackInfo.NodeCount, "Should have correct node count")

	// TODO add more RackInformation validation
}

func TestReconcileRacks(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.Datacenter,
	}

	mockPods := mockReadyPodsForStatefulSet(desiredStatefulSet, rc.Datacenter.Spec.ClusterName, rc.Datacenter.Name)
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()

	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestReconcileRacks_GetStatefulsetError(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, fmt.Errorf(""))

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo

	result, err := rc.ReconcileAllRacks()

	mockClient.AssertExpectations(t)

	assert.Errorf(t, err, "Should have returned an error")

	t.Skip("FIXME - Skipping assertion")

	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestReconcileRacks_WaitingForReplicas(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		desiredStatefulSet,
	}

	mockPods := mockReadyPodsForStatefulSet(desiredStatefulSet, rc.Datacenter.Spec.ClusterName, rc.Datacenter.Name)
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.True(t, result.Requeue, result, "Should requeue request")
}

func TestReconcileRacks_NeedMoreReplicas(t *testing.T) {
	t.Skip("FIXME - Skipping test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 3
	nextRack.SeedCount = 3

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestReconcileRacks_DoesntScaleDown(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	mockPods := mockReadyPodsForStatefulSet(preExistingStatefulSet, rc.Datacenter.Spec.ClusterName, rc.Datacenter.Name)
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.True(t, result.Requeue, result, "Should requeue request")
}

func TestReconcileRacks_NeedToPark(t *testing.T) {
	t.Skip()
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		3)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
		rc.Datacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	rc.Datacenter.Spec.Stopped = true
	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 0
	nextRack.SeedCount = 0

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Apply() should not have returned an error")
	assert.False(t, result.Requeue, "Should not requeue request")

	currentStatefulSet := &appsv1.StatefulSet{}
	nsName := types.NamespacedName{Name: preExistingStatefulSet.Name, Namespace: preExistingStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t, int32(0), *currentStatefulSet.Spec.Replicas, "The statefulset should be set to zero replicas")
}

func TestReconcileRacks_AlreadyReconciled(t *testing.T) {
	t.Skip("FIXME - Skipping this test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"default",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	desiredPdb := newPodDisruptionBudgetForDatacenter(rc.Datacenter)

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.Datacenter,
		desiredPdb,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	nextRack := &RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 2
	nextRack.SeedCount = 2

	rackInfo = append(rackInfo, nextRack)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{}, result, "Should not requeue request")
}

func TestReconcileRacks_FirstRackAlreadyReconciled(t *testing.T) {
	t.Skip("FIXME - Skipping this test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"rack0",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	secondDesiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"rack1",
		rc.Datacenter,
		1)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	secondDesiredStatefulSet.Status.ReadyReplicas = 1

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		secondDesiredStatefulSet,
		rc.Datacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	rack0 := &RackInformation{}
	rack0.RackName = "rack0"
	rack0.NodeCount = 2
	rack0.SeedCount = 2

	rack1 := &RackInformation{}
	rack1.RackName = "rack1"
	rack1.NodeCount = 2
	rack1.SeedCount = 1

	rackInfo = append(rackInfo, rack0, rack1)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	currentStatefulSet := &appsv1.StatefulSet{}
	nsName := types.NamespacedName{Name: secondDesiredStatefulSet.Name, Namespace: secondDesiredStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t, int32(2), *currentStatefulSet.Spec.Replicas, "The statefulset should be set to 2 replicas")
}

func TestReconcileRacks_UpdateRackNodeCount(t *testing.T) {
	type args struct {
		rc           *ReconciliationContext
		statefulSet  *appsv1.StatefulSet
		newNodeCount int32
	}

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var nextRack = &RackInformation{}

	nextRack.RackName = "default"
	nextRack.NodeCount = 2

	statefulSet, _, _ := rc.GetStatefulSetForRack(nextRack)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "check that replicas get increased",
			args: args{
				rc:           rc,
				statefulSet:  statefulSet,
				newNodeCount: 3,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackObjects := []runtime.Object{
				tt.args.statefulSet,
				rc.Datacenter,
			}

			rc.Client = fake.NewFakeClient(trackObjects...)

			if err := rc.UpdateRackNodeCount(tt.args.statefulSet, tt.args.newNodeCount); (err != nil) != tt.wantErr {
				t.Errorf("updateRackNodeCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.args.newNodeCount != *tt.args.statefulSet.Spec.Replicas {
				t.Errorf("StatefulSet spec should have different replica count, has = %v, want %v", *tt.args.statefulSet.Spec.Replicas, tt.args.newNodeCount)
			}
		})
	}
}

func TestReconcileRacks_UpdateConfig(t *testing.T) {
	t.Skip("FIXME - Skipping this test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForCassandraDatacenter(
		"rack0",
		rc.Datacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	desiredPdb := newPodDisruptionBudgetForDatacenter(rc.Datacenter)

	mockPods := mockReadyPodsForStatefulSet(desiredStatefulSet, rc.Datacenter.Spec.ClusterName, rc.Datacenter.Name)

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.Datacenter,
		desiredPdb,
	}
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*RackInformation

	rack0 := &RackInformation{}
	rack0.RackName = "rack0"
	rack0.NodeCount = 2

	rackInfo = append(rackInfo, rack0)

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err := rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: false}, result, "Should not requeue request")

	currentStatefulSet := &appsv1.StatefulSet{}
	nsName := types.NamespacedName{Name: desiredStatefulSet.Name, Namespace: desiredStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t,
		"{\"cluster-info\":{\"name\":\"cassandradatacenter-example-cluster\",\"seeds\":\"cassandradatacenter-example-cluster-seed-service\"},\"datacenter-info\":{\"name\":\"cassandradatacenter-example\"}}",
		currentStatefulSet.Spec.Template.Spec.InitContainers[0].Env[0].Value,
		"The statefulset env config should not contain a cassandra-yaml entry.")

	// Update the config and rerun the reconcile

	configJson := []byte("{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\"}}")

	rc.Datacenter.Spec.Config = configJson

	rc.desiredRackInformation = rackInfo
	rc.statefulSets = make([]*appsv1.StatefulSet, len(rackInfo))

	result, err = rc.ReconcileAllRacks()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	currentStatefulSet = &appsv1.StatefulSet{}
	nsName = types.NamespacedName{Name: desiredStatefulSet.Name, Namespace: desiredStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t,
		"{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\"},\"cluster-info\":{\"name\":\"cassandradatacenter-example-cluster\",\"seeds\":\"cassandradatacenter-example-cluster-seed-service\"},\"datacenter-info\":{\"name\":\"cassandradatacenter-example\"}}",
		currentStatefulSet.Spec.Template.Spec.InitContainers[0].Env[0].Value,
		"The statefulset should contain a cassandra-yaml entry.")
}

func mockReadyPodsForStatefulSet(sts *appsv1.StatefulSet, cluster, dc string) []*corev1.Pod {
	var pods []*corev1.Pod
	sz := int(*sts.Spec.Replicas)
	for i := 0; i < sz; i++ {
		pod := &corev1.Pod{}
		pod.Namespace = sts.Namespace
		pod.Name = fmt.Sprintf("%s-%d", sts.Name, i)
		pod.Labels = make(map[string]string)
		pod.Labels[api.ClusterLabel] = cluster
		pod.Labels[api.DatacenterLabel] = dc
		pod.Labels[api.CassNodeState] = "Started"
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Ready: true,
		}}
		pods = append(pods, pod)
	}
	return pods
}

func makeMockReadyStartedPod() *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Labels = make(map[string]string)
	pod.Labels[api.CassNodeState] = "Started"
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  "cassandra",
		Ready: true,
	}}
	return pod
}

func TestReconcileRacks_countReadyAndStarted(t *testing.T) {
	type fields struct {
		ReconcileContext       *ReconciliationContext
		desiredRackInformation []*RackInformation
		statefulSets           []*appsv1.StatefulSet
	}
	type args struct {
		podList *corev1.PodList
	}
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantReady   int
		wantStarted int
	}{
		{
			name: "test an empty podList",
			fields: fields{
				ReconcileContext:       rc,
				desiredRackInformation: []*RackInformation{},
				statefulSets:           []*appsv1.StatefulSet{},
			},
			args: args{
				podList: &corev1.PodList{},
			},
			wantReady:   0,
			wantStarted: 0,
		},
		{
			name: "test two ready and started pods",
			fields: fields{
				ReconcileContext:       rc,
				desiredRackInformation: []*RackInformation{},
				statefulSets:           []*appsv1.StatefulSet{},
			},
			args: args{
				podList: &corev1.PodList{
					Items: []corev1.Pod{
						*makeMockReadyStartedPod(),
						*makeMockReadyStartedPod(),
					},
				},
			},
			wantReady:   2,
			wantStarted: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc.desiredRackInformation = tt.fields.desiredRackInformation
			rc.statefulSets = tt.fields.statefulSets
			rc.dcPods = PodPtrsFromPodList(tt.args.podList)

			ready, started := rc.countReadyAndStarted()
			if ready != tt.wantReady {
				t.Errorf("ReconcileRacks.countReadyAndStarted() ready = %v, want %v", ready, tt.wantReady)
			}
			if started != tt.wantStarted {
				t.Errorf("ReconcileRacks.countReadyAndStarted() started = %v, want %v", started, tt.wantStarted)
			}
		})
	}
}

func Test_isServerReady(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	podThatHasNoServer := makeMockReadyStartedPod()
	podThatHasNoServer.Status.ContainerStatuses[0].Name = "nginx"
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "check a ready server pod",
			args: args{
				pod: makeMockReadyStartedPod(),
			},
			want: true,
		},
		{
			name: "check a ready non-server pod",
			args: args{
				pod: podThatHasNoServer,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isServerReady(tt.args.pod); got != tt.want {
				t.Errorf("isServerReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isMgmtApiRunning(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	readyServerContainer := makeMockReadyStartedPod()
	readyServerContainer.Status.ContainerStatuses[0].State.Running =
		&corev1.ContainerStateRunning{StartedAt: metav1.Date(2019, time.July, 4, 12, 12, 12, 0, time.UTC)}

	veryFreshServerContainer := makeMockReadyStartedPod()
	veryFreshServerContainer.Status.ContainerStatuses[0].State.Running =
		&corev1.ContainerStateRunning{StartedAt: metav1.Now()}

	podThatHasNoServer := makeMockReadyStartedPod()
	podThatHasNoServer.Status.ContainerStatuses[0].Name = "nginx"

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "check a ready server pod",
			args: args{
				pod: readyServerContainer,
			},
			want: true,
		},
		{
			name: "check a ready server pod that started as recently as possible",
			args: args{
				pod: veryFreshServerContainer,
			},
			want: false,
		},
		{
			name: "check a ready server pod that started as recently as possible",
			args: args{
				pod: podThatHasNoServer,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMgmtApiRunning(tt.args.pod); got != tt.want {
				t.Errorf("isMgmtApiRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_shouldUpdateLabelsForRackResource(t *testing.T) {
	clusterName := "cassandradatacenter-example-cluster"
	dcName := "cassandradatacenter-example"
	rackName := "rack1"
	dc := &api.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dcName,
			Namespace: "default",
		},
		Spec: api.CassandraDatacenterSpec{
			ClusterName: clusterName,
		},
	}

	goodRackLabels := map[string]string{
		api.ClusterLabel:        clusterName,
		api.DatacenterLabel:     dcName,
		api.RackLabel:           rackName,
		oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
	}

	type args struct {
		resourceLabels map[string]string
	}

	type result struct {
		changed bool
		labels  map[string]string
	}

	// cases where label updates are made
	tests := []struct {
		name string
		args args
		want result
	}{
		{
			name: "Cluster name different",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:    "some-other-cluster",
					api.DatacenterLabel: dcName,
					api.RackLabel:       rackName,
				},
			},
			want: result{
				changed: true,
				labels:  goodRackLabels,
			},
		},
		{
			name: "Rack name different",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:    clusterName,
					api.DatacenterLabel: dcName,
					api.RackLabel:       "some-other-rack",
				},
			},
			want: result{
				changed: true,
				labels:  goodRackLabels,
			},
		},
		{
			name: "Rack name different plus other labels",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:    clusterName,
					api.DatacenterLabel: dcName,
					api.RackLabel:       "some-other-rack",
					"foo":               "bar",
				},
			},
			want: result{
				changed: true,
				labels: utils.MergeMap(
					map[string]string{},
					goodRackLabels,
					map[string]string{"foo": "bar"}),
			},
		},
		{
			name: "No labels",
			args: args{
				resourceLabels: map[string]string{},
			},
			want: result{
				changed: true,
				labels:  goodRackLabels,
			},
		},
		{
			name: "Correct labels",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:        clusterName,
					api.DatacenterLabel:     dcName,
					api.RackLabel:           rackName,
					oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
				},
			},
			want: result{
				changed: false,
			},
		},
		{
			name: "Correct labels with some additional labels",
			args: args{
				resourceLabels: map[string]string{
					api.ClusterLabel:        clusterName,
					api.DatacenterLabel:     dcName,
					api.RackLabel:           rackName,
					oplabels.ManagedByLabel: oplabels.ManagedByLabelValue,
					"foo":                   "bar",
				},
			},
			want: result{
				changed: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want.changed {
				changed, newLabels := shouldUpdateLabelsForRackResource(tt.args.resourceLabels, dc, rackName)
				if !changed || !reflect.DeepEqual(newLabels, tt.want.labels) {
					t.Errorf("shouldUpdateLabelsForRackResource() = (%v, %v), want (%v, %v)", changed, newLabels, true, tt.want)
				}
			} else {
				// when the labels aren't supposed to be changed, we want to
				// make sure that the map returned *is* the map passed in and
				// that it is unchanged.
				resourceLabelsCopy := utils.MergeMap(map[string]string{}, tt.args.resourceLabels)
				changed, newLabels := shouldUpdateLabelsForRackResource(tt.args.resourceLabels, dc, rackName)
				if changed || !reflect.DeepEqual(resourceLabelsCopy, newLabels) {
					t.Errorf("shouldUpdateLabelsForRackResource() = (%v, %v), want (%v, %v)", changed, newLabels, true, tt.want)
				} else if reflect.ValueOf(tt.args.resourceLabels).Pointer() != reflect.ValueOf(newLabels).Pointer() {
					t.Error("shouldUpdateLabelsForRackResource() did not return original map")
				}
			}
		})
	}
}

func makeReloadTestPod() *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mypod",
			Namespace: "default",
			Labels: map[string]string{
				api.ClusterLabel:    "mycluster",
				api.DatacenterLabel: "mydc",
			},
		},
	}
	return pod
}

func Test_callPodEndpoint(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	res := &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}

	mockHttpClient := &mocks.HttpClient{}
	mockHttpClient.On("Do",
		mock.MatchedBy(
			func(req *http.Request) bool {
				return req != nil
			})).
		Return(res, nil).
		Once()

	client := httphelper.NodeMgmtClient{
		Client:   mockHttpClient,
		Log:      rc.ReqLogger,
		Protocol: "http",
	}

	pod := makeReloadTestPod()
	pod.Status.PodIP = "1.2.3.4"

	if err := client.CallReloadSeedsEndpoint(pod); err != nil {
		assert.Fail(t, "Should not have returned error")
	}
}

func Test_callPodEndpoint_BadStatus(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	res := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}

	mockHttpClient := &mocks.HttpClient{}
	mockHttpClient.On("Do",
		mock.MatchedBy(
			func(req *http.Request) bool {
				return req != nil
			})).
		Return(res, nil).
		Once()

	client := httphelper.NodeMgmtClient{
		Client:   mockHttpClient,
		Log:      rc.ReqLogger,
		Protocol: "http",
	}

	pod := makeReloadTestPod()

	if err := client.CallReloadSeedsEndpoint(pod); err == nil {
		assert.Fail(t, "Should have returned error")
	}
}

func Test_callPodEndpoint_RequestFail(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	res := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}

	mockHttpClient := &mocks.HttpClient{}
	mockHttpClient.On("Do",
		mock.MatchedBy(
			func(req *http.Request) bool {
				return req != nil
			})).
		Return(res, fmt.Errorf("")).
		Once()

	client := httphelper.NodeMgmtClient{
		Client:   mockHttpClient,
		Log:      rc.ReqLogger,
		Protocol: "http",
	}

	pod := makeReloadTestPod()

	if err := client.CallReloadSeedsEndpoint(pod); err == nil {
		assert.Fail(t, "Should have returned error")
	}
}
