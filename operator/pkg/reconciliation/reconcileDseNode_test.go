package reconciliation

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/riptano/dse-operator/operator/pkg/httphelper"
	"github.com/riptano/dse-operator/operator/pkg/mocks"
)

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

	request := httphelper.NodeMgmtRequest{
		Endpoint: "/api/v0/ops/seeds/reload",
		Host:     httphelper.GetPodHost("pod-name", rc.DseDatacenter.Spec.ClusterName, rc.DseDatacenter.Name, rc.DseDatacenter.Namespace),
		Client:   mockHttpClient,
		Method:   http.MethodPost,
	}

	if err := httphelper.CallNodeMgmtEndpoint(rc.ReqLogger, request); err != nil {
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

	request := httphelper.NodeMgmtRequest{
		Endpoint: "/api/v0/ops/seeds/reload",
		Host:     httphelper.GetPodHost("pod-name", rc.DseDatacenter.Spec.ClusterName, rc.DseDatacenter.Name, rc.DseDatacenter.Namespace),
		Client:   mockHttpClient,
		Method:   http.MethodPost,
	}

	if err := httphelper.CallNodeMgmtEndpoint(rc.ReqLogger, request); err == nil {
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

	request := httphelper.NodeMgmtRequest{
		Endpoint: "/api/v0/ops/seeds/reload",
		Host:     httphelper.GetPodHost("pod-name", rc.DseDatacenter.Spec.ClusterName, rc.DseDatacenter.Name, rc.DseDatacenter.Namespace),
		Client:   mockHttpClient,
		Method:   http.MethodPost,
	}

	if err := httphelper.CallNodeMgmtEndpoint(rc.ReqLogger, request); err == nil {
		assert.Fail(t, "Should have returned error")
	}
}
