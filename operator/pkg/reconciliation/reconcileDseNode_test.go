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

    datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeReloadTestPod() *corev1.Pod {
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: "mypod",
            Namespace: "default",
            Labels: map[string]string{
                datastaxv1alpha1.ClusterLabel: "mycluster",
                datastaxv1alpha1.DatacenterLabel: "mydc",
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
        Client: mockHttpClient,
        Log: rc.ReqLogger,
        Protocol: "http",
    }

    pod := makeReloadTestPod()

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
        Client: mockHttpClient,
        Log: rc.ReqLogger,
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
        Client: mockHttpClient,
        Log: rc.ReqLogger,
        Protocol: "http",
    }

    pod := makeReloadTestPod()

	if err := client.CallReloadSeedsEndpoint(pod); err == nil {
		assert.Fail(t, "Should have returned error")
	}
}
