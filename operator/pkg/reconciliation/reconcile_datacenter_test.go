// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/datastax/cass-operator/operator/pkg/mocks"
)

func TestDeletePVCs(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientList(mockClient, nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	k8sMockClientDelete(mockClient, nil)

	err := rc.deletePVCs()
	if err != nil {
		t.Fatalf("deletePVCs should not have failed")
	}
}

func TestDeletePVCs_FailedToList(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientList(mockClient, fmt.Errorf("failed to list PVCs for CassandraDatacenter")).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	k8sMockClientDelete(mockClient, nil)

	err := rc.deletePVCs()
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

}

func TestDeletePVCs_PVCsNotFound(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientList(mockClient, errors.NewNotFound(schema.GroupResource{}, "name")).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	err := rc.deletePVCs()
	if err != nil {
		t.Fatalf("deletePVCs should not have failed")
	}
}

func TestDeletePVCs_FailedToDelete(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	k8sMockClientList(mockClient, nil).
		Run(func(args mock.Arguments) {
			arg := args.Get(1).(*v1.PersistentVolumeClaimList)
			arg.Items = []v1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-1",
				},
			}}
		})

	k8sMockClientDelete(mockClient, fmt.Errorf("failed to delete"))

	err := rc.deletePVCs()
	if err == nil {
		t.Fatalf("deletePVCs should have failed")
	}

	assert.EqualError(t, err, "failed to delete")
}
