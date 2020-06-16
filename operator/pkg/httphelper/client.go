// Copyright DataStax, Inc.
// Please see the included license file for details.

package httphelper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-logr/logr"

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
	body     []byte
}

func buildEndpoint(path string, queryParams ...string) string {
	params := url.Values{}
	for i := 0; i < len(queryParams)-1; i = i + 2 {
		params[queryParams[i]] = []string{queryParams[i+1]}
	}

	url := &url.URL{
		Path:     path,
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

type NoPodIPError error

func newNoPodIPError(pod *corev1.Pod) NoPodIPError {
	return fmt.Errorf("pod %s has no IP", pod.Name)
}

func BuildPodHostFromPod(pod *corev1.Pod) (string, error) {
	// This function previously returned the dns hostname which includes the StatefulSet's headless service,
	// which is the datacenter service. There are times though that we want to make a mgmt api call to the pod
	// before the dns hostnames are available. It is therefore more reliable to simply use the PodIP.

	if len(pod.Status.PodIP) == 0 {
		return "", newNoPodIPError(pod)
	}

	return pod.Status.PodIP, nil
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

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return CassMetadataEndpoints{}, err
	}

	request := nodeMgmtRequest{
		endpoint: "/api/v0/metadata/endpoints",
		host:     podHost,
		method:   http.MethodGet,
	}

	bytes, err := callNodeMgmtEndpoint(client, request, "")
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
func (client *NodeMgmtClient) CallCreateRoleEndpoint(pod *corev1.Pod, username string, password string, superuser bool) error {
	client.Log.Info(
		"calling Management API create role - POST /api/v0/ops/auth/role",
		"pod", pod.Name,
	)

	postData := url.Values{}
	postData.Set("username", username)
	postData.Set("password", password)
	postData.Set("can_login", "true")
	postData.Set("is_superuser", strconv.FormatBool(superuser))

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return err
	}

	request := nodeMgmtRequest{
		endpoint: fmt.Sprintf("/api/v0/ops/auth/role?%s", postData.Encode()),
		host:     podHost,
		method:   http.MethodPost,
	}
	_, err = callNodeMgmtEndpoint(client, request, "")
	return err
}

func (client *NodeMgmtClient) CallProbeClusterEndpoint(pod *corev1.Pod, consistencyLevel string, rfPerDc int) error {
	client.Log.Info(
		"calling Management API cluster health - GET /api/v0/probes/cluster",
		"pod", pod.Name,
	)

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return err
	}

	request := nodeMgmtRequest{
		endpoint: fmt.Sprintf("/api/v0/probes/cluster?consistency_level=%s&rf_per_dc=%d", consistencyLevel, rfPerDc),
		host:     podHost,
		method:   http.MethodGet,
	}

	_, err = callNodeMgmtEndpoint(client, request, "")
	return err
}

func (client *NodeMgmtClient) CallDrainEndpoint(pod *corev1.Pod) error {
	client.Log.Info(
		"calling Management API drain node - POST /api/v0/ops/node/drain",
		"pod", pod.Name,
	)

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return err
	}

	request := nodeMgmtRequest{
		endpoint: "/api/v0/ops/node/drain",
		host:     podHost,
		method:   http.MethodPost,
		timeout:  time.Minute * 2,
	}

	_, err = callNodeMgmtEndpoint(client, request, "")
	return err
}

func (client *NodeMgmtClient) CallKeyspaceCleanupEndpoint(pod *corev1.Pod, jobs int, keyspaceName string, tables []string) error {
	client.Log.Info(
		"calling Management API keyspace cleanup - POST /api/v0/ops/keyspace/cleanup",
		"pod", pod.Name,
	)
	postData := make(map[string]interface{})
	if jobs > -1 {
		postData["jobs"] = strconv.Itoa(jobs)
	}

	if keyspaceName != "" {
		postData["keyspace_name"] = keyspaceName
	}

	if len(tables) > 0 {
		postData["tables"] = tables
	}

	body, err := json.Marshal(postData)
	if err != nil {
		return err
	}

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return err
	}

	request := nodeMgmtRequest{
		endpoint: "/api/v0/ops/keyspace/cleanup",
		host:     podHost,
		method:   http.MethodPost,
		timeout:  time.Second * 20,
		body:     body,
	}

	_, err = callNodeMgmtEndpoint(client, request, "application/json")
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

	_, err := callNodeMgmtEndpoint(client, request, "")
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

	podHost, err := BuildPodHostFromPod(pod)
	if err != nil {
		return err
	}

	request := nodeMgmtRequest{
		endpoint: "/api/v0/ops/seeds/reload",
		host:     podHost,
		method:   http.MethodPost,
	}

	_, err = callNodeMgmtEndpoint(client, request, "")
	return err
}

func callNodeMgmtEndpoint(client *NodeMgmtClient, request nodeMgmtRequest, contentType string) ([]byte, error) {
	client.Log.Info("client::callNodeMgmtEndpoint")

	url := fmt.Sprintf("%s://%s:8080%s", client.Protocol, request.host, request.endpoint)

	var reqBody io.Reader
	if len(request.body) > 0 {
		reqBody = bytes.NewBuffer(request.body)
	}

	req, err := http.NewRequest(request.method, url, reqBody)
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

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
