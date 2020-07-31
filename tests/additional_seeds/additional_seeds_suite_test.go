// Copyright DataStax, Inc.
// Please see the included license file for details.

package additional_seeds

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName                       = "Seed Selection"
	namespace                      = "test-additional-seeds"
	dcName                         = "dc1"
	dcYaml                         = "../testdata/additional-seeds-two-rack-four-node-dc.yaml"
	operatorYaml                   = "../testdata/operator.yaml"
	dcResource                     = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel                        = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	additionalSeedServiceResource  = "services/cluster1-dc1-additional-seed-service"
	additionalSeedEndpointResource = "endpoints/cluster1-dc1-additional-seed-service"
	ns                             = ginkgo_util.NewWrapper(testName, namespace)
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		kubectl.DumpAllLogs(logPath).ExecV()

		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

type Node struct {
	Name    string
	Rack    string
	Ready   bool
	Seed    bool
	Started bool
	IP      string
	Ordinal int
}

type DatacenterInfo struct {
	Size      int
	RackNames []string
	Nodes     []Node
}

func retrieveNodes() []Node {
	k := kubectl.Get("pods").
		WithLabel(dcLabel).
		FormatOutput("json")
	output := ns.OutputPanic(k)
	data := corev1.PodList{}
	err := json.Unmarshal([]byte(output), &data)
	Expect(err).ToNot(HaveOccurred())
	result := []Node{}
	for idx, _ := range data.Items {
		pod := &data.Items[idx]
		node := Node{}
		node.Name = pod.Name
		node.IP = pod.Status.PodIP
		node.Rack = pod.Labels["cassandra.datastax.com/rack"]
		isSeed, hasSeedLabel := pod.Labels["cassandra.datastax.com/seed-node"]
		node.Seed = hasSeedLabel && isSeed == "true"
		isStarted, hasStartedLabel := pod.Labels["cassandra.datastax.com/node-state"]
		node.Started = hasStartedLabel && isStarted == "Started"
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" {
				node.Ready = condition.Status == "True"
			}
		}
		result = append(result, node)
	}
	return result
}

func retrieveDatacenterInfo() DatacenterInfo {
	k := kubectl.Get(dcResource).
		FormatOutput("json")
	output := ns.OutputPanic(k)
	data := map[string]interface{}{}
	err := json.Unmarshal([]byte(output), &data)
	Expect(err).ToNot(HaveOccurred())

	err = json.Unmarshal([]byte(output), &data)

	spec := data["spec"].(map[string]interface{})
	rackNames := []string{}
	for _, rackData := range spec["racks"].([]interface{}) {
		name := rackData.(map[string]interface{})["name"]
		if name != nil {
			rackNames = append(rackNames, name.(string))
		}
	}

	dc := DatacenterInfo{
		Size:      int(spec["size"].(float64)),
		Nodes:     retrieveNodes(),
		RackNames: rackNames,
	}

	return dc
}

func retrieveNameSeedNodeForRack(rack string) string {
	info := retrieveDatacenterInfo()
	name := ""
	for _, node := range info.Nodes {
		if node.Rack == rack && node.Seed {
			name = node.Name
			break
		}
	}

	Expect(name).ToNot(Equal(""))
	return name
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func checkThereAreAtLeastThreeSeedsPerDc(info DatacenterInfo) {
	seedCount := 0

	for _, node := range info.Nodes {
		if node.Seed {
			seedCount += 1
		}
	}

	expectedSeedCount := MinInt(info.Size, 3)
	Expect(seedCount >= expectedSeedCount).To(BeTrue(),
		"Expected there to be at least %d seed nodes, but only found %d.",
		expectedSeedCount, seedCount)
}

func checkThereIsAtLeastOneSeedNodePerRack(info DatacenterInfo) {
	rackToFoundSeed := map[string]bool{}
	for _, node := range info.Nodes {
		if node.Seed {
			rackToFoundSeed[node.Rack] = true
		}
	}

	for _, rackName := range info.RackNames {
		value, ok := rackToFoundSeed[rackName]
		Expect(ok && value).To(BeTrue(), "Expected rack %s to have a seed node, but none found.", rackName)
	}
}

func checkDesignatedSeedNodesAreStartedAndReady(info DatacenterInfo) {
	for _, node := range info.Nodes {
		if node.Seed {
			Expect(node.Started).To(BeTrue(), "Expected %s to be labeled as started but was not.", node.Name)
			Expect(node.Ready).To(BeTrue(), "Expected %s to be ready but was not.", node.Name)
		}
	}
}

func checkCassandraSeedListsAlignWithSeedLabels(info DatacenterInfo) {
	expectedSeeds := []string{}
	for _, node := range info.Nodes {
		if node.Seed {
			expectedSeeds = append(expectedSeeds, node.IP)
		}
	}
	sort.Strings(expectedSeeds)

	re := regexp.MustCompile(`[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+`)
	for _, node := range info.Nodes {
		if node.Ready && node.Started {
			k := kubectl.ExecOnPod(node.Name, "--", "nodetool", "getseeds")
			output := ns.OutputPanic(k)
			seeds := re.FindAllString(output, -1)
			if node.Seed {
				seeds = append(seeds, node.IP)
			}
			sort.Strings(seeds)

			Expect(seeds).To(Equal(expectedSeeds), "Expected pod %s to have seeds %v but had %v", node.Name, expectedSeeds, seeds)
		}
	}
}

func checkSeedConstraints() {
	info := retrieveDatacenterInfo()
	// There should be 3 seed nodes for every datacenter
	checkThereAreAtLeastThreeSeedsPerDc(info)

	// There should be 2 seed nodes per rack
	// this is because of the additional seeds
	checkThereIsAtLeastOneSeedNodePerRack(info)

	// Seed nodes should not be down
	checkDesignatedSeedNodesAreStartedAndReady(info)

	// Ensure seed lists actually align
	//
	// NOTE: The following check does not presently work due to
	// the lag time between when we update a seed label and when
	// that change is reflected in DNS. Since we reload seed lists
	// right after upating the label, some cassandra nodes will
	// likely end up with slight out-of-date seed lists. KO-375
	//
	// checkCassandraSeedListsAlignWithSeedLabels(info)
}

func checkAdditionalSeedService() {
	// Check the service

	k := kubectl.Get(additionalSeedServiceResource).FormatOutput("json")
	output := ns.OutputPanic(k)
	data := map[string]interface{}{}
	err := json.Unmarshal([]byte(output), &data)
	Expect(err).ToNot(HaveOccurred())

	err = json.Unmarshal([]byte(output), &data)

	spec := data["spec"].(map[string]interface{})
	actualType := spec["type"].(string)
	Expect(actualType).To(Equal("ClusterIP"), "Expected additional seed service type %s to be ClusterIP", actualType)

	// Check the endpoint

	jsonpath := "jsonpath=\"{.subsets[0].addresses[0].ip}\""
	k = kubectl.Get(additionalSeedEndpointResource).FormatOutput(jsonpath)
	output = ns.OutputPanic(k)
	actualIp := ""
	err = json.Unmarshal([]byte(output), &actualIp)
	Expect(err).ToNot(HaveOccurred())

	err = json.Unmarshal([]byte(output), &actualIp)
	Expect(actualIp).To(Equal("192.168.1.1"), "Expected additional seed endpoints IP %s to be 192.168.1.1", actualIp)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can properly create an additional seed service", func() {
			var step string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "setting up cass-operator resources via helm chart"
			ns.HelmInstall("../../charts/cass-operator-chart")

			ns.WaitForOperatorReady()

			step = "creating a datacenter resource with 2 racks/4 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReady(dcName)

			checkSeedConstraints()

			step = "add additionalSeeds"
			json := `
			{
				"spec": {
					"additionalSeeds": ["192.168.1.1"]
				}
			}`
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterOperatorProgress(dcName, "Updating", 30)
			ns.WaitForDatacenterOperatorProgress(dcName, "Ready", 1800)

			checkSeedConstraints()

			checkAdditionalSeedService()

			// TODO provision a DC in another namespace and connect it to the first one
			// with additional seeds, and test that's working
		})
	})
})
