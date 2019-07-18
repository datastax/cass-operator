package reconciliation

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"

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

	if err := callNodeMgmtEndpoint(rc, mockHttpClient, corev1.Pod{}, "/api/v0/ops/seeds/reload"); err != nil {
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

	if err := callNodeMgmtEndpoint(rc, mockHttpClient, corev1.Pod{}, "/api/v0/ops/seeds/reload"); err == nil {
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

	if err := callNodeMgmtEndpoint(rc, mockHttpClient, corev1.Pod{}, "/api/v0/ops/seeds/reload"); err == nil {
		assert.Fail(t, "Should have returned error")
	}
}
