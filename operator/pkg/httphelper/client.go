package httphelper

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
)

type NodeMgmtRequest struct {
	Endpoint string
	Host     string
	Client   HttpClient
	Method   string
}

func GetPodHost(podName, clusterName, dcName, namespace string) string {
	nodeServicePattern := "%s.%s-%s-service.%s"

	return fmt.Sprintf(nodeServicePattern, podName, clusterName, dcName, namespace)
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
