package httphelper

import (
	"fmt"
	"github.com/go-logr/logr"
	"io/ioutil"
	"net/http"
	"net/url"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type NodeMgmtRequest struct {
	Endpoint string
	Host     string
	Client   HttpClient
	Method   string
}

func BuildPodHostFromPod(pod corev1.Pod) string {
	return GetPodHost(
		pod.Name,
		pod.Labels[datastaxv1alpha1.ClusterLabel],
		pod.Labels[datastaxv1alpha1.DatacenterLabel],
		pod.Namespace)
}

func GetPodHost(podName, clusterName, dcName, namespace string) string {
	nodeServicePattern := "%s.%s-%s-service.%s"

	return fmt.Sprintf(nodeServicePattern, podName, clusterName, dcName, namespace)
}

// Create a new superuser with the given username and password
func CallCreateRoleEndpoint(logger logr.Logger, pod corev1.Pod, username string, password string) error {
	logger.Info("client::callCreateRoleEndpoint")

	postData := url.Values{}
	postData.Set("username", username)
	postData.Set("password", password)
	postData.Set("can_login", "true")
	postData.Set("is_superuser", "true")

	request := NodeMgmtRequest{
		Endpoint: fmt.Sprintf("/api/v0/ops/auth/role?%s", postData.Encode()),
		Host:     BuildPodHostFromPod(pod),
		Client:   http.DefaultClient,
		Method:   http.MethodPost,
	}
	if err := CallNodeMgmtEndpoint(logger, request); err != nil {
		return err
	}

	return nil
}

func CallNodeMgmtEndpoint(logger logr.Logger, request NodeMgmtRequest) error {
	logger.Info("client::callNodeMgmtEndpoint")

	url := "http://" + request.Host + ":8080" + request.Endpoint
	req, err := http.NewRequest(request.Method, url, nil)
	if err != nil {
		logger.Error(err, "unable to create request for DSE Node Management Endpoint")
		return err
	}
	req.Close = true

	res, err := request.Client.Do(req)
	if err != nil {
		logger.Error(err, "unable to perform request to DSE Node Management Endpoint")
		return err
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			logger.Error(err, "unable to close response body")
		}
	}()

	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		logger.Error(err, "Unable to read response from DSE Node Management Endpoint")
		return err
	}

	goodStatus := res.StatusCode >= 200 && res.StatusCode < 300
	if !goodStatus {
		logger.Info("incorrect status code when calling DSE Node Management Endpoint",
			"statusCode", res.StatusCode,
			"pod", request.Host)

		return fmt.Errorf("incorrect status code of %d when calling endpoint", res.StatusCode)
	}

	return nil
}
