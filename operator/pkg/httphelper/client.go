// Copyright DataStax, Inc.
// Please see the included license file for details.

package httphelper

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/go-logr/logr"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

type NodeMgmtClient struct {
	Client   HttpClient
	Log      logr.Logger
	Protocol string
}

type nodeMgmtRequest struct {
	endpoint string
	host     string
	method   string
	timeout  time.Duration
}

func buildEndpoint(path string, queryParams ...string) string {
	params := url.Values{}
	for i := 0; i < len(queryParams) - 1; i = i + 2 {
		params[queryParams[i]] = []string{queryParams[i+1]}
	}

	url := &url.URL{
		Path: path,
		RawQuery: params.Encode(),
	}
	return url.String()
}

type EndpointState struct {
	HostID                 string `json:"HOST_ID"`
	IsAlive                string `json:"IS_ALIVE"`
	NativeTransportAddress string `json:"NATIVE_TRANSPORT_ADDRESS"`
	RpcAddress             string `json:"RPC_ADDRESS"`
}

func (x *EndpointState) GetRpcAddress() string {
	if x.NativeTransportAddress != "" {
		return x.NativeTransportAddress
	} else {
		return x.RpcAddress
	}
}

type CassMetadataEndpoints struct {
	Entity []EndpointState `json:"entity"`
}

func BuildPodHostFromPod(pod *corev1.Pod) string {
	return GetPodHost(
		pod.Name,
		pod.Labels[api.ClusterLabel],
		pod.Labels[api.DatacenterLabel],
		pod.Namespace)
}

func GetPodHost(podName, clusterName, dcName, namespace string) string {
	nodeServicePattern := "%s.%s-%s-service.%s"

	return fmt.Sprintf(nodeServicePattern, podName, clusterName, dcName, namespace)
}

func parseMetadataEndpointsResponseBody(body []byte) (*CassMetadataEndpoints, error) {
	endpoints := &CassMetadataEndpoints{}
	if err := json.Unmarshal(body, &endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func (client *NodeMgmtClient) CallMetadataEndpointsEndpoint(pod *corev1.Pod) (CassMetadataEndpoints, error) {
	client.Log.Info("requesting Cassandra metadata endpoints from Node Management API", "pod", pod.Name)

	request := nodeMgmtRequest{
		endpoint: "/api/v0/metadata/endpoints",
		host:     BuildPodHostFromPod(pod),
		method:   http.MethodGet,
	}

	bytes, err := callNodeMgmtEndpoint(client, request)
	if err != nil {
		return CassMetadataEndpoints{}, err

	}

	endpoints, err := parseMetadataEndpointsResponseBody(bytes)
	if err != nil {
		return CassMetadataEndpoints{}, err
	} else {
		return *endpoints, nil
	}
}

// Create a new superuser with the given username and password
func (client *NodeMgmtClient) CallCreateRoleEndpoint(pod *corev1.Pod, username string, password string) error {
	client.Log.Info(
		"calling Management API create role - POST /api/v0/ops/auth/role",
		"pod", pod.Name,
	)

	postData := url.Values{}
	postData.Set("username", username)
	postData.Set("password", password)
	postData.Set("can_login", "true")
	postData.Set("is_superuser", "true")

	request := nodeMgmtRequest{
		endpoint: fmt.Sprintf("/api/v0/ops/auth/role?%s", postData.Encode()),
		host:     BuildPodHostFromPod(pod),
		method:   http.MethodPost,
	}
	_, err := callNodeMgmtEndpoint(client, request)
	return err
}

func (client *NodeMgmtClient) CallProbeClusterEndpoint(pod *corev1.Pod, consistencyLevel string, rfPerDc int) error {
	client.Log.Info(
		"calling Management API cluster health - GET /api/v0/probes/cluster",
		"pod", pod.Name,
	)

	request := nodeMgmtRequest{
		endpoint: fmt.Sprintf("/api/v0/probes/cluster?consistency_level=%s&rf_per_dc=%d", consistencyLevel, rfPerDc),
		host:     BuildPodHostFromPod(pod),
		method:   http.MethodGet,
	}

	_, err := callNodeMgmtEndpoint(client, request)
	return err
}

func (client *NodeMgmtClient) CallDrainEndpoint(pod *corev1.Pod) error {
	client.Log.Info(
		"calling Management API drain node - POST /api/v0/ops/node/drain",
		"pod", pod.Name,
	)

	request := nodeMgmtRequest{
		endpoint: "/api/v0/ops/node/drain",
		host:     BuildPodHostFromPod(pod),
		method:   http.MethodPost,
		timeout:  time.Minute * 2,
	}

	_, err := callNodeMgmtEndpoint(client, request)
	return err
}

func (client *NodeMgmtClient) CallLifecycleStartEndpointWithReplaceIp(pod *corev1.Pod, replaceIp string) error {
	// talk to the pod via IP because we are dialing up a pod that isn't ready,
	// so it won't be reachable via the service and pod DNS
	podIP := pod.Status.PodIP

	client.Log.Info(
		"calling Management API start node - POST /api/v0/lifecycle/start",
		"pod", pod.Name,
		"podIP", podIP,
		"replaceIP", replaceIp,
	)

	endpoint := "/api/v0/lifecycle/start"
	
	if replaceIp != "" {
		endpoint = buildEndpoint(endpoint, "replace_ip", replaceIp)
	}

	request := nodeMgmtRequest{
		endpoint: endpoint,
		host:     podIP,
		method:   http.MethodPost,
		timeout:  10 * time.Second,
	}

	_, err := callNodeMgmtEndpoint(client, request)
	return err
}

func (client *NodeMgmtClient) CallLifecycleStartEndpoint(pod *corev1.Pod) error {
	return client.CallLifecycleStartEndpointWithReplaceIp(pod, "")
}

func (client *NodeMgmtClient) CallReloadSeedsEndpoint(pod *corev1.Pod) error {
	client.Log.Info(
		"calling Management API reload seeds - POST /api/v0/ops/seeds/reload",
		"pod", pod.Name,
	)

	request := nodeMgmtRequest{
		endpoint: "/api/v0/ops/seeds/reload",
		host:     BuildPodHostFromPod(pod),
		method:   http.MethodPost,
	}

	_, err := callNodeMgmtEndpoint(client, request)
	return err
}

func callNodeMgmtEndpoint(client *NodeMgmtClient, request nodeMgmtRequest) ([]byte, error) {
	client.Log.Info("client::callNodeMgmtEndpoint")

	url := fmt.Sprintf("%s://%s:8080%s", client.Protocol, request.host, request.endpoint)
	req, err := http.NewRequest(request.method, url, nil)
	if err != nil {
		client.Log.Error(err, "unable to create request for Node Management Endpoint")
		return nil, err
	}
	req.Close = true

	if request.timeout == 0 {
		request.timeout = 60 * time.Second
	}

	if request.timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), request.timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	res, err := client.Client.Do(req)
	if err != nil {
		client.Log.Error(err, "unable to perform request to Node Management Endpoint")
		return nil, err
	}

	defer func() {
		err := res.Body.Close()
		if err != nil {
			client.Log.Error(err, "unable to close response body")
		}
	}()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		client.Log.Error(err, "Unable to read response from Node Management Endpoint")
		return nil, err
	}

	goodStatus := res.StatusCode >= 200 && res.StatusCode < 300
	if !goodStatus {
		client.Log.Info("incorrect status code when calling Node Management Endpoint",
			"statusCode", res.StatusCode,
			"pod", request.host)

		return nil, fmt.Errorf("incorrect status code of %d when calling endpoint", res.StatusCode)
	}

	return body, nil
}
