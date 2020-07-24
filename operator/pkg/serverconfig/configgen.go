// Copyright DataStax, Inc.
// Please see the included license file for details.

package serverconfig

import (
	"strings"
)

// This needs to be outside of the apis package or else code-gen fails
type NodeConfig map[string]interface{}

// GetModelValues will gather the cluster model values for cluster and datacenter
func GetModelValues(
	seeds []string,
	clusterName string,
	dcName string,
	graphEnabled int,
	solrEnabled int,
	sparkEnabled int,
	cqlPort int,
	cqlSslPort int,
	broadcastPort int,
	broadcastSslPort int) NodeConfig {

	seedsString := strings.Join(seeds, ",")

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
		},
		"cassandra-yaml": NodeConfig{},
	}

	if cqlSslPort != 0 {
		modelValues["cassandra-yaml"].(NodeConfig)["native_transport_port_ssl"] = cqlSslPort
	} else if cqlPort != 0 {
		modelValues["cassandra-yaml"].(NodeConfig)["native_transport_port"] = cqlPort
	}

	if broadcastSslPort != 0 {
		modelValues["cassandra-yaml"].(NodeConfig)["ssl_storage_port"] = broadcastSslPort
	} else if broadcastPort != 0 {
		modelValues["cassandra-yaml"].(NodeConfig)["storage_port"] = broadcastPort
	}

	return modelValues
}
