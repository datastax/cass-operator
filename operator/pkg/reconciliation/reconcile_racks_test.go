package reconciliation

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"github.com/riptano/dse-operator/operator/pkg/dsereconciliation"
	"github.com/riptano/dse-operator/operator/pkg/mocks"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
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
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
			},
		},
		{
			name: "Has Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
				},
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: false,
			wantLabels: map[string]string{
				datastaxv1alpha1.CLUSTER_LABEL: "dseCluster",
			},
		}, {
			name: "DC Label, No Cluster Label",
			args: args{
				resourceLabels: map[string]string{
					datastaxv1alpha1.DATACENTER_LABEL: "dseDC",
				},
				rc: &dsereconciliation.ReconciliationContext{
					DseDatacenter: &datastaxv1alpha1.DseDatacenter{
						ObjectMeta: metav1.ObjectMeta{
							Name: "dseDC",
						},
						Spec: datastaxv1alpha1.DseDatacenterSpec{
							ClusterName: "dseCluster",
						},
					},
				},
			},
			want: true,
			wantLabels: map[string]string{
				datastaxv1alpha1.DATACENTER_LABEL: "dseDC",
				datastaxv1alpha1.CLUSTER_LABEL:    "dseCluster",
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
						ClusterName: "dsedatacenter-example-cluster",
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
	assert.Errorf(t, err, "Should have returned an error while calculating reconciliation actions")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
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
	assert.Errorf(t, err, "Should have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")

	mockClient.AssertExpectations(t)
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

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
}

func TestReconcileRacks_NeedMoreReplicas(t *testing.T) {
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

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
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

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{Requeue: true}, result, "Should requeue request")
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

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
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
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	desiredStatefulSet, err := newStatefulSetForDseDatacenter(
		"default",
		rc.DseDatacenter,
		2)
	assert.NoErrorf(t, err, "error occurred creating statefulset")

	desiredStatefulSet.Status.ReadyReplicas = 2

	trackObjects := []runtime.Object{
		desiredStatefulSet,
		rc.DseDatacenter,
	}

	rc.Client = fake.NewFakeClient(trackObjects...)

	var rackInfo []*dsereconciliation.RackInformation

	nextRack := &dsereconciliation.RackInformation{}
	nextRack.RackName = "default"
	nextRack.NodeCount = 2

	rackInfo = append(rackInfo, nextRack)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
	}

	result, err := reconcileRacks.Apply()
	assert.NoErrorf(t, err, "Should not have returned an error")
	assert.Equal(t, reconcile.Result{}, result, "Should not requeue request")
}

func TestReconcileRacks_FirstRackAlreadyReconciled(t *testing.T) {
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

	rack1 := &dsereconciliation.RackInformation{}
	rack1.RackName = "rack1"
	rack1.NodeCount = 2

	rackInfo = append(rackInfo, rack0, rack1)

	reconcileRacks := ReconcileRacks{
		ReconcileContext:       rc,
		desiredRackInformation: rackInfo,
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
