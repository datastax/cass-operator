package reconciliation

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
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
		rc             *dsereconciliation.ReconciliationContext
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
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							DseClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.ClusterLabel: "dseCluster",
			},
		}, {
			name: "Nil labels",
			args: args{
				resourceLabels: nil,
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							DseClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.ClusterLabel: "dseCluster",
			},
		},
		{
			name: "Has Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.ClusterLabel: "dseCluster",
				},
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							DseClusterName: "dseCluster",
						},
					},
				},
			},
			want: false,
			wantLabels: map[string]string{
				datastaxv1alpha1.ClusterLabel: "dseCluster",
			},
		}, {
			name: "DC Label, No Cluster Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.DatacenterLabel: "dseDC",
				},
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							DseClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.DatacenterLabel: "dseDC",
				datastaxv1alpha1.ClusterLabel:    "dseCluster",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := shouldUpdateLabelsForClusterResource(tt.args.resourceLabels, tt.args.rc.DseDatacenter)
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

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Spec.Replicas = &one
	desiredStatefulSet.Status.ReadyReplicas = one

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.DseDatacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo := []*dsereconciliation.RackInformation{nextRack}

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestReconcilePods(t *testing.T) {
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
				dseDatacenter := datastaxv1alpha1.DseDatacenter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dsedatacenter-example",
						Namespace: "default",
					},
					Spec: datastaxv1alpha1.DseDatacenterSpec{
						DseClusterName: "dsedatacenter-example-cluster",
					},
				}
				return reflect.DeepEqual(obj.GetLabels(), dseDatacenter.GetRackLabels("default"))
			})).
		Return(nil).
		Once()

	statefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	statefulSet.Status.Replicas = int32(1)

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	err = reconcileRacks.ReconcilePods(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")

	mockClient.AssertExpectations(t)
}

func TestReconcilePods_WithVolumes(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	statefulSet.Status.Replicas = int32(1)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dsedatacenter-example-cluster-dsedatacenter-example-default-sts-0",
			Namespace: statefulSet.Namespace,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{{
				Name: "dse-data",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "dse-data-example-cluster1-example-dsedatacenter1-rack0-sts-0",
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
	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}
	err = reconcileRacks.ReconcilePods(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")
}

// Note: getStatefulSetForRack is currently just a query,
// and there is really no logic to test.
// We can add a unit test later, if needed.

func TestReconcileNextRack(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	result, err := reconcileRacks.ReconcileNextRack(statefulSet)
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{}, result, "Should requeue request")

	// Validation:
	// Currently reconcileNextRack does two things
	// 1. Creates the given StatefulSet in k8s.
	// 2. Creates a PodDisruptionBudget for the StatefulSet.
	//
	// TODO: check if Create() has been called on the fake client

}

func TestReconcileNextRack_CreateError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	statefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientCreate(mockClient, fmt.Errorf(""))
	k8sMockClientUpdate(mockClient, nil).Times(1)

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	result, err := reconcileRacks.ReconcileNextRack(statefulSet)

	mockClient.AssertExpectations(t)

	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")

	t.Skip("FIXME - Skipping assertion")

	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestCalculateRackInformation(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}
	rec, err := reconcileRacks.CalculateRackInformation()
	assert.NoErrorf(t, err, "Should not have returned an error")

	rackInfo := rec.(*ReconcileRacks).desiredRackInformation[0]

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

	rc.DseDatacenter.Spec.Racks = []v1alpha1.DseRack{{
		Name: "rack0",
	}, {
		Name: "rack1",
	}, {
		Name: "rack2",
	}}

	rc.DseDatacenter.Spec.Size = 3

	reconcileRacks := ReconcileRacks{
		ReconcileContext: rc,
	}

	rec, err := reconcileRacks.CalculateRackInformation()
	assert.NoErrorf(t, err, "Should not have returned an error")

	rackInfo := rec.(*ReconcileRacks).desiredRackInformation[0]

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

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.NotNil(t, result, "Result should not be nil")
}

func TestReconcileRacks_GetStatefulsetError(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientGet(mockClient, fmt.Errorf(""))

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
	}

	result, err := reconcileRacks.Apply()

	mockClient.AssertExpectations(t)

	assert.Errorf(t, err, "Should have returned an error")

	t.Skip("FIXME - Skipping assertion")

	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestReconcileRacks_WaitingForReplicas(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		desiredStatefulSet,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.True(t, result.Requeue, result, "Should requeue request")
}

func TestReconcileRacks_NeedMoreReplicas(t *testing.T) {
	t.Skip("FIXME - Skipping test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 3
	nextRack.SeedCount = 3

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestReconcileRacks_DoesntScaleDown(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 1
	nextRack.SeedCount = 1

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.True(t, result.Requeue, result, "Should requeue request")
}

func TestReconcileRacks_NeedToPark(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	preExistingStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		3)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	trackObjects := []runtime.Object{
		preExistingStatefulSet,
		rc.DseDatacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	rc.DseDatacenter.Spec.Parked = true
	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 0
	nextRack.SeedCount = 0

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Apply() should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

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

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	desiredPdb := newPodDisruptionBudgetForDatacenter(rc.DseDatacenter)

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.DseDatacenter,
		desiredPdb,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 2
	nextRack.SeedCount = 2

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{}, result, "Should not requeue request")
}

func TestReconcileRacks_FirstRackAlreadyReconciled(t *testing.T) {
	t.Skip("FIXME - Skipping this test")

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"rack0",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	secondDesiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"rack1",
		rc.DseDatacenter,
		1)
	assert.NoErrorf(t, err, "error occurred creating statefulset")
	secondDesiredStatefulSet.Status.ReadyReplicas = 1

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		secondDesiredStatefulSet,
		rc.DseDatacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	rack0 := &dsereconciliation.RackInformation{}
	rack0.RackName = "rack0"
	rack0.NodeCount = 2
	rack0.SeedCount = 2

	rack1 := &dsereconciliation.RackInformation{}
	rack1.RackName = "rack1"
	rack1.NodeCount = 2
	rack1.SeedCount = 1

	rackInfo = append(rackInfo, rack0, rack1)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
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
		rc           *dsereconciliation.ReconciliationContext
		statefulSet  *appsv1.StatefulSet
		newNodeCount int32
	}

	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	var (
		nextRack       = &dsereconciliation.RackInformation{}
		reconcileRacks = ReconcileRacks{
			ReconcileContext: rc,
		}
	)

	nextRack.RackName = "default"
	nextRack.NodeCount = 2

	statefulSet, _, _ := reconcileRacks.GetStatefulSetForRack(nextRack)

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
				rc.DseDatacenter,
			}

			reconcileRacks.ReconcileContext.Client = fake.NewFakeClient(trackObjects...)

			if _, err := reconcileRacks.UpdateRackNodeCount(tt.args.statefulSet, tt.args.newNodeCount); (err != nil) != tt.wantErr {
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

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"rack0",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	desiredPdb := newPodDisruptionBudgetForDatacenter(rc.DseDatacenter)

	mockPods := mockReadyPodsForStatefulSet(desiredStatefulSet, rc.DseDatacenter.Spec.DseClusterName, rc.DseDatacenter.Name)

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.DseDatacenter,
		desiredPdb,
	}
	for idx := range mockPods {
		mp := mockPods[idx]
		trackObjects = append(trackObjects, mp)
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	rack0 := &dsereconciliation.RackInformation{}
	rack0.RackName = "rack0"
	rack0.NodeCount = 2

	rackInfo = append(rackInfo, rack0)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: false}, result, "Should not requeue request")

	currentStatefulSet := &appsv1.StatefulSet{}
	nsName := types.NamespacedName{Name: desiredStatefulSet.Name, Namespace: desiredStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t,
		"{\"cluster-info\":{\"name\":\"dsedatacenter-example-cluster\",\"seeds\":\"dsedatacenter-example-cluster-seed-service\"},\"datacenter-info\":{\"name\":\"dsedatacenter-example\"}}",
		currentStatefulSet.Spec.Template.Spec.InitContainers[0].Env[0].Value,
		"The statefulset env config should not contain a cassandra-yaml entry.")

	// Update the config and rerun the reconcile

	configJson := []byte("{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\"}}")

	rc.DseDatacenter.Spec.Config = configJson

	reconcileRacks = ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
		statefulSets:           make([]*appsv1.StatefulSet, len(rackInfo), len(rackInfo)),
	}

	result, err = reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	currentStatefulSet = &appsv1.StatefulSet{}
	nsName = types.NamespacedName{Name: desiredStatefulSet.Name, Namespace: desiredStatefulSet.Namespace}
	err = rc.Client.Get(rc.Ctx, nsName, currentStatefulSet)
	assert.NoErrorf(t, err, "Client.Get() should not have returned an error")

	assert.Equal(t,
		"{\"cassandra-yaml\":{\"authenticator\":\"AllowAllAuthenticator\"},\"cluster-info\":{\"name\":\"dsedatacenter-example-cluster\",\"seeds\":\"dsedatacenter-example-cluster-seed-service\"},\"datacenter-info\":{\"name\":\"dsedatacenter-example\"}}",
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
		pod.Labels[datastaxv1alpha1.ClusterLabel] = cluster
		pod.Labels[datastaxv1alpha1.DatacenterLabel] = dc
		pod.Labels[datastaxv1alpha1.DseNodeState] = "Started"
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
	pod.Labels[datastaxv1alpha1.DseNodeState] = "Started"
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Ready: true,
	}}
	return pod
}

func TestReconcileRacks_countReadyAndStarted(t *testing.T) {
	type fields struct {
		ReconcileContext       *dsereconciliation.ReconciliationContext
		desiredRackInformation []*dsereconciliation.RackInformation
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
				desiredRackInformation: []*dsereconciliation.RackInformation{},
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
				desiredRackInformation: []*dsereconciliation.RackInformation{},
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
			r := &ReconcileRacks{
				ReconcileContext:       tt.fields.ReconcileContext,
				desiredRackInformation: tt.fields.desiredRackInformation,
				statefulSets:           tt.fields.statefulSets,
			}
			ready, started := r.countReadyAndStarted(tt.args.podList)
			if ready != tt.wantReady {
				t.Errorf("ReconcileRacks.countReadyAndStarted() got = %v, want %v", ready, tt.wantReady)
			}
			if started != tt.wantStarted {
				t.Errorf("ReconcileRacks.countReadyAndStarted() got1 = %v, want %v", started, tt.wantStarted)
			}
		})
	}
}
