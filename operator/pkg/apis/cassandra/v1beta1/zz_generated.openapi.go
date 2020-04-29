// +build !ignore_autogenerated

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1beta1

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenter":       schema_pkg_apis_cassandra_v1beta1_CassandraDatacenter(ref),
		"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterSpec":   schema_pkg_apis_cassandra_v1beta1_CassandraDatacenterSpec(ref),
		"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterStatus": schema_pkg_apis_cassandra_v1beta1_CassandraDatacenterStatus(ref),
	}
}

func schema_pkg_apis_cassandra_v1beta1_CassandraDatacenter(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CassandraDatacenter is the Schema for the cassandradatacenters API",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterStatus"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterSpec", "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraDatacenterStatus", "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"},
	}
}

func schema_pkg_apis_cassandra_v1beta1_CassandraDatacenterSpec(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CassandraDatacenterSpec defines the desired state of a CassandraDatacenter",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"size": {
						SchemaProps: spec.SchemaProps{
							Description: "Desired number of Cassandra server nodes",
							Type:        []string{"integer"},
							Format:      "int32",
						},
					},
					"serverVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "Version string for config builder, used to generate Cassandra server configuration",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"serverImage": {
						SchemaProps: spec.SchemaProps{
							Description: "Cassandra server image name. More info: https://kubernetes.io/docs/concepts/containers/images",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"serverType": {
						SchemaProps: spec.SchemaProps{
							Description: "Server type: \"cassandra\" or \"dse\"",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"config": {
						SchemaProps: spec.SchemaProps{
							Description: "Config for the server, in YAML format",
							Type:        []string{"string"},
							Format:      "byte",
						},
					},
					"managementApiAuth": {
						SchemaProps: spec.SchemaProps{
							Description: "Config for the Management API certificates",
							Ref:         ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.ManagementApiAuthConfig"),
						},
					},
					"resources": {
						SchemaProps: spec.SchemaProps{
							Description: "Kubernetes resource requests and limits, per pod",
							Ref:         ref("k8s.io/api/core/v1.ResourceRequirements"),
						},
					},
					"racks": {
						SchemaProps: spec.SchemaProps{
							Description: "A list of the named racks in the datacenter, representing independent failure domains. The number of racks should match the replication factor in the keyspaces you plan to create, and the number of racks cannot easily be changed once a datacenter is deployed.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.Rack"),
									},
								},
							},
						},
					},
					"storageConfig": {
						SchemaProps: spec.SchemaProps{
							Description: "Describes the persistent storage request of each server node",
							Ref:         ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.StorageConfig"),
						},
					},
					"replaceNodes": {
						SchemaProps: spec.SchemaProps{
							Description: "A list of pod names that need to be replaced.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
					"clusterName": {
						SchemaProps: spec.SchemaProps{
							Description: "The name by which CQL clients and instances will know the cluster. If the same cluster name is shared by multiple Datacenters in the same Kubernetes namespace, they will join together in a multi-datacenter cluster.",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"stopped": {
						SchemaProps: spec.SchemaProps{
							Description: "A stopped CassandraDatacenter will have no running server pods, like using \"stop\" with traditional System V init scripts. Other Kubernetes resources will be left intact, and volumes will re-attach when the CassandraDatacenter workload is resumed.",
							Type:        []string{"boolean"},
							Format:      "",
						},
					},
					"configBuilderImage": {
						SchemaProps: spec.SchemaProps{
							Description: "Container image for the config builder init container.",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"canaryUpgrade": {
						SchemaProps: spec.SchemaProps{
							Description: "Indicates that configuration and container image changes should only be pushed to the first rack of the datacenter",
							Type:        []string{"boolean"},
							Format:      "",
						},
					},
					"allowMultipleNodesPerWorker": {
						SchemaProps: spec.SchemaProps{
							Description: "Turning this option on allows multiple server pods to be created on a k8s worker node. By default the operator creates just one server pod per k8s worker node using k8s podAntiAffinity and requiredDuringSchedulingIgnoredDuringExecution.",
							Type:        []string{"boolean"},
							Format:      "",
						},
					},
					"superuserSecretName": {
						SchemaProps: spec.SchemaProps{
							Description: "This secret defines the username and password for the Cassandra server superuser. If it is omitted, we will generate a secret instead.",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"serviceAccount": {
						SchemaProps: spec.SchemaProps{
							Description: "The k8s service account to use for the server pods",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"rollingRestartRequested": {
						SchemaProps: spec.SchemaProps{
							Description: "Whether to do a rolling restart at the next opportunity. The operator will set this back to false once the restart is in progress.",
							Type:        []string{"boolean"},
							Format:      "",
						},
					},
					"nodeSelector": {
						SchemaProps: spec.SchemaProps{
							Description: "A map of label keys and values to restrict Cassandra node scheduling to k8s workers with matchiing labels. More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector",
							Type:        []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Allows: true,
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
					"podTemplate": {
						SchemaProps: spec.SchemaProps{
							Description: "PodTemplate provides customisation options (labels, annotations, affinity rules, resource requests, and so on) for the cassandra pods",
							Ref:         ref("k8s.io/api/core/v1.PodTemplateSpec"),
						},
					},
				},
				Required: []string{"size", "serverVersion", "serverType", "storageConfig", "clusterName"},
			},
		},
		Dependencies: []string{
			"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.ManagementApiAuthConfig", "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.Rack", "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.StorageConfig", "k8s.io/api/core/v1.PodTemplateSpec", "k8s.io/api/core/v1.ResourceRequirements"},
	}
}

func schema_pkg_apis_cassandra_v1beta1_CassandraDatacenterStatus(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CassandraDatacenterStatus defines the observed state of CassandraDatacenter",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"superUserUpserted": {
						SchemaProps: spec.SchemaProps{
							Description: "The timestamp at which CQL superuser credentials were last upserted to the management API",
							Ref:         ref("k8s.io/apimachinery/pkg/apis/meta/v1.Time"),
						},
					},
					"lastServerNodeStarted": {
						SchemaProps: spec.SchemaProps{
							Description: "The timestamp when the operator last started a Server node with the management API",
							Ref:         ref("k8s.io/apimachinery/pkg/apis/meta/v1.Time"),
						},
					},
					"cassandraOperatorProgress": {
						SchemaProps: spec.SchemaProps{
							Description: "Last known progress state of the Cassandra Operator",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"lastRollingRestart": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.Time"),
						},
					},
					"nodeStatuses": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Allows: true,
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraNodeStatus"),
									},
								},
							},
						},
					},
					"nodeReplacements": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type:   []string{"string"},
										Format: "",
									},
								},
							},
						},
					},
				},
			},
		},
		Dependencies: []string{
			"github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1.CassandraNodeStatus", "k8s.io/apimachinery/pkg/apis/meta/v1.Time"},
	}
}
