// Copyright DataStax, Inc.
// Please see the included license file for details.

package httphelper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

func Test_BuildPodHostFromPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-foo",
			Namespace: "somenamespace",
			Labels: map[string]string{
				api.DatacenterLabel: "dc-bar",
				api.ClusterLabel:    "the-foobar-cluster",
			},
		},
		Status: corev1.PodStatus{
			PodIP: "1.2.3.4",
		},
	}

	result, err := BuildPodHostFromPod(pod)
	assert.NoError(t, err)

	expected := "1.2.3.4"

	assert.Equal(t, expected, result)
}

func Test_parseMetadataEndpointsResponseBody(t *testing.T) {
	endpoints, err := parseMetadataEndpointsResponseBody([]byte(`{
		"entity": [
		  {
			"DC": "dtcntr",
			"ENDPOINT_IP": "10.233.90.45",
			"HOST_ID": "95c157dc-2811-446a-a541-9faaab2e6930",
			"INTERNAL_IP": "10.233.90.45",
			"IS_ALIVE": "true",
			"LOAD": "72008.0",
			"NET_VERSION": "11",
			"RACK": "r0",
			"RELEASE_VERSION": "3.11.6",
			"RPC_ADDRESS": "10.233.90.45",
			"RPC_READY": "true",
			"SCHEMA": "e84b6a60-24cf-30ca-9b58-452d92911703",
			"STATUS": "NORMAL,2756844028858338669",
			"TOKENS": "\u0000\u0000\u0000\b&BG\t±B\rm\u0000\u0000\u0000\u0000"
		  },
		  {
			"DC": "dtcntr",
			"ENDPOINT_IP": "10.233.92.102",
			"HOST_ID": "828e6980-9cac-48f2-a2c9-0650edc4d114",
			"INTERNAL_IP": "10.233.92.102",
			"IS_ALIVE": "true",
			"LOAD": "71880.0",
			"NET_VERSION": "11",
			"RACK": "r0",
			"RELEASE_VERSION": "3.11.6",
			"RPC_ADDRESS": "10.233.92.102",
			"RPC_READY": "true",
			"SCHEMA": "e84b6a60-24cf-30ca-9b58-452d92911703",
			"STATUS": "NORMAL,-1589726493696519215",
			"TOKENS": "\u0000\u0000\u0000\béð(-=1\u0013Ñ\u0000\u0000\u0000\u0000"
		  }
		],
		"variant": {
		  "language": null,
		  "mediaType": {
			"type": "application",
			"subtype": "json",
			"parameters": {},
			"wildcardType": false,
			"wildcardSubtype": false
		  },
		  "encoding": null,
		  "languageString": null
		},
		"annotations": [],
		"mediaType": {
		  "type": "application",
		  "subtype": "json",
		  "parameters": {},
		  "wildcardType": false,
		  "wildcardSubtype": false
		},
		"language": null,
		"encoding": null
	  }`))

	assert.Nil(t, err)
	assert.Equal(t, 2, len(endpoints.Entity))
	assert.Equal(t, "10.233.90.45", endpoints.Entity[0].RpcAddress)
	assert.Equal(t, "95c157dc-2811-446a-a541-9faaab2e6930", endpoints.Entity[0].HostID)
}
