package serverconfig

import (
	"strings"
)

// This needs to be outside of the apis package or else code-gen fails
type NodeConfig map[string]interface{}

// GetModelValues will gather the cluster model values for cluster and datacenter
func GetModelValues(seeds []string, clusterName string, dcName string) NodeConfig {
	seedsString := strings.Join(seeds, ",")

	// Note: the operator does not currently support graph, solr, and spark
	modelValues := NodeConfig{
		"cluster-info": NodeConfig{
			"name":  clusterName,
			"seeds": seedsString,
		},
		"datacenter-info": NodeConfig{
			"name": dcName,
		}}

	return modelValues
}
