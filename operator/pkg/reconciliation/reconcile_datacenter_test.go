package reconciliation

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

func TestDeletePVCs(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(nil).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	err := reconcileDatacenter.deletePVCs()
	if err != nil {
		t.Fatalf("deletePVCs should not have failed")
	}
}

func TestDeletePVCs_FailedToList(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(fmt.Errorf("failed to list PVCs for dseDatacenter")).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(nil).
		Once()

	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	err := reconcileDatacenter.deletePVCs()
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

}

func TestDeletePVCs_PVCsNotFound(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(errors.NewNotFound(schema.GroupResource{}, "name")).
		Once()

	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	err := reconcileDatacenter.deletePVCs()
	if err != nil {
		t.Fatalf("deletePVCs should not have failed")
	}
}

func TestDeletePVCs_FailedToDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := mocks.Client{}
	rc.Client = &mockClient

	mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Run(func(args mock.Arguments) {
			arg := args.Get(2).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		}).
		Return(nil).
		Once()

	mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(fmt.Errorf("failed to delete")).
		Once()

	reconcileDatacenter := ReconcileDatacenter{
		ReconcileContext: rc,
	}

	err := reconcileDatacenter.deletePVCs()
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

	assert.EqualError(t, err, "failed to delete")
}
