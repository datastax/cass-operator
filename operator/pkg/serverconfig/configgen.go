// Copyright DataStax, Inc.
// Please see the included license file for details.

package serverconfig

import (
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"strings"
)

// This needs to be outside of the apis package or else code-gen fails
type NodeConfig map[string]interface{}

// GetModelValues will gather the cluster model values for cluster and datacenter
func GetModelValues(seeds []string, clusterName string, dcName string, serverType string, workloads api.DseWorkloads) NodeConfig {
	seedsString := strings.Join(seeds, ",")

	graphEnabled, solrEnabled, sparkEnabled := 0

	if serverType == "dse" && workloads != nil {
		if workloads.AnalyticsEnabled == true {
			sparkEnabled = 1
		}
		if workloads.GraphEnabled == true {
			graphEnabled = 1
		}
		if workloads.SearchEnabled == true {
			solrEnabled = 1
		}
	}

	// Note: the operator does not currently support graph, solr, and spark
	modelValues := NodeConfig{
		"cluster-info": NodeConfig{
			"name":  clusterName,
			"seeds": seedsString,
		},
		"datacenter-info": NodeConfig{
			"name":          dcName,
			"graph-enabled": graphEnabled,
			"solr-enabled":  solrEnabled,
			"spark-enabled": sparkEnabled,
		}}

	return modelValues
}
