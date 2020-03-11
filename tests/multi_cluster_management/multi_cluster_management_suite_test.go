package multi_cluster_management

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ginkgo_util "github.com/riptano/dse-operator/mage/ginkgo"
	"github.com/riptano/dse-operator/mage/kubectl"
)

var (
	testName         = "Multi-cluster Management"
	namespace        = "multi-cluster-management"
	dcNames          = [2]string{"dc1", "dc2"}
	dcYamls          = [2]string{"../testdata/default-three-rack-three-node-dc.yaml", "../testdata/default-single-rack-single-node-dc.yaml"}
	operatorYaml     = "../testdata/operator.yaml"
	ns               = ginkgo_util.NewWrapper(testName, namespace)
	defaultResources = []string{
		"../../operator/deploy/role.yaml",
		"../../operator/deploy/role_binding.yaml",
		"../../operator/deploy/service_account.yaml",
		"../../operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml",
	}
)

func dcResourceForName(dcName string) string {
	return fmt.Sprintf("CassandraDatacenter/%s", dcName)
}

func dcLabelForName(dcName string) string {
	return fmt.Sprintf("cassandra.datastax.com/datacenter=%s", dcName)
}

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
		Specify("the operator manages multiple clusters in the same namespace", func() {
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
			k = kubectl.ApplyFiles(dcYamls[0])
			ns.ExecAndLog(step, k)

			step = "creating another datacenter resource with 1 rack/1 node"
			k = kubectl.ApplyFiles(dcYamls[1])
			ns.ExecAndLog(step, k)

			step = "waiting for the dse node to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[0])).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true true true", 1200)

			step = "waiting for the nodes to become ready"
			json = "jsonpath={.items[*].status.containerStatuses[0].ready}"
			k = kubectl.Get("Pods").
				WithLabel(dcLabelForName(dcNames[1])).
				WithFlag("field-selector", "status.phase=Running").
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "true", 1200)

			step = "checking the dc label cassandra.datastax.com/operator-progress is set to Ready"
			json = "jsonpath={.metadata.labels['cassandra\\.datastax\\.com/operator-progress']}"
			k = kubectl.Get(dcResourceForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "Ready", 30)

			step = "deleting the first dc"
			k = kubectl.DeleteFromFiles(dcYamls[0])
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabelForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no dc pods remain"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[0])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "deleting the second dc"
			k = kubectl.DeleteFromFiles(dcYamls[1])
			ns.ExecAndLog(step, k)

			step = "checking that the dc no longer exists"
			json = "jsonpath={.items}"
			k = kubectl.Get("CassandraDatacenter").
				WithLabel(dcLabelForName(dcNames[1])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)

			step = "checking that no dc pods remain"
			json = "jsonpath={.items}"
			k = kubectl.Get("pods").
				WithLabel(dcLabelForName(dcNames[1])).
				FormatOutput(json)
			ns.WaitForOutputAndLog(step, k, "[]", 300)
		})
	})
})
