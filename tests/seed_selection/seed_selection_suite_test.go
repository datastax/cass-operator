// Copyright DataStax, Inc.
// Please see the included license file for details.

package seed_selection

import (
	"fmt"
	"testing"
	"strings"
	"sort"
	"regexp"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	ginkgo_util "github.com/datastax/cass-operator/mage/ginkgo"
	"github.com/datastax/cass-operator/mage/kubectl"
)

var (
	testName            = "Seed Selection"
	namespace           = "test-seed-selection"
	dcName              = "dc1"
	dcYaml              = "../testdata/default-three-rack-three-node-dc.yaml"
	operatorYaml        = "../testdata/operator.yaml"
	dcResource          = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel             = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns                  = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources    = []string{
		"../../operator/deploy/role.yaml",
		"../../operator/deploy/role_binding.yaml",
		"../../operator/deploy/service_account.yaml",
		"../../operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml",
	}
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		kubectl.DumpAllLogs(logPath).ExecV()

		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		_ = ns.Terminate()
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

func retrievePodNames(ns ginkgo_util.NsWrapper, dcName string) []string {
	json := "jsonpath={.items[*].metadata.name}"
	k := kubectl.Get("pods").
		WithFlag("selector", fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)).
		FormatOutput(json)

	output := ns.OutputPanic(k)
	podNames := strings.Split(output, " ")
	sort.Sort(sort.StringSlice(podNames))

	return podNames
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
		Size: int(spec["size"].(float64)),
		Nodes: retrieveNodes(),
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
	sort.Sort(sort.StringSlice(expectedSeeds))

	re := regexp.MustCompile(`[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+`)
	for _, node := range info.Nodes {
		if node.Ready && node.Started {
			k := kubectl.ExecOnPod(node.Name, "--", "nodetool", "getseeds")
			output := ns.OutputPanic(k)
			seeds := re.FindAllString(output, -1)
			if node.Seed {
				seeds = append(seeds, node.IP)
			}
			sort.Sort(sort.StringSlice(seeds))

			Expect(seeds).To(Equal(expectedSeeds), "Expected pod %s to have seeds %v but had %v", node.Name, expectedSeeds, seeds)
		}
	}
}

func checkSeedConstraints() {
	info := retrieveDatacenterInfo()
	// There should be 3 seed nodes for every datacenter
	checkThereAreAtLeastThreeSeedsPerDc(info)

	// There should be 1 seed node per rack
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

func duplicate(value string, count int) string {
	result := []string{}
	for i := 0; i < count; i++ {
		result = append(result, value)
	}

	return strings.Join(result, " ")
}

func waitDatacenterReady() {
	info := retrieveDatacenterInfo()

	step := "waiting for the node to become ready"
	json := "jsonpath={.items[*].status.containerStatuses[0].ready}"
	k := kubectl.Get("pods").
		WithLabel(dcLabel).
		WithFlag("field-selector", "status.phase=Running").
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, duplicate("true", info.Size), 1200)

	step = "checking the cassandra operator progress status is set to Ready"
	json = "jsonpath={.status.cassandraOperatorProgress}"
	k = kubectl.Get(dcResource).
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, "Ready", 30)
}

func waitPodNotStarted(podName string) {
	step := "verify that the pod is no longer marked as started"
	k := kubectl.Get("pod").
		WithFlag("field-selector", "metadata.name="+podName).
		WithFlag("selector", "cassandra.datastax.com/node-state=Started")
	ns.WaitForOutputAndLog(step, k, "", 60)
}

func waitPodStarted(podName string) {
	step := "verify that the pod is marked as started"
	json := "jsonpath={.items[*].metadata.name}"
	k := kubectl.Get("pod").
		WithFlag("field-selector", "metadata.name="+podName).
		WithFlag("selector", "cassandra.datastax.com/node-state=Started").
		FormatOutput(json)
	ns.WaitForOutputAndLog(step, k, podName, 60)
}

func makePodNotReady(podName string) {
	disableGossip(podName)
	waitPodNotStarted(podName)
}

func makePodReady(podName string) {
	enableGossip(podName)
	waitPodStarted(podName)
}

func disableGossip(podName string) {
	execArgs := []string{"-c", "cassandra",
		"--", "bash", "-c",
		"nodetool disablegossip",
	}
	k := kubectl.ExecOnPod(podName, execArgs...)
	ns.ExecVPanic(k)
}

func enableGossip(podName string) {
	execArgs := []string{"-c", "cassandra",
		"--", "bash", "-c",
		"nodetool enablegossip",
	}
	k := kubectl.ExecOnPod(podName, execArgs...)
	ns.ExecVPanic(k)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator properly updates seed nodes", func() {
			var step string
			var json string
			var k kubectl.KCmd

			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step = "creating default resources"
			k = kubectl.ApplyFiles(defaultResources...)
			ns.ExecAndLog(step, k)

			step = "creating the cass-operator resource"
			k = kubectl.ApplyFiles(operatorYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to become ready"
			json = "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel("name=cass-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			waitDatacenterReady()

			checkSeedConstraints()

			step = "scale up to 4 nodes"
			json = "{\"spec\": {\"size\": 4}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			waitDatacenterReady()

			checkSeedConstraints()

			rack1Seed := retrieveNameSeedNodeForRack("r1")
			makePodNotReady(rack1Seed)

			checkSeedConstraints()

			makePodReady(rack1Seed)

			checkSeedConstraints()
		})
	})
})
