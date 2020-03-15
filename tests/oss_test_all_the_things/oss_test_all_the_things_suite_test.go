package oss_test_all_the_things

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/riptano/dse-operator/mage/ginkgo"
	"github.com/riptano/dse-operator/mage/kubectl"
)

var (
	testName         = "OSS test all the things"
	namespace        = "oss-test-all-the-things"
	dcName           = "dc1"
	dcYaml           = "../testdata/oss-three-rack-three-node-dc.yaml"
	operatorYaml     = "../testdata/operator.yaml"
	dcResource       = fmt.Sprintf("CassandraDatacenter/%s", dcName)
	dcLabel          = fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
	ns               = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources = []string{
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

var _ = Describe(testName, func() {
	Context("when in a new cluster", func() {
		Specify("the operator can scale up, park, unpark, and terminate a dse datacenter", func() {
			By("creating a namespace")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			step := "creating default resources"
			k := kubectl.ApplyFiles(defaultResources...)
			ns.ExecAndLog(step, k)

			step = "creating the dse operator resource"
			k = kubectl.ApplyFiles(operatorYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the operator to become ready"
			json := "jsonpath={.items[0].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel("name=dse-operator").
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 120)

			step = "creating a datacenter resource with 3 racks/3 nodes"
			k = kubectl.ApplyFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "waiting for the dse nodes to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true", 1200)

			step = "checking the cassandra operator progress status is set to Ready"
			json = "jsonpath={.status.cassandraOperatorProgress}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "scale up to 4 nodes"
			json = "{\"spec\": {\"size\": 4}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "waiting for the 4th dse node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true true", 1200)

			step = "scale up to 5 nodes"
			json = "{\"spec\": {\"size\": 5}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "waiting for the 5th dse node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true true true", 1200)

			step = "parking the dc"
			json = "{\"spec\": {\"parked\": true}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "checking the spec size hasn't changed"
			json = "jsonpath={.spec.size}"
			k = kubectl.Get(dcResource).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "5", 20)

			step = "checking that no dc pods remain"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "unparking the dc"
			json = "{\"spec\": {\"parked\": false}}"
			k = kubectl.PatchMerge(dcResource, json)
			ns.ExecAndLog(step, k)

			step = "waiting for the dse nodes to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabel).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true true true", 1200)

			step = "deleting the dc"
			k = kubectl.DeleteFromFiles(dcYaml)
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabel).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})
