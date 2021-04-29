// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/datastax/cass-operator/operator/internal/result"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/mocks"
	"github.com/stretchr/testify/mock"
)

func TestRetryDecommissionNode(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()
	state := "UP"
	podIP := "192.168.101.11"

	mockClient := &mocks.Client{}
	rc.Client = mockClient

	rc.Datacenter.SetCondition(api.DatacenterCondition{
		Status: v1.ConditionTrue,
		Type:   api.DatacenterScalingDown,
	})
	res := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}
	mockHttpClient := &mocks.HttpClient{}
	mockHttpClient.On("Do",
		mock.MatchedBy(
			func(req *http.Request) bool {
				return req.URL.Path == "/api/v0/ops/node/decommission"
			})).
		Return(res, nil).
		Once()

	rc.NodeMgmtClient = httphelper.NodeMgmtClient{
		Client:   mockHttpClient,
		Log:      rc.ReqLogger,
		Protocol: "http",
	}

	labels := make(map[string]string)
	labels[api.CassNodeState] = stateDecommissioning

	rc.dcPods = []*v1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod-1",
			Labels: labels,
		},
		Status: v1.PodStatus{
			PodIP: podIP,
		},
	}}

	epData := httphelper.CassMetadataEndpoints{
		Entity: []httphelper.EndpointState{
			{
				RpcAddress: podIP,
				Status:     state,
			},
		},
	}
	r := rc.CheckDecommissioningNodes(epData)
	if r != result.RequeueSoon(5) {
		t.Fatalf("expected result of result.RequeueSoon(5) but got %s", r)
	}
}

func TestRemoveResourcesWhenDone(t *testing.T) {
	rc, _, cleanupMockScr := setupTest()
	defer cleanupMockScr()
	podIP := "192.168.101.11"
	state := "LEFT"

	mockClient := &mocks.Client{}
	rc.Client = mockClient
	rc.Datacenter.SetCondition(api.DatacenterCondition{
		Status: v1.ConditionTrue,
		Type:   api.DatacenterScalingDown,
	})
	mockStatus := &statusMock{}
	k8sMockClientStatus(mockClient, mockStatus)

	labels := make(map[string]string)
	labels[api.CassNodeState] = stateDecommissioning

	rc.dcPods = []*v1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "pod-1",
			Labels: labels,
		},
		Status: v1.PodStatus{
			PodIP: podIP,
		},
	}}

	makeInt := func(i int32) *int32 {
		return &i
	}
	ssLabels := make(map[string]string)
	rc.statefulSets = []*appsv1.StatefulSet{{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "ss-1",
			Labels: ssLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: makeInt(1),
		},
	}}

	epData := httphelper.CassMetadataEndpoints{
		Entity: []httphelper.EndpointState{
			{
				RpcAddress: podIP,
				Status:     state,
			},
		},
	}

	r := rc.CheckDecommissioningNodes(epData)
	if r != result.RequeueSoon(5) {
		t.Fatalf("expected result of blah but got %s", r)
	}
	if mockStatus.called != 1 {
		t.Fatalf("expected 1 call to mockStatus but had %v", mockStatus.called)
	}
}

type statusMock struct {
	called int
}

func (s *statusMock) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

func (s *statusMock) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	s.called = s.called + 1
	return nil
}
