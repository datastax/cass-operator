package dseconfig

import (
	"strings"
)

// This needs to be outside of the apis package or else code-gen fails
type DseConfigMap map[string]interface{}

// GetModelValues will gather the cluster model values for cluster and datacenter
func GetModelValues(seeds []string, clusterName string, dcName string) DseConfigMap {
	seedsString := strings.Join(seeds, ",")

	// Note: the operator does not currently support graph, solr, and spark
	modelValues := DseConfigMap{
		"cluster-info": DseConfigMap{
			"name":  clusterName,
			"seeds": seedsString,
		},
		"datacenter-info": DseConfigMap{
			"name": dcName,
		}}

	return modelValues
}
