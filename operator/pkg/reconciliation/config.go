package reconciliation

import (
	"encoding/json"
	"github.com/pkg/errors"
	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
	"strings"
)

type DseConfigMap map[string]interface{}

func mergeToDepth(mapA DseConfigMap, mapB DseConfigMap, depth int32) DseConfigMap {
	if mapA == nil {
		return mapB
	} else if mapB == nil {
		return mapA
	}

	for k, vb := range mapB {
		vbAsMap, bOk := vb.(DseConfigMap)
		vaAsMap, aOk := mapA[k].(DseConfigMap)

		if depth == 0 || !bOk || !aOk {
			mapA[k] = vb
		} else {
			mapA[k] = mergeToDepth(vaAsMap, vbAsMap, depth-1)
		}
	}

	return mapA
}

func merge(mapA DseConfigMap, mapB DseConfigMap) DseConfigMap {
	return mergeToDepth(mapA, mapB, 0)
}

func GenerateBaseConfig(dseDatacenter *datastaxv1alpha1.DseDatacenter) (DseConfigMap, error) {
	config := DseConfigMap{}
	if configString := dseDatacenter.Spec.Config; configString != "" {
		err := json.Unmarshal([]byte(configString), &config)
		if err != nil {
			return nil, errors.Wrap(err, "Configuration on DseDatacenter resource was not properly formatted JSON.")
		}
	}

	seeds := dseDatacenter.GetSeedList()
	seedsString := strings.Join(seeds, ",")

	// Note: we are not currently supporting graph, solr, and spark
	overrides := DseConfigMap{
		"cluster-info": DseConfigMap{
			"name":  dseDatacenter.Spec.ClusterName,
			"seeds": seedsString,
		},
		"datacenter-info": DseConfigMap{
			"name": dseDatacenter.Name,
		}}

	mergeToDepth(config, overrides, 1)

	return config, nil
}

func GenerateBaseConfigString(dseDatacenter *datastaxv1alpha1.DseDatacenter) (string, error) {
	config, err := GenerateBaseConfig(dseDatacenter)

	if err != nil {
		return "", err
	}

	var configBytes []byte
	configBytes, err = json.Marshal(config)

	if err != nil {
		return "", err
	}

	return string(configBytes), nil
}
