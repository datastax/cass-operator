// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"encoding/json"
	"fmt"

	"github.com/datastax/cass-operator/operator/pkg/utils"
)

type SeedProvider struct {
	ClassName   string
	Seeds       []string
	OtherParams []map[string]interface{}
}

func buildClassNameForSeedProvider(serverType string) string {
	var className string
	if serverType == "cassandra" {
		className = "org.apache.cassandra.locator.K8SeedProvider"
	} else {
		className = "org.apache.cassandra.locator.SimpleSeedProvider"
	}
	return className
}

func (dc *CassandraDatacenter) buildDefaultSeedProvider() *SeedProvider {
	return &SeedProvider{
		ClassName:   buildClassNameForSeedProvider(dc.Spec.ServerType),
		Seeds:       dc.Spec.StaticSeedIPs,
		OtherParams: nil,
	}
}

func findSeedsAndOtherParams(parameters interface{}) ([]map[string]interface{}, []string) {
	var otherParams []map[string]interface{}
	var seeds []string

	switch parameters := parameters.(type) {
	case []map[string]interface{}:
		for _, m := range parameters {
			if v, ok := m["seeds"]; ok {
				switch v := v.(type) {
				case []string:
					seeds = v
				}
			} else {
				otherParams = append(otherParams, m)
			}
		}
	}

	return otherParams, seeds
}

func (dc *CassandraDatacenter) getSeedProviderData(sp map[string]interface{}) *SeedProvider {
	// no existing seed providers found in dc spec config
	if sp == nil || len(sp) < 1 {
		return nil
	}

	parameters := sp["parameters"]
	//no parameters found in seed provider
	if parameters == nil {
		return nil
	}

	otherParams, seeds := findSeedsAndOtherParams(parameters)

	var className string
	if c, ok := sp["class_name"]; ok {
		className = fmt.Sprintf("%v", c)
	} else {
		className = buildClassNameForSeedProvider(dc.Spec.ServerType)
	}

	return &SeedProvider{ClassName: className, Seeds: seeds, OtherParams: otherParams}
}

func (dc *CassandraDatacenter) EnsureStaticSeedProviders() error {
	//"seed_provider":[
	// { "class_name":"org.apache.cassandra.locator.K8SeedProvider",
	//   "parameters":[
	// 	{ "seeds": ["seed1", "seed2"] }
	//   ]
	// }
	//]

	if len(dc.Spec.StaticSeedIPs) == 0 {
		return nil
	}
	config, err := dc.GetConfigAsJSON()
	if err != nil {
		return err
	}

	var f interface{}
	err = json.Unmarshal([]byte(config), &f)
	if err != nil {
		return err
	}

	m := f.(map[string]interface{})
	seedProviders := utils.SearchMap(m, "seed_provider")

	sp := dc.getSeedProviderData(seedProviders)

	updateRequired := false
	if sp == nil {
		sp = dc.buildDefaultSeedProvider()
		updateRequired = true
	}

	for _, newSeed := range dc.Spec.StaticSeedIPs {
		seedExists := false
		for _, existingSeed := range sp.Seeds {
			if existingSeed == newSeed {
				seedExists = true
				break
			}
		}

		if !seedExists {
			sp.Seeds = append(sp.Seeds, newSeed)
			updateRequired = true
		}

	}

	if updateRequired {
		UpdateSeedProviders(seedProviders, *sp)
		var configBytes []byte
		configBytes, err := json.Marshal(m)
		if err != nil {
			return err
		}
		dc.Spec.Config = configBytes
	}

	return nil
}

func UpdateSeedProviders(rawSp map[string]interface{}, updatedSp SeedProvider) {
	params := updatedSp.OtherParams
	seeds := map[string]interface{}{"seeds": updatedSp.Seeds}
	params = append(params, seeds)
	rawSp["parameters"] = params
	rawSp["class_name"] = updatedSp.ClassName
}
